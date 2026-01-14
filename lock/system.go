package lock

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// System implements webdav.LockSystem.
// It is safe for concurrent use by multiple goroutines.
type System struct {
	store Store
}

type LockNode struct {
	Details webdav.LockDetails
	Token   string
	Expiry  time.Time
}

// NewSystem creates a new lock system.
func NewSystem(store Store) webdav.LockSystem {
	return &System{
		store: store,
	}
}

// Confirm verifies that the given conditions allow access to the named resource.
// It returns a release function if the access is granted.
func (s *System) Confirm(now time.Time, name0, name1 string, conditions ...webdav.Condition) (func(), error) {
	name0 = normalizePath(name0)
	name1 = normalizePath(name1)

	// Collect all paths we need to check
	var paths []string
	if name0 != "" {
		paths = append(paths, name0)
	}
	if name1 != "" && name1 != name0 {
		paths = append(paths, name1)
	}

	// Get all locks affecting our paths
	affectingLocks := make(map[string]*LockNode)
	for _, path := range paths {
		locks, err := s.store.GetLocksByPath(path)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, lock := range locks {
			if !lock.Expiry.IsZero() && now.After(lock.Expiry) {
				_ = s.store.RemoveLock(lock.Token)
				continue
			}
			affectingLocks[lock.Token] = lock
		}
	}

	// No conditions provided
	if len(conditions) == 0 {
		if len(affectingLocks) > 0 {
			return nil, webdav.ErrLocked
		}
		return func() {}, nil
	}

	// Track satisfied locks and condition results
	satisfiedLocks := make(map[string]bool)
	hasAnyPositiveToken := false
	anyPositiveTokenSucceeded := false

	for _, cond := range conditions {
		token := cond.Token
		if token == "" {
			continue
		}

		token = strings.TrimPrefix(token, "<")
		token = strings.TrimSuffix(token, ">")

		node, err := s.store.GetLock(token)
		lockExists := err == nil

		if lockExists && !node.Expiry.IsZero() && now.After(node.Expiry) {
			_ = s.store.RemoveLock(token)
			lockExists = false
			node = nil
		}

		if cond.Not {
			// "Not <token>" succeeds if lock doesn't exist or doesn't cover path
			if lockExists {
				for _, path := range paths {
					if s.lockCoversPath(node, path) {
						return nil, webdav.ErrLocked
					}
				}
			}
			// Not condition passed - but doesn't satisfy any lock requirement
		} else {
			// Positive token condition
			hasAnyPositiveToken = true

			if !lockExists {
				// This specific condition failed - token doesn't exist
				continue
			}

			// Check if lock applies to our paths
			for _, path := range paths {
				if s.lockCoversPath(node, path) {
					satisfiedLocks[token] = true
					anyPositiveTokenSucceeded = true
					break
				}
			}
		}
	}

	// If we had positive token conditions and NONE succeeded, fail
	if hasAnyPositiveToken && !anyPositiveTokenSucceeded {
		return nil, webdav.ErrNoSuchLock
	}

	// Ensure ALL affecting locks are satisfied
	for token := range affectingLocks {
		if !satisfiedLocks[token] {
			return nil, webdav.ErrLocked
		}
	}

	return func() {}, nil
}

// lockCoversPath checks if a lock covers the given path
func (s *System) lockCoversPath(node *LockNode, path string) bool {
	root := normalizePath(node.Details.Root)
	path = normalizePath(path)

	if root == path {
		return true
	}

	// For depth-infinity locks, check if path is a child
	if !node.Details.ZeroDepth {
		return isChildOf(path, root)
	}

	return false
}

// Create creates a new lock.
func (s *System) Create(now time.Time, details webdav.LockDetails) (string, error) {
	details.Root = normalizePath(details.Root)

	// Check for conflicts with existing locks
	existingLocks, err := s.store.GetLocksByPath(details.Root)
	if err != nil {
		return "", errors.WithStack(err)
	}

	for _, existing := range existingLocks {
		// Remove expired locks
		if !existing.Expiry.IsZero() && now.After(existing.Expiry) {
			_ = s.store.RemoveLock(existing.Token)
			continue
		}
		// Any existing lock conflicts with a new lock request
		return "", webdav.ErrLocked
	}

	// Generate token
	token, err := generateUUID()
	if err != nil {
		return "", errors.WithStack(err)
	}

	// Calculate expiry
	var expiry time.Time
	if details.Duration > 0 {
		expiry = now.Add(details.Duration)
	}

	node := &LockNode{
		Details: details,
		Token:   token,
		Expiry:  expiry,
	}

	if err := s.store.ApplyLock(node, details.Root); err != nil {
		return "", errors.WithStack(err)
	}

	return token, nil
}

// Refresh refreshes an existing lock.
func (s *System) Refresh(now time.Time, token string, duration time.Duration) (webdav.LockDetails, error) {
	// Strip angle brackets if present
	token = strings.TrimPrefix(token, "<")
	token = strings.TrimSuffix(token, ">")

	node, err := s.store.GetLock(token)
	if err != nil {
		if errors.Is(err, ErrLockNotFound) {
			return webdav.LockDetails{}, webdav.ErrNoSuchLock
		}
		return webdav.LockDetails{}, errors.WithStack(err)
	}

	// Check if already expired
	if !node.Expiry.IsZero() && now.After(node.Expiry) {
		_ = s.store.RemoveLock(token)
		return webdav.LockDetails{}, webdav.ErrNoSuchLock
	}

	// Update expiry
	node.Details.Duration = duration
	if duration > 0 {
		node.Expiry = now.Add(duration)
	} else {
		node.Expiry = time.Time{}
	}

	if err := s.store.ApplyLock(node, node.Details.Root); err != nil {
		return webdav.LockDetails{}, errors.WithStack(err)
	}

	return node.Details, nil
}

// Unlock removes a lock.
func (s *System) Unlock(now time.Time, token string) error {
	// Strip angle brackets if present
	token = strings.TrimPrefix(token, "<")
	token = strings.TrimSuffix(token, ">")

	_, err := s.store.GetLock(token)
	if err != nil {
		if errors.Is(err, ErrLockNotFound) {
			return webdav.ErrNoSuchLock
		}
		return errors.WithStack(err)
	}

	if err := s.store.RemoveLock(token); err != nil {
		if errors.Is(err, ErrLockNotFound) {
			return webdav.ErrNoSuchLock
		}
		return errors.WithStack(err)
	}

	return nil
}

// normalizePath ensures consistent path format
func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	// Ensure leading slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Remove trailing slash (except for root)
	if path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

// isChildOf returns true if child is a descendant of parent
func isChildOf(child, parent string) bool {
	parent = strings.TrimSuffix(parent, "/")
	child = strings.TrimSuffix(child, "/")

	if parent == "" || parent == "/" {
		return true
	}

	return strings.HasPrefix(child, parent+"/")
}

func generateUUID() (string, error) {
	b := make([]byte, 16)

	if _, err := rand.Read(b); err != nil {
		return "", errors.WithStack(err)
	}

	// Set version (4) and variant (RFC 4122) bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

var _ webdav.LockSystem = &System{}
