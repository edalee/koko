// koko-worker runs Koko's scheduled work jobs: the stand-up, focus time, PR
// reviews and the team summary. launchd keeps `koko-worker serve` running. The Koko app
// switches it on and off and calls the other subcommands.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const tick = 30 * time.Second

// version is set at build time with -ldflags "-X main.version=v0.5.0". The
// Makefile and the release workflow set it from the git tag.
var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if os.Args[1] == "version" {
		fmt.Println(version)
		return
	}
	_ = os.Setenv("PATH", workerPATH(os.Getenv("PATH")))
	paths := workerPaths()
	if err := os.MkdirAll(paths.Logs, 0o700); err != nil {
		fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(ctx, paths)
	case "run":
		err = cmdRun(ctx, paths, os.Args[2:])
	case "check":
		err = cmdCheck(ctx, paths, os.Args[2:])
	case "status":
		err = cmdStatus(paths)
	case "install":
		err = install(paths, len(os.Args) > 2 && os.Args[2] == "--if-idle")
	case "uninstall":
		err = uninstall(paths)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fatal(err)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage:
  koko-worker serve                      run the scheduler (launchd starts this)
  koko-worker run <job> [--test [--date YYYY-MM-DD]] [--pr URL]
                                         run standup, focus, summary or review:<run> now.
                                         review alone is the first PRs run. --test prints
                                         instead of sending a DM, books nothing, and runs
                                         nothing in a repos run.
                                         --date runs as if it were 07:00 that day
  koko-worker check [name]               check connections: slack, github, claude, jira, calendar, reviewer, summary, wake
  koko-worker status                     JSON status for the Koko app
  koko-worker install [--if-idle]        switch the launchd agent on. --if-idle skips it while a job runs
  koko-worker uninstall                  switch the launchd agent off
  koko-worker version                    print the version this binary was built as
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "koko-worker:", err)
	os.Exit(1)
}

// dmSender gives the function that sends your Slack DMs. Tests swap it, so
// they never reach Slack.
var dmSender = func(cfg SlackConfig) func(context.Context, Message) error {
	return newSlack(cfg).post
}

func newEnv(cfg Config, paths Paths, test bool) Env {
	st := loadState(paths.State)
	env := Env{
		cfg: cfg, paths: paths, state: &st, test: test, now: time.Now(),
		claude:      claudeRunner{settingsPath: paths.Settings},
		send:        dmSender(cfg.Slack),
		post:        postComment,
		findComment: findComment,
		prHead:      prHead,
		online:      online,
		persist: func(change func(*State)) error {
			return updateState(paths.State, change)
		},
	}
	if test {
		env.send = func(_ context.Context, msg Message) error {
			fmt.Println(printMessage(msg))
			return nil
		}
		// A test run prints every comment, whatever is already on the PR.
		env.findComment = func(context.Context, string, int, string) (string, bool, error) { return "", false, nil }
		env.prHead = func(context.Context, string) (string, string, error) { return "OPEN", "", nil }
		env.post = func(_ context.Context, prURL, body string) (string, error) {
			fmt.Printf("── Comment for %s (not posted) ──\n%s\n\n", prURL, body)
			return "", nil
		}
		env.persist = func(func(*State)) error { return nil }
	}
	return env
}

func runJob(ctx context.Context, env Env, job, onlyURL string) error {
	switch job {
	case JobStandup:
		return runStandup(ctx, env)
	case JobFocus:
		return runFocus(ctx, env)
	case JobSummary:
		return runSummary(ctx, env)
	}
	if run, ok := env.cfg.reviewRun(job); ok {
		if err := env.cfg.runProblem(run); err != nil {
			return fmt.Errorf("%s: %w. Fix it in Settings > Worker > Review worker", job, err)
		}
		env.cfg.Reviewer = env.cfg.Reviewer.forRun(run)
		env.timeout = run.timeout()
		if run.Target == TargetRepos {
			if onlyURL != "" {
				return fmt.Errorf("%s works on repos, so --pr does not apply", job)
			}
			return runRepos(ctx, env, run)
		}
		return runReview(ctx, env, onlyURL)
	}
	return fmt.Errorf("unknown job %q", job)
}

