package local

import "golang.org/x/net/webdav"

func NewFileSystem(dirPath string) webdav.FileSystem {
	return webdav.Dir(dirPath)
}
