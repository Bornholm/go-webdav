package testsuite

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// RenameDirectoryWithChildren tests the ability to rename a directory that contains files and subdirectories
func RenameDirectoryWithChildren(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	// Create test parent directory
	testDir := "Test/RenameDirectoryWithChildren"
	if err := fs.Mkdir(ctx, testDir, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Create directory structure to rename
	srcPath := filepath.Join(testDir, "original-dir")
	if err := fs.Mkdir(ctx, srcPath, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Create subdirectories
	subdirs := []string{
		"subdir1",
		"subdir2",
		"subdir2/nested",
	}

	for _, subdir := range subdirs {
		subdirPath := filepath.Join(srcPath, subdir)
		if err := fs.Mkdir(ctx, subdirPath, os.ModePerm); err != nil {
			return errors.WithStack(err)
		}
	}

	// Create files in various locations within the directory structure
	files := map[string]string{
		"file1.txt":               "Content of file1",
		"subdir1/file2.txt":       "Content of file2",
		"subdir2/file3.txt":       "Content of file3",
		"subdir2/nested/file4.txt": "Content of file4",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(srcPath, filePath)
		file, err := fs.OpenFile(ctx, fullPath, os.O_CREATE|os.O_RDWR, os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}

		if _, err := io.WriteString(file, content); err != nil {
			file.Close()
			return errors.WithStack(err)
		}

		if err := file.Close(); err != nil {
			return errors.WithStack(err)
		}
	}

	// Verify original directory structure exists
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

	// Verify old directory path doesn't exist
	_, err = fs.Stat(ctx, srcPath)
	if err == nil {
		return errors.Errorf("original directory still exists after rename")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("expected ErrNotExist for original path, got: %v", err)
	}

	// Verify new directory path exists
	dstInfo, err := fs.Stat(ctx, dstPath)
	if err != nil {
		return errors.Errorf("failed to stat directory after rename: %v", err)
	}
	if !dstInfo.IsDir() {
		return errors.Errorf("'%s' should be a directory", dstPath)
	}

	// Verify subdirectories exist at new location
	for _, subdir := range subdirs {
		subdirPath := filepath.Join(dstPath, subdir)
		info, err := fs.Stat(ctx, subdirPath)
		if err != nil {
			return errors.Errorf("subdirectory '%s' not found after rename: %v", subdirPath, err)
		}
		if !info.IsDir() {
			return errors.Errorf("'%s' should be a directory", subdirPath)
		}
	}

	// Verify files exist and have correct content at new location
	for filePath, expectedContent := range files {
		fullPath := filepath.Join(dstPath, filePath)
		
		// Verify file exists
		fileInfo, err := fs.Stat(ctx, fullPath)
		if err != nil {
			return errors.Errorf("file '%s' not found after rename: %v", fullPath, err)
		}
		if fileInfo.IsDir() {
			return errors.Errorf("'%s' should be a file, not a directory", fullPath)
		}

		// Verify content
		file, err := fs.OpenFile(ctx, fullPath, os.O_RDONLY, os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}

		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return errors.WithStack(err)
		}

		if e, g := expectedContent, string(content); e != g {
			return errors.Errorf("file content: expected '%s', got '%s'", e, g)
		}
	}

	// Verify directory structure with Readdir
	dirFile, err := fs.OpenFile(ctx, dstPath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	entries, err := dirFile.Readdir(-1)
	if err != nil {
		dirFile.Close()
		return errors.WithStack(err)
	}
	dirFile.Close()

	// Should contain the top-level entries
	entryNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryNames = append(entryNames, entry.Name())
	}
	sort.Strings(entryNames)

	expectedEntries := []string{"file1.txt", "subdir1", "subdir2"}
	sort.Strings(expectedEntries)

	if !equalStringSlices(entryNames, expectedEntries) {
		return errors.Errorf("directory entries: expected '%v', got '%v'", expectedEntries, entryNames)
	}

	return nil
}