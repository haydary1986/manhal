// Package predator provides an advisory predatory-journal signal. It is
// deliberately conservative: it never makes a definitive claim, only combines an
// admin-curated watch list with a positive indexing signal (Scimago presence)
// into a low/medium/high risk hint for the user to verify themselves.
package predator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Flag is one entry in the admin-curated advisory watch list.
type Flag struct {
	Pattern string `yaml:"pattern"` // matched as a normalized substring against name/publisher
	Reason  string `yaml:"reason"`  // Arabic explanation shown to the user
}

// List is the loaded watch list.
type List struct {
	flags []Flag
}

// NewList builds a List from flags (used in tests and by the loader).
func NewList(flags []Flag) *List { return &List{flags: flags} }

// Len reports how many flags are loaded.
func (l *List) Len() int { return len(l.flags) }

type fileShape struct {
	Flags []Flag `yaml:"flags"`
}

// Load reads data/predatory.yaml, returning an empty list when absent so the
// bot still starts on a fresh checkout.
func Load(dataDir string) (*List, error) {
	path := filepath.Join(dataDir, "predatory.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewList(nil), nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc fileShape
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return NewList(doc.Flags), nil
}

// Check returns the flags whose pattern appears in any of the given fields
// (journal title, publisher, ...). Matching is case-insensitive substring.
func (l *List) Check(fields ...string) []Flag {
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
