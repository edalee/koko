package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Job names, used as keys in the config, the run record and the CLI.
const (
	JobStandup = "standup"
	JobFocus   = "focus"
	JobReview  = "review"
	// legacyJobTono is the review job's name before 0.5.4. Config, state and
	// the CLI still accept it.
	legacyJobTono = "tono"
)

var allJobs = []string{JobStandup, JobFocus, JobReview}

var markerPrefixRe = regexp.MustCompile(`^[a-z0-9-]+$`)

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
	// Command runs the reviewer, for example "{reviewer} {pr} --all" (plan
	// 031). See reviewerCommand for how it is split and filled in.
	Command string `json:"command"`
	// Logs is the folder the reviewer writes its logs to. Empty means off: the
	// reviewer writes its result to $REVIEW_RESULT. Set, the worker reads the
	// review from the logs, in the log format of plan 031.
	Logs string `json:"logs"`
	// MarkerPrefix starts the reviewer's comment markers, "<!-- acme:" for
	// "acme". A PR with a comment behind the marker counts as reviewed.
	MarkerPrefix string `json:"markerPrefix"`
	// Mine is your own open PRs, whoever the team is.
	Mine ScopeConfig `json:"mine"`
	// Team is PRs opened by Team's members: the ones that ask for your review,
	// and every open one in Repos.
	Team TeamScopeConfig `json:"team"`
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

