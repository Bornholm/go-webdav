package litmus

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/net/webdav"

	"github.com/bornholm/go-webdav/deadprops"
	"github.com/bornholm/go-webdav/lock"
)

type testLogConsumer struct {
	t *testing.T
}

// Printf implements log.Logger.
func (lc *testLogConsumer) Printf(format string, v ...any) {
	lc.t.Logf(format, v...)
}

// Accept prints the log to stdout
func (lc *testLogConsumer) Accept(l testcontainers.Log) {
	lc.t.Log(strings.TrimSpace(string(l.Content)))
}

func RunTestSuite(t *testing.T, fs webdav.FileSystem) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs = deadprops.Wrap(fs, deadprops.NewMemStore())

	handler := &webdav.Handler{
		FileSystem: fs,
		LockSystem: lock.NewSystem(lock.NewMemoryStore()),
		Prefix:     "/",
		Logger: func(r *http.Request, err error) {
			if err != nil && !(errors.Is(err, os.ErrNotExist) || errors.Is(err, context.Canceled)) {
				t.Logf("[ERROR] %s %s - %s", r.Method, r.URL.Path, err)
				return
			}
		},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatalf("could not create tcp listener: %+v", errors.WithStack(err))
	}

	defer listener.Close()

	server := &http.Server{
		Handler: handler,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("could not serve: %+v", errors.WithStack(err))
		}
	}()

	addr := listener.Addr().String()

	logger := &testLogConsumer{t}

	rootDir, err := getProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root directory: %+v", errors.WithStack(err))
	}

	req := testcontainers.GenericContainerRequest{
		Logger: logger,
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:       filepath.Join(rootDir, "./misc/litmus"),
				PrintBuildLog: false,
			},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.NetworkMode = "host"
			},
			Cmd:        []string{"/usr/local/bin/litmus", "-k", "-n", fmt.Sprintf("http://%s", addr)},
			WaitingFor: wait.ForExit(),
			LogConsumerCfg: &testcontainers.LogConsumerConfig{
				Consumers: []testcontainers.LogConsumer{
					logger,
				},
			},
		},
		Started: true,
	}

	ctr, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		t.Fatalf("Could not start container: %+v", errors.WithStack(err))
	}
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("failed to terminate container: %+v", errors.WithStack(err))
		}
	}()

	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	state, err := ctr.State(ctx)
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	if e, g := 0, state.ExitCode; e != g {
		t.Errorf("litmus exit code: expected %v, got %v", e, g)
	}
}

func getProjectRoot() (string, error) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", errors.WithStack(err)
	}

	// Loop to walk up the tree
	for {
		// Check if go.mod exists in this directory
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}

		// Check if we hit the root of the filesystem
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", errors.New("go.mod not found")
		}
		wd = parent
	}
}
