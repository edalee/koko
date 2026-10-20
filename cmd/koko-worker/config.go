package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Job names, used as keys in the config, the run record and the CLI. Each
// review run is a job of its own, "review:<name>" (see reviewJob).
const (
	JobStandup = "standup"
	JobFocus   = "focus"
	JobReview  = "review"
	JobSummary = "summary"
)

// fixedJobs are the jobs every config has. jobNames adds the review runs.
var fixedJobs = []string{JobStandup, JobFocus, JobSummary}

var markerPrefixRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// runNameRe is the form of a run's name, which is part of its job name and
// its lock file's name, so it is short too.
var runNameRe = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// Run targets.
const (
	TargetPRs   = "prs"   // once per PR, as a review with comments
	TargetRepos = "repos" // once per repo, in a clean checkout of its default branch
)

// repoTimeoutMins is the default time limit for one repo of a repos run.
const repoTimeoutMins = 60

// legacyRun is the name of the run an older worker.json becomes.
const legacyRun = "daily"

// legacyReviewTimes are a run's default times, the same as the old review job's.
var legacyReviewTimes = []string{"09:30", "12:00", "14:00"}

// Reviewer sources.
const (
	SourceManaged = "managed" // the worker keeps its own clone of Repo at Branch
	SourceLocal   = "local"   // the worker runs Path as it is
)

// ReviewerConfig is the review worker: where its reviewer code comes from,
// and which PRs it reviews.
type ReviewerConfig struct {
	Source string `json:"source"` // SourceManaged or SourceLocal
	Repo   string `json:"repo"`   // managed: "owner/repo" of the reviewer
	Branch string `json:"branch"` // managed: the branch to track
	// AutoUpdate fetches Branch before each scheduled run or Run now. A failed
	// fetch runs the last good copy.
	AutoUpdate bool   `json:"autoUpdate"`
	Script     string `json:"script"` // managed: the reviewer CLI, relative to the clone
	Path       string `json:"path"`   // local: the reviewer CLI
	// Command runs the reviewer, for example "{reviewer} {pr}" (plan
	// 031). See reviewerCommand for how it is split and filled in.
	Command string `json:"command"`
	// Logs is the folder the reviewer writes its logs to. Empty means off: the
	// reviewer writes its result to $REVIEW_RESULT. Set, the worker reads the
	// review from the logs, in the log format of plan 031.
	Logs string `json:"logs"`
	// MarkerPrefix starts the reviewer's comment markers, "<!-- acme:" for
	// "acme". A PR with a comment behind the marker counts as reviewed.
	MarkerPrefix string `json:"markerPrefix"`
	// Runs are the runs of the command, each a job with its own arguments
	// and schedule. A worker.json without runs gets one, named "daily" (see
	// fillDefaults).
	Runs []ReviewRun `json:"runs"`
	// Mine and Team are the scopes of the run in progress, which forRun sets.
	// In worker.json they are the old settings. fillDefaults turns them into
	// the "daily" run if the file has no runs.
	Mine ScopeConfig     `json:"mine"`
	Team TeamScopeConfig `json:"team"`
	// Orgs and ExcludeRepos narrow Mine and Team (plan 034). Orgs is the
	// owners whose PRs are reviewed, empty for every owner. ExcludeRepos is
	// "owner/repo" names never reviewed, even in one of Orgs.
	Orgs         []string `json:"orgs"`
	ExcludeRepos []string `json:"excludeRepos"`
}

// ReviewRun is one run of the shared command, with Args added, on its own
// schedule. A PRs run reviews each PR in its scopes once. A repos run runs
// once per repo in Repos, for a tool that changes repos, such as one that
// updates dependencies.
type ReviewRun struct {
	Name string `json:"name"` // "daily" or "weekly", for example
	Args string `json:"args"` // words added after the shared command
	JobConfig
	Target string `json:"target"` // TargetPRs or TargetRepos, empty for PRs
	// Mine is your own open PRs, whoever the team is. PRs runs only.
	Mine ScopeConfig `json:"mine"`
	// Team is PRs opened by Team's members: the ones that ask for your review,
	// and every open one in Repos. PRs runs only.
	Team TeamScopeConfig `json:"team"`
	// Repos are the "owner/repo" names a repos run works on, one after another.
	Repos []string `json:"repos"`
	// TimeoutMinutes caps one PR or one repo. Zero means 45 for a PRs run and
	// 60 for a repos run.
	TimeoutMinutes int `json:"timeoutMinutes"`
}

