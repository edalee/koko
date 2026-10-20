package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// twoRuns is testConfig with the daily PRs run and a weekly repos run on
// Monday at 08:00, with its own arguments.
func twoRuns() Config {
	cfg := testConfig()
	cfg.Reviewer.Runs = append(cfg.Reviewer.Runs, ReviewRun{
		Name: "weekly", Args: "--update-deps", Target: TargetRepos,
		JobConfig: JobConfig{Enabled: true, Times: []string{"08:00"}, Days: []string{"mon"}},
		Repos:     []string{"acme/api", "acme/web"},
	})
	return cfg
}

func TestOldConfigBecomesOneDailyRun(t *testing.T) {
	cfg := loadTestConfig(t, `{"reviewer": {"command": "{reviewer} {pr}", "markerPrefix": "acme",
	    "mine": {"enabled": true, "maxAgeDays": 7}, "team": {"enabled": true, "maxAgeDays": 2, "team": "acme/platform"}},
	  "jobs": {"review": {"enabled": true, "times": ["10:00", "15:00"], "days": ["mon", "thu"]}}}`)
	if len(cfg.Reviewer.Runs) != 1 {
		t.Fatalf("runs %+v, want one", cfg.Reviewer.Runs)
	}
	run := cfg.Reviewer.Runs[0]
	if run.Name != "daily" || run.Target != TargetPRs || !run.Enabled || strings.Join(run.Times, ",") != "10:00,15:00" || strings.Join(run.Days, ",") != "mon,thu" {
		t.Errorf("run %+v", run)
	}
	if run.Mine.MaxAgeDays != 7 || !run.Team.Enabled || run.Team.Team != "acme/platform" || run.Team.MaxAgeDays != 2 {
		t.Errorf("scopes %+v", run)
	}
	if _, ok := cfg.Jobs[JobReview]; ok {
		t.Error("the old review job is still a job")
	}
	if got := strings.Join(cfg.jobNames(), " "); got != "standup focus summary review:daily" {
		t.Errorf("jobs %q", got)
	}
	// With runs, the old job and scopes are ignored.
	cfg = loadTestConfig(t, `{"reviewer": {"mine": {"enabled": false}, "runs": [{"name": "weekly", "enabled": true, "times": ["08:00"], "mine": {"enabled": true}}]},
	  "jobs": {"review": {"enabled": true, "times": ["10:00"]}}}`)
	if got := strings.Join(cfg.jobNames(), " "); got != "standup focus summary review:weekly" {
		t.Errorf("jobs %q", got)
	}
	if run := cfg.Reviewer.Runs[0]; !run.Mine.Enabled || run.Mine.MaxAgeDays != 14 || run.Team.MaxAgeDays != 4 || run.Target != TargetPRs {
		t.Errorf("run defaults %+v", run)
	}
}

