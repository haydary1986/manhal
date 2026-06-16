package announce

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// Repo is an in-memory, read-only collection of announcements. Content is
// curated in data/announcements.yaml today; an admin-editable store comes later.
type Repo struct {
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
// absent so the bot still starts on a fresh checkout.
func Load(dataDir string) (*Repo, error) {
	path := filepath.Join(dataDir, "announcements.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewRepo(nil), nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc fileShape
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return NewRepo(doc.Announcements), nil
}

// Len reports how many announcements are loaded.
func (r *Repo) Len() int { return len(r.items) }

// List returns the announcements matching the filter, newest first. Expired
// items are dropped unless Filter.IncludeExpired is set.
func (r *Repo) List(now time.Time, f Filter) []Announcement {
	out := make([]Announcement, 0, len(r.items))
	for _, a := range r.items {
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
