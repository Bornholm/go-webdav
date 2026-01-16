package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"

	wd "github.com/bornholm/go-webdav"
	"github.com/bornholm/go-webdav/deadprops"
	"github.com/bornholm/go-webdav/filesystem"
	"github.com/bornholm/go-webdav/lock"
	"github.com/caarlos0/env/v11"
	"github.com/pkg/errors"
	sloghttp "github.com/samber/slog-http"
	"golang.org/x/net/webdav"

	_ "github.com/bornholm/go-webdav/filesystem/all"
	"github.com/bornholm/go-webdav/filesystem/cache"
	"github.com/go-playground/validator/v10"
)

var (
	address     string = ":3000"
	configFile  string = "config.json"
	rawLogLevel string = slog.LevelInfo.String()
)

func init() {
	flag.StringVar(&address, "address", address, "server listening address")
	flag.StringVar(&configFile, "config", configFile, "configuration file")
	flag.StringVar(&rawLogLevel, "log-level", rawLogLevel, "log level")
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flag.Parse()

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(rawLogLevel)); err != nil {
		slog.ErrorContext(ctx, "could not parse log level", slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(logLevel)

	rawConfig, err := os.ReadFile(configFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.ErrorContext(ctx, "could not read filesystem configuration file", slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}

	var conf config

	if rawConfig != nil {
		if err := json.Unmarshal(rawConfig, &conf); err != nil {
			slog.ErrorContext(ctx, "could not parse filesystem configuration file", slog.Any("error", errors.WithStack(err)))
			os.Exit(1)
		}
	}

	if err := env.ParseWithOptions(&conf, env.Options{Prefix: "GOWEBDAV_"}); err != nil {
		slog.ErrorContext(ctx, "could not parse environment variables", slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}

	validate := validator.New()
	if err := validate.StructCtx(ctx, &conf); err != nil {
		slog.ErrorContext(ctx, "could not validate config", slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}

	slog.InfoContext(ctx, "creating filesystem", "type", conf.Filesystem.Type)

	fs, err := filesystem.New(filesystem.Type(conf.Filesystem.Type), conf.Filesystem.Options.Value)
	if err != nil {
		slog.ErrorContext(ctx, "could not create filesystem", slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}

	fs = wd.WithLogger(fs, slog.Default())

	if conf.Cache.Enabled {
		slog.InfoContext(ctx, "enabling metadata cache", "ttl", conf.Cache.TTL)
		fs = cache.NewFileSystem(fs, cache.NewMemoryStore(conf.Cache.TTL))
	}

	fs = deadprops.Wrap(fs, deadprops.NewMemStore())

	var handler http.Handler = &webdav.Handler{
		FileSystem: fs,
		LockSystem: lock.NewSystem(lock.NewMemoryStore()),
		Prefix:     "",
		Logger: func(r *http.Request, err error) {
			if err != nil && !(errors.Is(err, context.Canceled)) {
				slog.ErrorContext(r.Context(), err.Error(), slog.Any("error", errors.WithStack(err)))
				return
			}
		},
	}

	slogMiddleware := sloghttp.New(slog.Default())
	handler = slogMiddleware(handler)

	if conf.Auth.Enabled && len(conf.Auth.Users) > 0 {
		slog.InfoContext(ctx, "enabling basic auth", "total_users", len(conf.Auth.Users))
		handler = basicAuth(handler, "go-webdav", conf.Auth.Users)
	}

	server := &http.Server{
		Addr:    address,
		Handler: handler,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	slog.InfoContext(ctx, "listening", "address", address)

	if err := server.ListenAndServe(); err != nil {
		slog.ErrorContext(ctx, err.Error(), slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}
}

func basicAuth(handler http.Handler, realm string, users map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		unauthorized := func() {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			w.Write([]byte("Unauthorised.\n"))
		}

		_, exists := users[user]
		if !exists {
			unauthorized()
			return
		}

		if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(users[user])) != 1 {
			unauthorized()
			return
		}

		handler.ServeHTTP(w, r)
	})
}
