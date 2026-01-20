package deadprops

import (
	"encoding/xml"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/net/webdav"
)

type Store interface {
	Get(filename string) (map[xml.Name]webdav.Property, error)
	Patch(filename string, patches []webdav.Proppatch) ([]webdav.Propstat, error)
	RemoveAll(filename string) error
	Rename(oldName, newName string) error
}

type MemStore struct {
	mu    sync.RWMutex
	props map[string]map[xml.Name]webdav.Property
}

// Rename implements DeadPropsStore.
func (m *MemStore) Rename(oldName string, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type moveOp struct {
		val    map[xml.Name]webdav.Property
		newKey string
	}
	var moves []moveOp
	var deletes []string

	oldPrefix := oldName
	if !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}

	for key, props := range m.props {
		if key == oldName {
			moves = append(moves, moveOp{val: props, newKey: newName})
			deletes = append(deletes, key)
		} else if strings.HasPrefix(key, oldPrefix) {
			suffix := strings.TrimPrefix(key, oldName)
			moves = append(moves, moveOp{val: props, newKey: newName + suffix})
			deletes = append(deletes, key)
		}
	}

	for _, mv := range moves {
		m.props[mv.newKey] = mv.val
	}
	for _, del := range deletes {
		delete(m.props, del)
	}

	return nil
}

// NewMemStore creates a new in-memory dead properties store.
func NewMemStore() *MemStore {
	return &MemStore{
		props: make(map[string]map[xml.Name]webdav.Property),
	}
}

// Get implements DeadPropsStore.
func (m *MemStore) Get(filename string) (map[xml.Name]webdav.Property, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	props, exists := m.props[filename]
	if !exists {
		return make(map[xml.Name]webdav.Property), nil
	}

	// Return a copy to avoid external modifications
	result := make(map[xml.Name]webdav.Property, len(props))
	for name, prop := range props {
		result[name] = prop
	}

	return result, nil
}

// Patch implements DeadPropsStore.
func (m *MemStore) Patch(filename string, patches []webdav.Proppatch) ([]webdav.Propstat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize props map for this file if it doesn't exist
	if m.props[filename] == nil {
		m.props[filename] = make(map[xml.Name]webdav.Property)
	}

	var propstats []webdav.Propstat

	for _, patch := range patches {
		var props []webdav.Property
		var status int

		if patch.Remove {
			// Remove properties
			for _, prop := range patch.Props {
				delete(m.props[filename], prop.XMLName)
				props = append(props, webdav.Property{XMLName: prop.XMLName})
			}
			status = http.StatusOK // OK
		} else {
			// Set properties
			for _, prop := range patch.Props {
				m.props[filename][prop.XMLName] = prop
				props = append(props, prop)
			}
			status = http.StatusOK // OK
		}

		if len(props) > 0 {
			propstats = append(propstats, webdav.Propstat{
				Props:  props,
				Status: status,
			})
		}
	}

	return propstats, nil
}

// RemoveAll implements DeadPropsStore.
func (m *MemStore) RemoveAll(filename string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove the exact filename
	delete(m.props, filename)

	// Remove all children files (files that start with filename + "/")
	prefix := filename
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for key := range m.props {
		if strings.HasPrefix(key, prefix) {
			delete(m.props, key)
		}
	}

	return nil
}

var _ Store = &MemStore{}
