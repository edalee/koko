package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const agentLabel = "com.koko.worker"

// agentPATH is the PATH for launchd, which otherwise has only /usr/bin:/bin.
func agentPATH() string {
	home, _ := os.UserHomeDir()
	return strings.Join([]string{
		filepath.Join(home, ".local", "bin"), "/opt/homebrew/bin", "/usr/local/bin",
		"/usr/bin", "/bin", "/usr/sbin", "/sbin",
	}, ":")
}

// workerPATH puts agentPATH's folders first, then keeps the rest of current.
// Koko started from the Dock has a bare PATH without gh or claude.
func workerPATH(current string) string {
	parts := strings.Split(agentPATH(), ":")
	seen := map[string]bool{}
	for _, p := range parts {
		seen[p] = true
	}
	for _, p := range strings.Split(current, ":") {
		if p != "" && !seen[p] {
			parts = append(parts, p)
			seen[p] = true
		}
	}
	return strings.Join(parts, ":")
}

func agentPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", agentLabel+".plist")
}

func agentLogPath(paths Paths) string { return filepath.Join(paths.Logs, "worker.log") }

func launchDomain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

// install copies this binary to a fixed path and starts it as a launchd agent.
// The fixed path means a rebuild or a moved Koko.app never breaks the agent.
func install(paths Paths) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	bin := filepath.Join(paths.Dir, "bin", "koko-worker")
	if err := os.MkdirAll(filepath.Dir(bin), 0o700); err != nil {
		return err
	}
	if err := copyFile(self, bin); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}

	logPath := agentLogPath(paths)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key><array><string>%s</string><string>serve</string></array>
  <key>WorkingDirectory</key><string>%s</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ThrottleInterval</key><integer>30</integer>
  <key>EnvironmentVariables</key><dict><key>PATH</key><string>%s</string></dict>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, agentLabel, bin, paths.Dir, agentPATH(), logPath, logPath)
	if err := os.MkdirAll(filepath.Dir(agentPlistPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(agentPlistPath(), []byte(plist), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", launchDomain()+"/"+agentLabel).Run()
	if out, err := exec.Command("launchctl", "bootstrap", launchDomain(), agentPlistPath()).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %s", strings.TrimSpace(string(out)))
	}
	fmt.Println("koko-worker agent installed and started")
	return nil
}

// uninstall stops the agent and removes its plist. Config, state and logs stay.
func uninstall(paths Paths) error {
	_ = exec.Command("launchctl", "bootout", launchDomain()+"/"+agentLabel).Run()
	if err := os.Remove(agentPlistPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	st := loadState(paths.State)
	if !st.WakeBooked.IsZero() && st.WakeBooked.After(time.Now()) {
		_ = pmset(context.Background(), "cancel", st.WakeBooked)
		st.WakeBooked = time.Time{}
		_ = saveState(paths.State, st)
	}
	fmt.Println("koko-worker agent stopped and removed")
	return nil
}

func agentLoaded() bool {
	return exec.Command("launchctl", "print", launchDomain()+"/"+agentLabel).Run() == nil
}

func copyFile(src, dst string) error {
	if src == dst {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// ---- Mac wake ----

// sudoersRule is what you add with `sudo visudo -f /etc/sudoers.d/koko-worker`.
// It allows booking and cancelling wakes, and nothing else.
func sudoersRule() string {
	user := os.Getenv("USER")
	return fmt.Sprintf("%s ALL=(root) NOPASSWD: /usr/bin/pmset schedule wake *, /usr/bin/pmset schedule cancel wake *", user)
}

// pmset books or cancels one wake. sudo -n fails at once if the sudoers rule
// is missing, instead of waiting for a password.
func pmset(ctx context.Context, action string, at time.Time) error {
	stamp := at.In(time.Local).Format("01/02/06 15:04:05")
	args := []string{"-n", "/usr/bin/pmset", "schedule"}
	if action == "cancel" {
		args = append(args, "cancel")
	}
	args = append(args, "wake", stamp)
	out, err := exec.CommandContext(ctx, "sudo", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pmset %s wake %s: %s", action, stamp, strings.TrimSpace(string(out)))
	}
	return nil
}

// bookWake keeps one wake booked, two minutes before the next slot. If the
// slots change, it cancels the old wake first.
func bookWake(ctx context.Context, cfg Config, st *State, now time.Time) {
	want, ok := nextWake(cfg, now)
	if !ok || !want.After(now) {
		// No slot ahead, or the wake for the next slot has already passed.
		return
	}
	if st.WakeBooked.Equal(want) || st.WakeTried.Equal(want) {
		// Booked already, or tried and failed: one log line per wake, not one per tick.
		return
	}
	st.WakeTried = want
	if !st.WakeBooked.IsZero() && st.WakeBooked.After(now) {
		if err := pmset(ctx, "cancel", st.WakeBooked); err != nil {
			log.Printf("wake: %v", err)
		}
		st.WakeBooked = time.Time{}
	}
	if err := pmset(ctx, "book", want); err != nil {
		log.Printf("wake: %v", err)
		return
	}
	st.WakeBooked = want
	log.Printf("wake: booked %s", want.In(time.Local).Format("Mon 15:04"))
}
