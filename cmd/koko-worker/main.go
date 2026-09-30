// koko-worker runs Koko's scheduled work jobs: the stand-up, focus time and
// tono reviews. launchd keeps `koko-worker serve` running. The Koko app
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
	"syscall"
	"time"
)

const tick = 30 * time.Second

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
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
                                         run standup, focus or tono now. --test prints
                                         instead of sending a DM, and books nothing.
                                         --date runs as if it were 07:00 that day
  koko-worker check [name]               check connections: slack, github, claude, jira, calendar, tono, wake
  koko-worker status                     JSON status for the Koko app
  koko-worker install [--if-idle]        switch the launchd agent on. --if-idle skips it while a job runs
  koko-worker uninstall                  switch the launchd agent off
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "koko-worker:", err)
	os.Exit(1)
}

func newEnv(cfg Config, paths Paths, test bool) Env {
	st := loadState(paths.State)
	slack := newSlack(cfg.Slack)
	env := Env{
		cfg: cfg, paths: paths, state: &st, test: test, now: time.Now(),
		claude:      claudeRunner{settingsPath: paths.Settings},
		send:        slack.post,
		post:        postComment,
		findComment: findComment,
		persist: func(change func(*State)) error {
			return updateState(paths.State, change)
		},
	}
	if test {
		env.send = func(_ context.Context, msg Message) error {
			fmt.Println(printMessage(msg))
			return nil
		}
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
	case JobTono:
		return runTono(ctx, env, onlyURL)
	}
	return fmt.Errorf("unknown job %q", job)
}

// cmdRun runs one job now. It ignores the schedule and the on/off switches.
// It holds the job's lock, so a scheduled run of the same job waits for it.
// A successful real run of a whole job (no --pr) is recorded against the
// slots it covers (see manualSlots), so the scheduler does not repeat it.
func cmdRun(ctx context.Context, paths Paths, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	test := fs.Bool("test", false, "print instead of DM, book nothing")
	pr := fs.String("pr", "", "tono only: review this PR URL")
	date := fs.String("date", "", "with --test: run as if it were 07:00 on this day (YYYY-MM-DD)")
	if len(args) == 0 {
		return fmt.Errorf("run needs a job: standup, focus or tono")
	}
	job := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	cfg, err := loadConfig(paths.Config)
	if err != nil {
		return err
	}
	if *date != "" && !*test {
		return fmt.Errorf("--date only works with --test")
	}
	// The lock comes before newEnv reads the state, so a run that waited
	// for a scheduled one sees what that run reviewed and posted.
	if !*test {
		unlock, err := lockFile(jobLockPath(paths, job))
		if err != nil {
			return err
		}
		defer unlock()
	}
	env := newEnv(cfg, paths, *test)
	if *date != "" {
		day, err := time.ParseInLocation("2006-01-02", *date, cfg.location())
		if err != nil {
			return fmt.Errorf("bad --date %q, want YYYY-MM-DD", *date)
		}
		env.now = day.Add(7 * time.Hour)
	}
	runErr := runJob(ctx, env, job, *pr)
	if !*test && runErr == nil && *pr == "" {
		run := DueRun{Job: job, Slots: manualSlots(cfg, job, env.now)}
		if err := env.persist(func(st *State) { recordRun(st, run, StatusOK, "run by hand", time.Now()) }); err != nil {
			return err
		}
	}
	return runErr
}

// serve is the scheduler loop. It reloads worker.json on every tick, so
// changes in the Koko UI apply within half a minute.
func serve(ctx context.Context, paths Paths) error {
	log.Printf("koko-worker started")
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
		for _, run := range due {
			if ctx.Err() != nil {
				break
			}
			runDue(ctx, cfg, paths, run)
		}
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
}

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
	err = runJob(ctx, env, run.Job, "")
	status, msg := StatusOK, ""
	var reported reportedError
	switch {
	case err == nil:
		log.Printf("%s: done", run.Job)
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

// online is true if Slack answers. Every job ends in a Slack DM, so without
// it there is no point in starting.
func online(ctx context.Context) bool {
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
	jobs := map[string]jobStatus{}
	for _, job := range allJobs {
		jc := cfg.Jobs[job]
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
