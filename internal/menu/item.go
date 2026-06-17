// Package menu models the bot's button menu as an editable hierarchy. The tree
// is loaded from data/menu.yaml and can be edited at runtime by admins (add /
// remove buttons). A later web admin edits the same file.
package menu

import (
	"errors"
	"fmt"
	"strings"
)

// RootID is the synthetic parent id for top-level buttons.
const RootID = "root"

var (
	// ErrParentNotFound is returned when an add targets a missing parent.
	ErrParentNotFound = errors.New("menu: parent not found")
	// ErrParentIsAction is returned when adding under an action (leaf) button.
	ErrParentIsAction = errors.New("menu: cannot add under an action button")
	// ErrNotFound is returned when an item id does not exist.
	ErrNotFound = errors.New("menu: item not found")
	// ErrDuplicateID is returned when an id already exists.
	ErrDuplicateID = errors.New("menu: duplicate id")
)

// Item is one menu button. A button is one of three kinds:
//   - feature leaf  (Action set)        — runs a built-in bot feature
//   - link leaf     (URL set)           — opens an external URL
//   - submenu       (Action & URL empty) — contains Children
type Item struct {
	ID       string `yaml:"id"`
	Label    string `yaml:"label"`
	Action   string `yaml:"action,omitempty"`
	URL      string `yaml:"url,omitempty"`
	Children []Item `yaml:"children,omitempty"`
}

// IsSubmenu reports whether the item opens a submenu (not a feature or a link).
func (i Item) IsSubmenu() bool { return i.Action == "" && i.URL == "" }

// IsLink reports whether the item opens an external URL.
func (i Item) IsLink() bool { return i.URL != "" }

// clone deep-copies an item so callers never share backing slices.
func clone(items []Item) []Item {
	if items == nil {
		return nil
	}
	out := make([]Item, len(items))
	for idx, it := range items {
		it.Children = clone(it.Children)
		out[idx] = it
	}
	return out
}

// find returns a copy of the item with the given id, searching the whole tree.
func find(items []Item, id string) (Item, bool) {
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
		if child, ok := find(it.Children, id); ok {
			return child, true
		}
	}
	return Item{}, false
}

// collectIDs appends every id in the tree to dst.
func collectIDs(items []Item, dst map[string]bool) {
	for _, it := range items {
		dst[it.ID] = true
		collectIDs(it.Children, dst)
	}
}

// add inserts child under parentID (RootID/"" => top level), returning a new
// tree. It validates parent existence, parent type, and id uniqueness.
func add(items []Item, parentID string, child Item) ([]Item, error) {
	child.ID = strings.TrimSpace(child.ID)
	if child.ID == "" || strings.TrimSpace(child.Label) == "" {
		return nil, fmt.Errorf("menu: item id and label are required")
	}
	existing := map[string]bool{}
	collectIDs(items, existing)
	if existing[child.ID] {
		return nil, ErrDuplicateID
	}

	tree := clone(items)
	if parentID == "" || parentID == RootID {
		return append(tree, child), nil
	}

	parent, ok := find(tree, parentID)
	if !ok {
		return nil, ErrParentNotFound
	}
	if !parent.IsSubmenu() {
		return nil, ErrParentIsAction
	}
	insertUnder(tree, parentID, child)
	return tree, nil
}

// insertUnder appends child to the parent's Children in place; reports success.
func insertUnder(items []Item, parentID string, child Item) bool {
	for idx := range items {
		if items[idx].ID == parentID {
			items[idx].Children = append(items[idx].Children, child)
			return true
		}
		if insertUnder(items[idx].Children, parentID, child) {
			return true
		}
	}
	return false
}

// remove deletes the item with the given id (and its subtree), returning a new
// tree and whether anything was removed.
func remove(items []Item, id string) ([]Item, bool) {
	out := make([]Item, 0, len(items))
	removed := false
	for _, it := range items {
		if it.ID == id {
			removed = true
			continue
		}
		newChildren, childRemoved := remove(it.Children, id)
		if childRemoved {
			removed = true
		}
		it.Children = newChildren
		out = append(out, it)
	}
	return out, removed
}
