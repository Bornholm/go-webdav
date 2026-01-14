package s3

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

const (
	separator = "/"
)

// FileSystem implements the webdav.FileSystem interface for S3 storage
type FileSystem struct {
	client *minio.Client
	bucket string
}

// Mkdir implements webdav.FileSystem.
func (f *FileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	name = clean(name)

	_, err := f.Stat(ctx, name)
	if err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.WithStack(err)
	}

	if err := f.checkParent(ctx, name); err != nil {
		return err
	}

	prefix := strings.Trim(name, separator)
	if !strings.HasSuffix(prefix, separator) {
		prefix += separator
	}

	key := filepath.Clean(prefix) + separator

	if _, err := f.client.PutObject(ctx, f.bucket, key, bytes.NewReader(nil), 0, minio.PutObjectOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// OpenFile implements webdav.FileSystem.
func (f *FileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	key := clean(name)

	isCreate := flag&os.O_CREATE != 0

	if isCreate {
		if err := f.checkParent(ctx, name); err != nil {
			return nil, err
		}
	}

	if flag&os.O_RDWR != 0 || flag&os.O_WRONLY != 0 || flag&os.O_CREATE != 0 || flag&os.O_TRUNC != 0 {
		pr, pw := io.Pipe()
		file := &File{
			ctx:        ctx,
			fs:         f,
			name:       name,
			key:        key,
			isWriter:   true,
			pipeWriter: pw,
			done:       make(chan error, 1),
		}

		go func() {
			defer close(file.done)
			_, err := f.client.PutObject(ctx, f.bucket, key, pr, -1, minio.PutObjectOptions{
				ContentType: "application/octet-stream",
				PartSize:    5 * 1024 * 1024,
			})
			_ = pr.CloseWithError(err)
			file.done <- err
		}()

		return file, nil
	}

	obj, err := f.client.GetObject(ctx, f.bucket, key, minio.GetObjectOptions{})
	if err == nil {
		info, err := obj.Stat()
		if err == nil {
			isDirMarker := strings.HasSuffix(key, "/") || info.ContentType == "application/x-directory"

			if isDirMarker {
				return &File{
					ctx:   ctx,
					fs:    f,
					name:  name,
					key:   key,
					isDir: true,
				}, nil
			}

			return &File{
				ctx:      ctx,
				fs:       f,
				name:     name,
				key:      key,
				isWriter: false,
				obj:      obj,
			}, nil
		}

		errResp := minio.ToErrorResponse(err)
		if errResp.Code != "NoSuchKey" {
			return nil, os.ErrNotExist
		}
	}

	dirKey := key
	if !strings.HasSuffix(dirKey, "/") {
		dirKey += "/"
	}

	opts := minio.ListObjectsOptions{
		Prefix:    dirKey,
		Recursive: true,
		MaxKeys:   1,
	}

	count := 0
	for range f.client.ListObjects(ctx, f.bucket, opts) {
		count++
		break
	}

	if count > 0 {
		return &File{
			ctx:   ctx,
			fs:    f,
			name:  name,
			key:   key, // Keep original key name (e.g. without trailing slash if requested so)
			isDir: true,
		}, nil
	}

	return nil, os.ErrNotExist
}

// RemoveAll implements webdav.FileSystem.
func (f *FileSystem) RemoveAll(ctx context.Context, name string) error {
	name = clean(name)

	stat, err := f.Stat(ctx, name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return errors.WithStack(err)
	}

	if !stat.IsDir() {
		return f.client.RemoveObject(ctx, f.bucket, name, minio.RemoveObjectOptions{
			ForceDelete: true,
		})
	}

	objectsCh := make(chan minio.ObjectInfo)

	go func() {
		defer close(objectsCh)

		prefix := name
		if !strings.HasSuffix(prefix, separator) {
			prefix += separator
		}

		opts := minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		}

		for object := range f.client.ListObjects(ctx, f.bucket, opts) {
			if object.Err != nil {
				slog.ErrorContext(ctx, "could not list objects", slog.Any("error", errors.WithStack(err)))
				continue
			}
			objectsCh <- object
		}
	}()

	errorCh := f.client.RemoveObjects(ctx, f.bucket, objectsCh, minio.RemoveObjectsOptions{
		GovernanceBypass: true,
	})

	for err := range errorCh {
		if err.Err != nil {
			return errors.WithStack(err.Err)
		}
	}

	_ = f.client.RemoveObject(ctx, f.bucket, name+separator, minio.RemoveObjectOptions{
		ForceDelete: true,
	})

	return nil
}

