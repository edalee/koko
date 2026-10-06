package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRun runs git in dir and fails the test on error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// reviewerRemote makes a bare repo at root/owner/name.git with a "tono"
// script on main and on dev, and returns a working clone for new commits.
func reviewerRemote(t *testing.T, root, repo string) (bare, work string) {
	t.Helper()
	bare = filepath.Join(root, repo+".git")
	work = filepath.Join(root, "work-"+strings.ReplaceAll(repo, "/", "-"))
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, bare, "init", "--quiet", "--bare", "--initial-branch=main")
	gitRun(t, root, "clone", "--quiet", bare, work)
	commitScript(t, work, "v1")
	gitRun(t, work, "push", "--quiet", "origin", "HEAD:main")
	gitRun(t, work, "push", "--quiet", "origin", "HEAD:dev")
	return bare, work
}

func commitScript(t *testing.T, work, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(work, "tono"), []byte("#!/bin/sh\necho "+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "add", "tono")
	gitRun(t, work, "commit", "--quiet", "-m", body)
	return gitRun(t, work, "rev-parse", "--short", "HEAD")
}

func TestPrepareReviewer(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	_, work := reviewerRemote(t, root, "o/reviewer")
	otherBare, _ := reviewerRemote(t, root, "o/other")

	// cloneRepo clones the local bare repo that stands in for GitHub.
	saved := cloneRepo
	t.Cleanup(func() { cloneRepo = saved })
	cloneRepo = func(_ context.Context, repo, dir string) error {
		gitRun(t, root, "clone", "--quiet", filepath.Join(root, repo+".git"), dir)
		return nil
	}

	st := &State{}
	cfg := defaultConfig()
	cfg.Reviewer.Repo = "o/reviewer"
	newEnv := func(test bool) Env {
		return Env{cfg: cfg, paths: Paths{Reviewer: filepath.Join(root, "clone")}, state: st, test: test,
			persist: func(change func(*State)) error { change(st); return nil }}
	}
	ctx := context.Background()
	head := func() string { return gitRun(t, filepath.Join(root, "clone"), "rev-parse", "--short", "HEAD") }

	// A test run never clones.
	if _, err := prepareReviewer(ctx, newEnv(true)); err == nil || !strings.Contains(err.Error(), "not cloned yet") {
		t.Errorf("a test run before the first clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "clone")); !os.IsNotExist(err) {
		t.Error("a test run must not create the clone")
	}

	// The first run clones and checks out main.
	cli, err := prepareReviewer(ctx, newEnv(false))
	if err != nil || cli != filepath.Join(root, "clone", "tono") || st.ReviewerBranch != "main" || st.ReviewerCommit != head() {
		t.Fatalf("first run: cli %q, err %v, state %+v", cli, err, st)
	}

	// A new commit on main: a scheduled run moves to it, a test run does not.
	v2 := commitScript(t, work, "v2")
	gitRun(t, work, "push", "--quiet", "origin", "HEAD:main")
	if _, err := prepareReviewer(ctx, newEnv(true)); err != nil || head() == v2 {
		t.Errorf("a test run must not update: err %v, head %s", err, head())
	}
	if _, err := prepareReviewer(ctx, newEnv(false)); err != nil || head() != v2 || st.ReviewerCommit != v2 {
		t.Errorf("update: err %v, head %s, want %s", err, head(), v2)
	}

	// A failed fetch keeps the last good copy, and the branch it is on.
	cfg.Reviewer.Branch = "no-such-branch"
	if _, err := prepareReviewer(ctx, newEnv(false)); err != nil || head() != v2 || st.ReviewerBranch != "main" {
		t.Errorf("fallback: err %v, head %s, branch %s", err, head(), st.ReviewerBranch)
	}

	// A branch change applies even with AutoUpdate off.
	cfg.Reviewer.Branch, cfg.Reviewer.AutoUpdate = "dev", false
	if _, err := prepareReviewer(ctx, newEnv(false)); err != nil || st.ReviewerBranch != "dev" || head() == v2 {
		t.Errorf("branch change: err %v, branch %s, head %s", err, st.ReviewerBranch, head())
	}

	// After a repo change, a test run leaves the clone alone, and a real run
	// replaces it.
	cfg.Reviewer.Repo, cfg.Reviewer.Branch = "o/other", "main"
	if _, err := prepareReviewer(ctx, newEnv(true)); err == nil || !strings.Contains(err.Error(), "not of o/other") {
		t.Errorf("a test run after a repo change: %v", err)
	}
	if origin := gitRun(t, filepath.Join(root, "clone"), "remote", "get-url", "origin"); originIs(origin, "o/other") {
		t.Error("a test run must not replace the clone")
	}
	if _, err := prepareReviewer(ctx, newEnv(false)); err != nil {
		t.Fatal(err)
	}
	if origin := gitRun(t, filepath.Join(root, "clone"), "remote", "get-url", "origin"); origin != otherBare {
		t.Errorf("repo change: origin %s, want %s", origin, otherBare)
	}

	// A local source with no path fails the review job only.
	cfg.Reviewer.Source, cfg.Reviewer.Path = SourceLocal, ""
	if _, err := prepareReviewer(ctx, newEnv(false)); err == nil || !strings.Contains(err.Error(), "no path is set") {
		t.Errorf("local without a path: %v", err)
	}
	if cfg.validate() != nil {
		t.Error("a local source without a path must not invalidate the whole config")
	}
}

func TestOriginIs(t *testing.T) {
	for _, url := range []string{
		"git@github.com:epidemicsound/tonometer.git",
		"https://github.com/epidemicsound/tonometer",
		"https://github.com/Epidemicsound/Tonometer.git",
	} {
		if !originIs(url, "epidemicsound/tonometer") {
			t.Errorf("%s should match", url)
		}
	}
	if originIs("git@github.com:epidemicsound/tonometer-fork.git", "epidemicsound/tonometer") {
		t.Error("a different repo must not match")
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := expandHome("~/repos/tonometer/tono"); got != filepath.Join(home, "repos/tonometer/tono") {
		t.Errorf("got %q", got)
	}
	if got := expandHome("/opt/tono"); got != "/opt/tono" {
		t.Errorf("got %q", got)
	}
}
