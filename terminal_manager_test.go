package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRingBuffer_Basic(t *testing.T) {
	rb := newRingBuffer(10)
	rb.Write([]byte("hello"))

	got := string(rb.Bytes())
	if got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestRingBuffer_Wrap(t *testing.T) {
	rb := newRingBuffer(5)
	rb.Write([]byte("abcdefgh")) // 8 bytes into 5-byte buffer

	got := string(rb.Bytes())
	if got != "defgh" {
		t.Fatalf("expected 'defgh', got %q", got)
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := newRingBuffer(10)
	got := rb.Bytes()
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(got))
	}
}

func TestRingBuffer_ExactSize(t *testing.T) {
	rb := newRingBuffer(5)
	rb.Write([]byte("abcde"))

	got := string(rb.Bytes())
	if got != "abcde" {
		t.Fatalf("expected 'abcde', got %q", got)
	}
}

func TestSubscribe_NonexistentSession(t *testing.T) {
	tm := NewTerminalManager()
	ch := tm.Subscribe("nonexistent")
	if ch != nil {
		t.Fatal("expected nil channel for nonexistent session")
	}
}

func TestSubscribe_Unsubscribe(t *testing.T) {
	tm := NewTerminalManager()

	// Manually inject a session (bypasses PTY creation)
	s := &session{
		id:          "test-1",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions["test-1"] = s
	tm.mu.Unlock()

	ch := tm.Subscribe("test-1")
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	s.mu.Lock()
	count := len(s.subscribers)
	s.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 subscriber, got %d", count)
	}

	tm.Unsubscribe("test-1", ch)

	s.mu.Lock()
	count = len(s.subscribers)
	s.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected 0 subscribers, got %d", count)
	}
}