// timeout is the time limit for one PR or one repo.
func (r ReviewRun) timeout() time.Duration {
	if r.TimeoutMinutes > 0 {
		return time.Duration(r.TimeoutMinutes) * time.Minute
	}
	if r.Target == TargetRepos {
		return repoTimeoutMins * time.Minute
	}
	return reviewTimeout
}

// reviewJob is the job name of the review run called name.
func reviewJob(name string) string { return JobReview + ":" + name }

// forRun is the reviewer settings for run: the shared ones, with the run's
// arguments and scopes.
func (r ReviewerConfig) forRun(run ReviewRun) ReviewerConfig {
	out := r
	if r.Command != "" && run.Args != "" {
		out.Command = r.Command + " " + run.Args
	}
	out.Mine, out.Team = run.Mine, run.Team
	return out
}

// ScopeConfig switches a set of PRs on, for those opened in the last MaxAgeDays.
type ScopeConfig struct {
	Enabled    bool `json:"enabled"`
	MaxAgeDays int  `json:"maxAgeDays"`
}

// TeamScopeConfig is ScopeConfig for the team's PRs.
type TeamScopeConfig struct {
	Enabled    bool     `json:"enabled"`
	MaxAgeDays int      `json:"maxAgeDays"`
	Team       string   `json:"team"`  // "org/team-slug"
	Repos      []string `json:"repos"` // "owner/repo" names
}

// dayNames are the names Days uses, indexed by time.Weekday.
var dayNames = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// weekdays are the days a job runs on if its Days is empty.
var weekdays = []string{"mon", "tue", "wed", "thu", "fri"}

// JobConfig is one job's switch, its run times ("HH:MM") and its days.
type JobConfig struct {
	Enabled bool     `json:"enabled"`
	Times   []string `json:"times"`
	// Days are the days it runs, "mon" to "sun". Empty means Monday to Friday,
	// so an older worker.json keeps its schedule.
	Days []string `json:"days"`
}

// days are the days the job runs: Days, or Monday to Friday if it has none.
func (j JobConfig) days() []string {
	if len(j.Days) == 0 {
		return weekdays
	}
	return j.Days
}

// runsOn is true if the job runs on t's day of the week.
func (j JobConfig) runsOn(t time.Time) bool {
	return slices.Contains(j.days(), dayNames[t.Weekday()])
}

// SummaryConfig is the team summary: whose work it reports, and where.
type SummaryConfig struct {
	Team  string   `json:"team"`  // "org/team-slug"
	Repos []string `json:"repos"` // "owner/repo" names. Empty means every repo in the team's org
	// Jira adds the tickets the members' PRs name, with each one's status and
	// whether it moved. It needs a Jira site.
	Jira bool `json:"jira"`
	// Prose has Claude write a paragraph per person. Off, or if Claude fails,
	// the DM lists the facts.
	Prose bool `json:"prose"`
}

// SlackConfig is the bot that sends the DMs. Koko's own Slack settings are not used.
type SlackConfig struct {
	BotToken string `json:"botToken"`
	UserID   string `json:"userId"`
}

// FocusConfig sets where focus blocks may go.
type FocusConfig struct {
	WindowStart string `json:"windowStart"` // "HH:MM"
	WindowEnd   string `json:"windowEnd"`   // "HH:MM"
	MinMinutes  int    `json:"minMinutes"`
}

// LunchConfig is a daily break that focus time books around. It is not
// written to the calendar.
type LunchConfig struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"` // "HH:MM"
	End     string `json:"end"`   // "HH:MM", after Start
}

