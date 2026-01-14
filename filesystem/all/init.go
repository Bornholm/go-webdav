package all

import (
	_ "github.com/bornholm/go-webdav/filesystem/local"
	_ "github.com/bornholm/go-webdav/filesystem/s3"
	_ "github.com/bornholm/go-webdav/filesystem/sqlite"
)
