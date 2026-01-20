package cache

import (
	"context"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/minio/minio-go/v7/pkg/singleflight"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

type FileSystem struct {
	backend             webdav.FileSystem
	store               Store
	statSingleFlight    *singleflight.Group[string, os.FileInfo]
	readdirSingleFlight *singleflight.Group[string, []os.FileInfo]
}

func NewFileSystem(backend webdav.FileSystem, store Store) *FileSystem {
	return &FileSystem{
		backend:             backend,
		store:               store,
		statSingleFlight:    &singleflight.Group[string, os.FileInfo]{},
		readdirSingleFlight: &singleflight.Group[string, []os.FileInfo]{},
	}
}

func (fs *FileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	if err := fs.invalidateWithParent(ctx, name); err != nil {
		return err
	}

	return fs.backend.Mkdir(ctx, name, perm)
}

func (fs *FileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	isWrite := flag&os.O_RDWR != 0 || flag&os.O_WRONLY != 0 || flag&os.O_APPEND != 0 || flag&os.O_CREATE != 0 || flag&os.O_TRUNC != 0

	if isWrite {
		if err := fs.invalidateWithParent(ctx, name); err != nil {
			return nil, err
		}
	}

	f, err := fs.backend.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	return &fileWrapper{ctx: ctx, file: f, fs: fs, name: name, isWrite: isWrite}, nil
}

func (fs *FileSystem) RemoveAll(ctx context.Context, name string) error {
	if err := fs.invalidateWithParent(ctx, name); err != nil {
		return err
	}
	return fs.backend.RemoveAll(ctx, name)
}

func (fs *FileSystem) Rename(ctx context.Context, oldName, newName string) error {
	if err := fs.invalidateWithParent(ctx, oldName); err != nil {
		return err
	}
	if err := fs.invalidateWithParent(ctx, newName); err != nil {
		return err
	}
	return fs.backend.Rename(ctx, oldName, newName)
}

func (fs *FileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if info, ok, _ := fs.store.Get(ctx, name); ok {
		return info, nil
	}

	info, err, _ := fs.statSingleFlight.Do(name, func() (os.FileInfo, error) {
		info, err := fs.backend.Stat(ctx, name)
		if err != nil {
			return nil, err
		}

		return info, nil
	})
	if err != nil {
		return nil, err
	}

	if err := fs.store.Put(ctx, name, info); err != nil {
		return nil, err
	}

	// Pre-populate cache with directory listing
	if info.IsDir() {
		go func() {
			ctx = context.Background()

			dir, err := fs.OpenFile(ctx, name, os.O_RDONLY, os.ModePerm)
			if err != nil {
				slog.ErrorContext(ctx, "could not open directory", "error", errors.WithStack(err))
				return
			}

			defer dir.Close()

			slog.DebugContext(ctx, "pre-populating directory listing cache", "name", name)

			if _, err := dir.Readdir(0); err != nil {
				slog.ErrorContext(ctx, "could not read directory", "error", errors.WithStack(err))
				return
			}

			slog.DebugContext(ctx, "directory cache populated", "name", name)
		}()
	}

	return info, nil
}

func (fs *FileSystem) invalidateWithParent(ctx context.Context, name string) error {
	if err := fs.store.Invalidate(ctx, name); err != nil {
		return err
	}

	cleanPath := strings.TrimSuffix(name, "/")
	if cleanPath == "" || cleanPath == "." {
		return nil
	}

	parent := path.Dir(cleanPath)

	if err := fs.store.InvalidateChildren(ctx, parent); err != nil {
		return err
	}

	return nil
}
