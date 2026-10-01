package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Job names, used as keys in the config, the run record and the CLI.
const (
	JobStandup = "standup"
	JobFocus   = "focus"
	JobTono    = "tono"
)

var allJobs = []string{JobStandup, JobFocus, JobTono}

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
	TonoPath   string `json:"tonoPath"`
	// TonoMine has tono review your own open PRs, whoever the team is and
	// however old the PR.
	TonoMine bool `json:"tonoMine"`
	// TonoTeamPRs has tono review PRs opened by TonoTeam's members in the last
	// TonoMaxAgeDays days: the ones that ask for your review, and every open
	// one in TonoRepos.
	TonoTeamPRs bool `json:"tonoTeamPRs"`
	// TonoTeam is "org/team-slug".
	TonoTeam       string `json:"tonoTeam"`
	TonoMaxAgeDays int    `json:"tonoMaxAgeDays"`
	// TonoRepos are "owner/repo" names.
	TonoRepos []string `json:"tonoRepos"`
	// TonoOwnPRsOnly is the old switch for TonoTeamPRs. true turns TonoTeamPRs off.
	TonoOwnPRsOnly bool                 `json:"tonoOwnPRsOnly,omitempty"`
	Slack          SlackConfig          `json:"slack"`
	Focus          FocusConfig          `json:"focus"`
	Jobs           map[string]JobConfig `json:"jobs"`
}

func defaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Enabled:        false,
		TimeZone:       "Europe/Stockholm",
		CalendarID:     "primary",
		TonoPath:       filepath.Join(home, "Projects", "es", "repos", "tonometer", "tono"),
		TonoMine:       true,
		TonoTeamPRs:    true,
		TonoTeam:       "epidemicsound/content-protection",
		TonoMaxAgeDays: 4,
		Focus:          FocusConfig{WindowStart: "09:00", WindowEnd: "17:00", MinMinutes: 30},
		Jobs: map[string]JobConfig{
			JobStandup: {Enabled: true, Times: []string{"07:00"}},
			JobFocus:   {Enabled: true, Times: []string{"09:15"}},
			JobTono:    {Enabled: true, Times: []string{"09:30", "12:00", "14:00"}},
		},
	}
}

// Paths the worker uses. All but Cache sit under ~/Library/Application
// Support/koko, next to Koko's own files.
type Paths struct {
	Config   string // worker.json
	Dir      string // worker/
	State    string // worker/state.json
	Logs     string // worker/logs/
	Settings string // worker/claude-settings.json
	TonoWrap string // worker/tono-claude.sh
	Cache    string // ~/.cache/koko-worker/repos, cache clones for tono
}

func workerPaths() Paths {
	configDir, _ := os.UserConfigDir()
	home, _ := os.UserHomeDir()
	koko := filepath.Join(configDir, "koko")
	dir := filepath.Join(koko, "worker")
	return Paths{
		Config:   filepath.Join(koko, "worker.json"),
		Dir:      dir,
		State:    filepath.Join(dir, "state.json"),
		Logs:     filepath.Join(dir, "logs"),
		Settings: filepath.Join(dir, "claude-settings.json"),
		TonoWrap: filepath.Join(dir, "tono-claude.sh"),
		Cache:    filepath.Join(home, ".cache", "koko-worker", "repos"),
	}
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
	if c.CalendarID == "" {
		c.CalendarID = def.CalendarID
	}
	if c.TonoPath == "" {
		c.TonoPath = def.TonoPath
	}
	if c.TonoTeam == "" {
		c.TonoTeam = def.TonoTeam
	}
	if c.TonoMaxAgeDays <= 0 {
		c.TonoMaxAgeDays = def.TonoMaxAgeDays
	}
	if c.TonoOwnPRsOnly {
		c.TonoTeamPRs = false
	}
	var repos []string
	for _, r := range c.TonoRepos {
		if r = strings.TrimSpace(r); r != "" {
			repos = append(repos, r)
		}
	}
	c.TonoRepos = repos
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
	if org, team, ok := strings.Cut(c.TonoTeam, "/"); !ok || org == "" || team == "" {
		return fmt.Errorf("tono team %q, want org/team-slug", c.TonoTeam)
	}
	for _, r := range c.TonoRepos {
		if owner, name, ok := strings.Cut(r, "/"); !ok || owner == "" || name == "" || strings.Contains(name, "/") {
			return fmt.Errorf("tono repo %q, want owner/repo", r)
		}
	}
	for _, t := range []string{c.Focus.WindowStart, c.Focus.WindowEnd} {
		if _, _, err := parseClock(t); err != nil {
			return fmt.Errorf("focus window: %w", err)
		}
	}
	return nil
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
