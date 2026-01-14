package testsuite

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// RenameEmptyDirectory tests the ability to rename an empty directory
func RenameEmptyDirectory(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	// Create test parent directory
	testDir := "Test/RenameEmptyDirectory"
	if err := fs.Mkdir(ctx, testDir, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Create an empty directory to rename
	srcPath := filepath.Join(testDir, "original-dir")
	if err := fs.Mkdir(ctx, srcPath, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Verify directory exists at original path
	srcInfo, err := fs.Stat(ctx, srcPath)
	if err != nil {
		return errors.Errorf("failed to stat directory before rename: %v", err)
	}
	if !srcInfo.IsDir() {
		return errors.Errorf("'%s' should be a directory", srcPath)
	}

	// Rename the directory
	dstPath := filepath.Join(testDir, "renamed-dir")
	if err := fs.Rename(ctx, srcPath, dstPath); err != nil {
		return errors.WithStack(err)
	}

	// Verify directory doesn't exist at original path
	_, err = fs.Stat(ctx, srcPath)
	if err == nil {
		return errors.Errorf("directory still exists at original path after rename")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("expected ErrNotExist for original path, got: %v", err)
	}

	// Verify directory exists at new path
	dstInfo, err := fs.Stat(ctx, dstPath)
	if err != nil {
		return errors.Errorf("failed to stat directory after rename: %v", err)
	}
	if !dstInfo.IsDir() {
		return errors.Errorf("'%s' should be a directory", dstPath)
	}

	if e, g := filepath.Base(dstPath), dstInfo.Name(); e != g {
		return errors.Errorf("fileInfo.Name: expected '%s', got '%s'", e, g)
	}

	return nil
}
