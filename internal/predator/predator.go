// Package predator provides an advisory predatory-journal signal. It is
// deliberately conservative: it never makes a definitive claim, only combines an
// admin-curated watch list with a positive indexing signal (Scimago presence)
// into a low/medium/high risk hint for the user to verify themselves.
package predator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ErrDuplicate / ErrNotFound report watch-list edit failures.
var (
	ErrDuplicate = errors.New("predator: duplicate pattern")
	ErrNotFound  = errors.New("predator: pattern not found")
)

// Flag is one entry in the admin-curated advisory watch list.
type Flag struct {
	Pattern string `yaml:"pattern"` // matched as a normalized substring against name/publisher
	Reason  string `yaml:"reason"`  // Arabic explanation shown to the user
}

// List is the watch list — thread-safe and persisted (the bot reads via Check
// while the admin web edits via Add/Remove).
type List struct {
	mu    sync.RWMutex
	path  string // empty => in-memory only (tests)
	flags []Flag
}

// NewList builds a List from flags (used in tests and by the loader).
func NewList(flags []Flag) *List { return &List{flags: flags} }

// Len reports how many flags are loaded.
func (l *List) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.flags)
}

// All returns a copy of the watch list.
func (l *List) All() []Flag {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Flag, len(l.flags))
	copy(out, l.flags)
	return out
}

type fileShape struct {
	Flags []Flag `yaml:"flags"`
}

// Load reads data/predatory.yaml, returning an empty list when absent so the
// bot still starts on a fresh checkout. Edits persist back to the same file.
func Load(dataDir string) (*List, error) {
	path := filepath.Join(dataDir, "predatory.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &List{path: path}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc fileShape
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &List{path: path, flags: doc.Flags}, nil
}

func (l *List) save() error {
	if l.path == "" {
		return nil
	}
	data, err := yaml.Marshal(fileShape{Flags: l.flags})
	if err != nil {
		return fmt.Errorf("marshal predatory: %w", err)
	}
	return os.WriteFile(l.path, data, 0o644)
}

// Add appends a watch-list entry (unique pattern) and persists.
func (l *List) Add(pattern, reason string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return fmt.Errorf("predator: pattern is required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, f := range l.flags {
		if norm(f.Pattern) == norm(pattern) {
			return ErrDuplicate
		}
	}
	l.flags = append(l.flags, Flag{Pattern: pattern, Reason: strings.TrimSpace(reason)})
	return l.save()
}

// Remove deletes the entry with the given pattern and persists.
func (l *List) Remove(pattern string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Flag, 0, len(l.flags))
	found := false
	for _, f := range l.flags {
		if f.Pattern == pattern {
			found = true
			continue
		}
		out = append(out, f)
	}
	if !found {
		return ErrNotFound
	}
	l.flags = out
	return l.save()
}

// Check returns the flags whose pattern appears in any of the given fields
// (journal title, publisher, ...). Matching is case-insensitive substring.
func (l *List) Check(fields ...string) []Flag {
	l.mu.RLock()
	defer l.mu.RUnlock()
	normFields := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = norm(f); f != "" {
			normFields = append(normFields, f)
		}
	}

	var matched []Flag
	for _, flag := range l.flags {
		p := norm(flag.Pattern)
		if p == "" {
			continue
		}
		for _, f := range normFields {
			if strings.Contains(f, p) {
				matched = append(matched, flag)
				break
			}
		}
	}
	return matched
}

func norm(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// Risk is the advisory risk level.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// Advisory is the combined assessment for one journal query.
type Advisory struct {
	Risk      Risk
	InScimago bool
	Flags     []Flag // matched watch-list entries (empty unless RiskHigh)
}

// Assess combines the indexing signal with watch-list matches:
//   - any watch-list match  -> high risk
//   - indexed in Scimago    -> low risk (a credible legitimacy signal)
//   - otherwise             -> medium risk (unknown; verify manually)
func Assess(inScimago bool, flags []Flag) Advisory {
	a := Advisory{InScimago: inScimago, Flags: flags}
	switch {
	case len(flags) > 0:
		a.Risk = RiskHigh
	case inScimago:
		a.Risk = RiskLow
	default:
		a.Risk = RiskMedium
	}
	return a
}
