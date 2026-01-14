package testsuite

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

func ReadDir(ctx context.Context, fs webdav.FileSystem) error {
	if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
		return errors.WithStack(err)
	}

	dir := "Test/ReadDir"

	if err := fs.Mkdir(ctx, dir, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	directories := []string{
		"sub1",
		"sub2",
	}

	for _, n := range directories {
		if err := fs.Mkdir(ctx, filepath.Join(dir, n), os.ModePerm); err != nil {
			return errors.WithStack(err)
		}
	}

	files := []string{
		"1.txt",
		"2.txt",
		"sub1/3.txt",
		"sub2/4.txt",
		"sub2/5.txt",
	}

	for _, n := range files {
		file, err := fs.OpenFile(ctx, filepath.Join(dir, n), os.O_CREATE, os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}

		if err := file.Close(); err != nil {
			return errors.WithStack(err)
		}
	}

	dirFile, err := fs.OpenFile(ctx, dir, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	fileInfos, err := dirFile.Readdir(-1)
	if err != nil {
		return errors.WithStack(err)
	}

	if e, g := 4, len(fileInfos); e != g {
		return errors.Errorf("len(fileInfos): expected '%d', got '%d'", e, g)
	}

	return nil
}