// Rename implements webdav.FileSystem.
func (f *FileSystem) Rename(ctx context.Context, oldName string, newName string) error {
	oldName = clean(oldName)
	newName = clean(newName)

	stat, err := f.Stat(ctx, oldName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.ErrNotExist
		}

		return errors.WithStack(err)
	}

	if err := f.checkParent(ctx, newName); err != nil {
		return err
	}

	if stat.IsDir() {
		if err := f.Mkdir(ctx, newName, os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
			return errors.WithStack(err)
		}

		fileInfos, err := readdir(ctx, f.client, f.bucket, oldName, -1)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, fi := range fileInfos {
			oldPath := filepath.Join(oldName, fi.Name())
			newPath := filepath.Join(newName, fi.Name())

			if fi.IsDir() {
				if err := f.Rename(ctx, oldPath, newPath); err != nil {
					return errors.WithStack(err)
				}
			} else {
				dest := minio.CopyDestOptions{
					Bucket: f.bucket,
					Object: newPath,
				}
				src := minio.CopySrcOptions{
					Bucket: f.bucket,
					Object: oldPath,
				}
				if _, err := f.client.CopyObject(ctx, dest, src); err != nil {
					return errors.WithStack(err)
				}
				if err := f.client.RemoveObject(ctx, f.bucket, oldPath, minio.RemoveObjectOptions{
					ForceDelete: true,
				}); err != nil {
					return errors.WithStack(err)
				}
			}
		}

		if err := f.client.RemoveObject(ctx, f.bucket, oldName+separator, minio.RemoveObjectOptions{
			ForceDelete: true,
		}); err != nil {
			return errors.WithStack(err)
		}

		return nil
	} else {
		dest := minio.CopyDestOptions{
			Bucket: f.bucket,
			Object: newName,
		}
		src := minio.CopySrcOptions{
			Bucket: f.bucket,
			Object: oldName,
		}
		if _, err := f.client.CopyObject(ctx, dest, src); err != nil {
			return errors.WithStack(err)
		}
		if err := f.client.RemoveObject(ctx, f.bucket, oldName, minio.RemoveObjectOptions{
			ForceDelete: true,
		}); err != nil {
			return errors.WithStack(err)
		}
		return nil
	}
}

// Stat implements webdav.FileSystem.
func (f *FileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	name = clean(name)

	fileInfo, err := stat(ctx, f.client, f.bucket, name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}

		return nil, errors.WithStack(err)
	}

	return fileInfo, nil
}

func (f *FileSystem) checkParent(ctx context.Context, name string) error {
	name = clean(name)
	// Root always exists and has no parent we care about
	if name == separator {
		return nil
	}

	dir := filepath.Dir(name)
	if dir == separator || dir == "." {
		return nil
	}

	// In S3, we check if the directory "object" exists
	stats, err := f.Stat(ctx, dir)
	if err != nil {
		return err // Likely os.ErrNotExist
	}
	if !stats.IsDir() {
		// Parent exists but is a file, not a directory.
		// strict webdav implies this path is invalid for a child.
		return os.ErrNotExist
	}
	return nil
}

// NewFileSystem creates a new S3 filesystem with the given client and bucket
func NewFileSystem(client *minio.Client, bucket string) *FileSystem {
	return &FileSystem{
		client: client,
		bucket: bucket,
	}
}

var _ webdav.FileSystem = &FileSystem{}

func clean(name string) string {
	name = strings.Trim(name, separator)
	if name == "" {
		name = separator
	}
	return name
}
