package menu

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"gopkg.in/yaml.v3"
)

const fileName = "menu.yaml"

// Manager holds the live menu tree with concurrency-safe edits and persistence.
type Manager struct {
	mu      sync.RWMutex
	dataDir string
	root    []Item
}

// NewManager builds a Manager from an in-memory tree (used in tests).
func NewManager(dataDir string, root []Item) *Manager {
	return &Manager{dataDir: dataDir, root: clone(root)}
}

type fileShape struct {
	Menu []Item `yaml:"menu"`
}

// Load reads data/menu.yaml, falling back to DefaultTree when the file is
// absent or empty. The file is created lazily on the first edit.
func Load(dataDir string) (*Manager, error) {
	path := filepath.Join(dataDir, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewManager(dataDir, DefaultTree()), nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc fileShape
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Menu) == 0 {
		return NewManager(dataDir, DefaultTree()), nil
	}
	return NewManager(dataDir, doc.Menu), nil
}

// Root returns a copy of the top-level items.
func (m *Manager) Root() []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return clone(m.root)
}

// Find returns a copy of the item with the given id.
func (m *Manager) Find(id string) (Item, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return find(m.root, id)
}

// GenID returns base, or base-N, ensuring the id is unique in the tree.
func (m *Manager) GenID(base string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := find(m.root, base); !ok {
		return base
	}
	for i := 2; ; i++ {
		cand := base + "-" + strconv.Itoa(i)
		if _, ok := find(m.root, cand); !ok {
			return cand
		}
	}
}

// Children returns the child items of a submenu (or the root for RootID/""),
// and whether id refers to a navigable container.
func (m *Manager) Children(id string) ([]Item, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id == "" || id == RootID {
		return clone(m.root), true
	}
	it, ok := find(m.root, id)
	if !ok || !it.IsSubmenu() {
		return nil, false
	}
	return clone(it.Children), true
}

// Add inserts a button under parentID and persists the tree.
func (m *Manager) Add(parentID string, item Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next, err := add(m.root, parentID, item)
	if err != nil {
		return err
	}
	m.root = next
	return m.save()
}

// Remove deletes a button (and its subtree) and persists the tree.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next, ok := remove(m.root, id)
	if !ok {
		return ErrNotFound
	}
	m.root = next
	return m.save()
}

// save writes the tree to data/menu.yaml. Caller holds the write lock.
func (m *Manager) save() error {
	data, err := yaml.Marshal(fileShape{Menu: m.root})
	if err != nil {
		return fmt.Errorf("marshal menu: %w", err)
	}
	path := filepath.Join(m.dataDir, fileName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
