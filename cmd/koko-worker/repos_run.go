package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A repos run (plan 035) runs the shared command, with the run's arguments,
// once per repo, for a tool that changes repos, such as one that updates
// dependencies. It has no markers, review records or comments:
//
//   - The command runs in a clean checkout of the repo's default branch, at
//     its newest commit. {reviewer}, {claude} and {repo} are filled in. {pr},
//     {url} and {sha} name a PR, so runProblem refuses them.
//   - The environment carries REVIEW_REPO and RUN_KIND=repos. Unlike a
//     review, gh and git are not read-only, so the tool can push and open PRs.
//     {claude} is still the review wrapper, which cannot push.
//   - Exit code 0 means done, 2 means nothing to do, anything else failed.
//
// Repos run one after another. One Slack DM reports them all.

// repoStatus is how one repo of a repos run ended.
const (
	repoDone    = "done"
	repoNothing = "nothing to do"
	repoFailed  = "failed"
)

// repoOutcome is how one repo of a repos run ended.
type repoOutcome struct {
	repo   string
	status string   // repoDone, repoNothing or repoFailed
	why    string   // for a failure: the error, or the last line of output
	tail   string   // the last outputLines lines of output
	urls   []string // links in the output, such as a PR the tool opened
	log    string   // the worker's log of the run
}

// outputLines is how much of a repo's output the worker keeps in memory.
const outputLines = 20

var urlRe = regexp.MustCompile(`https?://[^\s<>"'()\[\]]+`)

// repoCheckout gives a clean checkout of repo's default branch and a
// function to release it. Tests swap it for a local folder.
var repoCheckout = cleanCheckout

// cleanCheckout makes the cache clone of repo a clean copy of the newest
// commit on its default branch, and holds the clone's lock until released, so
// no review checks out a PR under the run. A clone with local changes or
// stray files is reset: only the worker uses it.
func cleanCheckout(ctx context.Context, paths Paths, repo string) (string, func(), error) {
	dir := filepath.Join(paths.Cache, strings.ReplaceAll(repo, "/", "__"))
	if err := os.MkdirAll(paths.Cache, 0o700); err != nil {
		return "", nil, err
	}
	unlock, err := lockFile(dir + ".lock")
	if err != nil {
		return "", nil, err
	}
	fail := func(err error) (string, func(), error) {
		unlock()
		return "", nil, err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		cloneCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		out, err := exec.CommandContext(cloneCtx, "gh", "repo", "clone", repo, dir, "--", "--filter=blob:none", "--quiet").CombinedOutput()
		cancel()
		if err != nil {
			_ = os.RemoveAll(dir) // a half-made clone would break every later run
			return fail(fmt.Errorf("clone %s: %v: %s", repo, err, truncate(strings.TrimSpace(string(out)), 300)))
		}
	}
	gitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	// origin/HEAD names the default branch. --auto asks origin, so a renamed
	// default branch is picked up.
	if _, err := gitOut(gitCtx, dir, "remote", "set-head", "origin", "--auto"); err != nil {
		return fail(err)
	}
	head, err := gitOut(gitCtx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return fail(err)
	}
	branch := strings.TrimPrefix(head, "origin/")
	for _, args := range [][]string{
		{"fetch", "--quiet", "origin", branch},
		{"checkout", "--quiet", "--force", "-B", branch, "FETCH_HEAD"},
		{"clean", "-ffdxq"},
	} {
		if _, err := gitOut(gitCtx, dir, args...); err != nil {
			return fail(err)
		}
	}
	return dir, unlock, nil
}