// Config is worker.json. Koko's UI writes it, the worker only reads it.
// It is a separate file because Koko's SaveConfig rewrites config.json from
// its own struct and would drop any key it does not know.
type Config struct {
	Enabled    bool   `json:"enabled"`
	TimeZone   string `json:"timeZone"`
	CalendarID string `json:"calendarId"`
	// JiraSite is the Atlassian site the stand-up and the team summary read
	// tickets from, such as example.atlassian.net. Empty means neither reads tickets.
	JiraSite string         `json:"jiraSite"`
	Reviewer ReviewerConfig `json:"reviewer"`
	Slack    SlackConfig    `json:"slack"`
	Focus    FocusConfig    `json:"focus"`
	// Lunch decodes over its default, so a file without it keeps lunch on.
	Lunch LunchConfig `json:"lunch"`
	// Summary decodes over its default too, so Jira and Prose start on.
	Summary SummaryConfig        `json:"summary"`
	Jobs    map[string]JobConfig `json:"jobs"`
}

func defaultConfig() Config {
	return Config{
		Enabled:    false,
		TimeZone:   localZone(),
		CalendarID: "primary",
		// No reviewer is set up, so the review run starts off.
		Reviewer: ReviewerConfig{
			Source: SourceManaged, Branch: "main", AutoUpdate: true,
			Mine: ScopeConfig{Enabled: true, MaxAgeDays: 14},
			Team: TeamScopeConfig{MaxAgeDays: 4},
			Runs: []ReviewRun{{
				Name:      legacyRun,
				Target:    TargetPRs,
				JobConfig: JobConfig{Times: legacyReviewTimes},
				Mine:      ScopeConfig{Enabled: true, MaxAgeDays: 14},
				Team:      TeamScopeConfig{MaxAgeDays: 4},
			}},
		},
		Focus: FocusConfig{WindowStart: "09:00", WindowEnd: "17:00", MinMinutes: 30},
		Lunch:   LunchConfig{Enabled: true, Start: "12:00", End: "13:00"},
		Summary: SummaryConfig{Jira: true, Prose: true},
		Jobs: map[string]JobConfig{
			JobStandup: {Enabled: true, Times: []string{"07:00"}},
			JobFocus:   {Enabled: true, Times: []string{"09:15"}},
			// No team is set, so the summary starts off.
			JobSummary: {Enabled: false, Times: []string{"08:30"}},
		},
	}
}

// Paths the worker uses. All but Cache, Reviewer and RepoRuns sit under
// ~/Library/Application Support/koko, next to Koko's own files.
type Paths struct {
	Config     string // worker.json
	Dir        string // worker/
	State      string // worker/state.json
	Logs       string // worker/logs/
	Settings   string // worker/claude-settings.json
	ClaudeWrap string // worker/reviewer-claude.sh
	Cache      string // ~/.cache/koko-worker/repos, cache clones of the PRs' repos
	Reviewer   string // ~/.cache/koko-worker/reviewer, the managed reviewer clone
	RepoRuns   string // ~/.cache/koko-worker/repos-runs, clones every repos run shares, and reviews never use
}

func workerPaths() Paths {
	configDir, _ := os.UserConfigDir()
	home, _ := os.UserHomeDir()
	koko := filepath.Join(configDir, "koko")
	dir := filepath.Join(koko, "worker")
	return Paths{
		Config:     filepath.Join(koko, "worker.json"),
		Dir:        dir,
		State:      filepath.Join(dir, "state.json"),
		Logs:       filepath.Join(dir, "logs"),
		Settings:   filepath.Join(dir, "claude-settings.json"),
		ClaudeWrap: filepath.Join(dir, "reviewer-claude.sh"),
		Cache:      filepath.Join(home, ".cache", "koko-worker", "repos"),
		Reviewer:   filepath.Join(home, ".cache", "koko-worker", "reviewer"),
		RepoRuns:   filepath.Join(home, ".cache", "koko-worker", "repos-runs"),
	}
}