// cmdRun runs one job now. It ignores the schedule and the on/off switches.
// It holds the job's lock, so a scheduled run of the same job waits for it.
// If another run holds a lock it needs, it fails at once with errBusy. A
// successful real run of a whole job (no --pr) is recorded against the slots
// it covers (see manualSlots), so the scheduler does not repeat it.
//
// --scheduled is how serveTick starts a repos run in its own process: it runs
// the job's due slots as the scheduler would (see runScheduled).
func cmdRun(ctx context.Context, paths Paths, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	test := fs.Bool("test", false, "print instead of DM, book nothing")
	pr := fs.String("pr", "", "review only: review this PR URL")
	date := fs.String("date", "", "with --test: run as if it were 07:00 on this day (YYYY-MM-DD)")
	scheduled := fs.Bool("scheduled", false, "run the job's due slots, as the scheduler does")
	if len(args) == 0 {
		return fmt.Errorf("run needs a job: standup, focus, summary, review or review:<run>")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	cfg, err := loadConfig(paths.Config)
	if err != nil {
		return err
	}
	// Before the lock, so an unknown name leaves no lock file behind.
	job, err := cfg.resolveJob(args[0])
	if err != nil {
		return err
	}
	if *scheduled {
		return runScheduled(ctx, cfg, paths, job)
	}
	if *date != "" && !*test {
		return fmt.Errorf("--date only works with --test")
	}
	// The lock comes before newEnv reads the state, so the run sees what the
	// last run of the job reviewed and posted.
	if !*test {
		unlock, err := tryLockFile(jobLockPath(paths, job))
		if err != nil {
			return err
		}
		defer unlock()
	}
	env := newEnv(cfg, paths, *test)
	env.noWait = true
	if *date != "" {
		day, err := time.ParseInLocation("2006-01-02", *date, cfg.location())
		if err != nil {
			return fmt.Errorf("bad --date %q, want YYYY-MM-DD", *date)
		}
		env.now = day.Add(7 * time.Hour)
	}
	slots := manualSlots(cfg, job, env.now)
	runErr := runJob(ctx, env, job, *pr)
	if !*test && runErr == nil && *pr == "" {
		run := DueRun{Job: job, Slots: slots}
		if err := env.persist(func(st *State) { recordRun(st, run, StatusOK, "run by hand", time.Now()) }); err != nil {
			return err
		}
	}
	return runErr
}

// serve is the scheduler loop. It reloads worker.json on every tick, so
// changes in the Koko UI apply within half a minute.
func serve(ctx context.Context, paths Paths) error {
	if l, err := openRotatingLog(agentLogPath(paths), logCap); err == nil {
		log.SetOutput(l)
		defer func() { _ = l.Close() }()
	} else {
		log.Printf("worker.log: %v", err) // stays on stderr
	}
	log.Printf("koko-worker %s started", version)
	var lastConfigErr string
	for {
		cfg, err := loadConfig(paths.Config)
		if err != nil {
			if err.Error() != lastConfigErr {
				log.Printf("config: %v", err)
				lastConfigErr = err.Error()
			}
		} else {
			lastConfigErr = ""
			serveTick(ctx, cfg, paths)
		}
		select {
		case <-ctx.Done():
			log.Printf("koko-worker stopped")
			return nil
		case <-time.After(tick):
		}
	}
}

func serveTick(ctx context.Context, cfg Config, paths Paths) {
	if err := updateState(paths.State, func(st *State) { syncTimes(cfg, st, time.Now()) }); err != nil {
		log.Printf("state: %v", err)
	}
	due := dueRuns(cfg, loadState(paths.State), time.Now())
	// Without internet no job runs, and nothing is recorded, so the next tick
	// tries again. The wake below is still booked.
	if len(due) > 0 && online(ctx) {
		stopAwake := keepAwake()
		runDueJobs(ctx, cfg, paths, due, time.Now())
		stopAwake()
	}
	if ctx.Err() != nil {
		// Stopping: a pmset call would fail on the cancelled context, and the
		// failed try would stop the restarted worker from booking this wake.
		return
	}
	if err := updateState(paths.State, func(st *State) {
		now := time.Now()
		pruneState(st, now)
		bookWake(ctx, cfg, st, now)
	}); err != nil {
		log.Printf("state: %v", err)
	}
	if time.Since(lastLogPrune) > 24*time.Hour {
		lastLogPrune = time.Now()
		if n := pruneReviewLogs(paths.Logs, lastLogPrune); n > 0 {
			log.Printf("logs: removed %d review logs older than 60 days", n)
		}
	}
}

// runDueJobs runs the due jobs one after another. A repos run can take hours,
// so it starts in its own process (startDetached), and the loop goes on. A
// PRs run waits for a later tick while a run holds the shared reviewer lock,
// so it never stalls the loop either.
//
// The repos runs start after the other jobs, so a PRs run due in the same
// tick never meets a lock a repos run has just taken.
func runDueJobs(ctx context.Context, cfg Config, paths Paths, due []DueRun, now time.Time) {
	var repos []string
	for _, run := range due {
		if ctx.Err() != nil {
			return
		}
		if r, ok := cfg.reviewRun(run.Job); ok {
			if r.Target == TargetRepos {
				repos = append(repos, run.Job)
				continue
			}
			if !lockFree(filepath.Join(paths.Dir, "reviewer.lock")) {
				log.Printf("%s: waiting, another run holds the reviewer", run.Job)
				continue
			}
		}
		runDue(ctx, cfg, paths, run)
	}
	for _, job := range repos {
		startDetached(paths, job, now)
	}
}

// detachedRun is a repos run serveTick started in its own process. ended is
// set before done closes.
type detachedRun struct {
	done  chan struct{}
	ended time.Time
}

// detached holds the repos runs serveTick started, by job.
var detached = map[string]*detachedRun{}

// startRun starts `koko-worker run <job> --scheduled` in its own session,
// so it outlives a restart of the agent. It returns a function that waits for
// it. Tests swap it, so they never start a real run.
var startRun = func(paths Paths, job string) (func() error, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(self, "run", job, "--scheduled")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Wait, nil
}

// startDetached starts a due repos run in its own process, unless it is still
// running, its lock is held, or it ended less than retryInterval ago. The
// child records the slot. If a child dies before it records the slot, the
// retryInterval wait stops startDetached from starting it again on every tick.
func startDetached(paths Paths, job string, now time.Time) {
	if d, ok := detached[job]; ok {
		select {
		case <-d.done:
			if now.Sub(d.ended) < retryInterval {
				return
			}
		default:
			return
		}
	}
	if !lockFree(jobLockPath(paths, job)) {
		return
	}
	wait, err := startRun(paths, job)
	if err != nil {
		log.Printf("%s: could not start: %v", job, err)
		return
	}
	log.Printf("%s: started in its own process", job)
	d := &detachedRun{done: make(chan struct{})}
	detached[job] = d
	go func() {
		if err := wait(); err != nil {
			log.Printf("%s: %v", job, err)
		}
		d.ended = time.Now()
		close(d.done)
	}()
}

// runScheduled is the process startDetached starts: it runs job's due slots
// under the job's lock, as serveTick would, and records them. It logs to
// worker.log and keeps the Mac awake. Its pid file lets uninstall stop it.
func runScheduled(ctx context.Context, cfg Config, paths Paths, job string) error {
	if l, err := openRotatingLog(agentLogPath(paths), logCap); err == nil {
		log.SetOutput(l)
		defer func() { _ = l.Close() }()
	}
	if remove, err := writePidFile(paths, job); err == nil {
		defer remove()
	} else {
		log.Printf("%s: pid file: %v", job, err)
	}
	for _, run := range dueRuns(cfg, loadState(paths.State), time.Now()) {
		if run.Job == job {
			stopAwake := keepAwake()
			defer stopAwake()
			runDue(ctx, cfg, paths, run)
		}
	}
	return nil
}

// pidPath is the file a detached run keeps next to its job lock: its pid and
// its start time, so uninstall stops it, and never a process that took the
// pid later.
func pidPath(paths Paths, job string) string { return filepath.Join(paths.Dir, job+".pid") }

// processStart is when pid started, as ps prints it, or "" if there is no
// such process.
func processStart(pid int) string {
	out, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// writePidFile writes this process's pid file for job, and gives the function
// that removes it.
func writePidFile(paths Paths, job string) (func(), error) {
	pid := os.Getpid()
	path := pidPath(paths, job)
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d %s\n", pid, processStart(pid))), 0o600); err != nil {
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}

// stopDetachedRuns stops the repos runs serveTick started in their own
// process. Each gets SIGTERM to its process group, if its pid still belongs
// to it, and then stops its tool. uninstall calls this, so switching the
// worker off stops a repos run. A restart of the agent does not.
func stopDetachedRuns(paths Paths) {
	files, _ := filepath.Glob(filepath.Join(paths.Dir, "*.pid"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		pidText, start, _ := strings.Cut(strings.TrimSpace(string(data)), " ")
		pid, err := strconv.Atoi(pidText)
		if err == nil && pid > 1 && start != "" && processStart(pid) == start {
			// Setsid made the run's process group id its pid.
			if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
				log.Printf("stop %s: %v", filepath.Base(f), err)
			}
		}
		_ = os.Remove(f)
	}
}

// lastLogPrune is when serveTick last pruned old review logs. Once a day is
// enough, and a restart prunes at once.
var lastLogPrune time.Time

// runDue runs one due job under the job's lock, which a "Run now" of the same
// job also takes. It rechecks under the lock, because a "Run now" that just
// finished may have covered the slot.
func runDue(ctx context.Context, cfg Config, paths Paths, run DueRun) {
	unlock, err := lockFile(jobLockPath(paths, run.Job))
	if err != nil {
		log.Printf("%s: %v", run.Job, err)
		return
	}
	defer unlock()
	if !stillDue(cfg, loadState(paths.State), run) {
		return
	}
	log.Printf("%s: running for %s", run.Job, run.Slots[len(run.Slots)-1].Format("15:04"))
	env := newEnv(cfg, paths, false)
	env.slot, env.scheduled = run.Slots[0], true
	err = runJob(ctx, env, run.Job, "")
	status, msg := StatusOK, ""
	var reported reportedError
	var stopped stopError
	switch {
	case err == nil:
		log.Printf("%s: done", run.Job)
	case errors.As(err, &stopped):
		// You stopped it, and the DM has said so. Checked before the network,
		// because a stop cancels the run's context, which reads as offline.
		status, msg = StatusFailed, err.Error()
		log.Printf("%s: %v", run.Job, err)
	case !online(ctx):
		// Only a real loss of internet counts as a network wait. An error that
		// merely mentions the network counts as a failure, so it cannot retry
		// all day without a warning.
		status, msg = StatusOffline, err.Error()
		log.Printf("%s: no internet, retrying in %s: %v", run.Job, retryInterval, err)
	case errors.As(err, &reported):
		// The job has already sent you the details.
		status, msg = StatusFailed, err.Error()
		log.Printf("%s: failed: %v", run.Job, err)
	case failures(*env.state, run)+1 < maxFailures:
		status, msg = StatusRetry, err.Error()
		log.Printf("%s: failed, retrying in %s: %v", run.Job, retryInterval, err)
	default:
		status, msg = StatusFailed, err.Error()
		log.Printf("%s: failed after %d tries: %v", run.Job, maxFailures, err)
		_ = env.notify(ctx, fmt.Sprintf(":warning: koko-worker %s failed after %d tries: %s", run.Job, maxFailures, truncate(err.Error(), 500)))
	}
	if err := env.persist(func(st *State) { recordRun(st, run, status, msg, time.Now()) }); err != nil {
		log.Printf("state: %v", err)
	}
}

func jobLockPath(paths Paths, job string) string {
	return filepath.Join(paths.Dir, job+".lock")
}

// keepAwake stops the Mac from idle-sleeping while jobs run, for example after
// a scheduled wake with nobody at the keyboard. It cannot stop the sleep that
// closing the lid causes on battery. Call the returned function when done.
func keepAwake() func() {
	cmd := exec.Command("caffeinate", "-i", "-w", fmt.Sprint(os.Getpid()))
	if err := cmd.Start(); err != nil {
		log.Printf("caffeinate: %v", err)
		return func() {}
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// stillDue is true if any of the run's slots is still waiting to run.
func stillDue(cfg Config, st State, run DueRun) bool {
	for _, d := range dueRuns(cfg, st, time.Now()) {
		if d.Job == run.Job {
			return true
		}
	}
	return false
}

// online is true if Slack's API answers, as a test of the internet. A job
// that fails while offline waits for the internet and retries, instead of
// counting as failed.
//
// It is a variable so tests can say they are online without the network.
var online = func(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "HEAD", "https://slack.com/api/api.test", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// ---- status ----

type jobStatus struct {
	Enabled bool       `json:"enabled"`
	Times   []string   `json:"times"`
	LastRun *RunRecord `json:"lastRun,omitempty"`
}

func cmdStatus(paths Paths) error {
	cfg, cfgErr := loadConfig(paths.Config)
	st := loadState(paths.State)
	out := map[string]any{
		"enabled":     cfg.Enabled,
		"agentLoaded": agentLoaded(),
		"configPath":  paths.Config,
		"logPath":     agentLogPath(paths),
		"version":     version,
	}
	if cfgErr != nil {
		out["configError"] = cfgErr.Error()
	}
	if slot, ok := nextSlot(cfg, time.Now()); ok {
		out["nextRun"] = slot
	}
	if !st.WakeBooked.IsZero() {
		out["wakeBooked"] = st.WakeBooked
	}
	reviewer := map[string]any{"source": cfg.Reviewer.Source, "cli": cfg.reviewerCLI(paths), "command": cfg.Reviewer.Command}
	if !cfg.Reviewer.usesReviewer() {
		reviewer["source"], reviewer["cli"] = "command", commandProgram(cfg.Reviewer.Command)
	} else if cfg.Reviewer.Source == SourceManaged {
		reviewer["repo"], reviewer["branch"] = cfg.Reviewer.Repo, cfg.Reviewer.Branch
		if st.ReviewerCommit != "" {
			reviewer["commit"], reviewer["commitBranch"] = st.ReviewerCommit, st.ReviewerBranch
			reviewer["updated"] = st.ReviewerUpdated
		}
	}
	out["reviewer"] = reviewer
	jobs := map[string]jobStatus{}
	for _, job := range cfg.jobNames() {
		jc, _ := cfg.job(job)
		js := jobStatus{Enabled: jc.Enabled, Times: jc.Times}
		for _, r := range st.Runs {
			if r.Job == job && (js.LastRun == nil || r.At.After(js.LastRun.At)) {
				rec := r
				js.LastRun = &rec
			}
		}
		jobs[job] = js
	}
	out["jobs"] = jobs
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
