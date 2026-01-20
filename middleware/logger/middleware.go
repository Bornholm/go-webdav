package logger

import (
	"log/slog"

	"github.com/bornholm/go-webdav"
)

func Middleware(logger *slog.Logger) webdav.Middleware {
	return func(next webdav.FileSystem) webdav.FileSystem {
		return NewFileSystem(next, logger)
	}
}
