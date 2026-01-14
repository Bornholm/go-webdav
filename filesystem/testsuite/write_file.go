package testsuite

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

func WriteFile(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	path := "Test/WriteFile/foo.txt"

	if err := fs.Mkdir(ctx, filepath.Dir(path), os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	file, err := fs.OpenFile(ctx, path, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	content := "bar"

	written, err := io.WriteString(file, content)
	if err != nil {
		return errors.WithStack(err)
	}

	if e, g := len(content), written; e != g {
		return errors.Errorf("written: expected '%d', got '%d'", e, g)
	}

	if err := file.Close(); err != nil {
		return errors.WithStack(err)
	}

	file, err = fs.OpenFile(ctx, path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return errors.WithStack(err)
	}

	if e, g := content, string(data); e != g {
		return errors.Errorf("data: expected '%s', got '%s'", e, g)
	}

	return nil
}
