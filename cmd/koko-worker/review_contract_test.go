package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	if got := commandProgram("RV_CLAUDE={claude} {reviewer} {pr}"); got != "{reviewer}" {
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

func TestOverallVerdict(t *testing.T) {
	// A reviewer with its own pass names is ready when every pass is.
	if got := overallVerdict(map[string]string{"security": verdictReady, "style": verdictReady}); got != verdictReady {
		t.Errorf("all ready: %q", got)
	}
	if got := overallVerdict(map[string]string{"security": verdictReady, "style": verdictFollowUps}); got != verdictFollowUps {
		t.Errorf("one follow-up: %q", got)
	}
}

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

func TestReviewerConfigLoad(t *testing.T) {
	home, _ := os.UserHomeDir()
	cfg := loadTestConfig(t, `{"reviewer": {"command": "", "logs": ""}}`)
	if r := cfg.Reviewer; r.Command != "" || r.Logs != "" || r.MarkerPrefix != "" {
		t.Errorf("an empty command stays empty: %+v", r)
	}
	cfg = loadTestConfig(t, `{"reviewer": {"command": "acme {pr}", "logs": "~/acme/logs", "markerPrefix": "acme"}}`)
	if r := cfg.Reviewer; r.Command != "acme {pr}" || r.Logs != filepath.Join(home, "acme/logs") {
		t.Errorf("config: %+v", r)
	}

	// No reviewer section is a new install.
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
	env := reviewerEnv(base, []string{"RV_CLAUDE=/wrap", "REVIEW_RESULT=/evil"}, "/shims", "/wrap", "/result.json", p, "abc")
	if last(env, "RV_CLAUDE") != "/wrap" || last(env, "REVIEW_RESULT") != "/result.json" || last(env, "PATH") != "/shims:/usr/bin" {
		t.Errorf("env: %q", env)
	}
	// A PATH from the command keeps the read-only gh and git in front.
	env = reviewerEnv(base, []string{"PATH=/opt/homebrew/bin"}, "/shims", "/wrap", "/r", p, "abc")
	if last(env, "PATH") != "/shims:/opt/homebrew/bin" {
		t.Errorf("PATH: %q", last(env, "PATH"))
	}
}

func TestHasHeadMarker(t *testing.T) {
	if !hasHeadMarker("Some text\n<!-- acme:review n=2 sha=e41b9a2 -->\n> **LGTM!**", "acme", "e41b9a2") {
		t.Error("a marker below the first line counts")
	}
	if hasHeadMarker("<!-- acme:review n=2 sha=0000000 -->", "acme", "e41b9a2") || hasHeadMarker("<!-- acme:security sha=e41b9a2 -->", "acme", "e41b9a2") {
		t.Error("another commit or pass must not count")
	}
}

// TestReviewWorkerRunsAReviewer runs a stand-in reviewer through the review
// worker. It checks out the PR's head, runs the command there and reads the
// result file. Any program that keeps the contract (plan 030) works the same.
func TestReviewWorkerRunsAReviewer(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh is not on PATH")
	}
	tmp := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	// An origin with the PR's head at refs/pull/7/head, as GitHub has it.
	origin := filepath.Join(tmp, "origin")
	git(tmp, "init", "--quiet", origin)
	git(origin, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "--allow-empty", "-m", "change")
	sha := git(origin, "rev-parse", "HEAD")
	git(origin, "update-ref", "refs/pull/7/head", sha)

	paths := Paths{Dir: filepath.Join(tmp, "worker"), Logs: filepath.Join(tmp, "logs"), Cache: filepath.Join(tmp, "repos"),
		ClaudeWrap: filepath.Join(tmp, "claude.sh")}
	p := SearchPR{URL: "https://github.com/acme/api/pull/7", Number: 7}
	p.Repository.NameWithOwner = "acme/api"
	git(tmp, "clone", "--quiet", origin, filepath.Join(paths.Cache, "acme__api"))

	// The stand-in reviewer writes one LGTM comment for the PR it was given.
	reviewer := filepath.Join(tmp, "reviewer")
	script := `#!/bin/sh
printf '{"status": "reviewed", "comments": [{"pass": "review", "verdict": "lgtm", "body": "<!-- acme:review sha=%s -->\\nLGTM for #%s"}]}' "$(git rev-parse --short HEAD)" "$1" > "$REVIEW_RESULT"
`
	if err := os.WriteFile(reviewer, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := defaultConfig()
	cfg.Reviewer.Command, cfg.Reviewer.MarkerPrefix = "{reviewer} {pr}", "acme"
	env := Env{cfg: cfg, paths: paths}

	out, err := runReviewer(context.Background(), env, reviewer, p, sha)
	if err != nil {
		t.Fatal(err)
	}
	comments, lgtm, failed := commentsFor(out, sha, "acme")
	if failed != "" || !lgtm || len(comments) != 1 || !strings.HasSuffix(comments[0], "LGTM for #7") ||
		!hasHeadMarker(comments[0], "acme", sha[:7]) {
		t.Errorf("comments %q, lgtm %v, failed %q", comments, lgtm, failed)
	}
}
