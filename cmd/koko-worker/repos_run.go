package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// A repos run (plan 035) runs the shared command, with the run's arguments,
// once per repo, for a tool that changes repos, such as one that updates
// dependencies. It has no markers, review records or comments:
//
//   - The command runs in a clone of the repo that only repos runs use, reset
//     to the newest commit of the default branch, with no hooks. {reviewer},
//     {claude} and {repo} are filled in. {pr}, {url} and {sha} name a PR, so
//     runProblem refuses them.
//   - The environment carries REVIEW_REPO and RUN_KIND=repos. gh and git are
//     not the read-only stand-ins of a review, so the tool can push and open
//     PRs, which is its job. {claude} is the review wrapper, which denies some
//     commands, such as git push, but is not a guard.
//   - Exit code 0 means done, 2 means nothing to do, anything else failed.
//
// Repos run one after another. A scheduled run saves each repo's outcome for
// the slot as it ends, so a retry of the slot never runs a repo twice. One
// Slack DM reports them all.

// The statuses one repo of a repos run can end in. repoStarted is saved
// once the repo's tool starts, so a retry knows a repo the run was stopped in.
const (
	repoStarted = "started"
	repoDone    = "done"
	repoNothing = "nothing to do"
	repoFailed  = "failed"
)

// RepoRecord is how one repo of a repos run's slot ended, kept in the state.
type RepoRecord struct {
	Status string    `json:"status"`
	Why    string    `json:"why,omitempty"`
	URLs   []string  `json:"urls,omitempty"`
	Log    string    `json:"log,omitempty"`
	At     time.Time `json:"at"`
}

// repoRunKey keys a repo of a repos run's slot in State.RepoRuns.
func repoRunKey(job string, slot time.Time, repo string) string {
	return slotKey(job, slot) + "|" + repo
}

// repoOutcome is how one repo of a repos run ended.
type repoOutcome struct {
	repo   string
	status string   // repoDone, repoNothing or repoFailed
	why    string   // for a failure: the error, then the last line of output
	urls   []string // links in the output, such as a PR the tool opened
	log    string   // path of the file with this repo's command output
}

func (o repoOutcome) record() RepoRecord {
	return RepoRecord{Status: o.status, Why: o.why, URLs: o.urls, Log: o.log, At: time.Now()}
}

// maxLineBytes caps the line of output kept in memory. Longer lines are cut.
const maxLineBytes = 4096

// maxURLs is how many links from a repo's output the DM shows: the last ones,
// because a tool prints the link to the PR it opened near the end.
const maxURLs = 5

var urlRe = regexp.MustCompile(`https?://[^\s<>"'()\[\]]+`)

// tailWriter keeps the last line of what is written to it, and the last
// maxURLs links, so a tool's output never grows the memory. The whole output
// goes to the repo's log file.
type tailWriter struct {
	last    string
	partial []byte
	urls    []string
}

func (t *tailWriter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			t.keep(p)
			break
		}
		t.keep(p[:i])
		t.line(string(t.partial))
		t.partial = t.partial[:0]
		p = p[i+1:]
	}
	return n, nil
}

// keep adds b to the line so far, up to maxLineBytes.
func (t *tailWriter) keep(b []byte) {
	if room := maxLineBytes - len(t.partial); room > 0 {
		t.partial = append(t.partial, b[:min(room, len(b))]...)
	}
}

// line takes one whole line: its links, and the line itself if it has text.
// A link seen before moves to the end.
func (t *tailWriter) line(l string) {
	for _, u := range urlRe.FindAllString(l, -1) {
		u = strings.TrimRight(u, ".,;:")
		t.urls = append(slices.DeleteFunc(t.urls, func(s string) bool { return s == u }), u)
		if len(t.urls) > maxURLs {
			t.urls = t.urls[len(t.urls)-maxURLs:]
		}
	}
	if s := strings.TrimSpace(l); s != "" {
		t.last = s
	}
}

// lastLine is the last line of output with text, a last line without a
// newline too.
func (t *tailWriter) lastLine() string {
	if s := strings.TrimSpace(string(t.partial)); s != "" {
		return s
	}
	return t.last
}

// repoCheckout gives a clean checkout of repo's default branch and a
// function to release it. Tests swap it for a local folder.
var repoCheckout = cleanCheckout

