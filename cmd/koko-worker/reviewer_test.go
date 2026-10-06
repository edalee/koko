package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func loadTestConfig(t *testing.T, body string) Config {
	t.Helper()
	path := t.TempDir() + "/worker.json"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// A worker.json from before 0.5.4, in the shape of a real one.
const legacyWorkerJSON = `{
  "calendarId": "primary", "enabled": true, "timeZone": "Europe/Stockholm",
  "focus": {"minMinutes": 30, "windowEnd": "17:00", "windowStart": "09:00"},
  "jobs": {
    "focus": {"enabled": true, "times": ["09:15"]},
    "standup": {"enabled": true, "times": ["07:00"]},
    "tono": {"enabled": false, "times": ["10:00", "15:00"]}
  },
  "tonoMaxAgeDays": 1, "tonoMine": true, "tonoPath": "", "tonoRepos": ["epidemicsound/kalimba"],
  "tonoTeam": "epidemicsound/content-protection", "tonoTeamPRs": true,
  "slack": {"botToken": "x", "userId": "U1"}
}`

func TestMigrateLegacyConfig(t *testing.T) {
	cfg := loadTestConfig(t, legacyWorkerJSON)
	r := cfg.Reviewer
	// An empty tonoPath meant the default clone, so the reviewer is managed.
	if r.Source != SourceManaged || r.Repo != "epidemicsound/tonometer" || r.Branch != "main" || !r.AutoUpdate {
		t.Errorf("source: %+v", r)
	}
	// The real values carry over, not the defaults.
	if !r.Mine.Enabled || r.Mine.MaxAgeDays != 14 || !r.Team.Enabled || r.Team.MaxAgeDays != 1 ||
		r.Team.Team != "epidemicsound/content-protection" || strings.Join(r.Team.Repos, ",") != "epidemicsound/kalimba" {
		t.Errorf("scopes: %+v", r)
	}
	// jobs.tono becomes jobs.review, times and switch included.
	job, ok := cfg.Jobs[JobReview]
	if !ok || job.Enabled || strings.Join(job.Times, ",") != "10:00,15:00" {
		t.Errorf("review job: %+v (present %v)", job, ok)
	}
	if _, left := cfg.Jobs[legacyJobTono]; left {
		t.Error("jobs.tono must be gone")
	}
}

func TestMigrateLegacyLocalPathAndOwnOnly(t *testing.T) {
	cfg := loadTestConfig(t, `{"tonoPath": "/opt/tono/tono", "tonoOwnPRsOnly": true, "tonoMineMaxAgeDays": 7}`)
	r := cfg.Reviewer
	if r.Source != SourceLocal || r.Path != "/opt/tono/tono" || r.Team.Enabled || r.Mine.MaxAgeDays != 7 {
		t.Errorf("got %+v", r)
	}
}

func TestNewConfigWins(t *testing.T) {
	// With a reviewer section, the legacy keys are ignored.
	cfg := loadTestConfig(t, `{"tonoPath": "/old/tono", "tonoMaxAgeDays": 9,
	  "reviewer": {"source": "managed", "repo": "o/reviewer", "branch": "dev", "autoUpdate": false,
	    "mine": {"enabled": false, "maxAgeDays": 0}, "team": {"enabled": true, "maxAgeDays": 3, "team": "o/t", "repos": [" o/a ", ""]}},
	  "jobs": {"review": {"enabled": true, "times": ["11:00"]}, "tono": {"enabled": true, "times": ["09:30"]}}}`)
	r := cfg.Reviewer
	if r.Source != SourceManaged || r.Repo != "o/reviewer" || r.Branch != "dev" || r.AutoUpdate || r.Script != "tono" ||
		r.Mine.Enabled || r.Mine.MaxAgeDays != 14 || r.Team.MaxAgeDays != 3 || strings.Join(r.Team.Repos, ",") != "o/a" {
		t.Errorf("got %+v", r)
	}
	if got := strings.Join(cfg.Jobs[JobReview].Times, ","); got != "11:00" {
		t.Errorf("review times %q: jobs.review must win over jobs.tono", got)
	}
}

func TestReviewerValidation(t *testing.T) {
	for _, bad := range []ReviewerConfig{
		{Source: "git", Repo: "o/r", Script: "tono"},
		{Source: SourceManaged, Repo: "tonometer", Script: "tono"},
		{Source: SourceManaged, Repo: "o/r", Script: "../../bin/sh"},
	} {
		cfg := defaultConfig()
		team := cfg.Reviewer.Team
		cfg.Reviewer = bad
		cfg.Reviewer.Team = team
		if cfg.Reviewer.problem() == nil || cfg.validate() != nil {
			t.Errorf("want a reviewer problem, and a valid config, for %+v", bad)
		}
	}
}

func TestReviewerCLI(t *testing.T) {
	paths := Paths{Reviewer: "/cache/reviewer"}
	cfg := defaultConfig()
	if got := cfg.reviewerCLI(paths); got != "/cache/reviewer/tono" {
		t.Errorf("managed: %q", got)
	}
	cfg.Reviewer.Source, cfg.Reviewer.Path = SourceLocal, "/opt/tono"
	if got := cfg.reviewerCLI(paths); got != "/opt/tono" {
		t.Errorf("local: %q", got)
	}
}

func TestRenameJobKeys(t *testing.T) {
	at := time.Date(2026, 10, 5, 9, 30, 0, 0, time.UTC)
	st := State{
		Runs:       map[string]RunRecord{"tono@2026-10-05 09:30": {Job: "tono", Status: StatusOK}, "standup@2026-10-05 07:00": {Job: "standup"}},
		TimesAdded: map[string]time.Time{"tono@09:30": at, "standup@07:00": at},
	}
	renameJobKeys(&st, legacyJobTono, JobReview)
	if r, ok := st.Runs["review@2026-10-05 09:30"]; !ok || r.Job != JobReview || r.Status != StatusOK {
		t.Errorf("runs: %+v", st.Runs)
	}
	if _, ok := st.Runs["tono@2026-10-05 09:30"]; ok {
		t.Error("old run key left")
	}
	if !st.TimesAdded["review@09:30"].Equal(at) || len(st.TimesAdded) != 2 {
		t.Errorf("times: %v", st.TimesAdded)
	}
}
