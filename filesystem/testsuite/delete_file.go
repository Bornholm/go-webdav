package testsuite

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// DeleteFile tests the ability to delete files from the filesystem
func DeleteFile(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	path := "Test/DeleteFile/file-to-delete.txt"

	// Create directory and file
	if err := fs.Mkdir(ctx, filepath.Dir(path), os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Create a file to delete
	file, err := fs.OpenFile(ctx, path, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	content := "content to be deleted"
	if _, err := io.WriteString(file, content); err != nil {
		file.Close()
		return errors.WithStack(err)
	}

	if err := file.Close(); err != nil {
		return errors.WithStack(err)
	}

	// Verify file exists
	_, err = fs.Stat(ctx, path)
	if err != nil {
		return errors.Errorf("failed to stat file before deletion: %v", err)
	}

	// Delete the file
	if err := fs.RemoveAll(ctx, path); err != nil {
		return errors.WithStack(err)
	}

	// Verify file doesn't exist anymore
	_, err = fs.Stat(ctx, path)
	if err == nil {
		return errors.Errorf("file still exists after deletion")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("expected ErrNotExist, got: %v", err)
	}

	return nil
}