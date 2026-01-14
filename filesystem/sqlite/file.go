package sqlite

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// File represents an open file in the SQLite filesystem
type File struct {
	ctx     context.Context
	fs      *FileSystem
	name    string
	flag    int
	isDir   bool
	size    int64
	modTime time.Time
	mode    os.FileMode
	offset  int64

	temp *os.File
}

// Close implements webdav.File.
func (f *File) Close() error {
	if f.temp != nil {
		if err := f.moveTempFileToBlob(); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// Read implements webdav.File.
func (f *File) Read(p []byte) (n int, err error) {
	if f.isDir {
		return 0, errors.New("cannot read from directory")
	}

	// Handle empty buffer request
	if len(p) == 0 {
		return 0, nil
	}

	// Check if we're at the end of the file
	if f.offset >= f.size {
		return 0, io.EOF
	}

	// Get a fresh connection for this read operation
	conn, err := f.fs.pool.Take(f.ctx)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	defer f.fs.pool.Put(conn)

	// Check if the file has content
	var hasContent bool
	err = sqlitex.Execute(conn, `
		SELECT 1 FROM file_contents WHERE path = ?
	`, &sqlitex.ExecOptions{
		Args: []interface{}{f.name},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			hasContent = true
			return nil
		},
	})
	if err != nil {
		return 0, errors.WithStack(err)
	}

	// No content to read
	if !hasContent {
		return 0, io.EOF
	}

	// Get the rowid for the blob
	var rowid int64
	var found bool
	err = sqlitex.Execute(conn, `
		SELECT rowid FROM file_contents WHERE path = ?
	`, &sqlitex.ExecOptions{
		Args: []interface{}{f.name},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			rowid = stmt.ColumnInt64(0)
			found = true
			return nil
		},
	})
	if err != nil {
		return 0, errors.WithStack(err)
	}

	if !found {
		return 0, io.EOF
	}

	// Calculate how much we can read
	toRead := int64(len(p))
	if f.offset+toRead > f.size {
		toRead = f.size - f.offset
	}

	// Nothing to read
	if toRead <= 0 {
		return 0, io.EOF
	}

	// Open the blob for reading within this operation
	blob, err := conn.OpenBlob("", "file_contents", "content", rowid, false)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	defer blob.Close()

	// Seek to the correct position
	if _, err := blob.Seek(f.offset, io.SeekStart); err != nil {
		return 0, errors.WithStack(err)
	}

	// Read directly into the buffer
	n, err = blob.Read(p[:toRead])
	if err != nil && err != io.EOF {
		return n, errors.WithStack(err)
	}

	// Update offset
	f.offset += int64(n)

	// Handle EOF condition
	if int64(n) < toRead {
		return n, io.EOF
	}

	return n, nil
}

// Seek implements webdav.File.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.isDir {
		return 0, errors.New("cannot seek in directory")
	}

	// Calculate new offset
	newOffset := int64(0)
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.offset + offset
	case io.SeekEnd:
		newOffset = f.size + offset
	default:
		return 0, errors.New("invalid whence")
	}

	// Check if new offset is valid
	if newOffset < 0 {
		return 0, errors.New("negative offset")
	}

	// Update offset
	f.offset = newOffset
	return f.offset, nil
}

// Readdir implements webdav.File.
func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	if !f.isDir {
		return nil, &os.PathError{
			Op:   "readdir",
			Path: f.name,
			Err:  syscall.ENOTDIR,
		}
	}

	// Get all children of this directory
	conn, err := f.fs.pool.Take(f.ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer f.fs.pool.Put(conn)

	var entries []os.FileInfo
	prefix := f.name
	if prefix != "/" {
		prefix += "/"
	}

	// Improved query to better filter direct children
	err = sqlitex.Execute(conn, `
		SELECT path, is_dir, mode, size, mtime FROM files 
		WHERE path LIKE ? AND path != ?
	`, &sqlitex.ExecOptions{
		Args: []interface{}{prefix + "%", f.name},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			path := stmt.ColumnText(0)

			// Skip entries that are not direct children
			relPath := strings.TrimPrefix(path, prefix)
			if relPath == "" || strings.Contains(relPath, "/") {
				return nil
			}

			info := &fileInfo{
				name:    path,
				isDir:   stmt.ColumnInt(1) == 1,
				mode:    os.FileMode(stmt.ColumnInt64(2)),
				size:    stmt.ColumnInt64(3),
				modTime: time.Unix(stmt.ColumnInt64(4), 0),
			}
			entries = append(entries, info)
			return nil
		},
	})

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return entries, nil
}