func TestSubscriberFanOut(t *testing.T) {
	tm := NewTerminalManager()

	s := &session{
		id:          "test-2",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions["test-2"] = s
	tm.mu.Unlock()

	ch1 := tm.Subscribe("test-2")
	ch2 := tm.Subscribe("test-2")

	// Simulate what readLoop does: fan out data to subscribers
	data := []byte("test output")
	s.mu.Lock()
	for ch := range s.subscribers {
		select {
		case ch <- append([]byte(nil), data...):
		default:
		}
	}
	s.mu.Unlock()

	// Both subscribers should receive the data
	select {
	case got := <-ch1:
		if string(got) != "test output" {
			t.Fatalf("ch1: expected 'test output', got %q", string(got))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch1: timeout waiting for data")
	}

	select {
	case got := <-ch2:
		if string(got) != "test output" {
			t.Fatalf("ch2: expected 'test output', got %q", string(got))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch2: timeout waiting for data")
	}
}

func TestReadOutput(t *testing.T) {
	tm := NewTerminalManager()

	s := &session{
		id:          "test-3",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	s.tailText.Write([]byte("some terminal output"))

	tm.mu.Lock()
	tm.sessions["test-3"] = s
	tm.mu.Unlock()

	output, err := tm.ReadOutput("test-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "some terminal output" {
		t.Fatalf("expected 'some terminal output', got %q", output)
	}
}

func TestReadOutput_NotFound(t *testing.T) {
	tm := NewTerminalManager()

	_, err := tm.ReadOutput("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestReadOutput_NilTailText(t *testing.T) {
	tm := NewTerminalManager()

	s := &session{
		id:          "test-4",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions["test-4"] = s
	tm.mu.Unlock()

	output, err := tm.ReadOutput("test-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected empty, got %q", output)
	}
}

func TestGetSessionState_NonexistentReturnsIdle(t *testing.T) {
	tm := NewTerminalManager()
	state := tm.GetSessionState("nonexistent")
	if state != "idle" {
		t.Fatalf("expected 'idle', got %q", state)
	}
}

// --- Claude session UUID capture ---

// newTestManager builds a manager without resolveLoginPath, which spawns an
// interactive login shell and costs over a second per call.
func newTestManager() *TerminalManager {
	return &TerminalManager{
		sessions:  make(map[string]*session),
		slugCount: make(map[string]int),
	}
}

// newDetectSession builds a session for detectClaudeSessionID tests.
// A zero firstInputAt means the user has not typed yet.
func newDetectSession(id string, startedAt, firstInputAt time.Time) *session {
	return &session{
		id:           id,
		slug:         id,
		dir:          "/tmp",
		startedAt:    startedAt,
		firstInputAt: firstInputAt,
		done:         make(chan struct{}),
		subscribers:  make(map[chan []byte]struct{}),
	}
}

func writeJSONL(t *testing.T, dir, name string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes %s: %v", name, err)
	}
}

// waitForUUID polls the session for up to d, so tests do not depend on the
// detector's exact poll interval.
func waitForUUID(s *session, d time.Duration) string {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		got := s.claudeSessionID
		s.mu.Unlock()
		if got != "" {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ""
}

func TestClaudeProjectDir_ReplacesSlashAndDot(t *testing.T) {
	// Claude maps both separators to "-", so a dotted path must not be
	// left with its dots intact.
	got := claudeProjectDir("/Users/e/.claude/plugins/pkg-0.4.1")
	want := "-Users-e--claude-plugins-pkg-0-4-1"
	if filepath.Base(got) != want {
		t.Fatalf("expected key %q, got %q", want, filepath.Base(got))
	}
}

func TestDetectClaudeSessionID_CapturesNewFile(t *testing.T) {
	dir := t.TempDir()
	tm := newTestManager()
	s := newDetectSession("test-1", time.Now().Add(-time.Minute), time.Now())

	// A file that was already there must be ignored.
	writeJSONL(t, dir, "old.jsonl", time.Now())
	preExisting := listJSONL(dir)

	go tm.detectClaudeSessionID(s, dir, preExisting)
	defer close(s.done)

	writeJSONL(t, dir, "new-uuid.jsonl", time.Now())

	if got := waitForUUID(s, 3*time.Second); got != "new-uuid" {
		t.Fatalf("expected new-uuid, got %q", got)
	}
}

func TestDetectClaudeSessionID_WaitsForFirstInput(t *testing.T) {
	dir := t.TempDir()
	tm := newTestManager()
	// Nothing typed, so any file here belongs to someone else.
	s := newDetectSession("test-idle", time.Now().Add(-time.Minute), time.Time{})

	preExisting := listJSONL(dir)
	writeJSONL(t, dir, "someone-else.jsonl", time.Now())

	go tm.detectClaudeSessionID(s, dir, preExisting)
	defer close(s.done)

	if got := waitForUUID(s, 1500*time.Millisecond); got != "" {
		t.Fatalf("expected no capture, got %q", got)
	}
}

func TestDetectClaudeSessionID_GivesUpAfterCaptureWindow(t *testing.T) {
	dir := t.TempDir()
	tm := newTestManager()
	// Typed long ago, so the capture window has closed.
	started := time.Now().Add(-2 * captureWindow)
	s := newDetectSession("test-late", started, started)

	preExisting := listJSONL(dir)

	stopped := make(chan struct{})
	go func() {
		tm.detectClaudeSessionID(s, dir, preExisting)
		close(stopped)
	}()
	defer close(s.done)

	writeJSONL(t, dir, "too-late.jsonl", time.Now())

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("detector kept polling past the capture window")
	}
	s.mu.Lock()
	got := s.claudeSessionID
	s.mu.Unlock()
	if got != "" {
		t.Fatalf("expected no capture, got %q", got)
	}
}

func TestDetectClaudeSessionID_SkipsWhenAnotherTypedSessionShareDir(t *testing.T) {
	dir := t.TempDir()
	tm := newTestManager()

	// A rival in the same directory has been typed into and has no UUID, so
	// a new file could belong to either session.
	rival := newDetectSession("test-rival", time.Now().Add(-time.Minute), time.Now())
	s := newDetectSession("test-amb", time.Now().Add(-time.Minute), time.Now())
	tm.mu.Lock()
	tm.sessions[rival.id] = rival
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	preExisting := listJSONL(dir)
	writeJSONL(t, dir, "whose-is-it.jsonl", time.Now())

	go tm.detectClaudeSessionID(s, dir, preExisting)
	defer close(s.done)

	if got := waitForUUID(s, 1500*time.Millisecond); got != "" {
		t.Fatalf("expected no capture while ambiguous, got %q", got)
	}
}

func TestDetectClaudeSessionID_SkipsUUIDHeldByLiveSession(t *testing.T) {
	dir := t.TempDir()
	tm := newTestManager()

	// A sibling session already owns this UUID. It is identified, so it does
	// not make the directory ambiguous.
	owner := newDetectSession("test-owner", time.Now().Add(-time.Minute), time.Now())
	owner.claudeSessionID = "shared-uuid"
	tm.mu.Lock()
	tm.sessions[owner.id] = owner
	tm.mu.Unlock()

	s := newDetectSession("test-2", time.Now().Add(-time.Minute), time.Now())
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	// Snapshot before writing, so the file counts as new and only the claim
	// check can stop it being captured.
	preExisting := listJSONL(dir)
	writeJSONL(t, dir, "shared-uuid.jsonl", time.Now())

	go tm.detectClaudeSessionID(s, dir, preExisting)
	defer close(s.done)

	if got := waitForUUID(s, 1500*time.Millisecond); got != "" {
		t.Fatalf("expected no capture, got %q", got)
	}
}

// A claimed file must not be blacklisted. Blacklisting turned one bad guess
// into a session that could never capture anything.
func TestDetectClaudeSessionID_RecoversAfterClaimClears(t *testing.T) {
	dir := t.TempDir()
	tm := newTestManager()

	owner := newDetectSession("test-owner2", time.Now().Add(-time.Minute), time.Now())
	owner.claudeSessionID = "contested"
	tm.mu.Lock()
	tm.sessions[owner.id] = owner
	tm.mu.Unlock()

	s := newDetectSession("test-3", time.Now().Add(-time.Minute), time.Now())
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	preExisting := listJSONL(dir)
	writeJSONL(t, dir, "contested.jsonl", time.Now())

	go tm.detectClaudeSessionID(s, dir, preExisting)
	defer close(s.done)

	if got := waitForUUID(s, 1*time.Second); got != "" {
		t.Fatalf("expected no capture while claimed, got %q", got)
	}

	// The owner goes away, so the file is free.
	tm.mu.Lock()
	delete(tm.sessions, owner.id)
	tm.mu.Unlock()

	if got := waitForUUID(s, 3*time.Second); got != "contested" {
		t.Fatalf("expected contested after the claim cleared, got %q", got)
	}
}

func TestDetectClaudeSessionID_SkipsFileOlderThanSession(t *testing.T) {
	dir := t.TempDir()
	tm := newTestManager()
	now := time.Now()
	s := newDetectSession("test-4", now, now)

	// Not in preExisting, but written before the session launched.
	preExisting := listJSONL(dir)
	writeJSONL(t, dir, "stale.jsonl", now.Add(-time.Hour))

	go tm.detectClaudeSessionID(s, dir, preExisting)
	defer close(s.done)

	if got := waitForUUID(s, 1500*time.Millisecond); got != "" {
		t.Fatalf("expected no capture, got %q", got)
	}
}

func TestDetectClaudeSessionID_StopsWhenSessionEnds(t *testing.T) {
	dir := t.TempDir()
	tm := newTestManager()
	s := newDetectSession("test-5", time.Now().Add(-time.Minute), time.Now())

	stopped := make(chan struct{})
	go func() {
		tm.detectClaudeSessionID(s, dir, listJSONL(dir))
		close(stopped)
	}()

	close(s.done)

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("detector did not stop when the session ended")
	}
}

func TestMostRecentJSONL(t *testing.T) {
	dir := t.TempDir()
	if got := mostRecentJSONL(dir); got != "" {
		t.Fatalf("expected empty for empty dir, got %q", got)
	}
	if got := mostRecentJSONL(""); got != "" {
		t.Fatalf("expected empty for empty path, got %q", got)
	}

	writeJSONL(t, dir, "older.jsonl", time.Now().Add(-time.Hour))
	writeJSONL(t, dir, "newer.jsonl", time.Now())
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := mostRecentJSONL(dir); got != "newer" {
		t.Fatalf("expected newer, got %q", got)
	}
}
