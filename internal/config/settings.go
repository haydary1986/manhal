package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// BotSettings controls the subscription gate and greeting. Editable from the
// admin dashboard (later) or directly via data/bot.yaml.
type BotSettings struct {
	RequireSubscription bool   `yaml:"require_subscription"`
	RequiredChannel     string `yaml:"required_channel"`
	WelcomeMessage      string `yaml:"welcome_message"`
}

// DefaultBotSettings is the seed used when no file is present.
func DefaultBotSettings() *BotSettings {
	return &BotSettings{
		RequireSubscription: false,
		RequiredChannel:     "",
		WelcomeMessage: "أهلاً بك في منهل 🎓\n" +
			"مساعدك الأكاديمي للبحث والدراسة.\n" +
			"اختر من القائمة:",
	}
}

// Validate checks the invariants the gate relies on.
func (b *BotSettings) Validate() error {
	if b.RequireSubscription && b.RequiredChannel == "" {
		return fmt.Errorf("required_channel must be set when require_subscription is enabled")
	}
	return nil
}

// LoadBotSettings reads data/bot.yaml, falling back to defaults if absent.
func LoadBotSettings(dataDir string) (*BotSettings, error) {
	path := filepath.Join(dataDir, "bot.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultBotSettings(), nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	b := DefaultBotSettings()
	if err := yaml.Unmarshal(data, b); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", path, err)
	}
	return b, nil
}

// SaveBotSettings writes settings to data/bot.yaml.
func SaveBotSettings(dataDir string, b *BotSettings) error {
	if err := b.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(b)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	path := filepath.Join(dataDir, "bot.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// SettingsManager gives concurrency-safe, persisted access to BotSettings,
// shared between the bot (which reads it live) and the admin web (which edits
// it) — mirroring how menu.Manager is shared.
type SettingsManager struct {
	mu      sync.RWMutex
	dataDir string
	cur     BotSettings
}

// NewSettingsManager wraps an initial settings snapshot. A nil initial uses the
// defaults.
func NewSettingsManager(dataDir string, initial *BotSettings) *SettingsManager {
	s := DefaultBotSettings()
	if initial != nil {
		s = initial
	}
	return &SettingsManager{dataDir: dataDir, cur: *s}
}

// Get returns a copy of the current settings (nil-safe).
func (m *SettingsManager) Get() BotSettings {
	if m == nil {
		return *DefaultBotSettings()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cur
}

// RequiredChannel returns the channel users must join (nil-safe).
func (m *SettingsManager) RequiredChannel() string { return m.Get().RequiredChannel }

// RequireSubscription reports whether the gate is enabled (nil-safe).
func (m *SettingsManager) RequireSubscription() bool { return m.Get().RequireSubscription }

// SetGate updates the subscription gate (channel + on/off) and persists it.
func (m *SettingsManager) SetGate(channel string, require bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.cur
	next.RequiredChannel = strings.TrimSpace(channel)
	next.RequireSubscription = require
	if err := next.Validate(); err != nil {
		return err
	}
	if err := SaveBotSettings(m.dataDir, &next); err != nil {
		return err
	}
	m.cur = next
	return nil
}
