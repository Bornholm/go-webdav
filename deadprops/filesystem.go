package deadprops

import (
	"context"
	"os"

	"golang.org/x/net/webdav"
)

type Filesystem struct {
	store   Store
	backend webdav.FileSystem
}

// Mkdir implements webdav.FileSystem.
func (fs *Filesystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return fs.backend.Mkdir(ctx, name, perm)
}

// OpenFile implements webdav.FileSystem.
func (fs *Filesystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	file, err := fs.backend.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	return &File{name: name, file: file, store: fs.store}, nil
}

// RemoveAll implements webdav.FileSystem.
func (fs *Filesystem) RemoveAll(ctx context.Context, name string) error {
	if err := fs.backend.RemoveAll(ctx, name); err != nil {
		return err
	}

	return fs.store.RemoveAll(name)
}

// Rename implements webdav.FileSystem.
func (fs *Filesystem) Rename(ctx context.Context, oldName string, newName string) error {
	if err := fs.backend.Rename(ctx, oldName, newName); err != nil {
		return err
	}

	return fs.store.Rename(oldName, newName)
}

// Stat implements webdav.FileSystem.
func (fs *Filesystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	return fs.backend.Stat(ctx, name)
}

var _ webdav.FileSystem = &Filesystem{}

func Wrap(backend webdav.FileSystem, store Store) *Filesystem {
	return &Filesystem{
		store:   store,
		backend: backend,
	}
}