// reviewerCLI is the CLI that {reviewer} in the command stands for: Path for
// a local reviewer, or Script inside the managed clone.
func (c Config) reviewerCLI(paths Paths) string {
	if c.Reviewer.Source == SourceLocal {
		return c.Reviewer.Path
	}
	return filepath.Join(paths.Reviewer, c.Reviewer.Script)
}

// loadConfig reads worker.json over the defaults. A missing or empty value
// keeps its default, and so does a job entry without times.
func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	// Clear the default run, so a file without runs gets one from its old
	// review settings in fillDefaults.
	cfg.Reviewer.Runs = nil
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("invalid %s: %w", path, err)
	}
	cfg.fillDefaults()
	return cfg, cfg.validate()
}

// fillDefaults puts back defaults the JSON left empty. The Koko UI sends
// every field, so an empty text box must not wipe a default. A job entry in
// the JSON replaces its default whole, so its times are put back too.
func (c *Config) fillDefaults() {
	def := defaultConfig()
	if c.TimeZone == "" {
		c.TimeZone = def.TimeZone
	}
	c.JiraSite = jiraHost(c.JiraSite)
	if c.CalendarID == "" {
		c.CalendarID = def.CalendarID
	}
	r, d := &c.Reviewer, def.Reviewer
	r.Path = expandHome(strings.TrimSpace(r.Path))
	if r.Source == "" {
		r.Source = d.Source
	}
	if r.Repo == "" {
		r.Repo = d.Repo
	}
	if r.Branch == "" {
		r.Branch = d.Branch
	}
	r.Command = strings.TrimSpace(r.Command)
	r.Logs = expandHome(strings.TrimSpace(r.Logs))
	r.MarkerPrefix = strings.TrimSpace(r.MarkerPrefix)
	if r.Script == "" {
		r.Script = d.Script
	}
	r.Orgs = trimList(r.Orgs)
	r.ExcludeRepos = trimList(r.ExcludeRepos)
	if c.Focus.WindowStart == "" {
		c.Focus.WindowStart = def.Focus.WindowStart
	}
	if c.Focus.WindowEnd == "" {
		c.Focus.WindowEnd = def.Focus.WindowEnd
	}
	if c.Focus.MinMinutes <= 0 {
		c.Focus.MinMinutes = def.Focus.MinMinutes
	}
	if c.Lunch.Start == "" {
		c.Lunch.Start = def.Lunch.Start
	}
	if c.Lunch.End == "" {
		c.Lunch.End = def.Lunch.End
	}
	c.Summary.Team = strings.TrimSpace(c.Summary.Team)
	c.Summary.Repos = trimList(c.Summary.Repos)
	if c.Jobs == nil {
		c.Jobs = map[string]JobConfig{}
	}
	for name, d := range def.Jobs {
		j, ok := c.Jobs[name]
		if !ok {
			c.Jobs[name] = d
			continue
		}
		c.Jobs[name] = j.withDefaults(d.Times)
	}
	// An older worker.json has one review job, and its scopes on the
	// reviewer. It becomes one run, so nothing changes until you add a run.
	if len(r.Runs) == 0 {
		old, ok := c.Jobs[JobReview]
		if !ok {
			old = JobConfig{Times: legacyReviewTimes}
		}
		r.Runs = []ReviewRun{{Name: legacyRun, JobConfig: old, Mine: r.Mine, Team: r.Team}}
	}
	// Runs replace the review job. An older Koko may still write it.
	delete(c.Jobs, JobReview)
	for i := range r.Runs {
		run := &r.Runs[i]
		run.Name = strings.TrimSpace(run.Name)
		run.Args = strings.TrimSpace(run.Args)
		run.Target = strings.TrimSpace(run.Target)
		if run.Target == "" {
			run.Target = TargetPRs
		}
		run.Repos = trimList(run.Repos)
		run.JobConfig = run.withDefaults(legacyReviewTimes)
		if run.Mine.MaxAgeDays <= 0 {
			run.Mine.MaxAgeDays = d.Mine.MaxAgeDays
		}
		if run.Team.MaxAgeDays <= 0 {
			run.Team.MaxAgeDays = d.Team.MaxAgeDays
		}
		run.Team.Team = strings.TrimSpace(run.Team.Team)
		run.Team.Repos = trimList(run.Team.Repos)
	}
}

