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
	// Premium / manual payment (no gateway): the admin describes the plans and
	// the account numbers users transfer to; the bot shows them on the subscribe
	// screen, and the admin confirms payment manually. PaymentLink is an optional
	// deep link / payment URL (e.g. ZainCash) opened by a "pay now" button.
	PremiumInfo    string `yaml:"premium_info"`
	PaymentDetails string `yaml:"payment_details"`
	PaymentLink    string `yaml:"payment_link"`
	// Daily AI-request quotas by tier. 0 = use the env default (free) or
	// unlimited (premium).
	FreeAILimit    int `yaml:"free_ai_limit"`
	PremiumAILimit int `yaml:"premium_ai_limit"`
	// API key managed from the admin page (seeded from the env at first run).
	DeepSeekKey string `yaml:"deepseek_key"`
	// Bot identity applied via the Telegram API on startup (name/description).
	BotName        string `yaml:"bot_name"`
	BotDescription string `yaml:"bot_description"`
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

// PremiumInfo returns the plans/benefits text shown on the subscribe screen.
func (m *SettingsManager) PremiumInfo() string { return m.Get().PremiumInfo }

// PaymentDetails returns the account numbers / payment instructions.
func (m *SettingsManager) PaymentDetails() string { return m.Get().PaymentDetails }

// PaymentLink returns the optional deep link / payment URL ("" if unset).
func (m *SettingsManager) PaymentLink() string { return m.Get().PaymentLink }

// FreeAILimit / PremiumAILimit return the configured daily AI quotas.
func (m *SettingsManager) FreeAILimit() int    { return m.Get().FreeAILimit }
func (m *SettingsManager) PremiumAILimit() int { return m.Get().PremiumAILimit }

// DeepSeekKey returns the current DeepSeek API key.
func (m *SettingsManager) DeepSeekKey() string { return m.Get().DeepSeekKey }

// WelcomeMessage / BotName / BotDescription expose the bot's identity & greeting.
func (m *SettingsManager) WelcomeMessage() string { return m.Get().WelcomeMessage }
func (m *SettingsManager) BotName() string        { return m.Get().BotName }
func (m *SettingsManager) BotDescription() string { return m.Get().BotDescription }

// SetIdentity updates the greeting (live) and the bot name/description (applied
// to Telegram on next start) and persists them.
func (m *SettingsManager) SetIdentity(welcome, name, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.cur
	if w := strings.TrimSpace(welcome); w != "" {
		next.WelcomeMessage = w
	}
	next.BotName = strings.TrimSpace(name)
	next.BotDescription = strings.TrimSpace(description)
	if err := SaveBotSettings(m.dataDir, &next); err != nil {
		return err
	}
	m.cur = next
	return nil
}

// SetDeepSeekKey updates and persists the DeepSeek API key.
func (m *SettingsManager) SetDeepSeekKey(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.cur
	next.DeepSeekKey = strings.TrimSpace(key)
	if err := SaveBotSettings(m.dataDir, &next); err != nil {
		return err
	}
	m.cur = next
	return nil
}

// SeedDeepSeekKey sets the key only if none is configured yet (used to seed from
// the environment at startup without overwriting an admin-set value).
func (m *SettingsManager) SeedDeepSeekKey(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(m.cur.DeepSeekKey) == "" {
		m.cur.DeepSeekKey = strings.TrimSpace(key)
	}
}

// SetLimits updates the per-tier daily AI quotas (clamped to >= 0) and persists.
func (m *SettingsManager) SetLimits(free, premium int) error {
	if free < 0 {
		free = 0
	}
	if premium < 0 {
		premium = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.cur
	next.FreeAILimit = free
	next.PremiumAILimit = premium
	if err := SaveBotSettings(m.dataDir, &next); err != nil {
		return err
	}
	m.cur = next
	return nil
}

// SetPayment updates the premium description, payment details and optional pay
// link, then persists.
func (m *SettingsManager) SetPayment(premiumInfo, paymentDetails, paymentLink string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.cur
	next.PremiumInfo = strings.TrimSpace(premiumInfo)
	next.PaymentDetails = strings.TrimSpace(paymentDetails)
	next.PaymentLink = strings.TrimSpace(paymentLink)
	if err := SaveBotSettings(m.dataDir, &next); err != nil {
		return err
	}
	m.cur = next
	return nil
}

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
