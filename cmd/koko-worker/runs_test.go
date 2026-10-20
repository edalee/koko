package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	for _, runs := range []string{
		`[{"name": "Weekly Run"}]`,
		`[{"name": ""}]`,
		`[{"name": "` + strings.Repeat("a", 33) + `"}]`,
		`[{"name": "daily"}, {"name": "daily"}]`,
		`[{"name": "daily", "target": "branches"}]`,
	} {
		path := filepath.Join(t.TempDir(), "worker.json")
		if err := os.WriteFile(path, []byte(`{"reviewer": {"runs": `+runs+`}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadConfig(path); err == nil {
			t.Errorf("%s: want an error", runs)
		}
	}
	cfg := twoRuns()
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
	if job, _ := cfg.resolveJob(JobReview); job != reviewDaily {
		t.Errorf("review is %q, want the first PRs run", job)
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
	paths := Paths{Cache: filepath.Join(tmp, "repos")}
	clone := filepath.Join(paths.Cache, "acme__api")
	gitRun(t, tmp, "clone", "--quiet", origin, clone)
	// A review left the clone at an old commit, with a stray file.
	head := commit("two")
	if err := os.WriteFile(filepath.Join(clone, "stray.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, release, err := cleanCheckout(context.Background(), paths, "acme/api")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if got := gitRun(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD %s, want %s", got, head)
	}
	if got := gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); got != "trunk" {
		t.Errorf("branch %q, want the default branch", got)
	}
	if got := gitRun(t, dir, "status", "--porcelain"); got != "" {
		t.Errorf("not clean: %q", got)
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