// runRepos runs the command once per repo of run, one after another, then
// sends one DM about them all. A test run prints what it would run, and runs
// nothing, because the tool changes the repos.
func runRepos(ctx context.Context, env Env, run ReviewRun) error {
	if !env.test {
		// The managed reviewer clone and the wrapper are shared with review runs.
		unlock, err := lockFile(filepath.Join(env.paths.Dir, "reviewer.lock"))
		if err != nil {
			return err
		}
		defer unlock()
	}
	cli, err := prepareReviewer(ctx, env)
	if err != nil {
		return err
	}
	if env.test {
		var b strings.Builder
		fmt.Fprintf(&b, "The %s run would run, one repo after another, in a clean checkout of the default branch:\n", run.Name)
		for _, repo := range run.Repos {
			_, prog, args, err := repoCommand(env, cli, repo)
			if err != nil {
				return err
			}
			fmt.Fprintf(&b, "• %s: %s\n", repo, strings.Join(append([]string{prog}, args...), " "))
		}
		fmt.Fprintf(&b, "Each repo may take up to %s. Nothing runs in a test.", env.runTimeout())
		return env.notify(ctx, b.String())
	}
	if err := os.WriteFile(env.paths.ClaudeWrap, []byte(claudeWrapper), 0o700); err != nil {
		return err
	}
	var outcomes []repoOutcome
	for _, repo := range run.Repos {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		o := runInRepo(ctx, env, cli, repo)
		log.Printf("%s: %s: %s %s", reviewJob(run.Name), repo, o.status, o.why)
		outcomes = append(outcomes, o)
	}
	if err := env.notify(ctx, reposMessage(run, outcomes)); err != nil {
		return err
	}
	var failed []string
	for _, o := range outcomes {
		if o.status == repoFailed {
			failed = append(failed, o.repo+": "+o.why)
		}
	}
	if len(failed) > 0 {
		// The DM has said so, so the scheduler does not send another.
		return reportedError{fmt.Errorf("%s: %s", run.Name, strings.Join(failed, "; "))}
	}
	return nil
}

// repoCommand is the command for one repo: the program, its arguments and
// the environment the command sets.
func repoCommand(env Env, cli, repo string) (cmdEnv []string, prog string, args []string, err error) {
	var p SearchPR
	p.Repository.NameWithOwner = repo
	return reviewerCommand(env.cfg.Reviewer.Command, cli, env.paths.ClaudeWrap, p, "")
}

// runInRepo runs the command in a clean checkout of repo, and keeps its log.
func runInRepo(ctx context.Context, env Env, cli, repo string) repoOutcome {
	o := repoOutcome{repo: repo, status: repoFailed}
	dir, release, err := repoCheckout(ctx, env.paths, repo)
	if err != nil {
		o.why = truncate(err.Error(), 300)
		return o
	}
	defer release()
	cmdEnv, prog, args, err := repoCommand(env, cli, repo)
	if err != nil {
		o.why = err.Error()
		return o
	}
	ctx, cancel := context.WithTimeout(ctx, env.runTimeout())
	defer cancel()
	cmd := groupCommand(ctx, prog, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), cmdEnv...), "REVIEW_REPO="+repo, "RUN_KIND=repos")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()

	_ = os.MkdirAll(env.paths.Logs, 0o700)
	o.log = filepath.Join(env.paths.Logs, fmt.Sprintf("%srun-%s-%s.log", reviewLogPrefix, shortRepo(repo), time.Now().Format("20060102-150405")))
	_ = os.WriteFile(o.log, out.Bytes(), 0o600)
	o.tail = lastLines(out.String(), outputLines)
	o.urls = findURLs(out.String())
	var exit *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		o.why = fmt.Sprintf("timed out after %s", env.runTimeout())
	case err == nil:
		o.status = repoDone
	case errors.As(err, &exit) && exit.ExitCode() == skipExit:
		o.status = repoNothing
	default:
		o.why = truncate(fmt.Sprintf("%v: %s", err, lastLines(o.tail, 1)), 300)
	}
	return o
}

// findURLs lists the links in out, once each, at most five.
func findURLs(out string) []string {
	var urls []string
	seen := map[string]bool{}
	for _, u := range urlRe.FindAllString(out, -1) {
		u = strings.TrimRight(u, ".,;:")
		if !seen[u] && len(urls) < 5 {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	return urls
}

// reposMessage is the DM after a repos run: one line per repo, with its
// links, and the log of a failed repo.
func reposMessage(run ReviewRun, outcomes []repoOutcome) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*The %s run*\n", run.Name)
	for _, o := range outcomes {
		fmt.Fprintf(&b, "• %s: %s", o.repo, o.status)
		if o.status == repoFailed {
			fmt.Fprintf(&b, " (%s)", o.why)
		}
		for _, u := range o.urls {
			b.WriteString(" " + slackLink(u, u))
		}
		if o.status == repoFailed && o.log != "" {
			fmt.Fprintf(&b, "\n    Log: %s", o.log)
		}
		b.WriteString("\n")
	}
	return b.String()
}
