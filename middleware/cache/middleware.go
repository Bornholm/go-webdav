package cache

import "github.com/bornholm/go-webdav"

func Middleware(store Store) webdav.Middleware {
	return func(next webdav.FileSystem) webdav.FileSystem {
		return NewFileSystem(next, store)
	}
}
