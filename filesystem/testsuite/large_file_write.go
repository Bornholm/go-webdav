package testsuite

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

func LargeFileWrite(ctx context.Context, fs webdav.FileSystem) error {
	tempDir, err := os.MkdirTemp("", "testdata-*")
	if err != nil {
		return errors.WithStack(err)
	}

	defer os.RemoveAll(tempDir)

	local, err := os.Create(filepath.Join(tempDir, "largefile"))
	if err != nil {
		return errors.WithStack(err)
	}

	if err := local.Truncate(1e8); err != nil {
		return errors.WithStack(err)
	}

	// Add a small random header to ensure data unicity
	buff := make([]byte, 32)
	if _, err := rand.Read(buff); err != nil {
		return errors.WithStack(err)
	}

	if _, err := local.WriteAt(buff, 0); err != nil {
		return errors.WithStack(err)
	}

	defer local.Close()

	localStat, err := local.Stat()
	if err != nil {
		return errors.WithStack(err)
	}

	localSHA, err := shasum(local)
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := local.Seek(0, io.SeekStart); err != nil {
		return errors.WithStack(err)
	}

	remote, err := fs.OpenFile(ctx, "largefile", os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := io.Copy(remote, local); err != nil {
		defer remote.Close()
		return errors.WithStack(err)
	}

	if err := remote.Close(); err != nil {
		return errors.WithStack(err)
	}

	remote, err = fs.OpenFile(ctx, "largefile", os.O_RDONLY, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	remoteStat, err := remote.Stat()
	if err != nil {
		return errors.WithStack(err)
	}

	if e, g := localStat.Size(), remoteStat.Size(); e != g {
		return errors.Errorf("remoteStat.Size(): expected '%d', got '%d'", e, g)
	}

	defer remote.Close()

	remoteSHA, err := shasum(remote)
	if err != nil {
		return errors.WithStack(err)
	}

	if e, g := localSHA, remoteSHA; e != g {
		return errors.Errorf("sha256: expected '%s', got '%s'", e, g)
	}

	return nil
}
