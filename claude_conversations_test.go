package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeConv writes a session file and stamps its modification time.
func writeConv(t *testing.T, dir, uuid string, modTime time.Time, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, uuid+".jsonl")
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", uuid, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes %s: %v", uuid, err)
	}
	return path
}

func titleLine(title string) string {
	return fmt.Sprintf(`{"type":"ai-title","aiTitle":%q,"sessionId":"s"}`, title)
}

func userLine(cwd, text string) string {
	return fmt.Sprintf(`{"type":"user","cwd":%q,"message":{"role":"user","content":[{"type":"text","text":%q}]}}`, cwd, text)
}

func assistantLine(text string) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":%q}]}}`, text)
}

func TestReadConversationHead_PrefersClaudesOwnTitle(t *testing.T) {
	dir := t.TempDir()
	path := writeConv(t, dir, "a", time.Now(),
		titleLine("Fix the resize bug"),
		userLine("/repo", "please fix the cropping"),
	)

	head := readConversationHead(path)
	if head.title != "Fix the resize bug" {
		t.Errorf("title = %q", head.title)
	}
	if head.cwd != "/repo" {
		t.Errorf("cwd = %q", head.cwd)
	}
}

func TestReadConversationHead_FallsBackToFirstPrompt(t *testing.T) {
	dir := t.TempDir()
	// Older files have no ai-title record.
	path := writeConv(t, dir, "b", time.Now(),
		`{"type":"system","cwd":"/repo","isMeta":true}`,
		`{"type":"user","cwd":"/repo","isMeta":true,"message":{"role":"user","content":[{"type":"text","text":"injected context"}]}}`,
		`{"type":"user","cwd":"/repo","isSidechain":true,"message":{"role":"user","content":[{"type":"text","text":"subagent turn"}]}}`,
		userLine("/repo", "the real first prompt"),
	)

	head := readConversationHead(path)
	if head.title != "the real first prompt" {
		t.Errorf("expected the first real prompt, got %q", head.title)
	}
}

func TestReadConversationHead_HandlesStringContent(t *testing.T) {
	dir := t.TempDir()
	path := writeConv(t, dir, "c", time.Now(),
		`{"type":"user","cwd":"/repo","message":{"role":"user","content":"a bare string"}}`,
	)
	if got := readConversationHead(path).title; got != "a bare string" {
		t.Errorf("title = %q", got)
	}
}

// A single attachment record can be far larger than one read window. The tail
// reader must step back past it rather than stop inside it.
func TestLastAssistantText_ReadsPastAHugeRecord(t *testing.T) {
	dir := t.TempDir()
	huge := fmt.Sprintf(`{"type":"attachment","blob":%q}`, strings.Repeat("x", 200*1024))
	path := writeConv(t, dir, "d", time.Now(),
		titleLine("t"),
		assistantLine("the message worth showing"),
		huge,
		`{"type":"last-prompt"}`,
	)

	if got := lastAssistantText(path); got != "the message worth showing" {
		t.Fatalf("expected the assistant message, got %q", got)
	}
}

func TestLastAssistantText_TakesTheLastOne(t *testing.T) {
	dir := t.TempDir()
	path := writeConv(t, dir, "e", time.Now(),
		assistantLine("first"),
		assistantLine("second"),
	)
	if got := lastAssistantText(path); got != "second" {
		t.Errorf("got %q", got)
	}
}

func TestLastAssistantText_EmptyWhenNone(t *testing.T) {
	dir := t.TempDir()
	path := writeConv(t, dir, "f", time.Now(), titleLine("t"), userLine("/repo", "hello"))
	if got := lastAssistantText(path); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestListConversations_MissingDirectoryIsNotAnError(t *testing.T) {
	cs := NewClaudeService()
	got, err := cs.ListConversations(filepath.Join(t.TempDir(), "never-used"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected none, got %d", len(got))
	}
}

// The rest of ListConversations needs the real project folder, which
// claudeProjectDir derives from HOME.
func withFakeHome(t *testing.T, dir string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	projectDir := claudeProjectDir(dir)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return projectDir
}

func TestListConversations_NewestFirstAndCapped(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)

	now := time.Now()
	for i := 0; i < maxConversations+5; i++ {
		writeConv(t, projectDir, fmt.Sprintf("c%02d", i),
			now.Add(-time.Duration(i)*time.Minute),
			titleLine(fmt.Sprintf("title %02d", i)),
			userLine(workDir, "prompt"),
			assistantLine("reply"),
		)
	}
	// Not a session file, so it must be ignored.
	if err := os.WriteFile(filepath.Join(projectDir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := NewClaudeService().ListConversations(workDir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != maxConversations {
		t.Fatalf("expected %d, got %d", maxConversations, len(got))
	}
	if got[0].UUID != "c00" {
		t.Errorf("expected the newest first, got %q", got[0].UUID)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].ModifiedAt < got[i].ModifiedAt {
			t.Fatalf("not sorted newest first at %d", i)
		}
	}
	if got[0].Title != "title 00" || got[0].Preview != "reply" || got[0].SizeBytes == 0 {
		t.Errorf("unexpected fields: %+v", got[0])
	}
}

// Two directories differing only in non-alphanumerics share one project
// folder, because the key folds them to the same string. The recorded cwd is
// what separates them.
func TestListConversations_DropsAnotherDirectorySharingTheFolder(t *testing.T) {
	base := t.TempDir()
	mine := filepath.Join(base, "foo_bar")
	theirs := filepath.Join(base, "foo-bar")
	for _, d := range []string{mine, theirs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	projectDir := withFakeHome(t, mine)
	if claudeProjectDir(mine) != claudeProjectDir(theirs) {
		t.Fatal("expected both directories to share one project folder")
	}

	now := time.Now()
	writeConv(t, projectDir, "mine", now, titleLine("mine"), userLine(mine, "p"))
	writeConv(t, projectDir, "theirs", now.Add(time.Minute), titleLine("theirs"), userLine(theirs, "p"))

	got, err := NewClaudeService().ListConversations(mine)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].UUID != "mine" {
		t.Fatalf("expected only mine, got %+v", got)
	}
}

// A file with no cwd at all is kept: dropping it would hide conversations
// from older Claude versions.
func TestListConversations_KeepsAFileWithNoRecordedCwd(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	writeConv(t, projectDir, "nocwd", time.Now(), titleLine("no cwd here"))

	got, err := NewClaudeService().ListConversations(workDir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
}

func TestShorten(t *testing.T) {
	if got := shorten("  hello  ", 120); got != "hello" {
		t.Errorf("got %q", got)
	}
	long := strings.Repeat("a", 200)
	got := shorten(long, 120)
	if len([]rune(got)) != 120 || !strings.HasSuffix(got, "...") {
		t.Errorf("length %d, got suffix %q", len([]rune(got)), got[len(got)-3:])
	}
	// Counting runes, so a cut cannot split a multi-byte character.
	emoji := strings.Repeat("🎉", 200)
	if !strings.HasSuffix(shorten(emoji, 10), "...") {
		t.Error("expected an ellipsis")
	}
	if strings.ContainsRune(shorten(emoji, 10), '�') {
		t.Error("cut split a character")
	}
}
