package webdav

type Middleware func(next FileSystem) FileSystem

func Chain(fs FileSystem, middlewares ...Middleware) FileSystem {
	for i := len(middlewares) - 1; i >= 0; i-- {
		fs = middlewares[i](fs)
	}

	return fs
}
