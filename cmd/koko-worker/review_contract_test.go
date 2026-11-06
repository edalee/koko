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
	env, prog, args, err := reviewerCommand(`A=1 B={repo} {reviewer} {pr} --all -R {repo} --url={url} "{sha} x" ''`, "/cli", p, "abc123")
	if err != nil || prog != "/cli" || strings.Join(env, " ") != "A=1 B=o/r" {
		t.Fatalf("env %q, prog %q, %v", env, prog, err)
	}
	if want := []string{"7", "--all", "-R", "o/r", "--url=https://github.com/o/r/pull/7", "abc123 x", ""}; strings.Join(args, "|") != strings.Join(want, "|") {
		t.Errorf("args %q, want %q", args, want)
	}
	home, _ := os.UserHomeDir()
	if _, prog, _, _ := reviewerCommand("~/bin/acme {pr}", "", p, "x"); prog != filepath.Join(home, "bin/acme") {
		t.Errorf("~ not expanded: %q", prog)
	}
	for _, bad := range []string{`acme "open`, "A=1", ""} {
		if _, _, _, err := reviewerCommand(bad, "", p, "x"); err == nil {
			t.Errorf("want an error for %q", bad)
		}
	}
	if got := commandProgram("A=1 {reviewer} {pr}"); got != "{reviewer}" {
		t.Errorf("commandProgram: %q", got)
	}
}

func TestReviewerDefaults(t *testing.T) {
	cfg := defaultConfig()
	r := cfg.Reviewer
	if r.Repo != "" || r.Script != "" || r.Team.Enabled || r.Team.Team != "" || r.Runs[0].Command != "" {
		t.Errorf("a new install names no reviewer, command or team: %+v", r)
	}
	if review, _ := cfg.job(reviewDaily); review.Enabled {
		t.Error("the review job starts off")
	}
	if err := cfg.runProblem(r.Runs[0]); err == nil || !strings.Contains(err.Error(), "no command") {
		t.Errorf("problem: %v", err)
	}
	if cfg.CommandPath != "" {
		t.Errorf("the PATH for commands starts empty: %q", cfg.CommandPath)
	}
}

func TestReviewerConfigLoad(t *testing.T) {
	// The shared command and a run's arguments from before plan 037 do not
	// carry over: they ran with a read-only gh, so they may not post.
	cfg := loadTestConfig(t, `{"reviewer": {"command": "acme {pr}", "logs": "~/acme/logs", "markerPrefix": "acme",
		"runs": [{"name": "daily", "args": "--all"}]}}`)
	if run := cfg.Reviewer.Runs[0]; run.Command != "" {
		t.Errorf("old command carried over: %q", run.Command)
	}
	if err := cfg.runProblem(cfg.Reviewer.Runs[0]); err == nil || !strings.Contains(err.Error(), "posting included") {
		t.Errorf("an old run must say to set a command: %v", err)
	}

	cfg = loadTestConfig(t, `{"commandPath": " /opt/bin:/usr/bin ", "reviewer": {"runs": [{"name": "daily", "command": " acme {url} -c "}]}}`)
	if run := cfg.Reviewer.Runs[0]; run.Command != "acme {url} -c" || cfg.CommandPath != "/opt/bin:/usr/bin" {
		t.Errorf("config: %+v, %q", run, cfg.CommandPath)
	}

	// No reviewer section is a new install.
	cfg = loadTestConfig(t, `{"enabled": true}`)
	if cfg.Reviewer.Runs[0].Command != "" || cfg.Reviewer.Repo != "" {
		t.Errorf("new install: %+v", cfg.Reviewer)
	}
}

func TestReviewerProblems(t *testing.T) {
	ok := ReviewerConfig{Command: "/opt/acme {pr}", Source: SourceManaged}
	if err := ok.problem(); err != nil {
		t.Errorf("a command with its own program needs no source: %v", err)
	}
	for name, bad := range map[string]ReviewerConfig{
		"no command":           {},
		"{claude}":             {Command: "A={claude} acme {pr}"},
		"unclosed quote":       {Command: `acme "x`},
		"only env":             {Command: "A=1"},
		"{reviewer}, no repo":  {Command: "{reviewer}", Source: SourceManaged, Script: "acme"},
		"{reviewer}, no path":  {Command: "{reviewer}", Source: SourceLocal},
		"team on, no team set": {Command: "acme", Team: TeamScopeConfig{Enabled: true}},
	} {
		if bad.problem() == nil {
			t.Errorf("%s: want a problem", name)
		}
	}
}

