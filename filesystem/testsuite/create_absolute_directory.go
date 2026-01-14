package testsuite

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

func CreateAbsoluteDirectory(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	path := "Test/CreateAbsoluteDirectory"

	if err := fs.Mkdir(ctx, path, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	fileInfo, err := fs.Stat(ctx, path)
	if err != nil {
		return errors.WithStack(err)
	}

	if e, g := filepath.Base(path), fileInfo.Name(); e != g {
		return errors.Errorf("fileInfo.Name: expected '%s', got '%s'", e, g)
	}

	if !fileInfo.IsDir() {
		return errors.Errorf("'%s' should be a directory", fileInfo.Name())
	}

	return nil
}