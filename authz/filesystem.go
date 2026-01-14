package authz

import (
	"context"
	"log/slog"
	"os"
	"slices"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

type Operation string

const (
	OperationMkdir  = "mkdir"
	OperationOpen   = "open"
	OperationRemove = "remove"
	OperationRename = "rename"
	OperationStat   = "stat"
)

type FileSystem struct {
	backend webdav.FileSystem
}

// Mkdir implements webdav.FileSystem.
func (f *FileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	err := f.assertAuthorization(ctx, OperationMkdir, map[string]any{
		"name": name,
		"perm": perm,
	})
	if err != nil {
		return err
	}

	return f.backend.Mkdir(ctx, name, perm)
}

// OpenFile implements webdav.FileSystem.
func (f *FileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	err := f.assertAuthorization(ctx, OperationOpen, map[string]any{
		"name": name,
		"flag": flag,
		"perm": perm,
	})
	if err != nil {
		return nil, err
	}

	return f.backend.OpenFile(ctx, name, flag, perm)
}

// RemoveAll implements webdav.FileSystem.
func (f *FileSystem) RemoveAll(ctx context.Context, name string) error {
	err := f.assertAuthorization(ctx, OperationRemove, map[string]any{
		"name": name,
	})
	if err != nil {
		return err
	}

	return f.backend.RemoveAll(ctx, name)
}

// Rename implements webdav.FileSystem.
func (f *FileSystem) Rename(ctx context.Context, oldName string, newName string) error {
	err := f.assertAuthorization(ctx, OperationRename, map[string]any{
		"oldName": oldName,
		"newName": newName,
	})
	if err != nil {
		return err
	}

	return f.backend.Rename(ctx, oldName, newName)
}

// Stat implements webdav.FileSystem.
func (f *FileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	err := f.assertAuthorization(ctx, OperationStat, map[string]any{
		"name": name,
	})
	if err != nil {
		return nil, err
	}

	return f.backend.Stat(ctx, name)
}

func (f *FileSystem) assertAuthorization(ctx context.Context, operation Operation, env map[string]any) error {
	user, err := ContextUser(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	if env == nil {
		env = map[string]any{}
	}

	env["operation"] = string(operation)
	env["user"] = user.Attrs()
	env["groups"] = slices.Collect(func(yield func(string) bool) {
		for _, g := range user.Groups() {
			if !yield(g.Name()) {
				return
			}
		}
	})

	env["OP_MKDIR"] = string(OperationMkdir)
	env["OP_OPEN"] = string(OperationOpen)
	env["OP_REMOVE"] = string(OperationRemove)
	env["OP_RENAME"] = string(OperationRename)
	env["OP_STAT"] = string(OperationStat)

	for _, r := range user.Rules() {
		slog.DebugContext(ctx, "executing rule", slog.Any("rule", r), slog.Any("env", env))

		allowed, err := r.Exec(env)
		if err != nil {
			return errors.WithStack(err)
		}

		slog.DebugContext(ctx, "rule result", slog.Any("rule", r), slog.Bool("result", allowed))

		if allowed {
			return nil
		}
	}

	return os.ErrPermission
}

func NewFileSystem(backend webdav.FileSystem) *FileSystem {
	return &FileSystem{
		backend: backend,
	}
}

var _ webdav.FileSystem = &FileSystem{}
