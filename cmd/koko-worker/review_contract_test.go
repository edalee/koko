package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const lgtmDraft = "<!-- tono:review n=3 sha=e41b9a2 -->\n## Code Review #3\n\n> **LGTM!** 🐊 🚀 No findings at `e41b9a2`.\n\nAll three points from #2 are fixed. Nice work on the retry tests 👑\n\n<sub>tono code review, read then verified, run with Claude</sub>"

func TestExpandArgs(t *testing.T) {
	p := SearchPR{URL: "https://github.com/o/r/pull/7", Number: 7}
	p.Repository.NameWithOwner = "o/r"
	got := strings.Join(expandArgs([]string{"{pr}", "--all", "-R", "{repo}", "--url={url}", "{sha}"}, p, "abc123"), " ")
	if got != "7 --all -R o/r --url=https://github.com/o/r/pull/7 abc123" {
		t.Errorf("got %q", got)
	}
	if got := strings.Join(expandArgs(defaultReviewerArgs(), p, "x"), " "); got != "7 --all -l high -R o/r" {
		t.Errorf("default args: %q", got)
	}
}

func writeResult(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadResult(t *testing.T) {
	out, err := readResult(writeResult(t, `{"status": "reviewed", "comments": [
	  {"pass": "review", "verdict": "lgtm", "body": "<!-- acme:review -->\nLGTM"},
	  {"pass": "security", "verdict": "follow-ups", "body": "<!-- acme:security -->\nTwo findings"}]}`))
	if err != nil || len(out.comments) != 2 || out.lgtmPasses != 1 || out.verdicts["review"] != verdictReady || out.verdicts["security"] != verdictFollowUps {
		t.Fatalf("reviewed: %+v, %v", out, err)
	}

	var skipped tonoSkippedError
	if _, err := readResult(writeResult(t, `{"status": "skipped", "reason": "already reviewed"}`)); !errors.As(err, &skipped) {
		t.Errorf("skipped: %v", err)
	}
	if _, err := readResult(writeResult(t, `{"status": "failed", "reason": "model unavailable"}`)); err == nil || !strings.Contains(err.Error(), "model unavailable") {
		t.Errorf("failed: %v", err)
	}
	if _, err := readResult(writeResult(t, `{"status": "done"}`)); err == nil {
		t.Error("an unknown status must fail")
	}
	if _, err := readResult(writeResult(t, `not json`)); err == nil {
		t.Error("bad JSON must fail")
	}
	if _, err := readResult(filepath.Join(t.TempDir(), "missing.json")); err == nil || !strings.Contains(err.Error(), "no result file") {
		t.Errorf("missing file: %v", err)
	}
}

func TestCommentsForResultJSON(t *testing.T) {
	out := tonoOutcome{format: FormatResultJSON, lgtmPasses: 1, comments: []reviewComment{{Pass: "review", Verdict: "lgtm", Body: "<!-- acme:review -->\nLGTM"}}}
	if c, lgtm, failed := commentsFor(out, "abc", "acme"); failed != "" || !lgtm || len(c) != 1 {
		t.Errorf("one LGTM: %v %v %q", c, lgtm, failed)
	}
	out.comments[0].Body = "LGTM without a marker"
	if _, _, failed := commentsFor(out, "abc", "acme"); !strings.Contains(failed, "marker") {
		t.Errorf("no marker: %q", failed)
	}
	if _, _, failed := commentsFor(tonoOutcome{format: FormatResultJSON}, "abc", "acme"); !strings.Contains(failed, "no comment") {
		t.Errorf("no comments: %q", failed)
	}
}

func TestCommentsForTonoLGTMDrafts(t *testing.T) {
	docs := strings.Replace(strings.Replace(lgtmDraft, "tono:review", "tono:docs-check", 1), "Code Review", "Docs Review", 1)
	all := tonoOutcome{format: FormatTonoLogs, expected: 3, passes: 3, lgtmPasses: 3, comments: []reviewComment{
		{Pass: "review", Verdict: contractLGTM, Body: lgtmDraft},
		{Pass: "docs", Verdict: contractLGTM, Body: docs},
		{Pass: "comments", Verdict: contractLGTM, Body: lgtmDraft},
	}}
	// Three clean passes post one LGTM, the code review's.
	if c, lgtm, failed := commentsFor(all, "e41b9a2", "tono"); failed != "" || !lgtm || len(c) != 1 || c[0] != lgtmDraft {
		t.Errorf("all LGTM: %d comments, lgtm %v, %q", len(c), lgtm, failed)
	}
	// One clean pass and one with findings post both, and it is not an LGTM.
	mixed := all
	mixed.lgtmPasses = 1
	mixed.comments = []reviewComment{{Pass: "review", Verdict: contractLGTM, Body: lgtmDraft}, {Pass: "docs", Body: "<!-- tono:docs-check n=1 -->\n- **MED**: x\n<sub>f</sub>"}}
	if c, lgtm, failed := commentsFor(mixed, "e41b9a2", "tono"); failed != "" || lgtm || len(c) != 2 {
		t.Errorf("mixed: %d comments, lgtm %v, %q", len(c), lgtm, failed)
	}
	// A code review tono skipped (its LGTM is on the head commit) leaves two
	// passes, which is the whole review.
	skipped := tonoOutcome{format: FormatTonoLogs, expected: 2, passes: 2, ran: map[string]bool{"docs": true, "comments": true},
		comments: []reviewComment{{Pass: "docs", Body: "<!-- tono:docs-check n=1 -->\n- **LOW**: y\n<sub>f</sub>"}}}
	if c, _, failed := commentsFor(skipped, "e41b9a2", "tono"); failed != "" || len(c) != 1 {
		t.Errorf("skipped code review: %d comments, %q", len(c), failed)
	}
}

func TestLGTMVerdict(t *testing.T) {
	if !isLGTMDraft(lgtmDraft) || isLGTMDraft("> **Mergeable.** One finding") {
		t.Error("isLGTMDraft")
	}
	if got := parseVerdict("## Draft\n\n```\n" + lgtmDraft + "\n```\n"); got != verdictReady {
		t.Errorf("LGTM verdict %q, want ready", got)
	}
	// A reviewer with its own pass names is ready when every pass is.
	if got := overallVerdict(map[string]string{"security": verdictReady, "style": verdictReady}); got != verdictReady {
		t.Errorf("all ready: %q", got)
	}
	if got := overallVerdict(map[string]string{"security": verdictReady, "style": verdictFollowUps}); got != verdictFollowUps {
		t.Errorf("one follow-up: %q", got)
	}
}

func TestReviewerContractConfig(t *testing.T) {
	r := defaultConfig().Reviewer
	if r.Format != FormatTonoLogs || r.MarkerPrefix != "tono" || strings.Join(r.Args, " ") != "{pr} --all -l high -R {repo}" {
		t.Errorf("defaults: %+v", r)
	}
	cfg := loadTestConfig(t, `{"reviewer": {"format": "result-json", "args": ["", " {url} "], "markerPrefix": "acme"}}`)
	if cfg.Reviewer.Format != FormatResultJSON || strings.Join(cfg.Reviewer.Args, " ") != "{url}" || cfg.Reviewer.MarkerPrefix != "acme" {
		t.Errorf("loaded: %+v", cfg.Reviewer)
	}
	cfg = loadTestConfig(t, `{"reviewer": {"args": []}}`)
	if strings.Join(cfg.Reviewer.Args, " ") != "{pr} --all -l high -R {repo}" {
		t.Errorf("empty args keep the default: %q", cfg.Reviewer.Args)
	}
	for _, bad := range []ReviewerConfig{
		{Format: "xml", MarkerPrefix: "tono"},
		{Format: FormatResultJSON, MarkerPrefix: "Bad Prefix"},
		{Format: FormatResultJSON, MarkerPrefix: `x") | .id, ("`},
	} {
		r := defaultConfig().Reviewer
		r.Format, r.MarkerPrefix = bad.Format, bad.MarkerPrefix
		if r.problem() == nil {
			t.Errorf("want a problem for %q / %q", bad.Format, bad.MarkerPrefix)
		}
	}
}
