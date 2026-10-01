package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// newDeleteApp returns an App over a fake HOME, so its ConfigService and the
// Claude project folder both live in temp directories.
func newDeleteApp(t *testing.T, workDir string) (*App, string) {
	t.Helper()
	projectDir := withFakeHome(t, workDir)
	return &App{tm: newTestManager(), cfg: NewConfigService()}, projectDir
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestDeleteConversation_RemovesTheFileAndItsFolder(t *testing.T) {
	workDir := t.TempDir()
	app, projectDir := newDeleteApp(t, workDir)
	path := writeConv(t, projectDir, "conv-1", time.Now(), userLine(workDir, "p"))
	side := filepath.Join(projectDir, "conv-1", "subagents")
	if err := os.MkdirAll(side, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := app.DeleteConversation(workDir, "conv-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if exists(path) || exists(filepath.Join(projectDir, "conv-1")) {
		t.Error("the session file or its folder survived")
	}
}

func TestDeleteConversation_RefusesOneASessionHolds(t *testing.T) {
	workDir := t.TempDir()
	app, projectDir := newDeleteApp(t, workDir)
	path := writeConv(t, projectDir, "conv-1", time.Now(), userLine(workDir, "p"))
	addSession(app.tm, "koko-1", workDir, "conv-1")

	err := app.DeleteConversation(workDir, "conv-1")
	if !errors.Is(err, ErrConversationHeld) {
		t.Fatalf("expected ErrConversationHeld, got %v", err)
	}
	if !exists(path) {
		t.Error("a held conversation was deleted")
	}
}

// After a restart a disconnected tab has no session, but it still holds its
// conversation, because reconnecting resumes it.
func TestDeleteConversation_RefusesOneASavedTabHolds(t *testing.T) {
	workDir := t.TempDir()
	app, projectDir := newDeleteApp(t, workDir)
	path := writeConv(t, projectDir, "conv-1", time.Now(), userLine(workDir, "p"))
	if err := app.cfg.SaveSessions(SessionsData{Sessions: []SessionRecord{
		{Slug: "koko-1", Directory: workDir, ClaudeSessionID: "conv-1", Status: "disconnected"},
	}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := app.DeleteConversation(workDir, "conv-1"); !errors.Is(err, ErrConversationHeld) {
		t.Fatalf("expected ErrConversationHeld, got %v", err)
	}
	if !exists(path) {
		t.Error("a saved tab's conversation was deleted")
	}
}

// A closed session no longer holds its conversation.
func TestDeleteConversation_AClosedRecordDoesNotHold(t *testing.T) {
	workDir := t.TempDir()
	app, projectDir := newDeleteApp(t, workDir)
	writeConv(t, projectDir, "conv-1", time.Now(), userLine(workDir, "p"))
	if err := app.cfg.SaveSessions(SessionsData{Sessions: []SessionRecord{
		{Slug: "koko-1", Directory: workDir, ClaudeSessionID: "conv-1", Status: "closed"},
	}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := app.DeleteConversation(workDir, "conv-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestDeleteConversation_RefusesAReservation(t *testing.T) {
	workDir := t.TempDir()
	app, projectDir := newDeleteApp(t, workDir)
	writeConv(t, projectDir, "conv-1", time.Now(), userLine(workDir, "p"))
	app.tm.mu.Lock()
	app.tm.pendingUUIDs["conv-1"] = "session-7"
	app.tm.mu.Unlock()

	if err := app.DeleteConversation(workDir, "conv-1"); !errors.Is(err, ErrConversationHeld) {
		t.Fatalf("expected ErrConversationHeld, got %v", err)
	}
}

// The id comes from the webview, so it must never reach outside the
// directory's project folder.
func TestDeleteConversation_RejectsAPathAsTheID(t *testing.T) {
	workDir := t.TempDir()
	app, projectDir := newDeleteApp(t, workDir)
	outside := filepath.Join(filepath.Dir(projectDir), "victim")
	if err := os.WriteFile(outside+".jsonl", []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, id := range []string{"", ".", "..", "../victim", "a/b"} {
		if err := app.DeleteConversation(workDir, id); err == nil {
			t.Errorf("id %q was accepted", id)
		}
	}
	if !exists(outside + ".jsonl") {
		t.Error("a file outside the project folder was deleted")
	}
}

func TestDeleteConversation_RefusesAnotherDirectorysConversation(t *testing.T) {
	base := t.TempDir()
	mine := filepath.Join(base, "foo_bar")
	theirs := filepath.Join(base, "foo-bar")
	for _, d := range []string{mine, theirs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	app, projectDir := newDeleteApp(t, mine)
	path := writeConv(t, projectDir, "theirs", time.Now(), userLine(theirs, "p"))

	if err := app.DeleteConversation(mine, "theirs"); err == nil {
		t.Fatal("deleted a conversation that belongs to another directory")
	}
	if !exists(path) {
		t.Error("the neighbour's conversation is gone")
	}
}

// Bulk delete uses the listing's directory test, but leaves out files with no
// recorded cwd, which in a shared folder may be a neighbour's.
func TestDeleteConversations_KeepsNeighboursHeldAndUnknown(t *testing.T) {
	base := t.TempDir()
	mine := filepath.Join(base, "foo_bar")
	theirs := filepath.Join(base, "foo-bar")
	for _, d := range []string{mine, theirs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	app, projectDir := newDeleteApp(t, mine)
	now := time.Now()
	writeConv(t, projectDir, "a", now, userLine(mine, "p"))
	writeConv(t, projectDir, "b", now, userLine(mine, "p"))
	held := writeConv(t, projectDir, "held", now, userLine(mine, "p"))
	neighbour := writeConv(t, projectDir, "theirs", now, userLine(theirs, "p"))
	unknown := writeConv(t, projectDir, "nocwd", now, titleLine("old"))
	addSession(app.tm, "koko-1", mine, "held")

	got, err := app.DeleteConversations(mine)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	sort.Strings(got.Deleted)
	if len(got.Deleted) != 2 || got.Deleted[0] != "a" || got.Deleted[1] != "b" {
		t.Errorf("deleted %v, want [a b]", got.Deleted)
	}
	if got.Skipped != 2 {
		t.Errorf("skipped %d, want 2 (held and unknown)", got.Skipped)
	}
	for _, p := range []string{held, neighbour, unknown} {
		if !exists(p) {
			t.Errorf("%s was deleted", filepath.Base(p))
		}
	}
}

// The picker shows 20, but "delete all" means all of them.
func TestDeleteConversations_HasNoCap(t *testing.T) {
	workDir := t.TempDir()
	app, projectDir := newDeleteApp(t, workDir)
	for i := 0; i < maxConversations+5; i++ {
		writeConv(t, projectDir, "c"+string(rune('a'+i)), time.Now(), userLine(workDir, "p"))
	}

	got, err := app.DeleteConversations(workDir)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(got.Deleted) != maxConversations+5 {
		t.Errorf("deleted %d, want %d", len(got.Deleted), maxConversations+5)
	}
}

func TestDeleteConversations_EmptyDirectoryReportsNothing(t *testing.T) {
	workDir := t.TempDir()
	app, _ := newDeleteApp(t, workDir)

	got, err := app.DeleteConversations(workDir)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got.Deleted == nil || len(got.Deleted) != 0 || got.Skipped != 0 {
		t.Errorf("unexpected result %+v", got)
	}
}
