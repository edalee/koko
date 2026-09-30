package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// addSession registers a hand-built session holding a conversation. It has no
// process, so it is only for tests that never close it.
func addSession(tm *TerminalManager, id, dir, uuid string) *session {
	s := &session{
		id:              id,
		slug:            id,
		dir:             dir,
		claudeSessionID: uuid,
		startedAt:       time.Now(),
		done:            make(chan struct{}),
		subscribers:     make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions[id] = s
	tm.mu.Unlock()
	return s
}

// fakeClaude swaps the session command for a long sleep, so the real create
// path runs without launching Claude. It records every script it was given.
func fakeClaude(t *testing.T) *[]string {
	t.Helper()
	var mu sync.Mutex
	scripts := &[]string{}
	prev := newShellCommand
	newShellCommand = func(_, script string) *exec.Cmd {
		mu.Lock()
		*scripts = append(*scripts, script)
		mu.Unlock()
		return exec.Command("/bin/sleep", "30")
	}
	t.Cleanup(func() { newShellCommand = prev })
	return scripts
}

// closeAll ends every session a test started, so no sleep outlives it.
func closeAll(t *testing.T, tm *TerminalManager) {
	t.Helper()
	t.Cleanup(func() {
		tm.mu.Lock()
		ids := make([]string, 0, len(tm.sessions))
		for id, s := range tm.sessions {
			if s.cmd != nil {
				ids = append(ids, id)
			}
		}
		tm.mu.Unlock()
		for _, id := range ids {
			_ = tm.CloseSession(id)
		}
	})
}

func TestCreateSessionWithOpts_RefusesAHeldConversation(t *testing.T) {
	tm := newTestManager()
	addSession(tm, "holder", "/repo", "conv-1")

	_, err := tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             "/repo",
		ClaudeSessionID: "conv-1",
	})
	if !errors.Is(err, ErrConversationBusy) {
		t.Fatalf("expected ErrConversationBusy, got %v", err)
	}
}

// A dead session still owns its conversation, because reconnecting that tab
// resumes it. Only the session being replaced is exempt.
func TestCreateSessionWithOpts_RefusesEvenWhenTheHolderIsDead(t *testing.T) {
	tm := newTestManager()
	holder := addSession(tm, "holder", "/repo", "conv-1")
	close(holder.done)

	_, err := tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             "/repo",
		ClaudeSessionID: "conv-1",
	})
	if !errors.Is(err, ErrConversationBusy) {
		t.Fatalf("expected ErrConversationBusy, got %v", err)
	}
}

// The trap the guard exists to avoid. readLoop leaves a dead session in the
// map and nothing removes it, so reconnect would otherwise be refused its own
// conversation. This runs the real create path.
func TestCreateSessionWithOpts_ReplaceTakesOverAndClosesTheOld(t *testing.T) {
	fakeClaude(t)
	tm := newTestManager()
	closeAll(t, tm)
	dir := t.TempDir()

	oldID, err := tm.CreateSessionWithOpts(CreateSessionOpts{Dir: dir, ClaudeSessionID: "conv-1"})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	newID, err := tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             dir,
		ClaudeSessionID: "conv-1",
		Replaces:        oldID,
	})
	if err != nil {
		t.Fatalf("reconnect was refused its own conversation: %v", err)
	}

	tm.mu.Lock()
	_, oldStill := tm.sessions[oldID]
	holder := tm.sessions[newID]
	tm.mu.Unlock()

	if oldStill {
		t.Error("the replaced session is still registered")
	}
	if holder == nil || holder.claudeSessionID != "conv-1" {
		t.Error("the new session does not hold the conversation")
	}
}

