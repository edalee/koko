package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func newTestSlackHandler(t *testing.T) *SlackCommandHandler {
	t.Helper()
	cfg := &ConfigService{
		filePath:     filepath.Join(t.TempDir(), "config.json"),
		sessionsPath: filepath.Join(t.TempDir(), "sessions.json"),
	}
	tm := NewTerminalManager()
	git := NewGitService()
	return NewSlackCommandHandler(cfg, tm, git)
}

func TestSlackCommand_Help(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("help")

	if !strings.Contains(reply, "Koko Commands") {
		t.Fatal("expected help text to contain 'Koko Commands'")
	}
	if !strings.Contains(reply, "sessions") {
		t.Fatal("expected help to mention 'sessions'")
	}
}

func TestSlackCommand_SessionsEmpty(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("sessions")

	if reply != "No active sessions." {
		t.Fatalf("expected 'No active sessions.', got %q", reply)
	}
}

func TestSlackCommand_SessionsWithData(t *testing.T) {
	h := newTestSlackHandler(t)

	s := &session{
		id:          "session-1",
		slug:        "project/1",
		name:        "My Session",
		dir:         "/tmp/project",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	h.tm.mu.Lock()
	h.tm.sessions["session-1"] = s
	h.tm.mu.Unlock()

	reply := h.handleCommand("sessions")

	if !strings.Contains(reply, "My Session") {
		t.Fatalf("expected session name in reply, got %q", reply)
	}
	if !strings.Contains(reply, "project/1") {
		t.Fatalf("expected slug in reply, got %q", reply)
	}
}

func TestSlackCommand_ResolveBySlug(t *testing.T) {
	h := newTestSlackHandler(t)

	s := &session{
		id:          "session-1",
		slug:        "koko/1",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	s.tailText.Write([]byte("output from koko"))
	h.tm.mu.Lock()
	h.tm.sessions["session-1"] = s
	h.tm.mu.Unlock()

	// Resolve by slug
	reply := h.handleCommand("status koko/1")
	if !strings.Contains(reply, "output from koko") {
		t.Fatalf("expected output via slug, got %q", reply)
	}

	// Resolve by PTY ID still works
	reply = h.handleCommand("status session-1")
	if !strings.Contains(reply, "output from koko") {
		t.Fatalf("expected output via PTY ID, got %q", reply)
	}
}

func TestSlackCommand_StatusNoArgs(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("status")

	if !strings.Contains(reply, "Usage") {
		t.Fatalf("expected usage message, got %q", reply)
	}
}

func TestSlackCommand_StatusDefaultsToSingle(t *testing.T) {
	h := newTestSlackHandler(t)

	s := &session{
		id:          "session-1",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	s.tailText.Write([]byte("hello world"))
	h.tm.mu.Lock()
	h.tm.sessions["session-1"] = s
	h.tm.mu.Unlock()

	reply := h.handleCommand("status")

	if !strings.Contains(reply, "hello world") {
		t.Fatalf("expected output in status, got %q", reply)
	}
}

func TestSlackCommand_StatusNotFound(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("status nonexistent")

	if !strings.Contains(reply, "not found") {
		t.Fatalf("expected 'not found', got %q", reply)
	}
}

func TestSlackCommand_SendMissingArgs(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("send")

	if !strings.Contains(reply, "Usage") {
		t.Fatalf("expected usage, got %q", reply)
	}
}

func TestSlackCommand_SendInsufficientArgs(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("send session-1")

	if !strings.Contains(reply, "Usage") {
		t.Fatalf("expected usage, got %q", reply)
	}
}

func TestSlackCommand_OutputNoArgs(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("output")

	if !strings.Contains(reply, "Usage") {
		t.Fatalf("expected usage, got %q", reply)
	}
}

func TestSlackCommand_OutputWithSession(t *testing.T) {
	h := newTestSlackHandler(t)

	s := &session{
		id:          "session-1",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	s.tailText.Write([]byte("line1\nline2\nline3"))
	h.tm.mu.Lock()
	h.tm.sessions["session-1"] = s
	h.tm.mu.Unlock()

	reply := h.handleCommand("output session-1")

	if !strings.Contains(reply, "line1") {
		t.Fatalf("expected output, got %q", reply)
	}
}

func TestSlackCommand_OutputEmpty(t *testing.T) {
	h := newTestSlackHandler(t)

	s := &session{
		id:          "session-1",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	h.tm.mu.Lock()
	h.tm.sessions["session-1"] = s
	h.tm.mu.Unlock()

	reply := h.handleCommand("output session-1")

	if reply != "(no output)" {
		t.Fatalf("expected '(no output)', got %q", reply)
	}
}

func TestSlackCommand_FilesNotFound(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("files nonexistent")

	if !strings.Contains(reply, "not found") {
		t.Fatalf("expected 'not found', got %q", reply)
	}
}

func TestSlackCommand_Unknown(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("foobar")

	if !strings.Contains(reply, "Unknown command") {
		t.Fatalf("expected 'Unknown command', got %q", reply)
	}
}

func TestSlackCommand_EmptyInput(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("")

	if reply != "" {
		t.Fatalf("expected empty reply for empty input, got %q", reply)
	}
}

func TestSlackCommand_CaseInsensitive(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("HELP")

	if !strings.Contains(reply, "Koko Commands") {
		t.Fatalf("expected help text for uppercase HELP, got %q", reply)
	}
}

func TestSlackCommand_WhitespaceHandling(t *testing.T) {
	h := newTestSlackHandler(t)
	reply := h.handleCommand("  help  ")

	if !strings.Contains(reply, "Koko Commands") {
		t.Fatalf("expected help text for padded input, got %q", reply)
	}
}