// cleanCheckout makes the repos runs' clone of repo, which every repos run
// shares, a clean copy of the newest commit on its default branch, and holds
// the clone's lock until released. It removes any hooks first: a tool may
// install some, such as with a hooks path, and git would run them on the next
// checkout. A clone with local changes or stray files is reset: only the
// worker uses it. If a git step fails, the clone is deleted, so the next run
// clones it fresh rather than meet the same broken clone.
func cleanCheckout(ctx context.Context, paths Paths, repo string) (string, func(), error) {
	dir := filepath.Join(paths.RepoRuns, strings.ReplaceAll(repo, "/", "__"))
	if err := os.MkdirAll(paths.RepoRuns, 0o700); err != nil {
		return "", nil, err
	}
	unlock, err := lockFile(dir + ".lock")
	if err != nil {
		return "", nil, err
	}
	fail := func(err error) (string, func(), error) {
		_ = os.RemoveAll(dir)
		unlock()
		return "", nil, err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		cloneCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		out, err := exec.CommandContext(cloneCtx, "gh", "repo", "clone", repo, dir, "--", "--filter=blob:none", "--quiet").CombinedOutput()
		cancel()
		if err != nil {
			return fail(fmt.Errorf("clone %s: %v: %s", repo, err, truncate(strings.TrimSpace(string(out)), 300)))
		}
	}
	if err := removeHooks(dir); err != nil {
		return fail(err)
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

// removeHooks unsets the clone's core.hooksPath and deletes its hooks, all
// but git's .sample files, so no git command the worker runs starts one.
func removeHooks(dir string) error {
	// Exit code 5 means the key was not set.
	cmd := exec.Command("git", "-C", dir, "config", "--unset-all", "core.hooksPath")
	var exit *exec.ExitError
	out, err := cmd.CombinedOutput()
	notSet := errors.As(err, &exit) && exit.ExitCode() == 5
	if err != nil && !notSet {
		return fmt.Errorf("git config: %s", truncate(strings.TrimSpace(string(out)), 300))
	}
	hooks := filepath.Join(dir, ".git", "hooks")
	entries, err := os.ReadDir(hooks)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sample") {
			if err := os.RemoveAll(filepath.Join(hooks, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// stopError is a scheduled repos run that stopped because you switched the
// worker or the run off, or removed the run. runDue records it as final.
type stopError struct{ error }

// stopReason says why a scheduled run of job should stop, or "". It reads
// worker.json again. A file it cannot read is no reason: the run goes on.
func stopReason(env Env, job string) string {
	cfg, err := loadConfig(env.paths.Config)
	if err != nil {
		return ""
	}
	run, ok := cfg.reviewRun(job)
	switch {
	case !cfg.Enabled:
		return "the worker is switched off"
	case !ok:
		return "the run was removed"
	case !run.Enabled:
		return "the run is switched off"
	}
	return ""
}

// runRepos runs the command once per repo of run, one after another, then
// sends one DM about them all. A test run prints what it would run, and runs
// nothing, because the tool changes the repos.
//
// A scheduled run saves each repo's outcome for its slot. A repo is saved
// once its tool starts, so a retry of the slot, after a stop, a failed DM or
// a failed checkout, runs only the repos whose tool never started. A repo the
// run was stopped in counts as failed, and does not run again, because the
// tool may have pushed part of its work. A repo whose clone or fetch failed is
// not saved: the error is one to retry, and the DM waits for that repo.
//
// A scheduled run reads worker.json again before each repo, and stops if you
// switched the worker or the run off, or removed the run.
//
// A Run now never reads or writes the slot's records: it runs every repo and
// sends its own DM.
func runRepos(ctx context.Context, env Env, run ReviewRun) error {
	job := reviewJob(run.Name)
	if !env.test {
		// The managed reviewer clone and the wrapper are shared with review runs.
		unlock, err := takeLock(filepath.Join(env.paths.Dir, "reviewer.lock"), env.noWait)
		if err != nil {
			return err
		}
		defer unlock()
		st := loadState(env.paths.State)
		env.state = &st
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
	scheduled := env.scheduled && !env.slot.IsZero()
	save := func(repo string, rec RepoRecord) error {
		if !scheduled {
			return nil
		}
		return env.persist(func(st *State) { st.RepoRuns[repoRunKey(job, env.slot, repo)] = rec })
	}
	var outcomes []repoOutcome
	var pending []string // repos whose checkout failed, for a retry
	for _, repo := range run.Repos {
		if scheduled {
			if why := stopReason(env, job); why != "" {
				return stopRepos(env, run, outcomes, why)
			}
		}
		if ctx.Err() != nil {
			break
		}
		if rec, ok := env.state.RepoRuns[repoRunKey(job, env.slot, repo)]; scheduled && ok {
			o := repoOutcome{repo: repo, status: rec.Status, why: rec.Why, urls: rec.URLs, log: rec.Log}
			if o.status == repoStarted {
				o.status, o.why = repoFailed, "the run stopped before it finished, so it does not run again in this slot"
			}
			outcomes = append(outcomes, o)
			continue
		}
		dir, release, err := repoCheckout(ctx, env.paths, repo)
		if err != nil {
			log.Printf("%s: %s: could not get the repo: %v", job, repo, err)
			pending = append(pending, fmt.Sprintf("%s (%s)", repo, truncate(err.Error(), 200)))
			continue
		}
		if err := save(repo, RepoRecord{Status: repoStarted, At: time.Now()}); err != nil {
			release()
			return err
		}
		o := runInRepo(ctx, env, cli, repo, dir)
		release()
		log.Printf("%s: %s: %s %s", job, repo, o.status, o.why)
		if err := save(repo, o.record()); err != nil {
			return err
		}
		outcomes = append(outcomes, o)
	}
	if ctx.Err() != nil {
		if scheduled {
			if why := stopReason(env, job); why != "" {
				return stopRepos(env, run, outcomes, why)
			}
		}
		return ctx.Err()
	}
	if len(pending) > 0 {
		// No DM yet: it waits for these repos. A retry runs only them.
		return fmt.Errorf("%s: could not get %s, trying again. So far: %s", run.Name, strings.Join(pending, ", "), outcomeLine(outcomes))
	}
	if !scheduled || !env.state.Runs[slotKey(job, env.slot)].Reported {
		if err := env.notify(ctx, reposMessage(run, outcomes)); err != nil {
			// Not reported, so the slot retries. The retry finds every repo
			// saved, and only sends the DM.
			return fmt.Errorf("%s: could not send the DM, trying again: %w", run.Name, err)
		}
		if err := markReported(env, job, scheduled); err != nil {
			return err
		}
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

// markReported saves that the slot's DM went out, for a scheduled run.
func markReported(env Env, job string, scheduled bool) error {
	if !scheduled {
		return nil
	}
	return env.persist(func(st *State) {
		rec := st.Runs[slotKey(job, env.slot)]
		rec.Reported = true
		st.Runs[slotKey(job, env.slot)] = rec
	})
}

// stopRepos ends a scheduled run you stopped: one DM with the repos done so
// far and why it stopped, then a stopError. The run's context may be
// cancelled, so the DM gets its own.
func stopRepos(env Env, run ReviewRun, outcomes []repoOutcome, why string) error {
	job := reviewJob(run.Name)
	log.Printf("%s: stopped: %s", job, why)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msg := reposMessage(run, outcomes) + fmt.Sprintf("Stopped before the other repos, because %s.\n", why)
	if err := env.notify(ctx, msg); err == nil {
		_ = markReported(env, job, true)
	}
	return stopError{fmt.Errorf("%s stopped: %s", run.Name, why)}
}

// outcomeLine is the outcomes in one line, such as "acme/api: done".
func outcomeLine(outcomes []repoOutcome) string {
	if len(outcomes) == 0 {
		return "no repo ran"
	}
	var parts []string
	for _, o := range outcomes {
		parts = append(parts, o.repo+": "+o.status)
	}
	return strings.Join(parts, ", ")
}

// repoCommand is the command for one repo: the program, its arguments and
// the environment the command sets.
func repoCommand(env Env, cli, repo string) (cmdEnv []string, prog string, args []string, err error) {
	var p SearchPR
	p.Repository.NameWithOwner = repo
	return reviewerCommand(env.cfg.Reviewer.Command, cli, env.paths.ClaudeWrap, p, "")
}

// runInRepo runs the command for repo in its checkout dir. The output goes to
// the repo's log as it comes, and only its last line stays in memory.
func runInRepo(ctx context.Context, env Env, cli, repo, dir string) repoOutcome {
	o := repoOutcome{repo: repo, status: repoFailed}
	cmdEnv, prog, args, err := repoCommand(env, cli, repo)
	if err != nil {
		o.why = err.Error()
		return o
	}
	_ = os.MkdirAll(env.paths.Logs, 0o700)
	o.log = filepath.Join(env.paths.Logs, fmt.Sprintf("%srun-%s-%s.log", reviewLogPrefix, shortRepo(repo), time.Now().Format("20060102-150405")))
	logFile := io.Writer(io.Discard)
	if f, err := os.OpenFile(o.log, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
		defer func() { _ = f.Close() }()
		logFile = f
	}
	tail := &tailWriter{}
	out := io.MultiWriter(logFile, tail)

	ctx, cancel := context.WithTimeout(ctx, env.runTimeout())
	defer cancel()
	cmd := groupCommand(ctx, prog, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), cmdEnv...), "REVIEW_REPO="+repo, "RUN_KIND=repos")
	cmd.Stdout, cmd.Stderr = out, out
	err = cmd.Run()

	o.urls = tail.urls
	var exit *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		o.why = fmt.Sprintf("timed out after %s", env.runTimeout())
	case err == nil:
		o.status = repoDone
	case errors.As(err, &exit) && exit.ExitCode() == skipExit:
		o.status = repoNothing
	default:
		o.why = truncate(fmt.Sprintf("%v: %s", err, tail.lastLine()), 300)
	}
	return o
}

// reposMessage is the DM after a repos run: one line per repo, with up to
// maxURLs links from its output, and the log of a failed repo.
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
