package sqlite

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

	dbPath := filepath.Join(cwd, "testdata", "webdav.db")

	files, err := filepath.Glob(dbPath + "*")
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	for _, f := range files {
		if err := os.RemoveAll(f); err != nil {
			t.Fatalf("%+v", errors.WithStack(err))
		}
	}

	fs := NewFileSystem(dbPath)

	return fs
}
