package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Tests for a reviewer in the log format: the worker reads each pass's
// verified output from Reviewer.Logs (plan 031).

const lgtmDraft = "<!-- acme:review n=3 sha=e41b9a2 -->\n## Code Review #3\n\n> **LGTM!** No findings at `e41b9a2`.\n\nAll three points from #2 are fixed.\n\n<sub>acme code review</sub>"

func TestParseVerdict(t *testing.T) {
	for text, want := range map[string]string{
		"blah\n> **Mergeable with follow-ups.** Six findings":     verdictFollowUps,
		"> **Not mergeable.** Seven findings":                     verdictNotMergeable,
		"PR is mergeable\n```\n> **Mergeable.** No findings\n```": verdictReady,
		"1. **Not mergeable** says the round":                     verdictNotMergeable,
		"nothing here":                                            "",
	} {
		if got := parseVerdict(text); got != want {
			t.Errorf("parseVerdict(%q) = %q, want %q", text, got, want)
		}
	}
	if !isLGTMDraft(lgtmDraft) || isLGTMDraft("> **Mergeable.** One finding") {
		t.Error("isLGTMDraft")
	}
	if got := parseVerdict("## Draft\n\n```\n" + lgtmDraft + "\n```\n"); got != verdictReady {
		t.Errorf("LGTM verdict %q, want ready", got)
	}
}

func TestDraftComment(t *testing.T) {
	// The shape of a real verified log: the heading above the block varies.
	verified := "## Findings\n\n- one\n\n## Draft comment (not posted)\n\n```\n<!-- acme:review n=1 sha=6ec793f -->\n## Code Review #1\n\n> **Mergeable.** One finding.\n\n<sub>acme code review</sub>\n```\n\nTwo MCP servers failed.\n"
	want := "<!-- acme:review n=1 sha=6ec793f -->\n## Code Review #1\n\n> **Mergeable.** One finding.\n\n<sub>acme code review</sub>"
	if got := draftComment(verified, "acme"); got != want {
		t.Errorf("got %q", got)
	}
	// A fenced block without a marker is not a draft.
	if got := draftComment("## Checked\n\n```json\n[]\n```\n", "acme"); got != "" {
		t.Errorf("want no draft, got %q", got)
	}
	lgtm := lgtmComment("acme", "55d37455abcdef")
	// The reviewer's LGTM check must find it, so a later run skips the code review.
	if !strings.HasPrefix(lgtm, "<!-- acme:review sha=55d3745 -->\n> **LGTM!**") || !isLGTMDraft(lgtm) || !hasHeadMarker(lgtm, "acme", "55d3745") {
		t.Errorf("lgtm = %q", lgtm)
	}
}

func TestDraftCommentFences(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"tildes", "## Draft\n\n~~~\n<!-- acme:docs n=1 -->\nBody\n<sub>f</sub>\n~~~\n", "<!-- acme:docs n=1 -->\nBody\n<sub>f</sub>"},
		{"indented", "1. Draft:\n\n   ```\n   <!-- acme:review n=1 -->\n   Body\n   <sub>f</sub>\n   ```\n", "<!-- acme:review n=1 -->\nBody\n<sub>f</sub>"},
		{"inner code block", "```\n<!-- acme:review n=1 -->\nFix:\n```go\nx := 1\n```\n<sub>f</sub>\n```\n", "<!-- acme:review n=1 -->\nFix:\n```go\nx := 1\n```\n<sub>f</sub>"},
		{"longer outer fence", "````\n<!-- acme:review n=1 -->\n```\ncode\n```\n<sub>f</sub>\n````\n", "<!-- acme:review n=1 -->\n```\ncode\n```\n<sub>f</sub>"},
		{"marker only quoted in prose", "- The docs quote `<!-- acme:... -->` as an example.\n", ""},
	}
	for _, c := range cases {
		if got := draftComment(c.in, "acme"); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	// A marker line outside any fence the finder can read is not clean.
	if !hasMarkerLine("## Draft\n\n<!-- acme:review n=1 -->\nBody\n", "acme") || hasMarkerLine("- quoted `<!-- acme:x -->`", "acme") {
		t.Error("hasMarkerLine")
	}
	out := reviewOutcome{fromLogs: true, expected: 3, passes: 3, unreadable: 1, verdicts: map[string]string{}}
	if c, _, failed := commentsFor(out, "abc", "acme"); failed == "" || c != nil {
		t.Error("an unreadable draft must fail, not post LGTM")
	}
}

