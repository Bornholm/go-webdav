package local

import (
	"os"

	"github.com/bornholm/go-webdav/filesystem"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

const Type filesystem.Type = "local"

func init() {
	filesystem.Register(Type, CreateFileSystemFromOptions)
}

type Options struct {
	Dir string `mapstructure:"dir"`
}

func CreateFileSystemFromOptions(options any) (webdav.FileSystem, error) {
	opts := Options{}

	if err := mapstructure.Decode(options, &opts); err != nil {
		return nil, errors.Wrapf(err, "could not parse '%s' filesystem options", Type)
	}

	if err := os.MkdirAll(opts.Dir, os.ModePerm|os.ModeDir); err != nil {
		return nil, errors.Wrapf(err, "could not create directory '%s'", opts.Dir)
	}

	fs := webdav.Dir(opts.Dir)

	return fs, nil
}
