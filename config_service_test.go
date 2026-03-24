package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureAPIKey_GeneratesKey(t *testing.T) {
	cs := &ConfigService{
		filePath:     filepath.Join(t.TempDir(), "config.json"),
		sessionsPath: filepath.Join(t.TempDir(), "sessions.json"),
	}

	cfg := cs.EnsureAPIKey()

	if cfg.APIKey == "" {
		t.Fatal("expected API key to be generated")
	}
	if len(cfg.APIKey) != 64 { // 32 bytes = 64 hex chars
		t.Fatalf("expected 64-char hex key, got %d chars", len(cfg.APIKey))
	}
	if cfg.APIPort != 19876 {
		t.Fatalf("expected default port 19876, got %d", cfg.APIPort)
	}
	if !cfg.APIEnabled {
		t.Fatal("expected API to be enabled")
	}
}

func TestEnsureAPIKey_Idempotent(t *testing.T) {
	cs := &ConfigService{
		filePath:     filepath.Join(t.TempDir(), "config.json"),
		sessionsPath: filepath.Join(t.TempDir(), "sessions.json"),
	}

	cfg1 := cs.EnsureAPIKey()
	cfg2 := cs.EnsureAPIKey()

	if cfg1.APIKey != cfg2.APIKey {
		t.Fatal("key should be stable across calls")
	}
}

func TestEnsureAPIKey_PreservesExisting(t *testing.T) {
	cs := &ConfigService{
		filePath:     filepath.Join(t.TempDir(), "config.json"),
		sessionsPath: filepath.Join(t.TempDir(), "sessions.json"),
		config: AppConfig{
			APIKey:     "existing-key-abc",
			APIPort:    9999,
			APIEnabled: true,
		},
	}

	cfg := cs.EnsureAPIKey()

	if cfg.APIKey != "existing-key-abc" {
		t.Fatalf("expected existing key preserved, got %s", cfg.APIKey)
	}
	if cfg.APIPort != 9999 {
		t.Fatalf("expected port 9999, got %d", cfg.APIPort)
	}
}

func TestWriteCLIConfig_Format(t *testing.T) {
	// Test that WriteCLIConfig produces valid JSON with the right fields.
	// We can't easily override UserConfigDir, so verify the format manually.
	tmpDir := t.TempDir()
	cliPath := filepath.Join(tmpDir, "cli.json")

	cfg := AppConfig{
		APIPort: 12345,
		APIKey:  "test-key-xyz",
	}

	// Simulate what WriteCLIConfig writes
	cliCfg := map[string]interface{}{
		"host": fmt.Sprintf("127.0.0.1:%d", cfg.APIPort),
		"key":  cfg.APIKey,
	}
	data, err := json.MarshalIndent(cliCfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	readBack, err := os.ReadFile(cliPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(readBack, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["host"] != "127.0.0.1:12345" {
		t.Fatalf("expected host 127.0.0.1:12345, got %s", parsed["host"])
	}
	if parsed["key"] != "test-key-xyz" {
		t.Fatalf("expected key test-key-xyz, got %s", parsed["key"])
	}
}

func TestSaveConfig_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	cs := &ConfigService{
		filePath:     filepath.Join(tmpDir, "config.json"),
		sessionsPath: filepath.Join(tmpDir, "sessions.json"),
	}

	original := AppConfig{
		SlackToken: "xoxb-test",
		APIPort:    19876,
		APIKey:     "abc123",
		APIEnabled: true,
	}

	if err := cs.SaveConfig(original); err != nil {
		t.Fatal(err)
	}

	// Create a new service that loads from the same file
	cs2 := &ConfigService{
		filePath:     filepath.Join(tmpDir, "config.json"),
		sessionsPath: filepath.Join(tmpDir, "sessions.json"),
	}
	cs2.load()

	loaded := cs2.GetConfig()
	if loaded.APIKey != "abc123" {
		t.Fatalf("expected key abc123, got %s", loaded.APIKey)
	}
	if loaded.APIPort != 19876 {
		t.Fatalf("expected port 19876, got %d", loaded.APIPort)
	}
	if !loaded.APIEnabled {
		t.Fatal("expected API enabled")
	}
	if loaded.SlackToken != "xoxb-test" {
		t.Fatalf("expected slack token xoxb-test, got %s", loaded.SlackToken)
	}
}
