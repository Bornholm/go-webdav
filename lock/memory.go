package lock

import (
	"sync"

	"github.com/pkg/errors"
)

type MemoryStore struct {
	mu    sync.RWMutex
	locks map[string]LockNode        // map[token]lockNode
	paths map[string]map[string]bool // map[path]map[token]bool (index)
}

// GetLock implements [Store].
func (m *MemoryStore) GetLock(token string) (*LockNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.locks[token]
	if !ok {
		return nil, errors.WithStack(ErrLockNotFound)
	}

	return &node, nil
}

// GetLocksByPath implements [Store].
func (m *MemoryStore) GetLocksByPath(path string) ([]*LockNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*LockNode

	// Check exact path match
	if tokens, ok := m.paths[path]; ok {
		for token := range tokens {
			if node, ok := m.locks[token]; ok {
				nodeCopy := node
				result = append(result, &nodeCopy)
			}
		}
	}

	// Check parent paths for depth-infinity locks
	for token, node := range m.locks {
		if node.Details.ZeroDepth {
			continue // Skip zero-depth locks
		}

		root := node.Details.Root
		if root == path {
			continue // Already added above
		}

		// Check if path is a child of the lock root
		if isChildOf(path, root) {
			// Check if we already have this token
			found := false
			for _, existing := range result {
				if existing.Token == token {
					found = true
					break
				}
			}
			if !found {
				nodeCopy := node
				result = append(result, &nodeCopy)
			}
		}
	}

	return result, nil
}

// ApplyLock implements [Store].
func (m *MemoryStore) ApplyLock(node *LockNode, paths ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.locks[node.Token] = *node

	for _, p := range paths {
		if m.paths[p] == nil {
			m.paths[p] = make(map[string]bool)
		}
		m.paths[p][node.Token] = true
	}

	return nil
}

// RemoveLock implements [Store].
func (m *MemoryStore) RemoveLock(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.locks[token]
	if !ok {
		return ErrLockNotFound
	}

	delete(m.locks, token)

	root := node.Details.Root
	if tokens, ok := m.paths[root]; ok {
		delete(tokens, token)
		if len(tokens) == 0 {
			delete(m.paths, root)
		}
	}

	return nil
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		locks: make(map[string]LockNode),
		paths: make(map[string]map[string]bool),
	}
}

var _ Store = &MemoryStore{}