// withDefaults drops empty times, puts back times if none are left, and
// tidies the day names.
func (j JobConfig) withDefaults(times []string) JobConfig {
	var kept []string
	for _, t := range j.Times {
		if t != "" {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		kept = times
	}
	j.Times = kept
	days := make([]string, len(j.Days))
	for i, day := range j.Days {
		days[i] = strings.ToLower(strings.TrimSpace(day))
	}
	j.Days = days
	return j
}

// jobNames are the fixed jobs, then one job per run. A run with a bad name is
// left out, because the name would make a bad lock file name. So is a run
// named like an earlier run, which would share its job. runProblem and check
// reviewer report both.
func (c Config) jobNames() []string {
	names := slices.Clone(fixedJobs)
	seen := map[string]bool{}
	for _, run := range c.Reviewer.Runs {
		if runNameRe.MatchString(run.Name) && !seen[run.Name] {
			names = append(names, reviewJob(run.Name))
		}
		seen[run.Name] = true
	}
	return names
}

// job is the switch, times and days of the job called name.
func (c Config) job(name string) (JobConfig, bool) {
	if run, ok := c.reviewRun(name); ok {
		return run.JobConfig, true
	}
	j, ok := c.Jobs[name]
	return j, ok
}

// reviewRun is the run behind the job name "review:<name>".
func (c Config) reviewRun(job string) (ReviewRun, bool) {
	name, ok := strings.CutPrefix(job, JobReview+":")
	if !ok {
		return ReviewRun{}, false
	}
	for _, run := range c.Reviewer.Runs {
		if run.Name == name {
			return run, true
		}
	}
	return ReviewRun{}, false
}

// resolveJob turns a job name from the CLI into a job in the config. A bare
// "review" is the first PRs run that is on, else the first PRs run, never a
// repos run.
func (c Config) resolveJob(job string) (string, error) {
	if job == JobReview {
		run, ok := c.firstPRsRun()
		if !ok {
			return "", fmt.Errorf("review needs a PRs run, and every run works on repos. Name one with review:<run>")
		}
		job = reviewJob(run.Name)
	}
	if slices.Contains(c.jobNames(), job) {
		return job, nil
	}
	return "", fmt.Errorf("unknown job %q, want standup, focus, review or review:<run>", job)
}

// prOnlyPlaceholders name a PR, so a repos run cannot use them.
var prOnlyPlaceholders = []string{"{pr}", "{url}", "{sha}"}

// runProblem is what is wrong with run's settings, or nil. A bad name or
// target fails only this run, not every job. A repos run needs no marker or
// scopes, but needs repos and a command without a PR in it.
func (c Config) runProblem(run ReviewRun) error {
	if !runNameRe.MatchString(run.Name) {
		return fmt.Errorf("run %q, want a name of up to 32 lowercase letters, digits and dashes", run.Name)
	}
	same := 0
	for _, other := range c.Reviewer.Runs {
		if other.Name == run.Name {
			same++
		}
	}
	if same > 1 {
		return fmt.Errorf("two runs are called %q, want each name once", run.Name)
	}
	if run.Target != TargetPRs && run.Target != TargetRepos {
		return fmt.Errorf("run %s: target %q, want %s or %s", run.Name, run.Target, TargetPRs, TargetRepos)
	}
	rv := c.Reviewer.forRun(run)
	if run.Target != TargetRepos {
		return rv.problem()
	}
	if err := rv.commandProblem(); err != nil {
		return err
	}
	for _, p := range prOnlyPlaceholders {
		if strings.Contains(rv.Command, p) {
			return fmt.Errorf("the %s run works on repos, so its command cannot use %s", run.Name, p)
		}
	}
	if len(run.Repos) == 0 {
		return fmt.Errorf("the %s run works on repos, but lists none", run.Name)
	}
	for _, repo := range run.Repos {
		if !ownerRepo(repo) {
			return fmt.Errorf("the %s run's repo %q, want owner/repo", run.Name, repo)
		}
	}
	return nil
}

// firstPRsRun is the first PRs run that is on, else the first PRs run. ok is
// false if every run works on repos. It never gives a repos run.
func (c Config) firstPRsRun() (ReviewRun, bool) {
	for _, run := range c.Reviewer.Runs {
		if run.Target == TargetPRs && run.Enabled {
			return run, true
		}
	}
	for _, run := range c.Reviewer.Runs {
		if run.Target == TargetPRs {
			return run, true
		}
	}
	return ReviewRun{}, false
}

// standupRun is the run whose team, repos and age limit the stand-up uses:
// the first PRs run that is on, else the first PRs run. With no PRs run, it
// has no scopes, so the stand-up lists no team.
func (c Config) standupRun() ReviewRun {
	run, _ := c.firstPRsRun()
	return run
}

func (c Config) validate() error {
	if _, err := time.LoadLocation(c.TimeZone); err != nil {
		return fmt.Errorf("unknown time zone %q", c.TimeZone)
	}
	for _, name := range c.jobNames() {
		job, _ := c.job(name)
		for _, t := range job.Times {
			if _, _, err := parseClock(t); err != nil {
				return fmt.Errorf("job %s: %w", name, err)
			}
		}
		for _, day := range job.Days {
			if !slices.Contains(dayNames, day) {
				return fmt.Errorf("job %s: unknown day %q, want mon to sun", name, day)
			}
		}
	}
	// The reviewer settings, run names and targets are checked per run
	// (runProblem), not here, so a typo there stops only that run.
	for _, t := range []string{c.Focus.WindowStart, c.Focus.WindowEnd} {
		if _, _, err := parseClock(t); err != nil {
			return fmt.Errorf("focus window: %w", err)
		}
	}
	for _, t := range []string{c.Lunch.Start, c.Lunch.End} {
		if _, _, err := parseClock(t); err != nil {
			return fmt.Errorf("lunch: %w", err)
		}
	}
	sh, sm, _ := parseClock(c.Lunch.Start)
	eh, em, _ := parseClock(c.Lunch.End)
	if eh*60+em <= sh*60+sm {
		return fmt.Errorf("lunch ends at %s, want a time after %s", c.Lunch.End, c.Lunch.Start)
	}
	return nil
}

// problem is what is wrong with the reviewer settings of a PRs run, or nil.
// runProblem calls it, so only that run and `check reviewer` fail on it.
func (r ReviewerConfig) problem() error {
	if err := r.commandProblem(); err != nil {
		return err
	}
	if r.MarkerPrefix == "" {
		return fmt.Errorf("no marker prefix is set")
	}
	if !markerPrefixRe.MatchString(r.MarkerPrefix) {
		return fmt.Errorf("marker prefix %q, want letters, digits and dashes", r.MarkerPrefix)
	}
	if org, team, ok := strings.Cut(r.Team.Team, "/"); r.Team.Enabled && (!ok || org == "" || team == "") {
		return fmt.Errorf("review team %q, want org/team-slug", r.Team.Team)
	}
	for _, repo := range r.Team.Repos {
		if !ownerRepo(repo) {
			return fmt.Errorf("review repo %q, want owner/repo", repo)
		}
	}
	for _, org := range r.Orgs {
		if strings.Contains(org, "/") {
			return fmt.Errorf("review org %q, want an org or user name", org)
		}
	}
	for _, repo := range r.ExcludeRepos {
		if !ownerRepo(repo) {
			return fmt.Errorf("excluded repo %q, want owner/repo", repo)
		}
	}
	return nil
}

// problem is what stops the summary from running, or nil. Only the summary
// job and `check summary` fail on it.
func (s SummaryConfig) problem() error {
	if s.Team == "" {
		return fmt.Errorf("no summary team is set")
	}
	if org, team, ok := strings.Cut(s.Team, "/"); !ok || org == "" || team == "" || strings.Contains(team, "/") {
		return fmt.Errorf("summary team %q, want org/team-slug", s.Team)
	}
	for _, repo := range s.Repos {
		if !ownerRepo(repo) {
			return fmt.Errorf("summary repo %q, want owner/repo", repo)
		}
	}
	return nil
}

// commandProblem is what is wrong with the command and its source, or nil.
// Every run needs these. Only PRs runs need the rest of problem.
func (r ReviewerConfig) commandProblem() error {
	if r.Command == "" {
		return fmt.Errorf("no reviewer command is set")
	}
	if _, _, err := splitEnv(r.Command); err != nil {
		return err
	}
	switch {
	case !r.usesReviewer():
		// The command names its own program, so the source is not used.
	case r.Source == SourceManaged:
		if !ownerRepo(r.Repo) {
			return fmt.Errorf("reviewer repo %q, want owner/repo", r.Repo)
		}
		if r.Script == "" || strings.HasPrefix(r.Script, "/") || strings.Contains(r.Script, "..") {
			return fmt.Errorf("reviewer script %q, want a path inside the clone", r.Script)
		}
	case r.Source == SourceLocal:
		if r.Path == "" {
			return fmt.Errorf("the reviewer source is a local path, but no path is set")
		}
	default:
		return fmt.Errorf("reviewer source %q, want %s or %s", r.Source, SourceManaged, SourceLocal)
	}
	return nil
}

// skipRepo returns why the review worker skips PRs in repo ("owner/repo"), or "" if it reviews them. Names match in any case.
func (r ReviewerConfig) skipRepo(repo string) string {
	for _, x := range r.ExcludeRepos {
		if strings.EqualFold(x, repo) {
			return "an excluded repo"
		}
	}
	if len(r.Orgs) == 0 {
		return ""
	}
	owner, _, _ := strings.Cut(repo, "/")
	for _, org := range r.Orgs {
		if strings.EqualFold(org, owner) {
			return ""
		}
	}
	return "not in the review orgs"
}

// trimList trims each entry of list and drops the empty ones.
func trimList(list []string) []string {
	var out []string
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// usesReviewer is true when the command runs the CLI from the source setting.
func (r ReviewerConfig) usesReviewer() bool { return strings.Contains(r.Command, "{reviewer}") }

// expandHome turns a leading "~" or "~/" into your home folder. Nothing else
// expands it: the worker runs the path as it is, without a shell.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// localZone is the Mac's time zone name, such as Europe/London. Go names the
// Mac's zone "Local", but Google Calendar needs a real name, so this reads
// the /etc/localtime link.
// If the link is missing or names no known zone, it returns UTC.
func localZone() string {
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return "UTC"
	}
	_, name, ok := strings.Cut(target, "zoneinfo/")
	if _, err := time.LoadLocation(name); !ok || err != nil {
		return "UTC"
	}
	return name
}

// jiraHost keeps only the host of a Jira site: no scheme, path or slash.
func jiraHost(site string) string {
	site = strings.TrimSpace(site)
	site = strings.TrimPrefix(strings.TrimPrefix(site, "https://"), "http://")
	host, _, _ := strings.Cut(site, "/")
	return host
}

func ownerRepo(s string) bool {
	owner, name, ok := strings.Cut(s, "/")
	return ok && owner != "" && name != "" && !strings.Contains(name, "/")
}

func (c Config) location() *time.Location {
	loc, err := time.LoadLocation(c.TimeZone)
	if err != nil {
		return time.Local
	}
	return loc
}

// parseClock parses "HH:MM".
func parseClock(s string) (hour, minute int, err error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, fmt.Errorf("bad time %q, want HH:MM", s)
	}
	return t.Hour(), t.Minute(), nil
}

// atClock returns day's date at the "HH:MM" time, in day's location.
func atClock(day time.Time, clock string) time.Time {
	h, m, _ := parseClock(clock)
	return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, day.Location())
}
