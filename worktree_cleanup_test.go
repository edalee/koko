package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newRepoWithWorktrees makes a repo with one commit and a linked worktree
// for each name, and an App over a fake HOME.
func newRepoWithWorktrees(t *testing.T, names ...string) (*App, string, []string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	base, _ := filepath.EvalSymlinks(t.TempDir())
	repo := filepath.Join(base, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	git(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "init")
	var paths []string
	for _, n := range names {
		p := filepath.Join(base, n)
		git(t, repo, "worktree", "add", "-q", "-b", n, p)
		paths = append(paths, p)
	}
	app := &App{tm: newTestManager(), cfg: NewConfigService(), git: NewGitService()}
	return app, repo, paths
}

func TestRemoveWorktrees_RemovesACleanOne(t *testing.T) {
	app, _, wt := newRepoWithWorktrees(t, "wt1")

	got := app.RemoveWorktrees(wt)

	if len(got.Removed) != 1 || len(got.Skipped) != 0 {
		t.Fatalf("unexpected result %+v", got)
	}
	if pathExists(wt[0]) {
		t.Error("the worktree is still there")
	}
}

// Forcing in bulk could destroy work git cannot give back.
func TestRemoveWorktrees_SkipsADirtyOne(t *testing.T) {
	app, _, wt := newRepoWithWorktrees(t, "wt1")
	if err := os.WriteFile(filepath.Join(wt[0], "new.txt"), []byte("work"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := app.RemoveWorktrees(wt)

	if len(got.Removed) != 0 || len(got.Skipped) != 1 || got.Skipped[0].Reason != "has uncommitted or ignored files" {
		t.Fatalf("unexpected result %+v", got)
	}
	if !pathExists(filepath.Join(wt[0], "new.txt")) {
		t.Error("uncommitted work was lost")
	}
}

func TestRemoveWorktrees_SkipsOneASessionUses(t *testing.T) {
	app, _, wt := newRepoWithWorktrees(t, "wt1")
	addSession(app.tm, "koko-1", filepath.Join(wt[0], "sub"), "")

	got := app.RemoveWorktrees(wt)

	if len(got.Skipped) != 1 || got.Skipped[0].Reason != "in use by a session" {
		t.Fatalf("unexpected result %+v", got)
	}
	if !pathExists(wt[0]) {
		t.Error("a worktree in use was removed")
	}
}

func TestRemoveWorktrees_SkipsOneASavedTabUses(t *testing.T) {
	app, _, wt := newRepoWithWorktrees(t, "wt1")
	if err := app.cfg.SaveSessions(SessionsData{Sessions: []SessionRecord{
		{Slug: "koko-1", Directory: wt[0], Status: "disconnected"},
	}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got := app.RemoveWorktrees(wt)

	if len(got.Skipped) != 1 || got.Skipped[0].Reason != "in use by a session" {
		t.Fatalf("unexpected result %+v", got)
	}
}

// A plain `git worktree remove` deletes ignored files, which are often a
// local .env or settings file.
func TestRemoveWorktrees_SkipsOneWithIgnoredFiles(t *testing.T) {
	app, repo, _ := newRepoWithWorktrees(t)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".env\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	git(t, repo, "add", ".gitignore")
	git(t, repo, "commit", "-q", "-m", "ignore")
	wt := filepath.Join(filepath.Dir(repo), "wt1")
	git(t, repo, "worktree", "add", "-q", "-b", "wt1", wt)
	if err := os.WriteFile(filepath.Join(wt, ".env"), []byte("local"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := app.RemoveWorktrees([]string{wt})

	if len(got.Skipped) != 1 || got.Skipped[0].Reason != "has uncommitted or ignored files" {
		t.Fatalf("unexpected result %+v", got)
	}
	if !pathExists(filepath.Join(wt, ".env")) {
		t.Error("an ignored file was deleted")
	}
}

// Closing a disconnected tab leaves its dead session entry behind. That
// entry must not block removal.
func TestRemoveWorktrees_ADeadSessionDoesNotHold(t *testing.T) {
	app, _, wt := newRepoWithWorktrees(t, "wt1")
	s := addSession(app.tm, "koko-1", wt[0], "")
	close(s.done)

	if got := app.RemoveWorktrees(wt); len(got.Removed) != 1 {
		t.Fatalf("unexpected result %+v", got)
	}
}

// A closed record is history, so the cleanup may remove its worktree.
func TestRemoveWorktrees_AClosedRecordDoesNotHold(t *testing.T) {
	app, _, wt := newRepoWithWorktrees(t, "wt1")
	if err := app.cfg.SaveSessions(SessionsData{Sessions: []SessionRecord{
		{Slug: "koko-1", Directory: wt[0], WorktreePath: wt[0], Status: "closed"},
	}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	if got := app.RemoveWorktrees(wt); len(got.Removed) != 1 {
		t.Fatalf("unexpected result %+v", got)
	}
}

func TestRemoveWorktrees_RefusesTheMainCheckout(t *testing.T) {
	app, repo, _ := newRepoWithWorktrees(t)

	got := app.RemoveWorktrees([]string{repo})

	if len(got.Skipped) != 1 || got.Skipped[0].Reason != "not a linked worktree" {
		t.Fatalf("unexpected result %+v", got)
	}
	if !pathExists(filepath.Join(repo, "a.txt")) {
		t.Error("the main checkout was touched")
	}
}

func TestRemoveWorktrees_RefusesAFolderInsideAWorktree(t *testing.T) {
	app, _, wt := newRepoWithWorktrees(t, "wt1")
	sub := filepath.Join(wt[0], "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := app.RemoveWorktrees([]string{sub})

	if len(got.Skipped) != 1 || got.Skipped[0].Reason != "not a linked worktree" {
		t.Fatalf("unexpected result %+v", got)
	}
}

// A path that no longer exists is reported as gone, so the frontend forgets
// it without claiming to have removed it.
func TestRemoveWorktrees_AMissingPathIsReportedGone(t *testing.T) {
	app, _, _ := newRepoWithWorktrees(t)
	gone := filepath.Join(t.TempDir(), "gone")

	got := app.RemoveWorktrees([]string{gone})

	if len(got.Removed) != 0 || len(got.Gone) != 1 || got.Gone[0] != gone {
		t.Fatalf("unexpected result %+v", got)
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
