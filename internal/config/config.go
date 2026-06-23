// Package config loads runtime configuration from the environment and the
// editable YAML settings files.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the runtime configuration loaded from environment variables.
type Config struct {
	BotToken       string
	DatabaseURL    string // empty => use the in-memory store
	DeepSeekKey    string
	CrossrefMailto string // contact email for the Crossref polite pool (optional)
	UnpaywallEmail string // required by Unpaywall; falls back to CrossrefMailto
	PexelsKey      string // Pexels API key for presentation images (optional)
	TavilyKey      string // Tavily API key for AI web search (optional)
	AdminIDs       []int64
	DataDir        string            // directory holding the editable YAML files
	WebAddr        string            // listen address for the admin web server
	AdminWebToken  string            // legacy single Basic-Auth password (user "admin")
	AdminWebUsers  map[string]string // additional web admins: username -> password
	AIDailyLimit   int               // per-user daily AI requests; <=0 means unlimited
	AlertIntervalM int               // alerts poll interval in minutes; <=0 disables
	ReminderDays   int               // remind this many days before a deadline
	DigestWeekday  int               // weekday for the weekly digest (0=Sunday..6=Saturday)
	OllamaURL      string            // local embeddings server base URL
	EmbedModel     string            // embedding model name; empty disables semantic features
}

// Load reads the .env file (if present) then the environment.
func Load() (*Config, error) {
	loadDotEnv(".env")

	c := &Config{
		BotToken:       os.Getenv("BOT_TOKEN"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		DeepSeekKey:    os.Getenv("DEEPSEEK_API_KEY"),
		CrossrefMailto: os.Getenv("CROSSREF_MAILTO"),
		UnpaywallEmail: os.Getenv("UNPAYWALL_EMAIL"),
		PexelsKey:      os.Getenv("PEXELS_API_KEY"),
		TavilyKey:      os.Getenv("TAVILY_API_KEY"),
		DataDir:        getenvDefault("DATA_DIR", "data"),
		AdminIDs:       parseIDs(os.Getenv("ADMIN_IDS")),
		WebAddr:        getenvDefault("WEB_ADDR", ":8080"),
		AdminWebToken:  os.Getenv("ADMIN_WEB_TOKEN"),
		AdminWebUsers:  parseAccounts(os.Getenv("ADMIN_WEB_USERS")),
		AIDailyLimit:   parseIntDefault(os.Getenv("AI_DAILY_LIMIT"), 5),
		AlertIntervalM: parseIntDefault(os.Getenv("ALERT_INTERVAL_MINUTES"), 360),
		ReminderDays:   parseIntDefault(os.Getenv("REMINDER_WINDOW_DAYS"), 7),
		DigestWeekday:  parseIntDefault(os.Getenv("DIGEST_WEEKDAY"), 0),
		OllamaURL:      getenvDefault("OLLAMA_URL", "http://localhost:11434"),
		EmbedModel:     os.Getenv("EMBED_MODEL"),
	}
	if c.BotToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN is required (set it in .env)")
	}
	if c.UnpaywallEmail == "" {
		c.UnpaywallEmail = c.CrossrefMailto // reuse the polite-pool contact email
	}
	return c, nil
}

// WebAccounts returns all valid web-admin credentials: the legacy ADMIN_WEB_TOKEN
// (as user "admin") merged with ADMIN_WEB_USERS. An empty map means the web admin
// is disabled.
func (c *Config) WebAccounts() map[string]string {
	accounts := make(map[string]string, len(c.AdminWebUsers)+1)
	for u, p := range c.AdminWebUsers {
		accounts[u] = p
	}
	if c.AdminWebToken != "" {
		accounts["admin"] = c.AdminWebToken
	}
	return accounts
}

// parseAccounts parses "user1:pass1,user2:pass2" into a credential map.
func parseAccounts(s string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		user, pass, ok := strings.Cut(part, ":")
		user = strings.TrimSpace(user)
		if ok && user != "" && pass != "" {
			out[user] = pass
		}
	}
	return out
}

// IsAdmin reports whether the Telegram user is configured as an admin.
func (c *Config) IsAdmin(id int64) bool {
	for _, a := range c.AdminIDs {
		if a == id {
			return true
		}
	}
	return false
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseIntDefault parses s as an int, returning def when it is empty or invalid.
func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

func parseIDs(s string) []int64 {
	var ids []int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// loadDotEnv loads KEY=VALUE pairs from a file into the environment without
// overriding variables that are already set.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
