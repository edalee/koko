package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// SafeWorkingConfig holds settings for work/break boundaries.
type SafeWorkingConfig struct {
	QuietHoursEnabled bool   `json:"quietHoursEnabled"`
	QuietHoursStart   string `json:"quietHoursStart"` // "HH:MM" format
	QuietHoursEnd     string `json:"quietHoursEnd"`   // "HH:MM" format
	BreakEnabled      bool   `json:"breakEnabled"`
	WorkMinutes       int    `json:"workMinutes"`  // e.g., 90
	BreakMinutes      int    `json:"breakMinutes"` // e.g., 15
}

// AppConfig holds all persisted application settings.
type AppConfig struct {
	SlackToken   string            `json:"slackToken"`
	SlackOwnerID string            `json:"slackOwnerId"` // only respond to this Slack user ID
	GitHubRepos  []string          `json:"githubRepos"`
	SafeWorking  SafeWorkingConfig `json:"safeWorking"`
	APIPort      int               `json:"apiPort"`
	APIKey       string            `json:"apiKey"`
	APIEnabled   bool              `json:"apiEnabled"`
}

// ConfigService manages reading and writing the app config file.
type ConfigService struct {
	mu           sync.Mutex
	config       AppConfig
	filePath     string
	sessionsPath string
}

// NewConfigService creates a ConfigService and loads any existing config.
func NewConfigService() *ConfigService {
	configDir, _ := os.UserConfigDir()
	kokoDir := filepath.Join(configDir, "koko")
	filePath := filepath.Join(kokoDir, "config.json")
	sessionsPath := filepath.Join(kokoDir, "sessions.json")

	cs := &ConfigService{filePath: filePath, sessionsPath: sessionsPath}
	cs.load()
	return cs
}

// GetConfig returns the current config.
func (cs *ConfigService) GetConfig() AppConfig {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.config
}

// SaveConfig persists the given config to disk.
func (cs *ConfigService) SaveConfig(config AppConfig) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.config = config

	dir := filepath.Dir(cs.filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cs.filePath, data, 0o600)
}

// EnsureAPIKey generates an API key if one doesn't exist and sets default port.
// Returns the current config after ensuring defaults.
func (cs *ConfigService) EnsureAPIKey() AppConfig {
	cs.mu.Lock()
	cfg := cs.config
	changed := false

	if cfg.APIPort == 0 {
		cfg.APIPort = 19876
		changed = true
	}
	if cfg.APIKey == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		cfg.APIKey = hex.EncodeToString(b)
		cfg.APIEnabled = true
		changed = true
	}
	if changed {
		cs.config = cfg
	}
	cs.mu.Unlock()

	if changed {
		_ = cs.SaveConfig(cfg)
		_ = cs.WriteCLIConfig(cfg)
	}
	return cfg
}

// WriteCLIConfig writes a minimal config file for the CLI companion at ~/.config/koko/cli.json.
func (cs *ConfigService) WriteCLIConfig(cfg AppConfig) error {
	configDir, _ := os.UserConfigDir()
	cliPath := filepath.Join(configDir, "koko", "cli.json")
	_ = os.MkdirAll(filepath.Dir(cliPath), 0o700)

	cliCfg := map[string]interface{}{
		"host": fmt.Sprintf("127.0.0.1:%d", cfg.APIPort),
		"key":  cfg.APIKey,
	}
	data, err := json.MarshalIndent(cliCfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cliPath, data, 0o600)
}

func (cs *ConfigService) load() {
	data, err := os.ReadFile(cs.filePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &cs.config)
}

// GetSessions returns persisted session state, migrating from WebKit localStorage if needed.
func (cs *ConfigService) GetSessions() SessionsData {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	data, err := os.ReadFile(cs.sessionsPath)
	if err == nil {
		var sessions SessionsData
		if json.Unmarshal(data, &sessions) == nil {
			return sessions
		}
	}

	// No sessions file — try migrating from WebKit localStorage
	sessions := cs.migrateFromWebKit()
	if len(sessions.Tabs) > 0 || len(sessions.History) > 0 || len(sessions.RecentDirs) > 0 {
		_ = cs.saveSessions(sessions)
	}
	return sessions
}

// SaveSessions persists session state to disk.
func (cs *ConfigService) SaveSessions(sessions SessionsData) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.saveSessions(sessions)
}

func (cs *ConfigService) saveSessions(sessions SessionsData) error {
	dir := filepath.Dir(cs.sessionsPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cs.sessionsPath, data, 0o600)
}

// migrateFromWebKit reads session data from all WebKit localStorage databases.
func (cs *ConfigService) migrateFromWebKit() SessionsData {
	home, err := os.UserHomeDir()
	if err != nil {
		return SessionsData{}
	}

	var best SessionsData
	bestScore := 0

	// Check both bundle IDs — data could be in either
	for _, bundleID := range []string{"com.wails.koko", "com.edalee.koko"} {
		webkitDir := filepath.Join(home, "Library", "WebKit", bundleID, "WebsiteData", "Default")
		dbGlob := filepath.Join(webkitDir, "*", "*", "LocalStorage", "localstorage.sqlite3")
		matches, err := filepath.Glob(dbGlob)
		if err != nil {
			continue
		}

		for _, dbPath := range matches {
			sessions := cs.readWebKitDB(dbPath)
			score := len(sessions.Tabs)*10 + len(sessions.History)*5 + len(sessions.RecentDirs)
			if score > bestScore {
				best = sessions
				bestScore = score
			}
		}
	}

	if bestScore > 0 {
		log.Printf("Migrated sessions from WebKit localStorage: %d tabs, %d history, %d dirs",
			len(best.Tabs), len(best.History), len(best.RecentDirs))
	}
	return best
}

// readWebKitDB reads koko session data from a WebKit localStorage SQLite database.
func (cs *ConfigService) readWebKitDB(dbPath string) SessionsData {
	var sessions SessionsData

	// Use sqlite3 CLI to avoid adding a Go dependency
	for _, key := range []string{"koko:session-tabs", "koko:session-history", "koko:recent-dirs"} {
		query := fmt.Sprintf("SELECT value FROM ItemTable WHERE key = '%s';", key)
		cmd := exec.Command("sqlite3", "-readonly", dbPath, query)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(out))
		if val == "" {
			continue
		}

		switch key {
		case "koko:session-tabs":
			_ = json.Unmarshal([]byte(val), &sessions.Tabs)
		case "koko:session-history":
			_ = json.Unmarshal([]byte(val), &sessions.History)
		case "koko:recent-dirs":
			_ = json.Unmarshal([]byte(val), &sessions.RecentDirs)
		}
	}
	return sessions
}
