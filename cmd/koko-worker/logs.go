package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// logCap is the size at which worker.log moves to worker.log.1, so the
	// scheduler's log takes at most about twice this on disk.
	logCap = 5 << 20
	// reviewLogRetention is how long a review run's log is kept. It matches
	// pruneState, which forgets a review's result after 60 days.
	reviewLogRetention = 60 * 24 * time.Hour
)

// rotatingLog is the scheduler's worker.log. Once a write would take it past
// limit, it moves the file to worker.log.1, replacing the older one, and
// starts a new file. The worker owns this file: launchd sends the agent's own
// stdout and stderr to agentStderrPath, so a rename never leaves launchd
// writing to the old file.
type rotatingLog struct {
	mu    sync.Mutex
	path  string
	limit int64
	f     *os.File
	size  int64
}

func openRotatingLog(path string, limit int64) (*rotatingLog, error) {
	l := &rotatingLog{path: path, limit: limit}
	return l, l.open()
}

func (l *rotatingLog) open() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	l.f, l.size = f, info.Size()
	return nil
}

func (l *rotatingLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.size > 0 && l.size+int64(len(p)) > l.limit {
		_ = l.f.Close()
		if err := os.Rename(l.path, l.path+".1"); err != nil && !os.IsNotExist(err) {
			return 0, err
		}
		if err := l.open(); err != nil {
			return 0, err
		}
	}
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}

func (l *rotatingLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// agentStderrPath is where launchd sends the agent's own stdout and stderr,
// such as a crash. The scheduler's log goes to worker.log instead.
func agentStderrPath(paths Paths) string { return filepath.Join(paths.Logs, "agent-stderr.log") }

// pruneReviewLogs deletes review run logs (tono-*.log) older than
// reviewLogRetention. worker.log and the stderr log are left alone.
func pruneReviewLogs(dir string, now time.Time) (removed int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "tono-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) <= reviewLogRetention {
			continue
		}
		if os.Remove(filepath.Join(dir, name)) == nil {
			removed++
		}
	}
	return removed
}
