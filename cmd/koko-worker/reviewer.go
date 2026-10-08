package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// prepareReviewer makes the reviewer CLI ready and returns its path.
//
// Bad reviewer settings fail only the review job, so the stand-up and focus
// time keep running. A command without {reviewer} runs its own program, and
// the result is "". A local reviewer runs as it is.
//
// A managed reviewer is the worker's own clone of Reviewer.Repo, so the
// branch you have checked out in your own clone never changes what reviews
// run. A scheduled run or Run now clones it the first time, and replaces a
// clone of another repo, so a repo changed in Settings takes effect. It then
// moves the clone to the newest commit on Reviewer.Branch, with AutoUpdate on
// or when the branch changed. A failed fetch keeps the last good copy.
//
// A test run never clones, replaces or updates the clone. It takes no job
// lock, so a scheduled review may be reading from the clone at the time.
func prepareReviewer(ctx context.Context, env Env) (string, error) {
	r := env.cfg.Reviewer
	if err := r.problem(); err != nil {
		return "", fmt.Errorf("review: %w. Fix it in Settings > Worker > Review worker", err)
	}
	if !r.usesReviewer() {
		return "", programProblem(r.Command)
	}
	cli := env.cfg.reviewerCLI(env.paths)
	if r.Source == SourceLocal {
		if !executable(cli) {
			return "", fmt.Errorf("review: no reviewer CLI at %s", cli)
		}
		return cli, nil
	}

	dir := env.paths.Reviewer
	_, statErr := os.Stat(filepath.Join(dir, ".git"))
	cloned := statErr == nil
	sameRepo := false
	if cloned {
		origin, err := gitOut(ctx, dir, "remote", "get-url", "origin")
		sameRepo = err == nil && originIs(origin, r.Repo)
	}
	if env.test {
		switch {
		case !cloned:
			return "", fmt.Errorf("review: the reviewer %s is not cloned yet. A scheduled run or Run now clones it, and a test does not", r.Repo)
		case !sameRepo:
			return "", fmt.Errorf("review: the reviewer clone is not of %s. A scheduled run or Run now replaces it, and a test does not", r.Repo)
		}
		if !executable(cli) {
			return "", fmt.Errorf("review: no reviewer CLI at %s in the %s clone", r.Script, r.Repo)
		}
		return cli, nil
	}

	fresh := false
	if cloned && !sameRepo {
		log.Printf("review: the reviewer clone is not of %s. Cloning it again", r.Repo)
		if err := os.RemoveAll(dir); err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if err := cloneRepo(ctx, r.Repo, dir); err != nil {
			return "", err
		}
		fresh = true
	}
	branch := env.state.ReviewerBranch
	switch {
	case fresh:
		if err := checkoutBranch(ctx, dir, r.Branch); err != nil {
			return "", err
		}
		branch = r.Branch
	case r.AutoUpdate || branch != r.Branch:
		if err := checkoutBranch(ctx, dir, r.Branch); err != nil {
			// Offline, or the branch is gone: review with what is there.
			log.Printf("review: updating the reviewer: %v. Using the last good copy", err)
		} else {
			branch = r.Branch
		}
	}
	if !executable(cli) {
		return "", fmt.Errorf("review: no reviewer CLI at %s in the %s clone", r.Script, r.Repo)
	}
	if sha, err := gitOut(ctx, dir, "rev-parse", "--short", "HEAD"); err == nil {
		_ = env.persist(func(st *State) {
			if st.ReviewerCommit != sha {
				st.ReviewerUpdated = time.Now()
			}
			st.ReviewerCommit, st.ReviewerBranch = sha, branch
		})
	}
	return cli, nil
}

// programProblem says why a command's program cannot run, or nil. A bare
// name is looked up on the worker's PATH, which under launchd is short.
func programProblem(command string) error {
	prog := commandProgram(command)
	switch {
	case strings.Contains(prog, "{"):
		return nil // a placeholder, such as {claude}, filled in at run time
	case strings.Contains(prog, "/") && !filepath.IsAbs(prog):
		// The reviewer runs in the PR's clone, so a relative path would
		// point somewhere else at run time than here.
		return fmt.Errorf("review: the reviewer program %s is a relative path. Use its full path", prog)
	}
	if strings.Contains(prog, "/") {
		if !executable(prog) {
			return fmt.Errorf("review: no reviewer program at %s", prog)
		}
		return nil
	}
	if _, err := exec.LookPath(prog); err != nil {
		return fmt.Errorf("review: the reviewer program %s is not on the worker's PATH. Use its full path", prog)
	}
	return nil
}

// originIs is true when a clone's origin URL points at "owner/repo", over
// https or ssh, with or without ".git".
func originIs(url, repo string) bool {
	u := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(url)), ".git")
	r := strings.ToLower(repo)
	return strings.HasSuffix(u, "/"+r) || strings.HasSuffix(u, ":"+r)
}

// cloneRepo clones repo into dir. Tests swap it for a clone of a local repo.
var cloneRepo = cloneReviewer

func cloneReviewer(ctx context.Context, repo, dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", "repo", "clone", repo, dir, "--", "--quiet").CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(dir) // a half-made clone would break every later run
		return fmt.Errorf("review: clone %s: %v: %s", repo, err, truncate(strings.TrimSpace(string(out)), 300))
	}
	return nil
}

// checkoutBranch moves the clone to the newest commit on branch. --force
// drops any change in the clone, which only the worker uses.
func checkoutBranch(ctx context.Context, dir, branch string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := gitOut(ctx, dir, "fetch", "--quiet", "origin", branch); err != nil {
		return err
	}
	_, err := gitOut(ctx, dir, "checkout", "--quiet", "--force", "--detach", "FETCH_HEAD")
	return err
}

func gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", args[0], truncate(strings.TrimSpace(string(out)), 300))
	}
	return strings.TrimSpace(string(out)), nil
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}
