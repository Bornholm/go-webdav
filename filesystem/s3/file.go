package s3

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

type File struct {
	ctx context.Context

	fs   *FileSystem
	name string
	key  string

	mu     sync.Mutex
	closed bool

	// Directory Mode
	isDir bool

	// Reader Mode (GET)
	obj *minio.Object

	// Writer Mode (PUT)
	isWriter   bool
	pipeWriter *io.PipeWriter
	done       chan error
}

// Readdir implements webdav.File
func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return readdir(f.ctx, f.fs.client, f.fs.bucket, f.name, count)
}

// Stat implements webdav.File
func (f *File) Stat() (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 1. Directory Case
	if f.isDir {
		return &FileInfo{
			name:    filepath.Base(f.name),
			size:    0,
			modTime: time.Now(), // Directories don't strictly have modtime in S3
			isDir:   true,
		}, nil
	}

	// 2. Reader Case (GET)
	if !f.isWriter && f.obj != nil {
		info, err := f.obj.Stat()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, os.ErrNotExist
			}

			return nil, errors.WithStack(err)
		}
		return &FileInfo{
			name:    filepath.Base(f.name),
			size:    info.Size,
			modTime: info.LastModified,
			isDir:   false,
		}, nil
	}

	// 3. Writer Case (PUT)
	return &FileInfo{
		name:    filepath.Base(f.name),
		size:    0,
		modTime: time.Now(),
		isDir:   false,
	}, nil
}

// Write, Read, Seek, Close, etc... exist as previously defined
// (Include the pipe implementation provided in the previous turn here)
func (f *File) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, os.ErrClosed
	}
	if !f.isWriter {
		return 0, errors.New("file opened for reading")
	}
	return f.pipeWriter.Write(p)
}

func (f *File) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, os.ErrClosed
	}
	if f.isWriter {
		return 0, errors.New("file opened for writing")
	}
	return f.obj.Read(p)
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, os.ErrClosed
	}
	if !f.isWriter {
		return f.obj.Seek(offset, whence)
	}
	if offset == 0 && whence == io.SeekCurrent {
		return 0, nil
	}
	return 0, errors.New("seek not supported during streaming upload")
}

func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return os.ErrClosed
	}
	f.closed = true

	if !f.isWriter {
		if f.obj != nil {
			return f.obj.Close()
		}
		return nil
	}

	if err := f.pipeWriter.Close(); err != nil {
		return errors.WithStack(err)
	}
	select {
	case err := <-f.done:
		if err != nil {
			return errors.Wrap(err, "s3 upload failed")
		}
		return nil
	case <-time.After(time.Hour * 2):
		return errors.New("timeout")
	}
}

var _ webdav.File = &File{}
