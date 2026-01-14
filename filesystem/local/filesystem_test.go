package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bornholm/go-webdav/filesystem/bench"
	"github.com/bornholm/go-webdav/filesystem/testsuite"
	"github.com/bornholm/go-webdav/litmus"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

func TestFileSystem(t *testing.T) {
	fs := createFileSystem(t)
	testsuite.TestFileSystem(t, fs)
}

func TestLitmus(t *testing.T) {
	fs := createFileSystem(t)
	litmus.RunTestSuite(t, fs)
}

func BenchmarkFileSystem(b *testing.B) {
	fs := createFileSystem(b)
	bench.RunTestSuite(b, fs)
}

func createFileSystem(t testing.TB) webdav.FileSystem {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	dir := filepath.Join(cwd, "testdata/.local")

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	fs := webdav.Dir(dir)

	return fs
}