// JobConfig is one job's switch and its run times ("HH:MM", Monday to Friday).
type JobConfig struct {
	Enabled bool     `json:"enabled"`
	Times   []string `json:"times"`
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

// Config is worker.json. Koko's UI writes it, the worker only reads it.
// It is a separate file because Koko's SaveConfig rewrites config.json from
// its own struct and would drop any key it does not know.
type Config struct {
	Enabled    bool   `json:"enabled"`
	TimeZone   string `json:"timeZone"`
	CalendarID string `json:"calendarId"`
	// JiraSite is the Atlassian site the stand-up reads tickets from.
	JiraSite string               `json:"jiraSite"`
	Reviewer ReviewerConfig       `json:"reviewer"`
	Slack    SlackConfig          `json:"slack"`
	Focus    FocusConfig          `json:"focus"`
	Jobs     map[string]JobConfig `json:"jobs"`
}

func defaultConfig() Config {
	return Config{
		Enabled:    false,
		TimeZone:   "Europe/Stockholm",
		CalendarID: "primary",
		JiraSite:   "epidemicsound.atlassian.net",
		// No reviewer is set up, so the review job starts off.
		Reviewer: ReviewerConfig{
			Source: SourceManaged, Branch: "main", AutoUpdate: true,
			Mine: ScopeConfig{Enabled: true, MaxAgeDays: 14},
			Team: TeamScopeConfig{MaxAgeDays: 4},
		},
		Focus: FocusConfig{WindowStart: "09:00", WindowEnd: "17:00", MinMinutes: 30},
		Jobs: map[string]JobConfig{
			JobStandup: {Enabled: true, Times: []string{"07:00"}},
			JobFocus:   {Enabled: true, Times: []string{"09:15"}},
			JobReview:  {Enabled: false, Times: []string{"09:30", "12:00", "14:00"}},
		},
	}
}

// legacyReviewer is the flat tono settings worker.json held before 0.5.4.
// Pointers tell a missing key from a zero value.
type legacyReviewer struct {
	Path     *string  `json:"tonoPath"`
	Mine     *bool    `json:"tonoMine"`
	MineDays *int     `json:"tonoMineMaxAgeDays"`
	TeamPRs  *bool    `json:"tonoTeamPRs"`
	OwnOnly  *bool    `json:"tonoOwnPRsOnly"`
	Team     *string  `json:"tonoTeam"`
	TeamDays *int     `json:"tonoMaxAgeDays"`
	Repos    []string `json:"tonoRepos"`
}

// legacyCommand is the reviewer settings before 0.5.8, which assumed tono.
// Pointers tell a missing key from a zero value.
type legacyCommand struct {
	Command      *string  `json:"command"`
	Format       string   `json:"format"` // "tono-logs", the default, or "result-json"
	Args         []string `json:"args"`
	MarkerPrefix string   `json:"markerPrefix"`
}

// The tono settings a worker.json from before 0.5.8 meant when it left them
// out. Only migrate uses them. WorkerSettings.tsx migrates the same way.
const (
	legacyArgs      = "{pr} --all -l high -R {repo}"
	legacyLogs      = "~/.cache/tono/logs"
	legacyMarker    = "tono"
	legacyRepo      = "epidemicsound/tonometer"
	legacyScript    = "tono"
	legacyTeam      = "epidemicsound/content-protection"
	legacyClaudeEnv = "TONO_CLAUDE={claude} "
)

// migrate carries an older worker.json over. It reads the raw JSON, because
// the defaults are already filled in on cfg.
//
//   - Before 0.5.4 the reviewer settings were flat tono keys, and the review
//     job was "tono". An empty tonoPath meant the default clone, which now
//     means a managed reviewer.
//   - Before 0.5.8 there was no command. A reviewer section without the
//     command key gets tono's command and logs. An empty command stays empty.
func (c *Config) migrate(data []byte) {
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return
	}
	r := &c.Reviewer
	rawReviewer, hasReviewer := raw["reviewer"]
	var old legacyReviewer
	_ = json.Unmarshal(data, &old)
	if !hasReviewer && old.any() {
		r.Repo, r.Script = legacyRepo, legacyScript
		r.Team.Enabled, r.Team.Team = true, legacyTeam
		if old.Path != nil && strings.TrimSpace(*old.Path) != "" {
			r.Source, r.Path = SourceLocal, strings.TrimSpace(*old.Path)
		}
		if old.Mine != nil {
			r.Mine.Enabled = *old.Mine
		}
		if old.MineDays != nil {
			r.Mine.MaxAgeDays = *old.MineDays
		}
		if old.TeamPRs != nil {
			r.Team.Enabled = *old.TeamPRs
		}
		if old.OwnOnly != nil && *old.OwnOnly {
			r.Team.Enabled = false
		}
		if old.Team != nil {
			r.Team.Team = *old.Team
		}
		if old.TeamDays != nil {
			r.Team.MaxAgeDays = *old.TeamDays
		}
		if old.Repos != nil {
			r.Team.Repos = old.Repos
		}
	}
	if hasReviewer || old.any() {
		var lc legacyCommand
		_ = json.Unmarshal(rawReviewer, &lc)
		if lc.Command == nil {
			r.Command, r.Logs = migratedCommand(lc.Format, lc.Args)
			if strings.TrimSpace(lc.MarkerPrefix) == "" {
				r.MarkerPrefix = legacyMarker
			}
		}
	}
	var jobs map[string]json.RawMessage
	if json.Unmarshal(raw["jobs"], &jobs) == nil {
		_, hasOld := jobs[legacyJobTono]
		_, hasNew := jobs[JobReview]
		if hasOld && !hasNew {
			c.Jobs[JobReview] = c.Jobs[legacyJobTono]
		}
	}
	delete(c.Jobs, legacyJobTono)
}

// any is true when the JSON held any of the flat tono settings.
func (o legacyReviewer) any() bool {
	return o.Path != nil || o.Mine != nil || o.MineDays != nil || o.TeamPRs != nil ||
		o.OwnOnly != nil || o.Team != nil || o.TeamDays != nil || o.Repos != nil
}

// migratedCommand is the command and logs folder for a reviewer from before
// 0.5.8: tono, run with its Claude wrapper and the old arguments. Its logs
// were read unless the format was "result-json".
func migratedCommand(format string, args []string) (command, logs string) {
	words := legacyArgs
	var kept []string
	for _, a := range args {
		if a = strings.TrimSpace(a); a != "" {
			kept = append(kept, quoteWord(a))
		}
	}
	if len(kept) > 0 {
		words = strings.Join(kept, " ")
	}
	if format != "result-json" {
		logs = legacyLogs
	}
	return legacyClaudeEnv + "{reviewer} " + words, expandHome(logs)
}

// quoteWord keeps a word with spaces or quotes as one word for splitCommand.
func quoteWord(w string) string {
	if !strings.ContainsAny(w, " \t\"'") {
		return w
	}
	if !strings.Contains(w, "'") {
		return "'" + w + "'"
	}
	return `"` + w + `"`
}

