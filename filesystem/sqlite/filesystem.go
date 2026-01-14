package sqlite

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitemigration"
	"zombiezen.com/go/sqlite/sqlitex"
)

type FileSystem struct {
	pool *sqlitemigration.Pool
}

// fileInfo implements os.FileInfo for a file or directory in the SQLite filesystem
type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *fileInfo) Name() string       { return path.Base(fi.name) }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) IsDir() bool        { return fi.isDir }
func (fi *fileInfo) Sys() interface{}   { return nil }

// Mkdir implements webdav.FileSystem.
func (f *FileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	name = cleanPath(name)

	// Check if parent directory exists
	parent := path.Dir(name)
	if parent != "/" && parent != "." {
		_, err := f.Stat(ctx, parent)
		if err != nil {
			if os.IsNotExist(err) {
				return os.ErrNotExist
			}
			return err
		}
	}

	// Check if path already exists
	_, err := f.Stat(ctx, name)
	if err == nil {
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}

	// Create the directory
	conn, err := f.pool.Take(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.pool.Put(conn)

	err = sqlitex.Execute(conn, `
		INSERT INTO files (path, is_dir, mode, size, mtime)
		VALUES (?, 1, ?, 0, ?)
	`, &sqlitex.ExecOptions{
		Args: []interface{}{name, uint32(perm), time.Now().Unix()},
	})

	return errors.WithStack(err)
}

// OpenFile implements webdav.FileSystem.
func (f *FileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	name = cleanPath(name)

	// Check if the file exists
	info, err := f.Stat(ctx, name)
	if err != nil {
		if os.IsNotExist(err) {
			// If file doesn't exist and we're not creating it, return error
			if flag&os.O_CREATE == 0 {
				return nil, os.ErrNotExist
			}

			// Ensure parent directory exists
			parent := path.Dir(name)
			if parent != "/" && parent != "." {
				_, err := f.Stat(ctx, parent)
				if err != nil {
					if os.IsNotExist(err) {
						return nil, os.ErrNotExist
					}
					return nil, err
				}
			}

			// Create new file
			conn, err := f.pool.Take(ctx)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			defer f.pool.Put(conn)

			err = sqlitex.Execute(conn, `
				INSERT INTO files (path, is_dir, mode, size, mtime)
				VALUES (?, 0, ?, 0, ?)
			`, &sqlitex.ExecOptions{
				Args: []interface{}{name, uint32(perm), time.Now().Unix()},
			})
			if err != nil {
				return nil, errors.WithStack(err)
			}

			// Create empty content
			err = sqlitex.Execute(conn, `
				INSERT INTO file_contents (path, content)
				VALUES (?, zeroblob(0))
			`, &sqlitex.ExecOptions{
				Args: []interface{}{name},
			})
			if err != nil {
				return nil, errors.WithStack(err)
			}

			// Get new file info
			info, err = f.Stat(ctx, name)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Check if trying to open a directory with write flags
	if info.IsDir() && flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		return nil, errors.New("cannot write to directory")
	}

	// Create file object
	file := &File{
		ctx:     ctx,
		fs:      f,
		name:    name,
		flag:    flag,
		isDir:   info.IsDir(),
		size:    info.Size(),
		modTime: info.ModTime(),
		mode:    info.Mode(),
		offset:  0,
	}

	// If it's a file and we need to truncate it
	if !file.isDir && flag&os.O_TRUNC != 0 {
		err = file.truncate()
		if err != nil {
			return nil, err
		}
	}

	return file, nil
}

// RemoveAll implements webdav.FileSystem.
func (f *FileSystem) RemoveAll(ctx context.Context, name string) error {
	name = cleanPath(name)

	// Check if path exists
	info, err := f.Stat(ctx, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	conn, err := f.pool.Take(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.pool.Put(conn)

	// Use a transaction to make sure all operations are atomic
	removeErr := func() error {
		// Use Save for automatic transaction management
		defer sqlitex.Save(conn)(&err)

		if info.IsDir() {
			// For directories, we need to remove all children as well
			err = sqlitex.Execute(conn, `
				DELETE FROM files 
				WHERE path = ? OR path LIKE ?
			`, &sqlitex.ExecOptions{
				Args: []interface{}{name, strings.TrimSuffix(name, "/") + "/" + "%"},
			})
			if err != nil {
				return errors.WithStack(err)
			}

			// Also remove any associated file contents
			err = sqlitex.Execute(conn, `
				DELETE FROM file_contents
				WHERE path = ? OR path LIKE ?
			`, &sqlitex.ExecOptions{
				Args: []interface{}{name, strings.TrimSuffix(name, "/") + "/" + "%"},
			})
			if err != nil {
				return errors.WithStack(err)
			}
		} else {
			// For files, remove from both tables

			// Delete from file_contents
			err = sqlitex.Execute(conn, `
				DELETE FROM file_contents
				WHERE path = ?
			`, &sqlitex.ExecOptions{
				Args: []interface{}{name},
			})
			if err != nil {
				return errors.WithStack(err)
			}

			// Delete from files
			err = sqlitex.Execute(conn, `
				DELETE FROM files
				WHERE path = ?
			`, &sqlitex.ExecOptions{
				Args: []interface{}{name},
			})
			if err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	}()

	return removeErr
}

// Rename implements webdav.FileSystem.
func (f *FileSystem) Rename(ctx context.Context, oldName string, newName string) error {
	oldName = cleanPath(oldName)
	newName = cleanPath(newName)

	// Check if old path exists
	oldInfo, err := f.Stat(ctx, oldName)
	if err != nil {
		return err
	}

	// Check if new path exists
	_, err = f.Stat(ctx, newName)
	if err == nil {
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}

	// Ensure parent of new path exists
	newParent := path.Dir(newName)
	if newParent != "/" && newParent != "." {
		_, err := f.Stat(ctx, newParent)
		if err != nil {
			if os.IsNotExist(err) {
				return os.ErrNotExist
			}
			return err
		}
	}

	conn, err := f.pool.Take(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.pool.Put(conn)

	renameErr := func() error {
		// Use Save for automatic transaction management
		defer sqlitex.Save(conn)(&err)

		if oldInfo.IsDir() {
			// For directories, we need to update the path for all children as well

			// Update the directory itself
			err = sqlitex.Execute(conn, `
				UPDATE files 
				SET path = ?
				WHERE path = ?
			`, &sqlitex.ExecOptions{
				Args: []interface{}{newName, oldName},
			})
			if err != nil {
				return errors.WithStack(err)
			}

			// Find all children and update their paths
			stmt, err := conn.Prepare(`SELECT path FROM files WHERE path LIKE ?`)
			if err != nil {
				return errors.WithStack(err)
			}
			defer stmt.Finalize()

			stmt.BindText(1, oldName+"/"+"%")

			var childPaths []string
			for {
				if hasRow, err := stmt.Step(); err != nil {
					return errors.WithStack(err)
				} else if !hasRow {
					break
				}

				childPaths = append(childPaths, stmt.ColumnText(0))
			}

			for _, oldPath := range childPaths {
				newPath := strings.Replace(oldPath, oldName, newName, 1)

				err = sqlitex.Execute(conn, `
					UPDATE files 
					SET path = ?
					WHERE path = ?
				`, &sqlitex.ExecOptions{
					Args: []interface{}{newPath, oldPath},
				})
				if err != nil {
					return errors.WithStack(err)
				}

				// Update file contents if needed
				err = sqlitex.Execute(conn, `
					UPDATE file_contents 
					SET path = ?
					WHERE path = ?
				`, &sqlitex.ExecOptions{
					Args: []interface{}{newPath, oldPath},
				})
				if err != nil {
					return errors.WithStack(err)
				}
			}
		} else {
			// For files, just update the specific path

			// Update file metadata
			err = sqlitex.Execute(conn, `
				UPDATE files 
				SET path = ?
				WHERE path = ?
			`, &sqlitex.ExecOptions{
				Args: []interface{}{newName, oldName},
			})
			if err != nil {
				return errors.WithStack(err)
			}

			// Update file content
			err = sqlitex.Execute(conn, `
				UPDATE file_contents 
				SET path = ?
				WHERE path = ?
			`, &sqlitex.ExecOptions{
				Args: []interface{}{newName, oldName},
			})
			if err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	}()

	return renameErr
}

// Stat implements webdav.FileSystem.
func (f *FileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	name = cleanPath(name)

	conn, err := f.pool.Take(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer f.pool.Put(conn)

	var isDir int
	var mode uint32
	var size int64
	var mtime int64
	var found bool

	err = sqlitex.Execute(conn, `
		SELECT is_dir, mode, size, mtime FROM files 
		WHERE path = ?
	`, &sqlitex.ExecOptions{
		Args: []interface{}{name},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			isDir = stmt.ColumnInt(0)
			mode = uint32(stmt.ColumnInt64(1))
			size = stmt.ColumnInt64(2)
			mtime = stmt.ColumnInt64(3)
			found = true
			return nil
		},
	})

	if err != nil {
		return nil, errors.WithStack(err)
	}

	// If no rows were returned, the file doesn't exist
	if !found {
		return nil, os.ErrNotExist
	}

	info := &fileInfo{
		name:    name,
		size:    size,
		mode:    os.FileMode(mode),
		modTime: time.Unix(mtime, 0),
		isDir:   isDir == 1,
	}

	return info, nil
}

// Helper function to clean and normalize paths
func cleanPath(name string) string {
	if name == "" {
		return "/"
	}

	// Handle relative paths by cleaning them first
	name = filepath.ToSlash(filepath.Clean(name))

	// For paths like "./dir" or "../dir", Clean will remove the leading "./" or "../"
	// but we still need to make sure it starts with "/"
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}

	return name
}

func NewFileSystem(dbPath string) *FileSystem {
	schema := sqlitemigration.Schema{
		Migrations: []string{
			`CREATE TABLE IF NOT EXISTS files (
					path TEXT PRIMARY KEY,     -- File path (used as unique identifier)
					is_dir INTEGER NOT NULL,   -- 1 if directory, 0 if file
					mode INTEGER NOT NULL,     -- File permissions
					size INTEGER NOT NULL,     -- File size in bytes (0 for directories)
					mtime INTEGER NOT NULL     -- Modification time (Unix timestamp)
				);
			`,
			`CREATE INDEX IF NOT EXISTS idx_parent_path ON files(path);`,
			`CREATE TABLE IF NOT EXISTS file_contents (
					path TEXT PRIMARY KEY REFERENCES files(path) ON DELETE CASCADE,
					content BLOB              -- File content
				);
			`,
		},
		RepeatableMigration: fmt.Sprintf(`INSERT OR IGNORE INTO files (path, is_dir, mode, size, mtime) VALUES ('/', 1, 493, 0, %d)`, time.Now().Unix()),
	}

	pool := sqlitemigration.NewPool(dbPath, schema, sqlitemigration.Options{
		Flags: sqlite.OpenCreate | sqlite.OpenReadWrite | sqlite.OpenWAL,
		PrepareConn: func(conn *sqlite.Conn) error {
			return sqlitex.ExecScript(conn, `PRAGMA foreign_keys = ON; PRAGMA auto_vacuum=FULL; PRAGMA busy_timeout = 5000;`)
		},
		OnError: func(e error) {
			log.Printf("%+v", e)
		},
	})

	return &FileSystem{
		pool: pool,
	}
}

var _ webdav.FileSystem = &FileSystem{}
