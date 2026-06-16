package menu

import (
	"os"
	"path/filepath"
	"testing"
)

func sampleTree() []Item {
	return []Item{
		{ID: "announcements", Label: "📢", Action: "announcements"},
		{ID: "refs", Label: "📚 المراجع", Children: []Item{
			{ID: "search", Label: "🔍", Action: "search"},
			{ID: "cite", Label: "📝", Action: "cite"},
		}},
	}
}

func TestAdd_TopLevel(t *testing.T) {
	tree, err := add(sampleTree(), RootID, Item{ID: "help", Label: "ℹ️", Action: "help"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, ok := find(tree, "help"); !ok {
		t.Error("help should be added at top level")
	}
	if len(tree) != 3 {
		t.Errorf("top-level len = %d, want 3", len(tree))
	}
}

func TestAdd_UnderSubmenu(t *testing.T) {
	tree, err := add(sampleTree(), "refs", Item{ID: "journal", Label: "🛡️", Action: "journal"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	refs, _ := find(tree, "refs")
	if len(refs.Children) != 3 {
		t.Errorf("refs children = %d, want 3", len(refs.Children))
	}
}

func TestAdd_Errors(t *testing.T) {
	if _, err := add(sampleTree(), "refs", Item{ID: "search", Label: "x", Action: "search"}); err != ErrDuplicateID {
		t.Errorf("duplicate id => %v, want ErrDuplicateID", err)
	}
	if _, err := add(sampleTree(), "missing", Item{ID: "z", Label: "z", Action: "a"}); err != ErrParentNotFound {
		t.Errorf("missing parent => %v, want ErrParentNotFound", err)
	}
	if _, err := add(sampleTree(), "announcements", Item{ID: "z", Label: "z", Action: "a"}); err != ErrParentIsAction {
		t.Errorf("action parent => %v, want ErrParentIsAction", err)
	}
}

func TestAdd_DoesNotMutateInput(t *testing.T) {
	orig := sampleTree()
	if _, err := add(orig, "refs", Item{ID: "journal", Label: "🛡️", Action: "journal"}); err != nil {
		t.Fatal(err)
	}
	refs, _ := find(orig, "refs")
	if len(refs.Children) != 2 {
		t.Error("add must not mutate the original tree")
	}
}

func TestRemove(t *testing.T) {
	tree, ok := remove(sampleTree(), "cite")
	if !ok {
		t.Fatal("cite should be removed")
	}
	if _, found := find(tree, "cite"); found {
		t.Error("cite still present after remove")
	}
	// Removing a submenu drops its subtree.
	tree2, ok := remove(sampleTree(), "refs")
	if !ok {
		t.Fatal("refs should be removed")
	}
	if _, found := find(tree2, "search"); found {
		t.Error("removing submenu should drop its children")
	}
	if _, ok := remove(sampleTree(), "nope"); ok {
		t.Error("removing a missing id should report false")
	}
}

func TestManager_AddRemovePersists(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, sampleTree())

	if err := m.Add("refs", Item{ID: "journal", Label: "🛡️", Action: "journal"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "menu.yaml")); err != nil {
		t.Errorf("menu.yaml should be written on edit: %v", err)
	}

	// Reload from disk and confirm the change survived.
	reloaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := reloaded.Find("journal"); !ok {
		t.Error("persisted journal button not found after reload")
	}

	if err := m.Remove("journal"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	reloaded2, _ := Load(dir)
	if _, ok := reloaded2.Find("journal"); ok {
		t.Error("journal should be gone after remove+reload")
	}
}

func TestManager_Children(t *testing.T) {
	m := NewManager(t.TempDir(), sampleTree())
	root, ok := m.Children(RootID)
	if !ok || len(root) != 2 {
		t.Errorf("root children = %d, ok=%v", len(root), ok)
	}
	kids, ok := m.Children("refs")
	if !ok || len(kids) != 2 {
		t.Errorf("refs children = %d, ok=%v", len(kids), ok)
	}
	if _, ok := m.Children("search"); ok {
		t.Error("an action leaf is not a navigable container")
	}
}

func TestLoad_DefaultWhenMissing(t *testing.T) {
	m, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Root()) != len(DefaultTree()) {
		t.Errorf("missing file should yield default tree")
	}
}
