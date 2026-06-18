// Package plans holds the editable subscription plans (الباقات): the catalogue
// of named durations and prices the bot offers and the admin manages. Plans are
// the single source of truth that the subscribe screen, the payment request, and
// the admin activation all reference.
package plans

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/erticaz/manhal/internal/domain"
	"gopkg.in/yaml.v3"
)

// Plan is one purchasable subscription option. Months 0 means a permanent grant.
type Plan struct {
	ID     string      `yaml:"id"`
	Name   string      `yaml:"name"`
	Months int         `yaml:"months"`
	Price  int         `yaml:"price"` // in local currency units (IQD)
	Tier   domain.Tier `yaml:"tier"`
}

// DefaultPlans is the seed catalogue used when no file exists.
func DefaultPlans() []Plan {
	return []Plan{
		{ID: "monthly", Name: "شهري", Months: 1, Price: 5000, Tier: domain.TierResearcher},
		{ID: "semiannual", Name: "نصف سنوي", Months: 6, Price: 25000, Tier: domain.TierResearcher},
		{ID: "annual", Name: "سنوي", Months: 12, Price: 45000, Tier: domain.TierResearcher},
	}
}

type fileShape struct {
	Plans []Plan `yaml:"plans"`
}

// Load reads data/plans.yaml, falling back to the defaults when absent or empty.
func Load(dataDir string) ([]Plan, error) {
	path := filepath.Join(dataDir, "plans.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPlans(), nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc fileShape
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Plans) == 0 {
		return DefaultPlans(), nil
	}
	return doc.Plans, nil
}

// ErrDuplicate / ErrNotFound report edit failures.
var (
	ErrNotFound = errors.New("plans: not found")
	ErrInvalid  = errors.New("plans: id and name are required")
)

// Manager gives concurrency-safe, persisted access to the plan catalogue,
// shared between the bot (reads) and the admin web (edits).
type Manager struct {
	mu      sync.RWMutex
	dataDir string
	items   []Plan
}

// NewManager wraps an initial catalogue.
func NewManager(dataDir string, initial []Plan) *Manager {
	return &Manager{dataDir: dataDir, items: initial}
}

// List returns a copy of the plans (nil-safe).
func (m *Manager) List() []Plan {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Plan, len(m.items))
	copy(out, m.items)
	return out
}

// Get returns the plan with the given id.
func (m *Manager) Get(id string) (Plan, bool) {
	if m == nil {
		return Plan{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.items {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// Upsert adds a new plan or updates an existing one (matched by id), then
// persists. Months/price are clamped to non-negative.
func (m *Manager) Upsert(p Plan) error {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	if p.ID == "" || p.Name == "" {
		return ErrInvalid
	}
	if p.Months < 0 {
		p.Months = 0
	}
	if p.Price < 0 {
		p.Price = 0
	}
	if p.Tier != domain.TierStudent && p.Tier != domain.TierResearcher {
		p.Tier = domain.TierResearcher
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == p.ID {
			m.items[i] = p
			return m.save()
		}
	}
	m.items = append(m.items, p)
	return m.save()
}

// Remove deletes a plan by id and persists.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Plan, 0, len(m.items))
	found := false
	for _, p := range m.items {
		if p.ID == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return ErrNotFound
	}
	m.items = out
	return m.save()
}

func (m *Manager) save() error {
	if m.dataDir == "" {
		return nil
	}
	data, err := yaml.Marshal(fileShape{Plans: m.items})
	if err != nil {
		return fmt.Errorf("marshal plans: %w", err)
	}
	return os.WriteFile(filepath.Join(m.dataDir, "plans.yaml"), data, 0o644)
}

// DurationLabel renders a human duration for a plan ("دائم" for permanent).
func (p Plan) DurationLabel() string {
	switch {
	case p.Months <= 0:
		return "دائم"
	case p.Months == 1:
		return "شهر"
	case p.Months == 12:
		return "سنة"
	default:
		return fmt.Sprintf("%d أشهر", p.Months)
	}
}
