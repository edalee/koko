package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WorkerService lets the Settings panel control koko-worker (cmd/koko-worker).
// The worker's logic stays in its own binary. This service only edits
// worker.json and calls the binary.
//
// Every method passes JSON strings, not structs, so the Wails bindings stay
// in their own files and never change the shared models.ts.
type WorkerService struct{}

// NewWorkerService creates a WorkerService.
func NewWorkerService() *WorkerService {
	return &WorkerService{}
}

func workerConfigPath() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, "koko", "worker.json")
}

// workerBinary finds koko-worker: next to the Koko binary (inside Koko.app),
// then in build/bin for `make dev`, then the copy the agent runs.
func workerBinary() (string, error) {
	var candidates []string
	if self, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(self), "koko-worker"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "build", "bin", "koko-worker"))
	}
	configDir, _ := os.UserConfigDir()
	candidates = append(candidates, filepath.Join(configDir, "koko", "worker", "bin", "koko-worker"))
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("koko-worker binary not found. Run `make build-worker`")
}

func runWorker(timeout time.Duration, args ...string) (string, error) {
	bin, err := workerBinary()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = cleanEnv()
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("koko-worker %s timed out", args[0])
	}
	if err != nil {
		return string(out), fmt.Errorf("koko-worker %s: %s", args[0], strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// GetConfig returns worker.json, or "{}" if it does not exist yet. The worker
// fills in its defaults for any missing key.
func (w *WorkerService) GetConfig() (string, error) {
	data, err := os.ReadFile(workerConfigPath())
	if os.IsNotExist(err) {
		return "{}", nil
	}
	return string(data), err
}

// SaveConfig writes worker.json. The running worker reloads it within 30 seconds.
func (w *WorkerService) SaveConfig(configJSON string) error {
	var check map[string]any
	if err := json.Unmarshal([]byte(configJSON), &check); err != nil {
		return fmt.Errorf("invalid worker config: %w", err)
	}
	pretty, _ := json.MarshalIndent(check, "", "  ")
	path := workerConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, pretty, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SetEnabled switches the worker on or off: it sets "enabled" in worker.json
// and installs or removes the launchd agent.
func (w *WorkerService) SetEnabled(enabled bool) error {
	raw, err := w.GetConfig()
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return err
	}
	cfg["enabled"] = enabled
	data, _ := json.Marshal(cfg)
	if err := w.SaveConfig(string(data)); err != nil {
		return err
	}
	action := "uninstall"
	if enabled {
		action = "install"
	}
	_, err = runWorker(30*time.Second, action)
	return err
}

// Status returns `koko-worker status` as JSON. If the agent runs an older
// copy of the worker than the one next to Koko, it reinstalls first, so the
// scheduler and "Run now" never run different builds.
func (w *WorkerService) Status() (string, error) {
	if stale, _ := installedCopyStale(); stale {
		if _, err := runWorker(30*time.Second, "install"); err != nil {
			return "", err
		}
	}
	return runWorker(15*time.Second, "status")
}

// installedCopyStale is true if the agent's copy exists and differs from the
// binary Koko would call.
func installedCopyStale() (bool, error) {
	bin, err := workerBinary()
	if err != nil {
		return false, err
	}
	configDir, _ := os.UserConfigDir()
	installed := filepath.Join(configDir, "koko", "worker", "bin", "koko-worker")
	if bin == installed {
		return false, nil
	}
	a, err := fileHash(bin)
	if err != nil {
		return false, err
	}
	b, err := fileHash(installed)
	if os.IsNotExist(err) {
		return false, nil // not installed, so nothing is stale
	}
	if err != nil {
		return false, err
	}
	return a != b, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Check runs one connection check, or all of them if name is empty, and returns JSON.
func (w *WorkerService) Check(name string) (string, error) {
	args := []string{"check"}
	if name != "" {
		args = append(args, name)
	}
	return runWorker(10*time.Minute, args...)
}

// RunNow runs a job at once. With test set, it returns the message instead
// of sending it, and books nothing. The limit is long because a tono run
// reviews each new PR in turn, up to 45 minutes each.
func (w *WorkerService) RunNow(job string, test bool) (string, error) {
	args := []string{"run", job}
	if test {
		args = append(args, "--test")
	}
	return runWorker(4*time.Hour, args...)
}

// ReadLog returns the end of the worker's log.
func (w *WorkerService) ReadLog(lines int) (string, error) {
	configDir, _ := os.UserConfigDir()
	data, err := os.ReadFile(filepath.Join(configDir, "koko", "worker", "logs", "worker.log"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n"), nil
}
