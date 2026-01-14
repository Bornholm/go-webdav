package testsuite

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// RenameFile tests the ability to rename a file in the filesystem
func RenameFile(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	// Create test directory
	testDir := "Test/RenameFile"
	if err := fs.Mkdir(ctx, testDir, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Create a file to rename
	srcPath := filepath.Join(testDir, "original.txt")
	file, err := fs.OpenFile(ctx, srcPath, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	content := "content for rename test"
	if _, err := io.WriteString(file, content); err != nil {
		file.Close()
		return errors.WithStack(err)
	}

	if err := file.Close(); err != nil {
		return errors.WithStack(err)
	}

	// Verify file exists at original path
	_, err = fs.Stat(ctx, srcPath)
	if err != nil {
		return errors.Errorf("failed to stat file before rename: %v", err)
	}

	// Rename the file
	dstPath := filepath.Join(testDir, "renamed.txt")
	if err := fs.Rename(ctx, srcPath, dstPath); err != nil {
		return errors.WithStack(err)
	}

	// Verify file doesn't exist at original path
	_, err = fs.Stat(ctx, srcPath)
	if err == nil {
		return errors.Errorf("file still exists at original path after rename")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("expected ErrNotExist for original path, got: %v", err)
	}

	// Verify file exists at new path
	fileInfo, err := fs.Stat(ctx, dstPath)
	if err != nil {
		return errors.Errorf("failed to stat file after rename: %v", err)
	}

	if e, g := filepath.Base(dstPath), fileInfo.Name(); e != g {
		return errors.Errorf("fileInfo.Name: expected '%s', got '%s'", e, g)
	}

	// Verify content is preserved
	file, err = fs.OpenFile(ctx, dstPath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return errors.WithStack(err)
	}

	if e, g := content, string(data); e != g {
		return errors.Errorf("file content: expected '%s', got '%s'", e, g)
	}

	return nil
}