func TestBadRunNames(t *testing.T) {
	for runs, jobs := range map[string]string{
		`[{"name": "Weekly Run"}]`:                   "standup focus summary",
		`[{"name": ""}]`:                             "standup focus summary",
		`[{"name": "` + strings.Repeat("a", 33) + `"}]`: "standup focus summary",
		// The first of two runs with one name keeps its job, and both fail.
		`[{"name": "daily"}, {"name": "daily"}]`: "standup focus summary review:daily",
		// A good name keeps its job, and the run fails on its target.
		`[{"name": "daily", "target": "branches"}]`: "standup focus summary review:daily",
	} {
		// A bad run fails only that run: the config loads, the other jobs
		// keep their schedule, and runProblem says what is wrong.
		path := filepath.Join(t.TempDir(), "worker.json")
		if err := os.WriteFile(path, []byte(`{"reviewer": {"runs": `+runs+`}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("%s: %v", runs, err)
		}
		last := cfg.Reviewer.Runs[len(cfg.Reviewer.Runs)-1]
		if cfg.runProblem(last) == nil {
			t.Errorf("%s: want a run problem", runs)
		}
		if got := strings.Join(cfg.jobNames(), " "); got != jobs {
			t.Errorf("%s: jobs %q, want %q", runs, got, jobs)
		}
	}
	// A name that leaves the worker folder never becomes a job.
	cfg := twoRuns()
	cfg.Reviewer.Runs[1].Name = "../x"
	if got := strings.Join(cfg.jobNames(), " "); got != "standup focus summary review:daily" {
		t.Errorf("jobs %q", got)
	}
	cfg = twoRuns()
	cfg.Reviewer.Runs[1].Name = strings.Repeat("a", 32)
	if err := cfg.validate(); err != nil {
		t.Errorf("a 32-letter name: %v", err)
	}
	cfg.Reviewer.Runs[1].Days = []string{"funday"}
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "review:aaaa") {
		t.Errorf("bad day in a run: %v", err)
	}
}

func TestTwoRunsKeepTheirOwnSchedule(t *testing.T) {
	cfg := twoRuns()
	st := readyState(cfg)
	if got := dueString(dueRuns(cfg, st, monday(13, 0))); got != "standup[07:00] focus[09:15] review:daily[09:30,12:00] review:weekly[08:00]" {
		t.Errorf("Monday: got %q", got)
	}
	tuesday := monday(13, 0).AddDate(0, 0, 1)
	if got := dueString(dueRuns(cfg, st, tuesday)); got != "standup[07:00] focus[09:15] review:daily[09:30,12:00]" {
		t.Errorf("Tuesday: got %q", got)
	}
	// Each run has its own slots: the daily run done does not cover the weekly one.
	recordRun(&st, DueRun{Job: reviewDaily, Slots: []time.Time{monday(9, 30)}}, StatusOK, "", monday(9, 31))
	if got := dueString(dueRuns(cfg, st, monday(9, 40))); !strings.Contains(got, "review:weekly[08:00]") {
		t.Errorf("after the daily run: got %q", got)
	}
	// With only the runs on, after Friday's last daily slot the next slot is
	// the weekly run's on Monday, before the daily run's 09:30.
	runsOnly := twoRuns()
	setJob(&runsOnly, JobStandup, JobConfig{Times: []string{"07:00"}})
	setJob(&runsOnly, JobFocus, JobConfig{Times: []string{"09:15"}})
	if slot, ok := nextSlot(runsOnly, monday(15, 0).AddDate(0, 0, 4)); !ok || slot.Format("Mon 15:04") != "Mon 08:00" {
		t.Errorf("next slot %v", slot)
	}
}

func TestRunSettings(t *testing.T) {
	cfg := twoRuns()
	daily, weekly := cfg.Reviewer.Runs[0], cfg.Reviewer.Runs[1]
	if rv := cfg.Reviewer.forRun(weekly); rv.Command != "acme-review {pr} --update-deps" {
		t.Errorf("command %q", rv.Command)
	}
	if daily.timeout() != reviewTimeout || weekly.timeout() != time.Hour {
		t.Errorf("default timeouts %s, %s", daily.timeout(), weekly.timeout())
	}
	weekly.TimeoutMinutes = 90
	if weekly.timeout() != 90*time.Minute {
		t.Errorf("timeout %s", weekly.timeout())
	}
	if err := cfg.runProblem(daily); err != nil {
		t.Errorf("daily: %v", err)
	}
	// The shared command names a PR, which a repos run does not have.
	if err := cfg.runProblem(weekly); err == nil || !strings.Contains(err.Error(), "{pr}") {
		t.Errorf("repos run with {pr}: %v", err)
	}
	for _, command := range []string{"acme {url}", "acme --at {sha}"} {
		cfg.Reviewer.Command = command
		if cfg.runProblem(weekly) == nil {
			t.Errorf("%q: want an error", command)
		}
	}
	// A repos run needs no marker, but needs repos.
	cfg.Reviewer.Command, cfg.Reviewer.MarkerPrefix = "acme {repo}", ""
	if err := cfg.runProblem(weekly); err != nil {
		t.Errorf("repos run: %v", err)
	}
	if cfg.runProblem(daily) == nil {
		t.Error("a PRs run without a marker: want an error")
	}
	for _, repos := range [][]string{nil, {"api"}} {
		weekly.Repos = repos
		if cfg.runProblem(weekly) == nil {
			t.Errorf("repos %q: want an error", repos)
		}
	}
	// The stand-up uses the first PRs run that is switched on.
	cfg = twoRuns()
	cfg.Reviewer.Runs = append([]ReviewRun{cfg.Reviewer.Runs[1]}, cfg.Reviewer.Runs[0], ReviewRun{Name: "late", Target: TargetPRs, JobConfig: JobConfig{Enabled: true}})
	cfg.Reviewer.Runs[1].Enabled = false
	if got := cfg.standupRun().Name; got != "late" {
		t.Errorf("stand-up run %q", got)
	}
	// A bare review is the first PRs run that is on, and never a repos run.
	if job, _ := cfg.resolveJob(JobReview); job != reviewJob("late") {
		t.Errorf("review is %q, want the first PRs run that is on", job)
	}
	cfg.Reviewer.Runs = cfg.Reviewer.Runs[:1]
	if job, err := cfg.resolveJob(JobReview); err == nil || !strings.Contains(err.Error(), "needs a PRs run") {
		t.Errorf("only a repos run: %q, %v", job, err)
	}
	if got := cfg.standupRun(); got.Name != "" || got.Team.Enabled {
		t.Errorf("stand-up run with no PRs run: %+v", got)
	}
}

// reposEnv is an Env for a repos run of the script, with each repo checked
// out in its own folder and the DMs kept.
func reposEnv(t *testing.T, script string) (Env, *[]string, map[string]string) {
	t.Helper()
	tmp := t.TempDir()
	prog := filepath.Join(tmp, "tool")
	if err := os.WriteFile(prog, []byte("#!/bin/sh\n"+script), 0o700); err != nil {
		t.Fatal(err)
	}
	dirs := map[string]string{}
	old := repoCheckout
	t.Cleanup(func() { repoCheckout = old })
	repoCheckout = func(_ context.Context, _ Paths, repo string) (string, func(), error) {
		dir := filepath.Join(tmp, "checkouts", strings.ReplaceAll(repo, "/", "__"))
		dirs[repo] = dir
		return dir, func() {}, os.MkdirAll(dir, 0o700)
	}
	cfg := testConfig()
	cfg.Reviewer.Command = prog + " --update-deps {repo}"
	var dms []string
	env := Env{
		cfg:   cfg,
		paths: Paths{Dir: tmp, Logs: filepath.Join(tmp, "logs"), ClaudeWrap: filepath.Join(tmp, "claude.sh")},
		send: func(_ context.Context, m Message) error {
			dms = append(dms, m.Sections...)
			return nil
		},
	}
	return env, &dms, dirs
}

func TestReposRun(t *testing.T) {
	env, dms, dirs := reposEnv(t, `printf '%s|%s|%s|%s\n' "$(pwd)" "$*" "$REVIEW_REPO" "$RUN_KIND" > ran.txt
case "$2" in
acme/api) echo "Opened https://github.com/acme/api/pull/9." ;;
acme/web) exit 2 ;;
*) echo boom; exit 1 ;;
esac
`)
	run := ReviewRun{Name: "weekly", Target: TargetRepos, Repos: []string{"acme/api", "acme/web", "acme/old"}}
	err := runRepos(context.Background(), env, run)
	var reported reportedError
	if !errors.As(err, &reported) || !strings.Contains(err.Error(), "acme/old") {
		t.Errorf("err %v, want a reported failure of acme/old", err)
	}
	for _, repo := range run.Repos {
		got, _ := os.ReadFile(filepath.Join(dirs[repo], "ran.txt"))
		dir, _ := filepath.EvalSymlinks(dirs[repo])
		if want := dir + "|--update-deps " + repo + "|" + repo + "|repos\n"; string(got) != want {
			t.Errorf("%s: ran %q, want %q", repo, got, want)
		}
	}
	if len(*dms) != 1 {
		t.Fatalf("dms %q, want one", *dms)
	}
	dm := (*dms)[0]
	for _, want := range []string{
		"*The weekly run*",
		"• acme/api: done <https://github.com/acme/api/pull/9|https://github.com/acme/api/pull/9>",
		"• acme/web: nothing to do\n",
		"• acme/old: failed (exit status 1: boom)",
		"Log: " + env.paths.Logs,
	} {
		if !strings.Contains(dm, want) {
			t.Errorf("DM %q lacks %q", dm, want)
		}
	}
}

func TestReposRunTimeout(t *testing.T) {
	env, dms, _ := reposEnv(t, "sleep 5\n")
	env.timeout = 300 * time.Millisecond
	start := time.Now()
	err := runRepos(context.Background(), env, ReviewRun{Name: "weekly", Target: TargetRepos, Repos: []string{"acme/api"}})
	if err == nil || time.Since(start) > 4*time.Second {
		t.Errorf("err %v after %s", err, time.Since(start))
	}
	if len(*dms) != 1 || !strings.Contains((*dms)[0], "timed out after 300ms") {
		t.Errorf("dms %q", *dms)
	}
}

func TestReposRunTestRunsNothing(t *testing.T) {
	env, dms, dirs := reposEnv(t, "touch ran.txt\n")
	env.test = true
	run := ReviewRun{Name: "weekly", Target: TargetRepos, Repos: []string{"acme/api"}}
	if err := runRepos(context.Background(), env, run); err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 0 || len(*dms) != 1 || !strings.Contains((*dms)[0], "• acme/api: "+filepath.Join(filepath.Dir(env.paths.ClaudeWrap), "tool")+" --update-deps acme/api") {
		t.Errorf("checkouts %v, dms %q", dirs, *dms)
	}
}

func TestCleanCheckout(t *testing.T) {
	tmp := t.TempDir()
	origin := filepath.Join(tmp, "origin")
	gitRun(t, tmp, "init", "--quiet", "--initial-branch=trunk", origin)
	commit := func(msg string) string {
		gitRun(t, origin, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "--allow-empty", "-m", msg)
		return gitRun(t, origin, "rev-parse", "HEAD")
	}
	commit("one")
	paths := Paths{Cache: filepath.Join(tmp, "repos"), RepoRuns: filepath.Join(tmp, "repos-runs")}
	clone := filepath.Join(paths.RepoRuns, "acme__api")
	gitRun(t, tmp, "clone", "--quiet", origin, clone)
	// The last run left the clone at an old commit, with a stray file, a
	// hooks path and a hook that would run on the next checkout.
	head := commit("two")
	ran := filepath.Join(tmp, "hook-ran")
	hook := "#!/bin/sh\ntouch " + ran + "\n"
	for _, f := range []string{"stray.txt", ".git/hooks/post-checkout", "tool-hooks/post-checkout"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(clone, f)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(clone, f), []byte(hook), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, clone, "config", "core.hooksPath", "tool-hooks")
	dir, release, err := cleanCheckout(context.Background(), paths, "acme/api")
	if err != nil {
		t.Fatal(err)
	}
	if got := gitRun(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD %s, want %s", got, head)
	}
	if got := gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); got != "trunk" {
		t.Errorf("branch %q, want the default branch", got)
	}
	if got := gitRun(t, dir, "status", "--porcelain"); got != "" {
		t.Errorf("not clean: %q", got)
	}
	if dir != clone || strings.HasPrefix(dir, paths.Cache+string(filepath.Separator)) {
		t.Errorf("checkout in %s, want the repos runs' clone", dir)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("a hook ran during the checkout")
	}
	if _, err := os.Stat(filepath.Join(clone, ".git", "hooks", "post-checkout")); !os.IsNotExist(err) {
		t.Error("the hook is still there")
	}
	if out, _ := exec.Command("git", "-C", dir, "config", "--get", "core.hooksPath").Output(); len(out) > 0 {
		t.Errorf("core.hooksPath is still %q", out)
	}
	release()

	// A git step that fails deletes the clone, so the next run clones it fresh.
	gitRun(t, clone, "remote", "set-url", "origin", filepath.Join(tmp, "gone"))
	if _, _, err := cleanCheckout(context.Background(), paths, "acme/api"); err == nil {
		t.Fatal("want an error from a missing origin")
	}
	if _, err := os.Stat(clone); !os.IsNotExist(err) {
		t.Errorf("the broken clone is still there: %v", err)
	}
}

func TestRunCommandPicksTheRun(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{Config: filepath.Join(dir, "worker.json"), Dir: dir, State: filepath.Join(dir, "state.json"), Logs: filepath.Join(dir, "logs")}
	// No command is set, so a run fails on its settings before it reaches GitHub.
	body := `{"reviewer": {"runs": [{"name": "weekly", "target": "repos", "repos": ["acme/api"]}, {"name": "daily"}]}}`
	if err := os.WriteFile(paths.Config, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, job := range []string{"review", "review:weekly"} {
		err := cmdRun(ctx, paths, []string{job})
		if err == nil || !strings.Contains(err.Error(), "no reviewer command") {
			t.Errorf("%s: %v", job, err)
		}
	}
	// A bare review is the first PRs run.
	for _, lock := range []string{"review:daily.lock", "review:weekly.lock"} {
		if _, err := os.Stat(filepath.Join(dir, lock)); err != nil {
			t.Errorf("%s: %v", lock, err)
		}
	}
	if err := cmdRun(ctx, paths, []string{"review:monthly"}); err == nil || !strings.Contains(err.Error(), "unknown job") {
		t.Errorf("unknown run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "review:monthly.lock")); !os.IsNotExist(err) {
		t.Error("an unknown run left a lock file")
	}
}

// scheduledRepos sets up a weekly repos run over acme/api and acme/web, due a
// minute ago on every day, for runDue to run as the scheduler does. The tool
// adds each repo it runs to a count file. DMs are kept, and fail while
// failDM is set. Nothing reaches Slack or the network.
type scheduledRepos struct {
	cfg     Config
	paths   Paths
	run     DueRun
	dms     []string
	failDM  bool
	count   string
	failGet map[string]bool // repos whose checkout fails
}

func newScheduledRepos(t *testing.T) *scheduledRepos {
	t.Helper()
	now := time.Now().UTC()
	if now.Hour() == 0 && now.Minute() < 2 {
		t.Skip("too close to midnight for a slot a minute ago")
	}
	tmp := t.TempDir()
	sr := &scheduledRepos{count: filepath.Join(tmp, "count"), failGet: map[string]bool{}}
	sr.paths = Paths{
		Config: filepath.Join(tmp, "worker.json"), Dir: tmp, State: filepath.Join(tmp, "state.json"),
		Logs: filepath.Join(tmp, "logs"), Settings: filepath.Join(tmp, "claude-settings.json"),
		ClaudeWrap: filepath.Join(tmp, "claude.sh"), Cache: filepath.Join(tmp, "repos"),
		Reviewer: filepath.Join(tmp, "reviewer"), RepoRuns: filepath.Join(tmp, "repos-runs"),
	}
	prog := filepath.Join(tmp, "tool")
	if err := os.WriteFile(prog, []byte("#!/bin/sh\necho \"$1\" >> "+sr.count+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := defaultConfig()
	cfg.Enabled, cfg.TimeZone = true, "UTC"
	cfg.Reviewer.Command, cfg.Reviewer.MarkerPrefix = prog+" {repo}", "acme"
	cfg.Reviewer.Runs = []ReviewRun{{
		Name: "weekly", Target: TargetRepos, Repos: []string{"acme/api", "acme/web"},
		JobConfig: JobConfig{Enabled: true, Times: []string{now.Add(-time.Minute).Format("15:04")}, Days: slices.Clone(dayNames)},
	}}
	for _, job := range fixedJobs {
		jc := cfg.Jobs[job]
		jc.Enabled = false
		cfg.Jobs[job] = jc
	}
	sr.cfg = cfg
	sr.writeConfig(t)
	st := loadState(sr.paths.State)
	syncTimes(cfg, &st, now.Add(-48*time.Hour))
	if err := saveState(sr.paths.State, st); err != nil {
		t.Fatal(err)
	}
	for _, r := range dueRuns(cfg, st, time.Now()) {
		if r.Job == "review:weekly" {
			sr.run = r
		}
	}
	if sr.run.Job == "" {
		t.Fatal("the weekly run is not due")
	}

	oldOnline, oldSender, oldCheckout := online, dmSender, repoCheckout
	t.Cleanup(func() { online, dmSender, repoCheckout = oldOnline, oldSender, oldCheckout })
	online = func(context.Context) bool { return true }
	dmSender = func(SlackConfig) func(context.Context, Message) error {
		return func(_ context.Context, m Message) error {
			if sr.failDM {
				sr.failDM = false
				return errors.New("slack is down")
			}
			sr.dms = append(sr.dms, m.Sections...)
			return nil
		}
	}
	repoCheckout = func(_ context.Context, _ Paths, repo string) (string, func(), error) {
		if sr.failGet[repo] {
			delete(sr.failGet, repo)
			return "", nil, errors.New("fetch: could not resolve host")
		}
		dir := filepath.Join(tmp, "checkouts", strings.ReplaceAll(repo, "/", "__"))
		return dir, func() {}, os.MkdirAll(dir, 0o700)
	}
	return sr
}

func (sr *scheduledRepos) writeConfig(t *testing.T) {
	t.Helper()
	data, err := json.Marshal(sr.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sr.paths.Config, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// ran is the repos the tool ran, in order.
func (sr *scheduledRepos) ran() string {
	b, _ := os.ReadFile(sr.count)
	return strings.Join(strings.Fields(string(b)), " ")
}

// record is the slot's run record.
func (sr *scheduledRepos) record() RunRecord {
	return loadState(sr.paths.State).Runs[slotKey(sr.run.Job, sr.run.Slots[0])]
}

// retryNow makes the slot's retry due now, not in retryInterval.
func (sr *scheduledRepos) retryNow(t *testing.T) {
	t.Helper()
	if err := updateState(sr.paths.State, func(st *State) {
		rec := st.Runs[slotKey(sr.run.Job, sr.run.Slots[0])]
		rec.NextTry = time.Now().Add(-time.Second)
		st.Runs[slotKey(sr.run.Job, sr.run.Slots[0])] = rec
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAFailedDMRetriesAndRunsNoRepoTwice(t *testing.T) {
	sr := newScheduledRepos(t)
	sr.failDM = true
	runDue(context.Background(), sr.cfg, sr.paths, sr.run)
	if rec := sr.record(); rec.Status != StatusRetry || rec.Reported {
		t.Fatalf("after the failed DM: %+v, want a retry", rec)
	}
	if got := sr.ran(); got != "acme/api acme/web" {
		t.Fatalf("ran %q", got)
	}
	sr.retryNow(t)
	runDue(context.Background(), sr.cfg, sr.paths, sr.run)
	if rec := sr.record(); rec.Status != StatusOK || !rec.Reported {
		t.Errorf("after the retry: %+v, want ok", rec)
	}
	if got := sr.ran(); got != "acme/api acme/web" {
		t.Errorf("the retry ran a repo again: %q", got)
	}
	if len(sr.dms) != 1 || !strings.Contains(sr.dms[0], "acme/api: done") || !strings.Contains(sr.dms[0], "acme/web: done") {
		t.Errorf("dms %q", sr.dms)
	}
}

func TestAFailedCheckoutRetriesThatRepoOnly(t *testing.T) {
	sr := newScheduledRepos(t)
	sr.failGet["acme/web"] = true
	runDue(context.Background(), sr.cfg, sr.paths, sr.run)
	rec := sr.record()
	if rec.Status != StatusRetry || !strings.Contains(rec.Message, "could not get acme/web") {
		t.Fatalf("after the failed checkout: %+v", rec)
	}
	if _, saved := loadState(sr.paths.State).RepoRuns[repoRunKey(sr.run.Job, sr.run.Slots[0], "acme/web")]; saved {
		t.Error("the repo whose checkout failed was saved, so no retry would run it")
	}
	if got := sr.ran(); got != "acme/api" || len(sr.dms) != 0 {
		t.Fatalf("ran %q, dms %q: want acme/api only, and no DM yet", got, sr.dms)
	}
	sr.retryNow(t)
	runDue(context.Background(), sr.cfg, sr.paths, sr.run)
	if got := sr.ran(); got != "acme/api acme/web" {
		t.Errorf("ran %q, want acme/web only on the retry", got)
	}
	if len(sr.dms) != 1 || !strings.Contains(sr.dms[0], "acme/web: done") {
		t.Errorf("dms %q", sr.dms)
	}
	if rec := sr.record(); rec.Status != StatusOK {
		t.Errorf("after the retry: %+v", rec)
	}
}

func TestAStoppedRepoDoesNotRunAgain(t *testing.T) {
	sr := newScheduledRepos(t)
	if err := updateState(sr.paths.State, func(st *State) {
		st.RepoRuns[repoRunKey(sr.run.Job, sr.run.Slots[0], "acme/api")] = RepoRecord{Status: repoStarted, At: time.Now()}
	}); err != nil {
		t.Fatal(err)
	}
	runDue(context.Background(), sr.cfg, sr.paths, sr.run)
	if got := sr.ran(); got != "acme/web" {
		t.Errorf("ran %q, want only acme/web", got)
	}
	if len(sr.dms) != 1 || !strings.Contains(sr.dms[0], "acme/api: failed (the run stopped before it finished") {
		t.Errorf("dms %q", sr.dms)
	}
	if rec := sr.record(); rec.Status != StatusFailed {
		t.Errorf("record %+v, want failed and reported", rec)
	}
}

func TestSwitchingTheRunOffStopsIt(t *testing.T) {
	sr := newScheduledRepos(t)
	checkout := repoCheckout
	repoCheckout = func(ctx context.Context, p Paths, repo string) (string, func(), error) {
		// You switch the run off while the first repo runs.
		sr.cfg.Reviewer.Runs[0].Enabled = false
		sr.writeConfig(t)
		return checkout(ctx, p, repo)
	}
	runDue(context.Background(), sr.cfg, sr.paths, sr.run)
	if got := sr.ran(); got != "acme/api" {
		t.Errorf("ran %q, want acme/api only", got)
	}
	if len(sr.dms) != 1 || !strings.Contains(sr.dms[0], "Stopped before the other repos, because the run is switched off.") {
		t.Errorf("dms %q", sr.dms)
	}
	if rec := sr.record(); rec.Status != StatusFailed || !strings.Contains(rec.Message, "stopped") {
		t.Errorf("record %+v, want a final stop", rec)
	}
}

func TestRunNowKeepsNoSlotRecords(t *testing.T) {
	sr := newScheduledRepos(t)
	env := newEnv(sr.cfg, sr.paths, false)
	env.noWait = true
	run, _ := sr.cfg.reviewRun(sr.run.Job)
	env.cfg.Reviewer = env.cfg.Reviewer.forRun(run)
	if err := runRepos(context.Background(), env, run); err != nil {
		t.Fatal(err)
	}
	st := loadState(sr.paths.State)
	if len(st.RepoRuns) != 0 {
		t.Errorf("Run now saved repo records: %v", st.RepoRuns)
	}
	for k, r := range st.Runs {
		if r.Reported {
			t.Errorf("Run now marked %s reported", k)
		}
	}
	// The scheduled run of the slot then runs every repo, with one DM.
	runDue(context.Background(), sr.cfg, sr.paths, sr.run)
	if got := sr.ran(); got != "acme/api acme/web acme/api acme/web" {
		t.Errorf("ran %q", got)
	}
	if len(sr.dms) != 2 {
		t.Errorf("dms %q, want one from Run now and one from the slot", sr.dms)
	}
}

func TestTailWriterKeepsTheLastLineAndLinks(t *testing.T) {
	var tw tailWriter
	for i := range 100 {
		_, _ = fmt.Fprintf(&tw, "line %d https://example.com/%d\n", i, i%7)
	}
	// A link seen before moves to the end.
	_, _ = fmt.Fprintf(&tw, "again https://example.com/0\n\n")
	if got := tw.lastLine(); got != "again https://example.com/0" {
		t.Errorf("last line %q", got)
	}
	want := []string{"https://example.com/4", "https://example.com/5", "https://example.com/6", "https://example.com/1", "https://example.com/0"}
	if !slices.Equal(tw.urls, want) {
		t.Errorf("urls %q, want %q", tw.urls, want)
	}
	_, _ = tw.Write([]byte(strings.Repeat("x", 3*maxLineBytes)))
	if got := tw.lastLine(); len(got) != maxLineBytes {
		t.Errorf("a long last line kept %d bytes", len(got))
	}
}

func TestUninstallStopsADetachedRun(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{Dir: dir}
	start := func() *exec.Cmd {
		cmd := exec.Command("sleep", "30")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		return cmd
	}
	ours, other := start(), start()
	write := func(job string, pid int, started string) {
		if err := os.WriteFile(pidPath(paths, job), []byte(fmt.Sprintf("%d %s\n", pid, started)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("review:weekly", ours.Process.Pid, processStart(ours.Process.Pid))
	// A pid file whose start time does not match belongs to a process that
	// took the pid later, so it is left alone.
	write("review:monthly", other.Process.Pid, "Mon Jan  1 00:00:00 2001")
	stopDetachedRuns(paths)
	done := make(chan error, 1)
	go func() { done <- ours.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the detached run is still running")
	}
	if err := other.Process.Signal(syscall.Signal(0)); err != nil {
		t.Errorf("the other process was stopped: %v", err)
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "*.pid")); len(files) != 0 {
		t.Errorf("pid files left: %v", files)
	}
}

func TestRunNowFailsAtOnceOnAHeldLock(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{Config: filepath.Join(dir, "worker.json"), Dir: dir, State: filepath.Join(dir, "state.json"), Logs: filepath.Join(dir, "logs")}
	unlock, err := lockFile(jobLockPath(paths, reviewDaily))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	done := make(chan error, 1)
	go func() { done <- cmdRun(context.Background(), paths, []string{"review:daily"}) }()
	select {
	case err := <-done:
		if !errors.Is(err, errBusy) {
			t.Errorf("err %v, want errBusy", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run now waited for the lock")
	}
	// The shared reviewer lock too, for a Run now only.
	shared := filepath.Join(dir, "reviewer.lock")
	hold, err := lockFile(shared)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	if _, err := takeLock(shared, true); !errors.Is(err, errBusy) {
		t.Errorf("shared lock: %v", err)
	}
}

func TestRepoRunsStartInTheirOwnProcess(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{Dir: dir, State: filepath.Join(dir, "state.json"), Logs: filepath.Join(dir, "logs")}
	cfg := twoRuns()
	cfg.Reviewer.Command = "" // a PRs run that did run would fail, and record it
	var started []string
	release := make(chan struct{})
	oldStart, oldDetached := startRun, detached
	t.Cleanup(func() { startRun, detached = oldStart, oldDetached })
	detached = map[string]*detachedRun{}
	startRun = func(_ Paths, job string) (func() error, error) {
		started = append(started, job)
		return func() error { <-release; return nil }, nil
	}
	// A repos run holds the shared reviewer lock.
	hold, err := lockFile(filepath.Join(dir, "reviewer.lock"))
	if err != nil {
		t.Fatal(err)
	}
	now := monday(9, 40)
	due := dueRuns(cfg, readyState(cfg), now)
	var later []DueRun
	for _, r := range due {
		if strings.HasPrefix(r.Job, "review:") {
			later = append(later, r)
		}
	}
	runDueJobs(context.Background(), cfg, paths, later, now)
	if strings.Join(started, " ") != "review:weekly" {
		t.Errorf("started %q, want the repos run only", started)
	}
	if st := loadState(paths.State); len(st.Runs) != 0 {
		t.Errorf("the PRs run ran while the shared lock was held: %v", st.Runs)
	}
	// Still running: not started again.
	runDueJobs(context.Background(), cfg, paths, later, now.Add(time.Minute))
	// Ended a minute ago: not yet.
	close(release)
	<-detached["review:weekly"].done
	hold()
	runDueJobs(context.Background(), cfg, paths, later[1:], time.Now())
	if len(started) != 1 {
		t.Errorf("started again too soon: %q", started)
	}
	runDueJobs(context.Background(), cfg, paths, later[1:], time.Now().Add(retryInterval))
	if len(started) != 2 {
		t.Errorf("not started again after the wait: %q", started)
	}
}

func TestPRsRunsGoBeforeRepoRunsInATick(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{Dir: dir, State: filepath.Join(dir, "state.json"), Logs: filepath.Join(dir, "logs")}
	cfg := twoRuns()
	cfg.Reviewer.Command = ""
	// The repos run comes first in the config.
	cfg.Reviewer.Runs = []ReviewRun{cfg.Reviewer.Runs[1], cfg.Reviewer.Runs[0]}
	oldStart, oldDetached := startRun, detached
	t.Cleanup(func() { startRun, detached = oldStart, oldDetached })
	detached = map[string]*detachedRun{}
	var prsFirst bool
	startRun = func(_ Paths, job string) (func() error, error) {
		// runDue makes the PRs run's lock file when it starts.
		_, err := os.Stat(jobLockPath(paths, reviewDaily))
		prsFirst = err == nil
		unlock, err := lockFile(filepath.Join(dir, "reviewer.lock"))
		if err != nil {
			return nil, err
		}
		return func() error { unlock(); return nil }, nil
	}
	now := monday(9, 40)
	due := dueRuns(cfg, readyState(cfg), now)
	var runs []DueRun
	for _, r := range due {
		if strings.HasPrefix(r.Job, "review:") {
			runs = append(runs, r)
		}
	}
	if len(runs) != 2 || runs[0].Job != "review:weekly" {
		t.Fatalf("due %v, want the repos run first", runs)
	}
	runDueJobs(context.Background(), cfg, paths, runs, now)
	if !prsFirst {
		t.Error("the repos run started before the PRs run due in the same tick")
	}
	<-detached["review:weekly"].done
}
