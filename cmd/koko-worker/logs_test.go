package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotatingLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.log")
	l, err := openRotatingLog(path, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	for _, line := range []string{"first line\n", "second line\n", "third line\n"} {
		if _, err := l.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	cur, _ := os.ReadFile(path)
	old, _ := os.ReadFile(path + ".1")
	// Each line would take the file past 20 bytes, so each rotation keeps
	// only the previous line in worker.log.1.
	if string(cur) != "third line\n" || string(old) != "second line\n" {
		t.Errorf("worker.log %q, worker.log.1 %q", cur, old)
	}
}

func TestRotatingLogAppendsToAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worker.log")
	if err := os.WriteFile(path, []byte("before restart\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := openRotatingLog(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = l.Write([]byte("after restart\n"))
	_ = l.Close()
	if got, _ := os.ReadFile(path); string(got) != "before restart\nafter restart\n" {
		t.Errorf("got %q", got)
	}
}

func TestPruneReviewLogs(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	files := map[string]time.Duration{
		"tono-kalimba-28-abc.log": 61 * 24 * time.Hour, // old review log: removed
		"tono-zufolo-554-def.log": 59 * 24 * time.Hour, // recent: kept
		"worker.log":              90 * 24 * time.Hour, // not a review log: kept
		"agent-stderr.log":        90 * 24 * time.Hour,
		"tono-notes.txt":          90 * 24 * time.Hour,
	}
	for name, age := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	if n := pruneReviewLogs(dir, now); n != 1 {
		t.Errorf("removed %d, want 1", n)
	}
	entries, _ := os.ReadDir(dir)
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if got := strings.Join(left, ","); got != "agent-stderr.log,tono-notes.txt,tono-zufolo-554-def.log,worker.log" {
		t.Errorf("left %s", got)
	}
}
