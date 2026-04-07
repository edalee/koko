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
	HiddenPRs    map[string]bool   `json:"hiddenPrs,omitempty"` // key: "repo#number"
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

// HidePR marks a PR as hidden so it won't appear in the list.
func (cs *ConfigService) HidePR(repo string, number int) error {
	cs.mu.Lock()
	key := fmt.Sprintf("%s#%d", repo, number)
	if cs.config.HiddenPRs == nil {
		cs.config.HiddenPRs = make(map[string]bool)
	}
	cs.config.HiddenPRs[key] = true
	cfg := cs.config
	cs.mu.Unlock()
	return cs.SaveConfig(cfg)
}

// UnhidePR removes the hidden flag from a PR.
func (cs *ConfigService) UnhidePR(repo string, number int) error {
	cs.mu.Lock()
	key := fmt.Sprintf("%s#%d", repo, number)
	delete(cs.config.HiddenPRs, key)
	cfg := cs.config
	cs.mu.Unlock()
	return cs.SaveConfig(cfg)
}

// ClearHiddenPRs removes all hidden PR entries.
func (cs *ConfigService) ClearHiddenPRs() (int, error) {
	cs.mu.Lock()
	count := len(cs.config.HiddenPRs)
	cs.config.HiddenPRs = nil
	cfg := cs.config
	cs.mu.Unlock()
	return count, cs.SaveConfig(cfg)
}

// GetHiddenPRs returns the set of hidden PR keys.
func (cs *ConfigService) GetHiddenPRs() map[string]bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	result := make(map[string]bool, len(cs.config.HiddenPRs))
	for k, v := range cs.config.HiddenPRs {
		result[k] = v
	}
	return result
}

func (cs *ConfigService) load() {
	data, err := os.ReadFile(cs.filePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &cs.config)
}

// GetSessions returns persisted session state, migrating from legacy format if needed.
func (cs *ConfigService) GetSessions() SessionsData {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Try reading sessions file (may be corrupt after crash)
	data, err := os.ReadFile(cs.sessionsPath)
	if err == nil {
		var sessions SessionsData
		if json.Unmarshal(data, &sessions) == nil {
			// Migrate legacy format: Tabs/History → Sessions
			if len(sessions.Sessions) == 0 && (len(sessions.Tabs) > 0 || len(sessions.History) > 0) {
				sessions = cs.migrateLegacySessions(sessions)
				_ = cs.saveSessions(sessions)
			}
			return sessions
		}
	}

	// Main file missing or corrupt — try backup
	bakData, bakErr := os.ReadFile(cs.sessionsPath + ".bak")
	if bakErr == nil {
		var sessions SessionsData
		if json.Unmarshal(bakData, &sessions) == nil {
			log.Printf("Recovered sessions from backup")
			_ = cs.saveSessions(sessions)
			return sessions
		}
	}

	// No sessions file — try migrating from WebKit localStorage
	sessions := cs.migrateFromWebKit()
	if len(sessions.Tabs) > 0 || len(sessions.History) > 0 || len(sessions.RecentDirs) > 0 {
		sessions = cs.migrateLegacySessions(sessions)
		_ = cs.saveSessions(sessions)
	}
	return sessions
}

// migrateLegacySessions converts Tabs + History to unified Sessions records.
func (cs *ConfigService) migrateLegacySessions(old SessionsData) SessionsData {
	var sessions []SessionRecord

	// Convert tabs to disconnected sessions
	for _, t := range old.Tabs {
		slug := dirSlugFromPath(t.Directory)
		sessions = append(sessions, SessionRecord{
			Slug:      slug,
			Name:      t.Name,
			Directory: t.Directory,
			CreatedAt: t.CreatedAt,
			Status:    "disconnected",
		})
	}

	// Convert history to closed sessions
	for _, h := range old.History {
		slug := dirSlugFromPath(h.Directory)
		sessions = append(sessions, SessionRecord{
			Slug:      slug,
			Name:      h.Name,
			Directory: h.Directory,
			CreatedAt: h.CreatedAt,
			ClosedAt:  h.ClosedAt,
			Status:    "closed",
			LastMsg:   h.LastMessage,
		})
	}

	log.Printf("Migrated %d tabs + %d history → %d session records", len(old.Tabs), len(old.History), len(sessions))
	return SessionsData{
		Sessions:   sessions,
		RecentDirs: old.RecentDirs,
	}
}

// dirSlugFromPath generates a simple slug from a directory path for migration.
func dirSlugFromPath(dir string) string {
	dir = strings.TrimRight(dir, "/")
	parts := strings.Split(dir, "/")
	if len(parts) == 0 {
		return "session-1"
	}
	return parts[len(parts)-1] + "-1"
}

// SaveSessions persists session state to disk with atomic write + backup.
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

	// Clear legacy fields when writing new format
	sessions.Tabs = nil
	sessions.History = nil

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to .new, backup old, rename .new → sessions.json
	newPath := cs.sessionsPath + ".new"
	if err := os.WriteFile(newPath, data, 0o600); err != nil {
		return err
	}

	// Backup existing file (ignore error if it doesn't exist)
	_ = os.Rename(cs.sessionsPath, cs.sessionsPath+".bak")

	// Atomic rename
	return os.Rename(newPath, cs.sessionsPath)
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
