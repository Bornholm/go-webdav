package filesystem

import (
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

var (
	ErrNotRegistered = errors.New("not registered")
	ErrNotSupported  = errors.New("not supported")
)

type Type string

type Factory func(options any) (webdav.FileSystem, error)

var factories = make(map[Type]Factory, 0)

func Register(fsType Type, factory Factory) {
	factories[fsType] = factory
}

func Registered() []Type {
	types := make([]Type, 0, len(factories))
	for t := range factories {
		types = append(types, t)
	}
	return types
}

func New(fsType Type, options any) (webdav.FileSystem, error) {
	factory, exists := factories[fsType]
	if !exists {
		return nil, errors.Wrapf(ErrNotRegistered, "no filesystem associated with type '%s'", fsType)
	}

	fs, err := factory(options)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return fs, nil
}