func TestLooksLikeReview(t *testing.T) {
	for _, broken := range []string{
		"API Error: Can't reach the API server, check your internet or DNS (ENOTFOUND)\n",
		"Not logged in. Please run /login\n",
		"stub: ok\n",
		"",
	} {
		if looksLikeReview(broken) {
			t.Errorf("%q reads as a review", broken)
		}
	}
	clean := "## What I checked\n\n- Every comment in the diff.\n\n## PR comment\n\nNone. The pass found nothing.\n"
	if !looksLikeReview(clean) {
		t.Error("a clean pass must read as a review")
	}
	// Two clean passes and one that broke: no LGTM.
	out := reviewOutcome{fromLogs: true, expected: 3, passes: 2, broken: []string{"review: API Error"}, verdicts: map[string]string{}}
	if c, _, failed := commentsFor(out, "abc", "acme"); c != nil || !strings.Contains(failed, "API Error") {
		t.Errorf("broken pass: comments %v, failed %q", c, failed)
	}
}

func TestCommentsFor(t *testing.T) {
	draft := "<!-- acme:review n=1 sha=abc1234 -->\n## Code Review #1\n\n<sub>acme code review</sub>"
	cases := []struct {
		name string
		out  reviewOutcome
		want string // "draft", "lgtm" or "failed"
	}{
		{"drafts are posted", reviewOutcome{fromLogs: true, expected: 3, comments: []reviewComment{{Pass: "review", Body: draft}}, passes: 3, verdicts: map[string]string{"review": verdictFollowUps}}, "draft"},
		{"clean, no verdicts", reviewOutcome{fromLogs: true, expected: 3, passes: 3, verdicts: map[string]string{}}, "lgtm"},
		{"clean, ready", reviewOutcome{fromLogs: true, expected: 3, passes: 3, verdicts: map[string]string{"review": verdictReady}}, "lgtm"},
		{"a pass did not finish", reviewOutcome{fromLogs: true, expected: 3, passes: 2, verdicts: map[string]string{}}, "failed"},
		{"drafts, but a pass did not finish", reviewOutcome{fromLogs: true, expected: 3, comments: []reviewComment{{Pass: "review", Body: draft}}, passes: 2, verdicts: map[string]string{"review": verdictFollowUps}}, "failed"},
		{"follow-ups but no draft", reviewOutcome{fromLogs: true, expected: 3, passes: 3, verdicts: map[string]string{"review": verdictFollowUps}}, "failed"},
		{"not mergeable but no draft", reviewOutcome{fromLogs: true, expected: 3, passes: 3, verdicts: map[string]string{"review": verdictNotMergeable}}, "failed"},
		// overallVerdict needs a clean code review for "ready", so a lone docs
		// "mergeable" reads as follow-ups: no LGTM without a code verdict.
		{"docs mergeable, no code verdict", reviewOutcome{fromLogs: true, expected: 3, passes: 3, verdicts: map[string]string{"docs": verdictReady}}, "failed"},
		{"cut-short draft", reviewOutcome{fromLogs: true, expected: 3, comments: []reviewComment{{Pass: "review", Body: "<!-- acme:review n=1 -->\n## Code Review #1\n\n- **HIGH**: see"}}, passes: 3}, "failed"},
	}
	for _, c := range cases {
		comments, _, failed := commentsFor(c.out, "abc1234def", "acme")
		got := "failed"
		switch {
		case failed != "" && len(comments) > 0:
			t.Errorf("%s: both comments and a failure", c.name)
		case failed != "":
		case len(comments) == 1 && comments[0] == lgtmComment("acme", "abc1234def"):
			got = "lgtm"
		case len(comments) > 0:
			got = "draft"
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s (failed %q)", c.name, got, c.want, failed)
		}
	}
}

