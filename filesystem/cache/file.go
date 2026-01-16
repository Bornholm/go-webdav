package cache

import (
	"context"
	"io/fs"
	"os"
	"path"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/net/webdav"
)

type fileWrapper struct {
	file    webdav.File
	ctx     context.Context
	fs      *FileSystem
	name    string
	isWrite bool
}

// Read implements [webdav.File].
func (w *fileWrapper) Read(p []byte) (n int, err error) {
	return w.file.Read(p)
}

// Seek implements [webdav.File].
func (w *fileWrapper) Seek(offset int64, whence int) (int64, error) {
	return w.file.Seek(offset, whence)
}

// Stat implements [webdav.File].
func (w *fileWrapper) Stat() (fs.FileInfo, error) {
	return w.file.Stat()
}

// Write implements [webdav.File].
func (w *fileWrapper) Write(p []byte) (n int, err error) {
	return w.file.Write(p)
}

func (w *fileWrapper) Readdir(count int) ([]os.FileInfo, error) {
	spew.Dump(count)
	if count > 0 {
		return w.file.Readdir(count)
	}

	children, ok, err := w.fs.store.GetChildren(w.ctx, w.name)
	if err != nil {
		return nil, err
	}

	if ok {
		return children, nil
	}

	children, err = w.file.Readdir(count)
	if err != nil {
		return nil, err
	}

	if err := w.fs.store.PutChildren(w.ctx, w.name, children); err != nil {
		return nil, err
	}

	for _, child := range children {
		fullPath := path.Join(w.name, child.Name())
		if err := w.fs.store.Put(w.ctx, fullPath, child); err != nil {
			return nil, err
		}
	}

	return children, nil
}

func (w *fileWrapper) Close() error {
	if w.isWrite {
		if err := w.fs.invalidateWithParent(w.ctx, w.name); err != nil {
			return err
		}
	}
	return w.file.Close()
}

var _ webdav.File = &fileWrapper{}