func TestCommandEnv(t *testing.T) {
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
	env := commandEnv(base, "", []string{"A=1", "REVIEW_PR=9"}, "REVIEW_PR=7")
	if last(env, "PATH") != "/usr/bin" || last(env, "A") != "1" || last(env, "REVIEW_PR") != "7" {
		t.Errorf("env: %q", env)
	}
	// The PATH for commands replaces the worker's, and the command can still set its own.
	env = commandEnv(base, "/opt/bin:/usr/bin", nil)
	if last(env, "PATH") != "/opt/bin:/usr/bin" || strings.Count(strings.Join(env, " "), "PATH=") != 1 {
		t.Errorf("PATH: %q", env)
	}
	if env = commandEnv(base, "/opt/bin", []string{"PATH=/x"}); last(env, "PATH") != "/x" {
		t.Errorf("command's PATH: %q", env)
	}
}

func TestFindProgram(t *testing.T) {
	dir := t.TempDir()
	prog := filepath.Join(dir, "acme")
	if err := os.WriteFile(prog, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, err := findProgram("acme", "/nowhere:"+dir); err != nil || got != prog {
		t.Errorf("on the PATH for commands: %q, %v", got, err)
	}
	if _, err := findProgram("acme", "/nowhere"); err == nil || !strings.Contains(err.Error(), "PATH for commands") {
		t.Errorf("missing: %v", err)
	}
	if got, err := findProgram("/opt/acme", ""); err != nil || got != "/opt/acme" {
		t.Errorf("a path is used as it is: %q, %v", got, err)
	}
	if err := programProblem("acme {pr}", dir); err != nil {
		t.Errorf("programProblem with the PATH for commands: %v", err)
	}
}

// TestReviewWorkerRunsACommand runs a stand-in tool through runReviewer. It
// checks out the PR's head, runs the command there with the PR in its
// environment, and reads only the exit code (plan 037).
func TestReviewWorkerRunsACommand(t *testing.T) {
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

	paths := Paths{Dir: filepath.Join(tmp, "worker"), Logs: filepath.Join(tmp, "logs"), Cache: filepath.Join(tmp, "repos")}
	p := SearchPR{URL: "https://github.com/acme/api/pull/7", Number: 7}
	p.Repository.NameWithOwner = "acme/api"
	git(tmp, "clone", "--quiet", origin, filepath.Join(paths.Cache, "acme__api"))

	// The stand-in tool writes what it saw to a file, then exits with $1.
	seen := filepath.Join(tmp, "seen")
	tool := filepath.Join(tmp, "tool")
	script := `#!/bin/sh
echo "$(git rev-parse HEAD) $REVIEW_PR $REVIEW_REPO $REVIEW_PR_URL $RUN_KIND $2" > ` + seen + `
echo "the last line"
exit "$1"
`
	if err := os.WriteFile(tool, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(code string) error {
		cfg := defaultConfig()
		cfg.Reviewer.Command = "{reviewer} " + code + " {url}"
		return runReviewer(context.Background(), Env{cfg: cfg, paths: paths}, tool, p, sha)
	}

	if err := run("0"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(seen)
	if want := sha + " 7 acme/api " + p.URL + " prs " + p.URL + "\n"; string(got) != want {
		t.Errorf("the tool saw %q, want %q", got, want)
	}
	var skipped reviewSkippedError
	if err := run("2"); !errors.As(err, &skipped) {
		t.Errorf("exit 2: %v", err)
	}
	err := run("1")
	if err == nil || errors.As(err, &skipped) || !strings.Contains(err.Error(), "the last line") || !strings.Contains(err.Error(), paths.Logs) {
		t.Errorf("exit 1 must fail with its last line and log: %v", err)
	}
}
