package testsuite

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// RecursiveDirectory tests operations on nested directory structures
func RecursiveDirectory(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	basePath := "Test/RecursiveDirectory"

	// Create the base directory
	if err := fs.Mkdir(ctx, basePath, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Create a nested directory structure
	dirs := []string{
		"level1",
		"level1/level2a",
		"level1/level2b",
		"level1/level2a/level3",
	}

	for _, dir := range dirs {
		dirPath := filepath.Join(basePath, dir)
		if err := fs.Mkdir(ctx, dirPath, os.ModePerm); err != nil {
			return errors.WithStack(err)
		}

		// Verify directory was created
		info, err := fs.Stat(ctx, dirPath)
		if err != nil {
			return errors.WithStack(err)
		}
		if !info.IsDir() {
			return errors.Errorf("expected '%s' to be a directory", dirPath)
		}
	}

	// Create files in different levels
	files := []string{
		"file1.txt",
		"level1/file2.txt",
		"level1/level2a/file3.txt",
		"level1/level2b/file4.txt",
		"level1/level2a/level3/file5.txt",
	}

	for _, file := range files {
		filePath := filepath.Join(basePath, file)
		f, err := fs.OpenFile(ctx, filePath, os.O_CREATE|os.O_RDWR, os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}

		content := "Content for " + file
		if _, err := f.Write([]byte(content)); err != nil {
			f.Close()
			return errors.WithStack(err)
		}

		if err := f.Close(); err != nil {
			return errors.WithStack(err)
		}
	}

	// Test reading the directory structure
	// First check the base level
	dirFile, err := fs.OpenFile(ctx, basePath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	entries, err := dirFile.Readdir(-1)
	if err != nil {
		dirFile.Close()
		return errors.WithStack(err)
	}
	dirFile.Close()

	// Should contain file1.txt and level1 directory
	entryNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryNames = append(entryNames, entry.Name())
	}
	sort.Strings(entryNames)

	expectedEntries := []string{"file1.txt", "level1"}
	sort.Strings(expectedEntries)

	if !equalStringSlices(entryNames, expectedEntries) {
		return errors.Errorf("base directory entries: expected '%v', got '%v'", expectedEntries, entryNames)
	}

	// Now test removing a directory recursively
	// Remove level1/level2a and everything under it
	dirToRemove := filepath.Join(basePath, "level1/level2a")
	if err := fs.RemoveAll(ctx, dirToRemove); err != nil {
		return errors.WithStack(err)
	}

	// Verify the directory is gone
	_, err = fs.Stat(ctx, dirToRemove)
	if err == nil {
		return errors.Errorf("directory '%s' still exists after RemoveAll", dirToRemove)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("expected ErrNotExist, got: %v", err)
	}

	// Verify level1/level2b still exists
	remainingDir := filepath.Join(basePath, "level1/level2b")
	_, err = fs.Stat(ctx, remainingDir)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// equalStringSlices compares two string slices for equality
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
