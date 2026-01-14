package webdav

import (
	"context"
	"log/slog"
	"os"

	"golang.org/x/net/webdav"
)

type LoggerFilesystem struct {
	logger  *slog.Logger
	backend webdav.FileSystem
}

// Mkdir implements webdav.FileSystem.
func (fs *LoggerFilesystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	fs.logger.DebugContext(ctx, "webdav operation", slog.String("operation", "mkdir"), slog.String("name", name), slog.Any("perm", perm))
	return fs.backend.Mkdir(ctx, name, perm)
}

// OpenFile implements webdav.FileSystem.
func (fs *LoggerFilesystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	fs.logger.DebugContext(ctx, "webdav operation", slog.String("operation", "openfile"), slog.String("name", name), slog.Int("flag", flag), slog.Any("perm", perm))
	return fs.backend.OpenFile(ctx, name, flag, perm)
}

// RemoveAll implements webdav.FileSystem.
func (fs *LoggerFilesystem) RemoveAll(ctx context.Context, name string) error {
	fs.logger.DebugContext(ctx, "webdav operation", slog.String("operation", "removeall"), slog.String("name", name))
	return fs.backend.RemoveAll(ctx, name)
}

// Rename implements webdav.FileSystem.
func (fs *LoggerFilesystem) Rename(ctx context.Context, oldName string, newName string) error {
	fs.logger.DebugContext(ctx, "webdav operation", slog.String("operation", "rename"), slog.String("oldName", oldName), slog.String("newName", newName))
	return fs.backend.Rename(ctx, oldName, newName)
}

// Stat implements webdav.FileSystem.
func (fs *LoggerFilesystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	fs.logger.DebugContext(ctx, "webdav operation", slog.String("operation", "stat"), slog.String("name", name))
	return fs.backend.Stat(ctx, name)
}

func WithLogger(backend webdav.FileSystem, logger *slog.Logger) *LoggerFilesystem {
	return &LoggerFilesystem{
		backend: backend,
		logger:  logger,
	}
}

var _ webdav.FileSystem = &LoggerFilesystem{}