// Replaces exempts only the session it names. Without a Replaces, the same
// request is refused.
func TestCreateSessionWithOpts_WithoutReplacesTheSameRequestIsRefused(t *testing.T) {
	fakeClaude(t)
	tm := newTestManager()
	closeAll(t, tm)
	dir := t.TempDir()

	if _, err := tm.CreateSessionWithOpts(CreateSessionOpts{Dir: dir, ClaudeSessionID: "conv-1"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := tm.CreateSessionWithOpts(CreateSessionOpts{Dir: dir, ClaudeSessionID: "conv-1"})
	if !errors.Is(err, ErrConversationBusy) {
		t.Fatalf("expected ErrConversationBusy, got %v", err)
	}
}

// Plan 028 step 8: resuming needs an explicit conversation id. --continue
// resumed the newest conversation in the directory, which with a sibling
// live was the sibling's. A create with no id starts a fresh conversation
// and never guesses, even when a stored conversation is sitting there.
func TestCreateSessionWithOpts_NoIDStartsFresh(t *testing.T) {
	scripts := fakeClaude(t)
	tm := newTestManager()
	closeAll(t, tm)

	workDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	projectDir := claudeProjectDir(workDir)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A stored conversation --continue would have picked up.
	if err := os.WriteFile(filepath.Join(projectDir, "newest.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	id, err := tm.CreateSessionWithOpts(CreateSessionOpts{Dir: workDir})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	tm.mu.Lock()
	got := tm.sessions[id].claudeSessionID
	tm.mu.Unlock()
	if got != "" {
		t.Errorf("session took %q without being asked for it", got)
	}
	last := (*scripts)[len(*scripts)-1]
	if strings.Contains(last, "--continue") || strings.Contains(last, "--resume") {
		t.Errorf("expected a bare claude, got %q", last)
	}
}

// An explicit id still resumes exactly that conversation.
func TestCreateSessionWithOpts_ResumesAnExplicitID(t *testing.T) {
	scripts := fakeClaude(t)
	tm := newTestManager()
	closeAll(t, tm)

	id, err := tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             t.TempDir(),
		ClaudeSessionID: "conv-7",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	tm.mu.Lock()
	got := tm.sessions[id].claudeSessionID
	tm.mu.Unlock()
	if got != "conv-7" {
		t.Errorf("expected conv-7, got %q", got)
	}
	if !strings.HasSuffix((*scripts)[len(*scripts)-1], "--resume 'conv-7'") {
		t.Errorf("expected --resume 'conv-7', got %q", (*scripts)[len(*scripts)-1])
	}
}

// The id goes into an sh -c script and reaches here from the API, MCP and
// saved sessions. Unquoted, `x;touch pwned` ran as a second command.
func TestCreateSessionWithOpts_QuotesTheIDForTheShell(t *testing.T) {
	scripts := fakeClaude(t)
	tm := newTestManager()
	closeAll(t, tm)

	if _, err := tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             t.TempDir(),
		ClaudeSessionID: "x;touch /tmp/pwned 'quoted'",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	want := `--resume 'x;touch /tmp/pwned '\''quoted'\'''`
	if got := (*scripts)[len(*scripts)-1]; !strings.HasSuffix(got, want) {
		t.Errorf("expected the id quoted as one argument\n got: %q\nwant suffix: %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"conv-1":    `'conv-1'`,
		"a b":       `'a b'`,
		"it's":      `'it'\''s'`,
		"$(rm -rf)": `'$(rm -rf)'`,
		"":          `''`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// A reservation blocks every path, not just other creates. The detector goes
// through uuidClaimedLocked too.
func TestUUIDClaimed_SeesAReservation(t *testing.T) {
	tm := newTestManager()
	tm.mu.Lock()
	tm.pendingUUIDs["conv-1"] = "session-7"
	held := tm.uuidClaimedLocked("conv-1", "")
	ownOwn := tm.uuidClaimedLocked("conv-1", "session-7")
	tm.mu.Unlock()

	if !held {
		t.Error("a reserved conversation read as free")
	}
	if ownOwn {
		t.Error("the create holding the reservation was blocked by it")
	}
}

// Keying by owner: one create's release must not delete another's reservation.
func TestReleaseUUID_OnlyReleasesItsOwn(t *testing.T) {
	tm := newTestManager()
	tm.mu.Lock()
	tm.pendingUUIDs["conv-1"] = "session-B"
	tm.mu.Unlock()

	tm.releaseUUID("conv-1", "session-A")

	tm.mu.Lock()
	owner := tm.pendingUUIDs["conv-1"]
	tm.mu.Unlock()
	if owner != "session-B" {
		t.Fatalf("session-A released session-B's reservation, owner now %q", owner)
	}

	tm.releaseUUID("conv-1", "session-B")
	tm.mu.Lock()
	_, still := tm.pendingUUIDs["conv-1"]
	tm.mu.Unlock()
	if still {
		t.Fatal("the owner could not release its reservation")
	}
}

// A refused create must not leave the conversation reserved, or the next
// attempt would be refused too.
func TestCreateSessionWithOpts_RefusalLeavesNoReservation(t *testing.T) {
	tm := newTestManager()
	addSession(tm, "holder", "/repo", "conv-1")

	_, _ = tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             "/repo",
		ClaudeSessionID: "conv-1",
	})

	tm.mu.Lock()
	n := len(tm.pendingUUIDs)
	tm.mu.Unlock()
	if n != 0 {
		t.Fatalf("a refused create left %d reservations", n)
	}
}

// Every session without a conversation yet holds "", so an empty query must
// not match one of them.
func TestUUIDClaimed_EmptyIDIsNeverHeld(t *testing.T) {
	tm := newTestManager()
	addSession(tm, "fresh", "/repo", "")
	tm.mu.Lock()
	held := tm.uuidClaimedLocked("", "")
	tm.mu.Unlock()
	if held {
		t.Error("an empty conversation id read as held")
	}
}

// readLoop owns cmd.Wait. CloseSession used to call it too, which raced.
// Run under -race to catch a regression.
func TestCloseSession_DoesNotRaceReadLoop(t *testing.T) {
	fakeClaude(t)
	tm := newTestManager()
	id, err := tm.CreateSessionWithOpts(CreateSessionOpts{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_ = tm.CloseSession(id)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("CloseSession did not return")
	}
}
