package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/bornholm/go-webdav"
	"github.com/bornholm/go-webdav/lock"
	"github.com/bornholm/go-webdav/middleware/deadprops"
	"github.com/pkg/errors"
	wd "golang.org/x/net/webdav"
)

type Logger func(r *http.Request, err error)

type Options struct {
	Prefix      string
	Middlewares []webdav.Middleware
	LockSystem  wd.LockSystem
	Logger      Logger
}

type OptionFunc func(opts *Options)

func WithPrefix(prefix string) OptionFunc {
	return func(opts *Options) {
		opts.Prefix = prefix
	}
}

func WithMiddlewares(middewares ...webdav.Middleware) OptionFunc {
	return func(opts *Options) {
		opts.Middlewares = middewares
	}
}

func WithLockSystem(lockSystem wd.LockSystem) OptionFunc {
	return func(opts *Options) {
		opts.LockSystem = lockSystem
	}
}

func WithLogger(logger Logger) OptionFunc {
	return func(opts *Options) {
		opts.Logger = logger
	}
}

func NewOptions(funcs ...OptionFunc) *Options {
	opts := &Options{
		Prefix: "",
		Middlewares: []webdav.Middleware{
			deadprops.Middleware(deadprops.NewMemStore()),
		},
		LockSystem: lock.NewSystem(lock.NewMemoryStore()),
		Logger: func(r *http.Request, err error) {
			if err != nil && !(errors.Is(err, context.Canceled)) {
				slog.ErrorContext(r.Context(), err.Error(), "method", r.Method, "path", r.URL.Path)
			}
		},
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

type Handler struct {
	webdav *wd.Handler
}

// ServeHTTP implements [http.Handler].
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.webdav.ServeHTTP(w, r)
}

func New(fs wd.FileSystem, funcs ...OptionFunc) *Handler {
	opts := NewOptions(funcs...)

	webdav := &wd.Handler{
		FileSystem: webdav.Chain(fs, opts.Middlewares...),
		LockSystem: opts.LockSystem,
		Prefix:     opts.Prefix,
		Logger:     opts.Logger,
	}

	return &Handler{
		webdav: webdav,
	}
}

var _ http.Handler = &Handler{}
