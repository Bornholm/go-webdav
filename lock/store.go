package lock

import "errors"

var (
	ErrLockNotFound = errors.New("lock not found")
)

type Store interface {
	GetLock(token string) (*LockNode, error)
	GetLocksByPath(path string) ([]*LockNode, error)
	ApplyLock(node *LockNode, paths ...string) error
	RemoveLock(token string) error
}