func TestCommentsForLogLGTMDrafts(t *testing.T) {
	docs := strings.Replace(strings.Replace(lgtmDraft, "acme:review", "acme:docs", 1), "Code Review", "Docs Review", 1)
	all := reviewOutcome{fromLogs: true, expected: 3, passes: 3, lgtmPasses: 3, comments: []reviewComment{
		{Pass: "review", Verdict: contractLGTM, Body: lgtmDraft},
		{Pass: "docs", Verdict: contractLGTM, Body: docs},
		{Pass: "comments", Verdict: contractLGTM, Body: lgtmDraft},
	}}
	// Three clean passes post one LGTM, the code review's.
	if c, lgtm, failed := commentsFor(all, "e41b9a2", "acme"); failed != "" || !lgtm || len(c) != 1 || c[0] != lgtmDraft {
		t.Errorf("all LGTM: %d comments, lgtm %v, %q", len(c), lgtm, failed)
	}
	// One clean pass and one with findings post both, and it is not an LGTM.
	mixed := all
	mixed.lgtmPasses = 1
	mixed.comments = []reviewComment{{Pass: "review", Verdict: contractLGTM, Body: lgtmDraft}, {Pass: "docs", Body: "<!-- acme:docs n=1 -->\n- **MED**: x\n<sub>f</sub>"}}
	if c, lgtm, failed := commentsFor(mixed, "e41b9a2", "acme"); failed != "" || lgtm || len(c) != 2 {
		t.Errorf("mixed: %d comments, lgtm %v, %q", len(c), lgtm, failed)
	}
	// A code review the reviewer skipped (its LGTM is on the head commit) leaves two
	// passes, which is the whole review.
	skipped := reviewOutcome{fromLogs: true, expected: 2, passes: 2, ran: map[string]bool{"docs": true, "comments": true},
		comments: []reviewComment{{Pass: "docs", Body: "<!-- acme:docs n=1 -->\n- **LOW**: y\n<sub>f</sub>"}}}
	if c, _, failed := commentsFor(skipped, "e41b9a2", "acme"); failed != "" || len(c) != 1 {
		t.Errorf("skipped code review: %d comments, %q", len(c), failed)
	}
}

func TestCommentsForCleanPR(t *testing.T) {
	// A clean code review writes an LGTM, and clean docs and comments passes
	// write nothing.
	out := reviewOutcome{fromLogs: true, expected: 3, passes: 3, lgtmPasses: 1,
		verdicts: map[string]string{"review": verdictReady},
		comments: []reviewComment{{Pass: "review", Verdict: contractLGTM, Body: lgtmDraft}}}
	if c, lgtm, failed := commentsFor(out, "e41b9a2", "acme"); failed != "" || !lgtm || len(c) != 1 {
		t.Errorf("clean PR: %d comments, lgtm %v, %q", len(c), lgtm, failed)
	}
	// An LGTM code review with a docs pass that wants changes is not an LGTM.
	out.verdicts["docs"] = verdictFollowUps
	if _, lgtm, failed := commentsFor(out, "e41b9a2", "acme"); lgtm || failed != "" {
		t.Errorf("docs follow-ups: lgtm %v, %q", lgtm, failed)
	}
	// A rerun at a head the reviewer already approved: the code review is
	// skipped and the clean passes write nothing. Nothing new to post.
	rerun := reviewOutcome{fromLogs: true, expected: 2, passes: 2, lgtmOnHead: true,
		ran: map[string]bool{"docs": true, "comments": true}, verdicts: map[string]string{"docs": verdictReady}}
	if c, lgtm, failed := commentsFor(rerun, "e41b9a2", "acme"); failed != "" || !lgtm || len(c) != 0 {
		t.Errorf("rerun: %d comments, lgtm %v, %q", len(c), lgtm, failed)
	}
}