// Stat implements webdav.File.
func (f *File) Stat() (os.FileInfo, error) {
	return &fileInfo{
		name:    f.name,
		size:    f.size,
		mode:    f.mode,
		modTime: f.modTime,
		isDir:   f.isDir,
	}, nil
}

// Write implements webdav.File with a reliable streaming approach for large files.
func (f *File) Write(p []byte) (n int, err error) {
	if f.isDir {
		return 0, &os.PathError{
			Path: f.name,
			Op:   "write",
			Err:  syscall.EISDIR,
		}
	}

	if f.flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		return 0, os.ErrPermission
	}

	if f.temp == nil {
		tempDir, err := os.MkdirTemp("", "calli-*")
		if err != nil {
			return 0, errors.WithStack(err)
		}

		tempName := filepath.Join(tempDir, "file")

		temp, err := os.OpenFile(tempName, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return 0, errors.WithStack(err)
		}

		f.temp = temp
	}

	return f.temp.Write(p)
}

func (f *File) moveTempFileToBlob() error {
	defer func() {
		f.temp.Close()
		os.RemoveAll(f.temp.Name())
		f.temp = nil
	}()

	// Get a fresh connection for this read operation
	conn, err := f.fs.pool.Take(f.ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.fs.pool.Put(conn)

	err = withImmediate(conn, func() error {
		if err := f.temp.Close(); err != nil {
			return errors.WithStack(err)
		}

		file, err := os.Open(f.temp.Name())
		if err != nil {
			return errors.WithStack(err)
		}

		defer file.Close()

		var rowID int64

		err = sqlitex.Execute(conn, `
			SELECT fc.rowid
			FROM file_contents fc
			JOIN files f ON fc.path = f.path
			WHERE f.path = ?
		`, &sqlitex.ExecOptions{
			Args: []any{f.name},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				rowID = stmt.ColumnInt64(0)
				return nil
			},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		stat, err := file.Stat()
		if err != nil {
			return errors.WithStack(err)
		}

		err = sqlitex.Execute(conn, `
			UPDATE file_contents SET content = zeroblob(?) WHERE path = ?
		`, &sqlitex.ExecOptions{
			Args: []any{stat.Size(), f.name},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		err = sqlitex.Execute(conn, `
			UPDATE files SET size = ? WHERE path = ?
		`, &sqlitex.ExecOptions{
			Args: []any{stat.Size(), f.name},
		})
		if err != nil {
			return errors.WithStack(err)
		}

		blob, err := conn.OpenBlob("", "file_contents", "content", rowID, true)
		if err != nil {
			return errors.WithStack(err)
		}

		defer blob.Close()

		if _, err := io.Copy(blob, file); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// truncate truncates the file to zero size
func (f *File) truncate() error {
	// Get a connection
	conn, err := f.fs.pool.Take(f.ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.fs.pool.Put(conn)

	// Update file size to 0
	err = sqlitex.Execute(conn, `
		UPDATE files SET size = 0, mtime = ? WHERE path = ?
	`, &sqlitex.ExecOptions{
		Args: []interface{}{time.Now().Unix(), f.name},
	})
	if err != nil {
		return errors.WithStack(err)
	}

	// Replace content with empty blob
	err = sqlitex.Execute(conn, `
		UPDATE file_contents SET content = zeroblob(0) WHERE path = ?
	`, &sqlitex.ExecOptions{
		Args: []interface{}{f.name},
	})
	if err != nil {
		return errors.WithStack(err)
	}

	// Update file info
	f.size = 0
	f.modTime = time.Now()

	return nil
}

var _ webdav.File = &File{}

func withSave(conn *sqlite.Conn, fn func() error) (err error) {
	defer sqlitex.Save(conn)(&err)
	err = fn()
	return errors.WithStack(err)
}

func withImmediate(conn *sqlite.Conn, fn func() error) (err error) {
	end, err := sqlitex.ImmediateTransaction(conn)
	if err != nil {
		return errors.WithStack(err)
	}
	defer end(&err)

	err = fn()

	return err
}
