package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const lgtmDraft = "<!-- tono:review n=3 sha=e41b9a2 -->\n## Code Review #3\n\n> **LGTM!** 🐊 🚀 No findings at `e41b9a2`.\n\nAll three points from #2 are fixed. Nice work on the retry tests 👑\n\n<sub>tono code review, read then verified, run with Claude</sub>"

func TestReviewerCommand(t *testing.T) {
	p := SearchPR{URL: "https://github.com/o/r/pull/7", Number: 7}
	p.Repository.NameWithOwner = "o/r"
	env, prog, args, err := reviewerCommand(`A=1 B={claude} {reviewer} {pr} --all -R {repo} --url={url} "{sha} x" ''`, "/cli", "/wrap", p, "abc123")
	if err != nil || prog != "/cli" || strings.Join(env, " ") != "A=1 B=/wrap" {
		t.Fatalf("env %q, prog %q, %v", env, prog, err)
	}
	if want := []string{"7", "--all", "-R", "o/r", "--url=https://github.com/o/r/pull/7", "abc123 x", ""}; strings.Join(args, "|") != strings.Join(want, "|") {
		t.Errorf("args %q, want %q", args, want)
	}
	home, _ := os.UserHomeDir()
	if _, prog, _, _ := reviewerCommand("~/bin/acme {pr}", "", "", p, "x"); prog != filepath.Join(home, "bin/acme") {
		t.Errorf("~ not expanded: %q", prog)
	}
	for _, bad := range []string{`acme "open`, "A=1", ""} {
		if _, _, _, err := reviewerCommand(bad, "", "", p, "x"); err == nil {
			t.Errorf("want an error for %q", bad)
		}
	}
	if got := commandProgram("TONO_CLAUDE={claude} {reviewer} {pr}"); got != "{reviewer}" {
		t.Errorf("commandProgram: %q", got)
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

	var skipped reviewSkippedError
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
	out := reviewOutcome{lgtmPasses: 1, comments: []reviewComment{{Pass: "review", Verdict: "lgtm", Body: "<!-- acme:review -->\nLGTM"}}}
	if c, lgtm, failed := commentsFor(out, "abc", "acme"); failed != "" || !lgtm || len(c) != 1 {
		t.Errorf("one LGTM: %v %v %q", c, lgtm, failed)
	}
	out.comments[0].Body = "LGTM without a marker"
	if _, _, failed := commentsFor(out, "abc", "acme"); !strings.Contains(failed, "marker") {
		t.Errorf("no marker: %q", failed)
	}
	if _, _, failed := commentsFor(reviewOutcome{}, "abc", "acme"); !strings.Contains(failed, "no comment") {
		t.Errorf("no comments: %q", failed)
	}
}

func TestCommentsForLogLGTMDrafts(t *testing.T) {
	docs := strings.Replace(strings.Replace(lgtmDraft, "tono:review", "tono:docs-check", 1), "Code Review", "Docs Review", 1)
	all := reviewOutcome{fromLogs: true, expected: 3, passes: 3, lgtmPasses: 3, comments: []reviewComment{
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
	// A code review the reviewer skipped (its LGTM is on the head commit) leaves two
	// passes, which is the whole review.
	skipped := reviewOutcome{fromLogs: true, expected: 2, passes: 2, ran: map[string]bool{"docs": true, "comments": true},
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

// v057Reviewer is a real 0.5.7 reviewer section: no command, format,
// args or marker, so the tono defaults of the time applied.
const v057Reviewer = `{"reviewer": {"autoUpdate": true, "branch": "main", "mine": {"enabled": true, "maxAgeDays": 14},
  "path": "", "repo": "epidemicsound/tonometer", "script": "tono", "source": "managed",
  "team": {"enabled": true, "maxAgeDays": 1, "repos": [], "team": "epidemicsound/content-protection"}}}`

func TestReviewerDefaults(t *testing.T) {
	cfg := defaultConfig()
	r := cfg.Reviewer
	if r.Command != "" || r.Logs != "" || r.MarkerPrefix != "" || r.Repo != "" || r.Script != "" || r.Team.Enabled || r.Team.Team != "" {
		t.Errorf("a new install names no reviewer and no team: %+v", r)
	}
	if cfg.Jobs[JobReview].Enabled {
		t.Error("the review job starts off")
	}
	if err := r.problem(); err == nil || !strings.Contains(err.Error(), "no reviewer command") {
		t.Errorf("problem: %v", err)
	}
}

func TestReviewerMigration(t *testing.T) {
	home, _ := os.UserHomeDir()
	tonoLogs := filepath.Join(home, ".cache/tono/logs")

	cfg := loadTestConfig(t, v057Reviewer)
	r := cfg.Reviewer
	if r.Command != "TONO_CLAUDE={claude} {reviewer} {pr} --all -l high -R {repo}" || r.Logs != tonoLogs || r.MarkerPrefix != "tono" {
		t.Errorf("0.5.7 config: command %q, logs %q, marker %q", r.Command, r.Logs, r.MarkerPrefix)
	}
	if r.Repo != "epidemicsound/tonometer" || r.Team.Team != "epidemicsound/content-protection" || r.problem() != nil {
		t.Errorf("0.5.7 config kept: %+v, %v", r, r.problem())
	}
	// The migrated command still hands tono the wrapper, as TONO_CLAUDE.
	env, prog, _, _ := reviewerCommand(r.Command, "/cli", "/wrap", SearchPR{}, "x")
	if prog != "/cli" || strings.Join(env, " ") != "TONO_CLAUDE=/wrap" {
		t.Errorf("migrated command runs %q with %q", prog, env)
	}

	cfg = loadTestConfig(t, `{"reviewer": {"format": "result-json", "args": ["{url}", "a b"], "markerPrefix": "acme"}}`)
	if r := cfg.Reviewer; r.Command != "TONO_CLAUDE={claude} {reviewer} {url} 'a b'" || r.Logs != "" || r.MarkerPrefix != "acme" {
		t.Errorf("0.5.7 result-json: command %q, logs %q, marker %q", r.Command, r.Logs, r.MarkerPrefix)
	}

	// A command key, even an empty one, is a 0.5.8 config: nothing changes.
	cfg = loadTestConfig(t, `{"reviewer": {"command": "", "logs": ""}}`)
	if r := cfg.Reviewer; r.Command != "" || r.Logs != "" || r.MarkerPrefix != "" {
		t.Errorf("an empty command stays empty: %+v", r)
	}
	cfg = loadTestConfig(t, `{"reviewer": {"command": "acme {pr}", "logs": "~/acme/logs", "markerPrefix": "acme"}}`)
	if r := cfg.Reviewer; r.Command != "acme {pr}" || r.Logs != filepath.Join(home, "acme/logs") {
		t.Errorf("0.5.8 config: %+v", r)
	}

	// No reviewer section and no tono keys is a new install.
	cfg = loadTestConfig(t, `{"enabled": true}`)
	if cfg.Reviewer.Command != "" || cfg.Reviewer.Repo != "" {
		t.Errorf("new install: %+v", cfg.Reviewer)
	}
}

func TestReviewerProblems(t *testing.T) {
	ok := ReviewerConfig{Command: "/opt/acme {pr}", MarkerPrefix: "acme", Source: SourceManaged}
	if err := ok.problem(); err != nil {
		t.Errorf("a command with its own program needs no source: %v", err)
	}
	for name, bad := range map[string]ReviewerConfig{
		"bad marker":           {Command: "acme", MarkerPrefix: "Bad Prefix"},
		"jq in marker":         {Command: "acme", MarkerPrefix: `x") | .id, ("`},
		"no marker":            {Command: "acme"},
		"unclosed quote":       {Command: `acme "x`, MarkerPrefix: "acme"},
		"only env":             {Command: "A=1", MarkerPrefix: "acme"},
		"{reviewer}, no repo":  {Command: "{reviewer}", MarkerPrefix: "acme", Source: SourceManaged, Script: "acme"},
		"{reviewer}, no path":  {Command: "{reviewer}", MarkerPrefix: "acme", Source: SourceLocal},
		"team on, no team set": {Command: "acme", MarkerPrefix: "acme", Team: TeamScopeConfig{Enabled: true}},
	} {
		if bad.problem() == nil {
			t.Errorf("%s: want a problem", name)
		}
	}
}

func TestReviewerEnv(t *testing.T) {
	p := SearchPR{URL: "u", Number: 7}
	p.Repository.NameWithOwner = "o/r"
	last := func(env []string, name string) string {
		v := ""
		for _, kv := range env {
			if x, ok := strings.CutPrefix(kv, name+"="); ok {
				v = x
			}
		}
		return v
	}
	base := []string{"PATH=/usr/bin", "HOME=/h"}
	env := reviewerEnv(base, []string{"TONO_CLAUDE=/wrap", "REVIEW_RESULT=/evil"}, "/shims", "/wrap", "/result.json", p, "abc")
	if last(env, "TONO_CLAUDE") != "/wrap" || last(env, "REVIEW_RESULT") != "/result.json" || last(env, "PATH") != "/shims:/usr/bin" {
		t.Errorf("env: %q", env)
	}
	// A PATH from the command keeps the read-only gh and git in front.
	env = reviewerEnv(base, []string{"PATH=/opt/homebrew/bin"}, "/shims", "/wrap", "/r", p, "abc")
	if last(env, "PATH") != "/shims:/opt/homebrew/bin" {
		t.Errorf("PATH: %q", last(env, "PATH"))
	}
}

func TestCommentsForTonoCleanPR(t *testing.T) {
	// tono 0.5.x after tonometer #5: a clean code review writes an LGTM, and
	// clean docs and comments passes write nothing.
	out := reviewOutcome{fromLogs: true, expected: 3, passes: 3, lgtmPasses: 1,
		verdicts: map[string]string{"review": verdictReady},
		comments: []reviewComment{{Pass: "review", Verdict: contractLGTM, Body: lgtmDraft}}}
	if c, lgtm, failed := commentsFor(out, "e41b9a2", "tono"); failed != "" || !lgtm || len(c) != 1 {
		t.Errorf("clean PR: %d comments, lgtm %v, %q", len(c), lgtm, failed)
	}
	// An LGTM code review with a docs pass that wants changes is not an LGTM.
	out.verdicts["docs"] = verdictFollowUps
	if _, lgtm, failed := commentsFor(out, "e41b9a2", "tono"); lgtm || failed != "" {
		t.Errorf("docs follow-ups: lgtm %v, %q", lgtm, failed)
	}

	// A rerun at a head tono already LGTM'd: the code review is skipped and
	// the clean docs and comments passes write nothing. Nothing new to post.
	rerun := reviewOutcome{fromLogs: true, expected: 2, passes: 2, lgtmOnHead: true,
		ran: map[string]bool{"docs": true, "comments": true}, verdicts: map[string]string{"docs": verdictReady}}
	if c, lgtm, failed := commentsFor(rerun, "e41b9a2", "tono"); failed != "" || !lgtm || len(c) != 0 {
		t.Errorf("rerun: %d comments, lgtm %v, %q", len(c), lgtm, failed)
	}
}

func TestHasHeadMarker(t *testing.T) {
	if !hasHeadMarker("Some text\n<!-- tono:review n=2 sha=e41b9a2 -->\n> **LGTM!**", "tono", "e41b9a2") {
		t.Error("a marker below the first line counts, as in tono")
	}
	if hasHeadMarker("<!-- tono:review n=2 sha=0000000 -->", "tono", "e41b9a2") || hasHeadMarker("<!-- tono:docs-check sha=e41b9a2 -->", "tono", "e41b9a2") {
		t.Error("another commit or pass must not count")
	}
}