// TestReadLogs reads real files from a log folder, as the reviewer leaves
// them, and checks what the worker would post.
func TestReadLogs(t *testing.T) {
	since := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	clean := "## What I checked\n\n- Every change in the diff.\n\n## Draft\n\n```\n" + lgtmDraft + "\n```\n"
	// writeLog writes one pass's verified output, modified at since plus offset.
	writeLog := func(t *testing.T, dir, run, pass, body string, offset time.Duration) {
		t.Helper()
		path := filepath.Join(dir, run+"-"+logKey("acme/api", 7)+"-"+pass+"-merged.log")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		at := since.Add(offset)
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	read := func(dir string) (reviewOutcome, []string, bool, string) {
		out := readLogs(dir, since, "acme/api", 7, "acme")
		c, lgtm, failed := commentsFor(out, "e41b9a2", "acme")
		return out, c, lgtm, failed
	}

	t.Run("three clean passes post one LGTM", func(t *testing.T) {
		dir := t.TempDir()
		for _, p := range logPasses {
			writeLog(t, dir, "run1", p.name, clean, time.Minute)
		}
		out, c, lgtm, failed := read(dir)
		if out.passes != 3 || out.lgtmPasses != 3 || !lgtm || len(c) != 1 || failed != "" {
			t.Errorf("passes %d, lgtm passes %d, lgtm %v, comments %d, failed %q", out.passes, out.lgtmPasses, lgtm, len(c), failed)
		}
	})

	t.Run("a pass whose output is not a review is broken", func(t *testing.T) {
		dir := t.TempDir()
		writeLog(t, dir, "run1", "review", "API Error: Can't reach the API server\n", time.Minute)
		writeLog(t, dir, "run1", "docs", clean, time.Minute)
		writeLog(t, dir, "run1", "comments", clean, time.Minute)
		out, c, lgtm, failed := read(dir)
		if out.ran["review"] || len(out.broken) != 1 || !strings.HasPrefix(out.broken[0], "review: API Error") {
			t.Errorf("ran %v, broken %q", out.ran, out.broken)
		}
		if lgtm || c != nil || !strings.Contains(failed, "API Error") {
			t.Errorf("a broken pass must fail with no LGTM: lgtm %v, comments %v, failed %q", lgtm, c, failed)
		}
	})

	t.Run("an LGTM draft counts only when its pass ran", func(t *testing.T) {
		dir := t.TempDir()
		// An LGTM from an earlier run, before since, is not this run's.
		writeLog(t, dir, "run0", "review", clean, -time.Hour)
		// An LGTM draft with no heading or list does not read as a review.
		writeLog(t, dir, "run1", "docs", "```\n<!-- acme:docs n=1 -->\n> **LGTM!**\n<sub>f</sub>\n```\n", time.Minute)
		writeLog(t, dir, "run1", "comments", clean, time.Minute)
		out, c, lgtm, failed := read(dir)
		if out.ran["review"] || out.ran["docs"] || out.lgtmPasses != 1 || len(out.broken) != 1 || !strings.HasPrefix(out.broken[0], "docs:") {
			t.Errorf("ran %v, lgtm passes %d, broken %q", out.ran, out.lgtmPasses, out.broken)
		}
		if lgtm || c != nil || failed == "" {
			t.Errorf("lgtm %v, comments %v, failed %q", lgtm, c, failed)
		}
	})

	t.Run("the newest log after since wins", func(t *testing.T) {
		dir := t.TempDir()
		for _, p := range logPasses {
			writeLog(t, dir, "run1", p.name, "API Error: overloaded\n", time.Minute)
			writeLog(t, dir, "run2", p.name, clean, 2*time.Minute)
		}
		writeLog(t, dir, "run1", "docs", clean, 3*time.Minute)
		writeLog(t, dir, "run2", "docs", "API Error: overloaded\n", 2*time.Minute)
		out, _, lgtm, failed := read(dir)
		if out.passes != 3 || len(out.broken) != 0 || !lgtm || failed != "" {
			t.Errorf("passes %d, broken %q, lgtm %v, failed %q", out.passes, out.broken, lgtm, failed)
		}
		// The same files, but the newest docs log is the broken one.
		writeLog(t, dir, "run2", "docs", "API Error: overloaded\n", 4*time.Minute)
		if out, _, lgtm, _ := read(dir); out.ran["docs"] || lgtm {
			t.Errorf("the newer broken docs log must win: ran %v, lgtm %v", out.ran, lgtm)
		}
	})
}
