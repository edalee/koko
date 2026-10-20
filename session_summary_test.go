package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// copyFixture copies a transcript from testdata into a project folder as
// uuid.jsonl, and stamps its modification time. The fixtures record their cwd
// as /work/app, which becomes workDir.
func copyFixture(t *testing.T, projectDir, workDir, fixture, uuid string, modTime time.Time) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	body = []byte(strings.ReplaceAll(string(body), `"cwd":"/work/app"`, fmt.Sprintf(`"cwd":%q`, workDir)))
	path := filepath.Join(projectDir, uuid+".jsonl")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", uuid, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes %s: %v", uuid, err)
	}
	return path
}

func ms(ts string) int64 {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		panic(err)
	}
	return t.UnixMilli()
}

func humanLine(text string) string {
	return fmt.Sprintf(`{"type":"user","origin":{"kind":"human"},"message":{"role":"user","content":%q},"entrypoint":"cli","timestamp":"2026-10-01T10:00:00Z"}`, text)
}

func branchLine(branch string) string {
	return fmt.Sprintf(`{"type":"assistant","message":{"role":"assistant","content":[]},"gitBranch":%q}`, branch)
}

func prLine(repo, url string) string {
	return fmt.Sprintf(`{"type":"pr-link","prNumber":7,"prRepository":%q,"prUrl":%q}`, repo, url)
}