// Paths the worker uses. All but Cache and Reviewer sit under ~/Library/Application
// Support/koko, next to Koko's own files.
type Paths struct {
	Config     string // worker.json
	Dir        string // worker/
	State      string // worker/state.json
	Logs       string // worker/logs/
	Settings   string // worker/claude-settings.json
	ClaudeWrap string // worker/reviewer-claude.sh
	Cache      string // ~/.cache/koko-worker/repos, cache clones of the PRs' repos
	Reviewer   string // ~/.cache/koko-worker/reviewer, the managed reviewer clone
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
	}
}

// reviewerCLI is the reviewer command to run: Path for a local reviewer, or
// Script inside the managed clone.
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
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("invalid %s: %w", path, err)
	}
	cfg.migrate(data)
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
	if c.JiraSite == "" {
		c.JiraSite = def.JiraSite
	}
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
	if r.Mine.MaxAgeDays <= 0 {
		r.Mine.MaxAgeDays = d.Mine.MaxAgeDays
	}
	if r.Team.MaxAgeDays <= 0 {
		r.Team.MaxAgeDays = d.Team.MaxAgeDays
	}
	if r.Team.Team == "" {
		r.Team.Team = d.Team.Team
	}
	var repos []string
	for _, repo := range r.Team.Repos {
		if repo = strings.TrimSpace(repo); repo != "" {
			repos = append(repos, repo)
		}
	}
	r.Team.Repos = repos
	if c.Focus.WindowStart == "" {
		c.Focus.WindowStart = def.Focus.WindowStart
	}
	if c.Focus.WindowEnd == "" {
		c.Focus.WindowEnd = def.Focus.WindowEnd
	}
	if c.Focus.MinMinutes <= 0 {
		c.Focus.MinMinutes = def.Focus.MinMinutes
	}
	if c.Jobs == nil {
		c.Jobs = map[string]JobConfig{}
	}
	for name, d := range def.Jobs {
		j, ok := c.Jobs[name]
		if !ok {
			c.Jobs[name] = d
			continue
		}
		var times []string
		for _, t := range j.Times {
			if t != "" {
				times = append(times, t)
			}
		}
		if len(times) == 0 {
			times = d.Times
		}
		j.Times = times
		c.Jobs[name] = j
	}
}

func (c Config) validate() error {
	if _, err := time.LoadLocation(c.TimeZone); err != nil {
		return fmt.Errorf("unknown time zone %q", c.TimeZone)
	}
	for name, job := range c.Jobs {
		for _, t := range job.Times {
			if _, _, err := parseClock(t); err != nil {
				return fmt.Errorf("job %s: %w", name, err)
			}
		}
	}
	// The reviewer settings are checked by the review job (problem), not here:
	// a typo in them must not stop the stand-up and focus time.
	for _, t := range []string{c.Focus.WindowStart, c.Focus.WindowEnd} {
		if _, _, err := parseClock(t); err != nil {
			return fmt.Errorf("focus window: %w", err)
		}
	}
	return nil
}

// problem is what is wrong with the reviewer settings, or nil. Only the
// review job and `check reviewer` fail on it.
func (r ReviewerConfig) problem() error {
	if r.Command == "" {
		return fmt.Errorf("no reviewer command is set")
	}
	if _, _, err := splitEnv(r.Command); err != nil {
		return err
	}
	if r.MarkerPrefix == "" {
		return fmt.Errorf("no marker prefix is set")
	}
	if !markerPrefixRe.MatchString(r.MarkerPrefix) {
		return fmt.Errorf("marker prefix %q, want letters, digits and dashes", r.MarkerPrefix)
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
	if org, team, ok := strings.Cut(r.Team.Team, "/"); r.Team.Enabled && (!ok || org == "" || team == "") {
		return fmt.Errorf("review team %q, want org/team-slug", r.Team.Team)
	}
	for _, repo := range r.Team.Repos {
		if !ownerRepo(repo) {
			return fmt.Errorf("review repo %q, want owner/repo", repo)
		}
	}
	return nil
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

func isWeekday(t time.Time) bool {
	return t.Weekday() != time.Saturday && t.Weekday() != time.Sunday
}
