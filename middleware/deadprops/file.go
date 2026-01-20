package deadprops

import (
	"encoding/xml"
	"io/fs"

	"golang.org/x/net/webdav"
)

type File struct {
	name  string
	file  webdav.File
	store Store
}

// DeadProps implements webdav.DeadPropsHolder.
func (f *File) DeadProps() (map[xml.Name]webdav.Property, error) {
	return f.store.Get(f.name)
}

// Patch implements webdav.DeadPropsHolder.
func (f *File) Patch(patches []webdav.Proppatch) ([]webdav.Propstat, error) {
	return f.store.Patch(f.name, patches)
}

// Close implements webdav.File.
func (f *File) Close() error {
	return f.file.Close()
}

// Read implements webdav.File.
func (f *File) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

// Readdir implements webdav.File.
func (f *File) Readdir(count int) ([]fs.FileInfo, error) {
	return f.file.Readdir(count)
}

// Seek implements webdav.File.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

// Stat implements webdav.File.
func (f *File) Stat() (fs.FileInfo, error) {
	return f.file.Stat()
}

// Write implements webdav.File.
func (f *File) Write(p []byte) (n int, err error) {
	return f.file.Write(p)
}

var (
	_ webdav.File            = &File{}
	_ webdav.DeadPropsHolder = &File{}
)
