package cache

import (
	"context"
	"os"
	"time"

	"github.com/bornholm/go-webdav/syncx"
)

type cachedEntry struct {
	info   os.FileInfo
	expiry time.Time
}

type cachedChildren struct {
	children []os.FileInfo
	expiry   time.Time
}

type MemoryStore struct {
	ttl      time.Duration
	items    syncx.Map[string, cachedEntry]
	children syncx.Map[string, cachedChildren]
}

func NewMemoryStore(ttl time.Duration) *MemoryStore {
	m := &MemoryStore{
		ttl: ttl,
	}
	go m.garbageCollect()
	return m
}

// --- Item Implementation ---

func (m *MemoryStore) Get(ctx context.Context, path string) (os.FileInfo, bool, error) {
	entry, ok := m.items.Load(path)
	if !ok {
		return nil, false, nil
	}
	if time.Now().After(entry.expiry) {
		m.items.Delete(path)
		return nil, false, nil
	}

	return entry.info, true, nil
}

func (m *MemoryStore) Put(ctx context.Context, path string, info os.FileInfo) error {
	m.items.Store(path, cachedEntry{
		info:   info,
		expiry: time.Now().Add(m.ttl),
	})
	return nil
}

func (m *MemoryStore) Invalidate(ctx context.Context, path string) error {
	m.items.Delete(path)
	return nil
}

// --- Children Implementation ---

func (m *MemoryStore) GetChildren(ctx context.Context, path string) ([]os.FileInfo, bool, error) {
	entry, ok := m.children.Load(path)
	if !ok {
		return nil, false, nil
	}
	if time.Now().After(entry.expiry) {
		m.children.Delete(path)
		return nil, false, nil
	}
	return entry.children, true, nil
}

func (m *MemoryStore) PutChildren(ctx context.Context, path string, children []os.FileInfo) error {
	m.children.Store(path, cachedChildren{
		children: children,
		expiry:   time.Now().Add(m.ttl),
	})
	return nil
}

func (m *MemoryStore) InvalidateChildren(ctx context.Context, path string) error {
	m.children.Delete(path)
	return nil
}

func (m *MemoryStore) garbageCollect() {
	ticker := time.NewTicker(m.ttl * 2)
	for range ticker.C {
		now := time.Now()
		// Clean items
		m.items.Range(func(key string, val cachedEntry) bool {
			if now.After(val.expiry) {
				m.items.Delete(key)
			}
			return true
		})
		// Clean children
		m.children.Range(func(key string, val cachedChildren) bool {
			if now.After(val.expiry) {
				m.children.Delete(key)
			}
			return true
		})
	}
}

var _ Store = &MemoryStore{}
