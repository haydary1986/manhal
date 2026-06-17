package announce

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrDuplicateID is returned when adding an announcement whose id already exists.
var ErrDuplicateID = errors.New("announce: duplicate id")

// ErrNotFound is returned when an announcement id does not exist.
var ErrNotFound = errors.New("announce: not found")

// Repo is a thread-safe, persisted collection of announcements. The bot reads it
// (List) while the admin web edits it (Add/Remove); edits are saved to
// data/announcements.yaml so they survive restarts.
type Repo struct {
	mu    sync.RWMutex
	path  string // empty => in-memory only (tests)
	items []Announcement
}

// NewRepo builds a Repo from a slice (used in tests and by the loader).
func NewRepo(items []Announcement) *Repo {
	return &Repo{items: items}
}

// fileShape mirrors the YAML document layout.
type fileShape struct {
	Announcements []Announcement `yaml:"announcements"`
}

// Load reads data/announcements.yaml, returning an empty Repo when the file is
// absent so the bot still starts on a fresh checkout. The returned Repo persists
// later edits back to the same file.
func Load(dataDir string) (*Repo, error) {
	path := filepath.Join(dataDir, "announcements.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Repo{path: path}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc fileShape
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &Repo{path: path, items: doc.Announcements}, nil
}

// save writes the current items to disk (no-op when path is empty). Callers hold
// the write lock.
func (r *Repo) save() error {
	if r.path == "" {
		return nil
	}
	data, err := yaml.Marshal(fileShape{Announcements: r.items})
	if err != nil {
		return fmt.Errorf("marshal announcements: %w", err)
	}
	if err := os.WriteFile(r.path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", r.path, err)
	}
	return nil
}

// Len reports how many announcements are stored.
func (r *Repo) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

// All returns a copy of every announcement, newest first (admin view).
func (r *Repo) All() []Announcement {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Announcement, len(r.items))
	copy(out, r.items)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].PostedAt.After(out[j].PostedAt)
	})
	return out
}

// Add appends a new announcement (unique id) and persists.
func (r *Repo) Add(a Announcement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, it := range r.items {
		if it.ID == a.ID {
			return ErrDuplicateID
		}
	}
	r.items = append(r.items, a)
	return r.save()
}

// Remove deletes the announcement with the given id and persists.
func (r *Repo) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Announcement, 0, len(r.items))
	found := false
	for _, it := range r.items {
		if it.ID == id {
			found = true
			continue
		}
		out = append(out, it)
	}
	if !found {
		return ErrNotFound
	}
	r.items = out
	return r.save()
}

// List returns the announcements matching the filter, newest first. Expired
// items are dropped unless Filter.IncludeExpired is set; scheduled items hidden
// until their publish time.
func (r *Repo) List(now time.Time, f Filter) []Announcement {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Announcement, 0, len(r.items))
	for _, a := range r.items {
		if !a.Visible(now) {
			continue
		}
		if !f.matchesKind(a.Kind) {
			continue
		}
		if !a.MatchesDiscipline(f.Discipline) {
			continue
		}
		if !f.IncludeExpired && a.Expired(now) {
			continue
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].PostedAt.After(out[j].PostedAt)
	})
	return out
}
