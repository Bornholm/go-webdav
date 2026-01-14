package testsuite

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// FileMetadata tests the ability to store and retrieve file metadata
func FileMetadata(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	path := "Test/FileMetadata/metadata-test.txt"

	// Create directory and file
	if err := fs.Mkdir(ctx, filepath.Dir(path), os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	// Create a file with known content
	file, err := fs.OpenFile(ctx, path, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	content := "content for metadata test"
	contentSize := int64(len(content))
	
	if _, err := io.WriteString(file, content); err != nil {
		file.Close()
		return errors.WithStack(err)
	}

	if err := file.Close(); err != nil {
		return errors.WithStack(err)
	}

	// Get initial creation time to compare with modification time later
	beforeModify := time.Now()

	// Get file info and verify metadata
	fileInfo, err := fs.Stat(ctx, path)
	if err != nil {
		return errors.WithStack(err)
	}

	// Verify name
	if e, g := filepath.Base(path), fileInfo.Name(); e != g {
		return errors.Errorf("fileInfo.Name: expected '%s', got '%s'", e, g)
	}

	// Verify size
	if e, g := contentSize, fileInfo.Size(); e != g {
		return errors.Errorf("fileInfo.Size: expected '%d', got '%d'", e, g)
	}

	// Verify mode
	if !fileInfo.Mode().IsRegular() {
		return errors.Errorf("fileInfo.Mode: expected regular file, got '%s'", fileInfo.Mode().String())
	}

	// Modify the file after a short delay to ensure modification time changes
	time.Sleep(1 * time.Second)

	file, err = fs.OpenFile(ctx, path, os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	newContent := "updated content for metadata test"
	newContentSize := int64(len(newContent))
	
	if _, err := io.WriteString(file, newContent); err != nil {
		file.Close()
		return errors.WithStack(err)
	}

	if err := file.Close(); err != nil {
		return errors.WithStack(err)
	}

	// Get updated file info
	updatedFileInfo, err := fs.Stat(ctx, path)
	if err != nil {
		return errors.WithStack(err)
	}

	// Verify updated size
	if e, g := newContentSize, updatedFileInfo.Size(); e != g {
		return errors.Errorf("updatedFileInfo.Size: expected '%d', got '%d'", e, g)
	}

	// Verify modification time changed
	if !updatedFileInfo.ModTime().After(beforeModify) {
		return errors.Errorf("updatedFileInfo.ModTime: expected time after %v, got %v", 
			beforeModify, updatedFileInfo.ModTime())
	}

	return nil
}