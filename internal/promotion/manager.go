package promotion

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Manager gives concurrency-safe, persisted access to the promotion Rules,
// shared between the bot (reads the live rules) and the admin web (edits them as
// YAML). Edits swap the whole *Rules pointer atomically, so a read that already
// captured a Rank/Activity keeps a consistent snapshot.
type Manager struct {
	mu      sync.RWMutex
	dataDir string
	rules   *Rules
}

// NewManager wraps an initial rule set.
func NewManager(dataDir string, rules *Rules) *Manager {
	if rules == nil {
		rules = DefaultRules()
	}
	return &Manager{dataDir: dataDir, rules: rules}
}

func (m *Manager) current() *Rules {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rules
}

// Ranks returns the configured ranks (live).
func (m *Manager) Ranks() []Rank { return m.current().Ranks }

// FindRank delegates to the live rules.
func (m *Manager) FindRank(key string) (Rank, bool) { return m.current().FindRank(key) }

// ActivitiesByTable delegates to the live rules.
func (m *Manager) ActivitiesByTable(table int) []Activity {
	return m.current().ActivitiesByTable(table)
}

// Compute delegates to the live rules.
func (m *Manager) Compute(in Input) (Result, bool) { return m.current().Compute(in) }

// YAML returns the current rules serialized as the data/promotion.yaml document,
// so the admin editor can show and round-trip them (even when no file exists yet
// and the defaults are in effect).
func (m *Manager) YAML() (string, error) {
	r := m.current()
	data, err := yaml.Marshal(fileShape{Activities: r.Activities, Ranks: r.Ranks})
	if err != nil {
		return "", fmt.Errorf("marshal promotion rules: %w", err)
	}
	return string(data), nil
}

// SetYAML validates the edited document, then swaps it in and persists it. The
// live rules are left untouched if validation fails, so a typo can never break
// the bot.
func (m *Manager) SetYAML(text string) error {
	rules, err := parseRules([]byte(text))
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.rules = rules
	m.mu.Unlock()
	return m.save(text)
}

// ResetDefault restores the ministry defaults and persists them.
func (m *Manager) ResetDefault() error {
	def := DefaultRules()
	data, err := yaml.Marshal(fileShape{Activities: def.Activities, Ranks: def.Ranks})
	if err != nil {
		return fmt.Errorf("marshal defaults: %w", err)
	}
	m.mu.Lock()
	m.rules = def
	m.mu.Unlock()
	return m.save(string(data))
}

func (m *Manager) save(text string) error {
	if m.dataDir == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(m.dataDir, "promotion.yaml"), []byte(text), 0o644)
}

// parseRules unmarshals and validates a promotion rules document.
func parseRules(data []byte) (*Rules, error) {
	var doc fileShape
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("صيغة YAML غير صحيحة: %w", err)
	}
	if len(doc.Ranks) == 0 {
		return nil, fmt.Errorf("يجب تعريف رتبة واحدة على الأقل (ranks)")
	}
	if len(doc.Activities) == 0 {
		return nil, fmt.Errorf("يجب تعريف بند واحد على الأقل (activities)")
	}
	for _, rk := range doc.Ranks {
		if rk.Key == "" || rk.Label == "" {
			return nil, fmt.Errorf("كل رتبة تحتاج key و label")
		}
	}
	for _, a := range doc.Activities {
		if a.Key == "" {
			return nil, fmt.Errorf("كل بند يحتاج key")
		}
		if a.Table != 1 && a.Table != 2 {
			return nil, fmt.Errorf("البند %q: table يجب أن يكون 1 أو 2", a.Key)
		}
	}
	return &Rules{Activities: doc.Activities, Ranks: doc.Ranks}, nil
}
