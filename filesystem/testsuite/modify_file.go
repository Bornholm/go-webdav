package testsuite

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// ModifyFile tests the ability to modify an existing file in the filesystem
func ModifyFile(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	path := "Test/ModifyFile/file-to-modify.txt"

	// Create directory and file
	if err := fs.Mkdir(ctx, filepath.Dir(path), os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Create a file with initial content
	file, err := fs.OpenFile(ctx, path, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	initialContent := "initial content"
	if _, err := io.WriteString(file, initialContent); err != nil {
		file.Close()
		return errors.WithStack(err)
	}

	if err := file.Close(); err != nil {
		return errors.WithStack(err)
	}

	// Verify initial content
	file, err = fs.OpenFile(ctx, path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		file.Close()
		return errors.WithStack(err)
	}

	if err := file.Close(); err != nil {
		return errors.WithStack(err)
	}

	if e, g := initialContent, string(data); e != g {
		return errors.Errorf("initial content: expected '%s', got '%s'", e, g)
	}

	// Modify the file with new content
	file, err = fs.OpenFile(ctx, path, os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	modifiedContent := "modified content"
	if _, err := io.WriteString(file, modifiedContent); err != nil {
		file.Close()
		return errors.WithStack(err)
	}

	if err := file.Close(); err != nil {
		return errors.WithStack(err)
	}

	// Verify modified content
	file, err = fs.OpenFile(ctx, path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	defer file.Close()

	data, err = io.ReadAll(file)
	if err != nil {
		return errors.WithStack(err)
	}

	if e, g := modifiedContent, string(data); e != g {
		return errors.Errorf("modified content: expected '%s', got '%s'", e, g)
	}

	return nil
}