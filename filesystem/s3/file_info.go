package s3

import (
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

// FileInfo adapts minio.ObjectInfo to os.FileInfo
type FileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (fi *FileInfo) Name() string { return fi.name }
func (fi *FileInfo) Size() int64  { return fi.size }
func (fi *FileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (fi *FileInfo) ModTime() time.Time { return fi.modTime }
func (fi *FileInfo) IsDir() bool        { return fi.isDir }
func (fi *FileInfo) Sys() any           { return nil }

// convObjectInfo converts a minio object to an fs.FileInfo
// dirPath is required to strip the prefix from the object name provided by S3
func convObjectInfo(info minio.ObjectInfo, dirPath string) *FileInfo {
	// S3 returns full paths (e.g., "photos/vacation/img.jpg").
	// Readdir expects relative names (e.g., "img.jpg").
	name := info.Key

	// If it's a "CommonPrefix" (virtual directory), S3 returns "photos/vacation/"
	// We need to trim the suffix for clean display, but keep track it's a dir
	isDir := strings.HasSuffix(name, "/")

	// Get the base name relative to the parent directory
	// path.Base will strip trailing slashes, which is what we want for Name()
	baseName := path.Base(name)

	// Handle the edge case where S3 returns the directory itself as an object
	if isDir && name == dirPath {
		baseName = "."
	}

	return &FileInfo{
		name:    baseName,
		size:    info.Size,
		modTime: info.LastModified,
		isDir:   isDir,
	}
}
