package sqlite

import (
	"github.com/bornholm/go-webdav/filesystem"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

const Type filesystem.Type = "sqlite"

func init() {
	filesystem.Register(Type, CreateFileSystemFromOptions)
}

type Options struct {
	Path string `mapstructure:"path"`
}

func CreateFileSystemFromOptions(options any) (webdav.FileSystem, error) {
	opts := Options{}

	if err := mapstructure.Decode(options, &opts); err != nil {
		return nil, errors.Wrapf(err, "could not parse '%s' filesystem options", Type)
	}

	fs := NewFileSystem(opts.Path)

	return fs, nil
}