// The fixture mixes the user's prompts with tool results, `!` and `/`
// commands, a task notification, a message from another session and injected
// context. Only the two typed prompts may come through, newest first.
func TestGetSessionSummary_ReadsOnlyWhatTheUserAskedAndTheLatestRecords(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	modTime := time.Now().Add(-time.Hour)
	copyFixture(t, projectDir, workDir, "session-mixed.jsonl", "mixed", modTime)

	got, err := NewClaudeService().GetSessionSummary(workDir, "mixed")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	want := SessionSummary{
		Title:      "Retry uploads with back-off",
		Recap:      "Added retries with back-off to the upload client and opened a PR.",
		LastPrompt: "Now make the retry back off exponentially",
		RecentPrompts: []SessionPrompt{
			{Text: "Now make the retry back off exponentially", Timestamp: ms("2026-10-01T10:05:00Z")},
			{Text: "Add a retry to the upload client", Timestamp: ms("2026-10-01T10:00:00Z")},
		},
		Branch:    "feat/retry-uploads",
		PR:        &SessionPR{Number: 42, Repo: "acme/app", URL: "https://github.com/acme/app/pull/42"},
		UpdatedAt: modTime.UnixMilli(),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("summary mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestSessionTicket_Order(t *testing.T) {
	pr := &SessionPR{Repo: "acme/app", URL: "https://example.test/browse/PRK-2"}
	prompts := []SessionPrompt{{Text: "newest mentions NEW-5"}, {Text: "older mentions OLD-6"}}
	cases := []struct {
		name string
		s    SessionSummary
		want string
	}{
		{"branch wins", SessionSummary{Branch: "feat/ACME-123-retry", PR: pr, Title: "TTL-3 title", RecentPrompts: prompts}, "ACME-123"},
		{"then the PR", SessionSummary{Branch: "main", PR: pr, Title: "TTL-3 title", RecentPrompts: prompts}, "PRK-2"},
		{"then the title", SessionSummary{Branch: "main", Title: "Fix TTL-3 title", RecentPrompts: prompts}, "TTL-3"},
		{"then the newest prompt", SessionSummary{Branch: "main", Title: "no key", RecentPrompts: prompts}, "NEW-5"},
		{"lower case is not a key", SessionSummary{Branch: "feat/acme-123"}, ""},
		{"UTF-8 is not a key", SessionSummary{Title: "fix UTF-8 decoding"}, ""},
		{"SHA-256 is not a key", SessionSummary{RecentPrompts: []SessionPrompt{{Text: "use SHA-256"}}}, ""},
		{"CVE-2024-1234 is not a key", SessionSummary{Title: "patch CVE-2024-1234"}, ""},
		{"GPT-4 is not a key", SessionSummary{Title: "try GPT-4"}, ""},
		{"ISO-8601 is not a key", SessionSummary{Title: "ISO-8601 dates"}, ""},
		{"a term does not hide a key after it", SessionSummary{Title: "UTF-8 bug in ACME-77"}, "ACME-77"},
		{"a digit in the project is not a key", SessionSummary{Title: "see A1B-2"}, ""},
		{"none", SessionSummary{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sessionTicket(c.s); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// The ticket comes through GetSessionSummary too, from the branch.
func TestGetSessionSummary_TicketFromBranch(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	writeConv(t, projectDir, "tkt", time.Now(),
		humanLine("look at ACME-9 please"),
		prLine("acme/app", "https://github.com/acme/app/pull/7"),
		branchLine("feat/ACME-123-retry"),
	)
	got, _ := NewClaudeService().GetSessionSummary(workDir, "tkt")
	if got.Ticket != "ACME-123" {
		t.Errorf("ticket = %q, want ACME-123", got.Ticket)
	}
}

// Two tabs in one repo. Each must get its own conversation, even when the
// other file is newer. The old preview read the newest file.
func TestGetSessionSummary_TwoTranscriptsInOneFolder(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	now := time.Now()
	writeConv(t, projectDir, "tab-a", now.Add(-time.Hour), titleLine("Tab A"), humanLine("prompt for A"), assistantLine("reply A"))
	writeConv(t, projectDir, "tab-b", now, titleLine("Tab B"), humanLine("prompt for B"), assistantLine("reply B"))

	cs := NewClaudeService()
	for _, c := range []struct{ uuid, title, prompt string }{
		{"tab-a", "Tab A", "prompt for A"},
		{"tab-b", "Tab B", "prompt for B"},
	} {
		got, _ := cs.GetSessionSummary(workDir, c.uuid)
		if got.Title != c.title || got.LastPrompt != c.prompt {
			t.Errorf("%s: got title %q, prompt %q", c.uuid, got.Title, got.LastPrompt)
		}
	}
}

// No UUID, or one that is not a bare file name, gives an empty summary and
// never the newest file.
func TestGetSessionSummary_NoUUIDIsEmpty(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	writeConv(t, projectDir, "other", time.Now(), titleLine("Someone else's"), humanLine("p"))

	cs := NewClaudeService()
	for _, uuid := range []string{"", ".", "..", "../other", "missing"} {
		got, err := cs.GetSessionSummary(workDir, uuid)
		if err != nil {
			t.Fatalf("%q: err %v", uuid, err)
		}
		if !reflect.DeepEqual(got, emptySummary()) {
			t.Errorf("%q: expected empty, got %+v", uuid, got)
		}
		if got.RecentPrompts == nil {
			t.Errorf("%q: prompts must be [] not null", uuid)
		}
	}
}

// A headless run from another tool is never shown. Any "sdk-" entrypoint is
// headless: `claude -p` writes "sdk-cli" and the Go SDK "sdk-go".
func TestHeadlessTranscript_EmptySummaryAndNotListed(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	now := time.Now()
	copyFixture(t, projectDir, workDir, "session-headless.jsonl", "headless", now)
	writeConv(t, projectDir, "go-sdk", now,
		fmt.Sprintf(`{"type":"user","origin":{"kind":"human"},"cwd":%q,"entrypoint":"sdk-go","message":{"role":"user","content":"call a tool"}}`, workDir),
		titleLine("Tool call"),
	)
	copyFixture(t, projectDir, workDir, "session-mixed.jsonl", "mixed", now.Add(-time.Minute))

	cs := NewClaudeService()
	for _, uuid := range []string{"headless", "go-sdk"} {
		got, _ := cs.GetSessionSummary(workDir, uuid)
		if !reflect.DeepEqual(got, emptySummary()) {
			t.Errorf("%s: expected an empty summary, got %+v", uuid, got)
		}
	}

	list, err := cs.ListConversations(workDir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(list) != 1 || list[0].UUID != "mixed" {
		t.Fatalf("expected only the interactive conversation, got %+v", list)
	}
	// The picker shows the latest title, not the first one.
	if list[0].Title != "Retry uploads with back-off" {
		t.Errorf("title = %q", list[0].Title)
	}
	if !strings.HasPrefix(list[0].Recap, "Added retries") {
		t.Errorf("recap = %q", list[0].Recap)
	}
}

func TestReadConversationHead_Entrypoint(t *testing.T) {
	dir := t.TempDir()
	path := writeConv(t, dir, "ep", time.Now(),
		`{"type":"queue-operation"}`,
		`{"type":"user","cwd":"/repo","entrypoint":"sdk-cli","message":{"role":"user","content":"x"}}`,
	)
	if got := readConversationHead(path).entrypoint; got != "sdk-cli" {
		t.Errorf("entrypoint = %q", got)
	}

	// From a record too long to parse. The text quotes a decoy key, which is
	// escaped inside the string and so must not match.
	body := `"entrypoint":"cli"` + strings.Repeat("C", 600*1024)
	path = writeConv(t, dir, "ep-huge", time.Now(),
		fmt.Sprintf(`{"type":"user","message":{"role":"user","content":%q},"entrypoint":"sdk-cli","cwd":"/repo"}`, body),
	)
	if got := readConversationHead(path).entrypoint; got != "sdk-cli" {
		t.Errorf("entrypoint from a huge record = %q", got)
	}
}

// Old files have no ai-title at all. The first prompt is the title then.
func TestGetSessionSummary_TitleFallsBackToTheFirstPrompt(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	writeConv(t, projectDir, "old", time.Now(), userLine(workDir, "the first thing asked"), assistantLine("ok"))
	got, _ := NewClaudeService().GetSessionSummary(workDir, "old")
	if got.Title != "the first thing asked" {
		t.Errorf("title = %q", got.Title)
	}
}

// An unchanged file is served from the cache. A changed size or time is read
// again.
func TestGetSessionSummary_Cache(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	path := writeConv(t, projectDir, "c", stamp, titleLine("Title one"))

	cs := NewClaudeService()
	if got, _ := cs.GetSessionSummary(workDir, "c"); got.Title != "Title one" {
		t.Fatalf("title = %q", got.Title)
	}

	// Same size and time, new content: the cache must answer.
	writeConv(t, projectDir, "c", stamp, titleLine("Title two"))
	if got, _ := cs.GetSessionSummary(workDir, "c"); got.Title != "Title one" {
		t.Errorf("expected the cached title, got %q", got.Title)
	}

	// A new time means a changed file.
	later := stamp.Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if got, _ := cs.GetSessionSummary(workDir, "c"); got.Title != "Title two" {
		t.Errorf("expected a re-read, got %q", got.Title)
	}

	// So does a new size.
	writeConv(t, projectDir, "c", later, titleLine("Title three, longer"))
	if got, _ := cs.GetSessionSummary(workDir, "c"); got.Title != "Title three, longer" {
		t.Errorf("expected a re-read, got %q", got.Title)
	}
}

func TestSummaryCache_IsBounded(t *testing.T) {
	dir := t.TempDir()
	var c summaryCache
	for i := 0; i < summaryCacheSize+5; i++ {
		path := writeConv(t, dir, fmt.Sprintf("f%02d", i), time.Now(), titleLine("t"))
		info, _ := os.Stat(path)
		c.put(path, info, emptySummary())
	}
	if len(c.entries) != summaryCacheSize {
		t.Errorf("entries = %d, want %d", len(c.entries), summaryCacheSize)
	}
}

func TestEachLineBackwards(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x", 3*tailChunk+100) // spans several chunks
	path := writeConv(t, dir, "lines", time.Now(), "first", big, "last")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	info, _ := f.Stat()

	collect := func(budget int64) []string {
		var out []string
		eachLineBackwards(f, info.Size(), budget, func(line []byte) walk {
			if len(line) > 10 {
				out = append(out, "big")
			} else {
				out = append(out, string(line))
			}
			return walkOn
		})
		return out
	}

	if got := collect(tailBudget); !reflect.DeepEqual(got, []string{"last", "big", "first"}) {
		t.Errorf("whole file: %v", got)
	}
	// The budget ends inside the big record, so only the line after it is whole.
	if got := collect(int64(2 * tailChunk)); !reflect.DeepEqual(got, []string{"last"}) {
		t.Errorf("cut budget: %v", got)
	}

	// walkStop stops the walk at once.
	var n int
	eachLineBackwards(f, info.Size(), tailBudget, func([]byte) walk { n++; return walkStop })
	if n != 1 {
		t.Errorf("expected one call, got %d", n)
	}

	// walkChunk finishes the lines already read, then stops. The first read
	// holds "last" and the end of the big record, so nothing older is passed.
	var seen []string
	eachLineBackwards(f, info.Size(), tailBudget, func(line []byte) walk {
		seen = append(seen, string(line[:min(len(line), 5)]))
		return walkChunk
	})
	if !reflect.DeepEqual(seen, []string{"last"}) {
		t.Errorf("walkChunk: %v", seen)
	}
}

// The walk stops once it has the title, recap, branch and ten prompts. A PR
// link further back than what it read by then is not looked for.
func TestReadSummary_StopsOnceTheMainFieldsAreFound(t *testing.T) {
	lines := func(withPadding bool) []string {
		out := []string{prLine("acme/app", "https://github.com/acme/app/pull/7")}
		if withPadding {
			out = append(out, fmt.Sprintf(`{"type":"attachment","blob":%q}`, strings.Repeat("p", 3*tailChunk)))
		}
		for i := 0; i < maxRecentPrompts; i++ {
			out = append(out, humanLine(fmt.Sprintf("prompt %d", i)))
		}
		return append(out, titleLine("Title"), recapLine("Recap"), branchLine("main"))
	}

	dir := t.TempDir()
	far := writeConv(t, dir, "far", time.Now(), lines(true)...)
	if got := readSummary(far); got.PR != nil {
		t.Errorf("expected no PR from beyond the stop, got %+v", got.PR)
	}
	near := writeConv(t, dir, "near", time.Now(), lines(false)...)
	if got := readSummary(near); got.PR == nil || got.PR.Number != 7 {
		t.Errorf("expected the PR within the read, got %+v", got.PR)
	}
}

// Closed sessions saved before plan 036 can point at a headless run. They are
// left out of the history. Tabs and closed sessions of real conversations stay.
func TestGetSessions_HidesHeadlessHistory(t *testing.T) {
	workDir := t.TempDir()
	projectDir := withFakeHome(t, workDir)
	now := time.Now()
	copyFixture(t, projectDir, workDir, "session-headless.jsonl", "headless", now)
	copyFixture(t, projectDir, workDir, "session-mixed.jsonl", "mixed", now)

	cs := &ConfigService{sessionsPath: filepath.Join(t.TempDir(), "sessions.json")}
	saved := SessionsData{Sessions: []SessionRecord{
		{Slug: "a-1", Directory: workDir, ClaudeSessionID: "headless", Status: "closed"},
		{Slug: "a-2", Directory: workDir, ClaudeSessionID: "mixed", Status: "closed"},
		{Slug: "a-3", Directory: workDir, ClaudeSessionID: "headless", Status: "disconnected"},
		{Slug: "a-4", Directory: workDir, Status: "closed"},
	}}
	if err := cs.SaveSessions(saved); err != nil {
		t.Fatal(err)
	}

	var slugs []string
	for _, r := range cs.GetSessions().Sessions {
		slugs = append(slugs, r.Slug)
	}
	if want := []string{"a-2", "a-3", "a-4"}; !reflect.DeepEqual(slugs, want) {
		t.Errorf("slugs = %v, want %v", slugs, want)
	}
}
