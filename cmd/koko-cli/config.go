package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type cliConfig struct {
	Host string `json:"host"`
	Key  string `json:"key"`
}

func loadConfig() (*cliConfig, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find config dir: %w", err)
	}

	cliPath := filepath.Join(configDir, "koko", "cli.json")
	data, err := os.ReadFile(cliPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w\nMake sure Koko desktop app is running", cliPath, err)
	}

	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if cfg.Host == "" {
		cfg.Host = "127.0.0.1:19876"
	}

	return &cfg, nil
}
