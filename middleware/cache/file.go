package cache

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"strings"

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
	cacheKey := strings.TrimSuffix(w.name, "/")
	stat, err, _ := w.fs.statSingleFlight.Do(cacheKey, func() (os.FileInfo, error) {
		info, ok, err := w.fs.store.Get(w.ctx, w.name)
		if err != nil {
			return nil, err
		}

		if ok {
			slog.DebugContext(w.ctx, "cache hit", "name", w.name)
			return info, nil
		}

		slog.DebugContext(w.ctx, "cache miss", "name", w.name)

		stat, err := w.file.Stat()
		if err != nil {
			return nil, err
		}

		if err := w.fs.store.Put(w.ctx, cacheKey, stat); err != nil {
			return nil, err
		}

		return stat, nil
	})

	return stat, err
}

// Write implements [webdav.File].
func (w *fileWrapper) Write(p []byte) (n int, err error) {
	return w.file.Write(p)
}

func (w *fileWrapper) Readdir(count int) ([]os.FileInfo, error) {
	cacheKey := strings.TrimSuffix(w.name, "/")
	children, err, _ := w.fs.readdirSingleFlight.Do(fmt.Sprintf("%s-%d", cacheKey, count), func() ([]os.FileInfo, error) {
		if count > 0 {
			return w.file.Readdir(count)
		}

		children, ok, err := w.fs.store.GetChildren(w.ctx, cacheKey)
		if err != nil {
			return nil, err
		}

		if ok {
			slog.DebugContext(w.ctx, "cache hit", "name", w.name)
			return children, nil
		}

		slog.DebugContext(w.ctx, "cache miss", "name", w.name)

		children, err = w.file.Readdir(count)
		if err != nil {
			return nil, err
		}

		if err := w.fs.store.PutChildren(w.ctx, cacheKey, children); err != nil {
			return nil, err
		}

		for _, child := range children {
			fullPath := path.Join(w.name, child.Name())

			slog.DebugContext(w.ctx, "caching file stat", "name", fullPath)

			if err := w.fs.store.Put(w.ctx, fullPath, child); err != nil {
				return nil, err
			}
		}

		return children, nil
	})

	return children, err
}

func (w *fileWrapper) Close() error {
	var invErr error
	if w.isWrite {
		if err := w.fs.invalidateWithParent(w.ctx, w.name); err != nil {
			invErr = err
		}
	}

	closeErr := w.file.Close()

	if invErr != nil {
		return invErr
	}

	return closeErr
}

var _ webdav.File = &fileWrapper{}
