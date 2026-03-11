package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const version = "0.1.0"

type App struct {
	ctx context.Context
	tm  *TerminalManager
}

func NewApp(tm *TerminalManager) *App {
	return &App{tm: tm}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.tm.setContext(ctx)
	a.installStatusLine()
}

func (a *App) installStatusLine() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Install the script to ~/.koko/
	scriptDir := filepath.Join(home, ".koko")
	_ = os.MkdirAll(scriptDir, 0o755)
	_ = os.MkdirAll(filepath.Join(scriptDir, "context"), 0o755)

	scriptPath := filepath.Join(scriptDir, "koko-statusline.sh")
	script := `#!/bin/bash
input=$(cat)
SESSION_ID=$(echo "$input" | jq -r '.session_id // empty')
USED_PCT=$(echo "$input" | jq -r '.context_window.used_percentage // 0')
MODEL=$(echo "$input" | jq -r '.model.display_name // "Claude"')
if [ -n "$SESSION_ID" ]; then
  mkdir -p ~/.koko/context
  echo "$input" | jq '{usedPercentage: (.context_window.used_percentage // 0), remainingPercentage: (.context_window.remaining_percentage // 100), model: (.model.display_name // "Claude")}' > ~/.koko/context/"${SESSION_ID}".json
fi
echo "${MODEL} · ${USED_PCT}% context"
`
	_ = os.WriteFile(scriptPath, []byte(script), 0o755)

	// Configure Claude Code to use it
	_ = a.EnsureStatusLine()
}

func (a *App) shutdown(ctx context.Context) {}

func (a *App) GetVersion() string {
	return version
}

func (a *App) PickDirectory() (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Choose Project Directory",
	})
}

// ContextInfo holds Claude session context window data written by the status line script.
type ContextInfo struct {
	UsedPercentage      int    `json:"usedPercentage"`
	RemainingPercentage int    `json:"remainingPercentage"`
	Model               string `json:"model"`
}

// GetContextInfo reads the context window data for a Claude session.
// The status line script writes to ~/.koko/context/{sessionID}.json.
func (a *App) GetContextInfo(sessionID string) (*ContextInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".koko", "context", sessionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no context data: %w", err)
	}
	var info ContextInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("invalid context data: %w", err)
	}
	return &info, nil
}

// EnsureStatusLine installs the Koko status line script into Claude Code's settings
// so that context data is written per session.
func (a *App) EnsureStatusLine() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Read existing Claude settings
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	var settings map[string]interface{}

	data, err := os.ReadFile(settingsPath)
	if err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	// Check if status line is already configured
	if sl, ok := settings["statusLine"]; ok {
		if slMap, ok := sl.(map[string]interface{}); ok {
			if cmd, ok := slMap["command"].(string); ok {
				if filepath.Base(cmd) == "koko-statusline.sh" || cmd == statusLineCommand() {
					return nil // Already configured
				}
			}
		}
	}

	// Install status line
	settings["statusLine"] = map[string]interface{}{
		"type":    "command",
		"command": statusLineCommand(),
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, out, 0o644)
}

func statusLineCommand() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".koko", "koko-statusline.sh")
}

// UpdateInfo holds information about an available update.
type UpdateInfo struct {
	Available  bool   `json:"available"`
	Version    string `json:"version"`
	CurrentVer string `json:"currentVersion"`
	URL        string `json:"url"`
}

// CheckForUpdate checks the GitHub Releases API for a newer version.
func (a *App) CheckForUpdate() (*UpdateInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/edalee/koko/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return &UpdateInfo{Available: false, CurrentVer: version}, nil
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	if latestVer != version && latestVer > version {
		return &UpdateInfo{
			Available:  true,
			Version:    latestVer,
			CurrentVer: version,
			URL:        release.HTMLURL,
		}, nil
	}

	return &UpdateInfo{Available: false, CurrentVer: version}, nil
}
