package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	GitHubRepos  []string          `json:"githubRepos"`
	SlackEnabled bool              `json:"slackEnabled"`
	SafeWorking  SafeWorkingConfig `json:"safeWorking"`
}

// ConfigService manages reading and writing the app config file.
type ConfigService struct {
	mu       sync.Mutex
	config   AppConfig
	filePath string
}

// NewConfigService creates a ConfigService and loads any existing config.
func NewConfigService() *ConfigService {
	configDir, _ := os.UserConfigDir()
	filePath := filepath.Join(configDir, "koko", "config.json")

	cs := &ConfigService{filePath: filePath}
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

// SetSlackToken updates just the Slack token.
func (cs *ConfigService) SetSlackToken(token string) error {
	cs.mu.Lock()
	cfg := cs.config
	cs.mu.Unlock()

	cfg.SlackToken = token
	cfg.SlackEnabled = token != ""
	return cs.SaveConfig(cfg)
}

func (cs *ConfigService) load() {
	data, err := os.ReadFile(cs.filePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &cs.config)
}
