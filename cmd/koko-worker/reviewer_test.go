package main

import (
	"os"
	"strings"
	"testing"
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

func TestReviewerConfig(t *testing.T) {
	cfg := loadTestConfig(t, `{"reviewer": {"source": "managed", "repo": "o/reviewer", "branch": "dev", "autoUpdate": false,
	    "mine": {"enabled": false, "maxAgeDays": 0}, "team": {"enabled": true, "maxAgeDays": 3, "team": "o/t", "repos": [" o/a ", ""]}},
	  "jobs": {"review": {"enabled": true, "times": ["11:00"]}}}`)
	r := cfg.Reviewer
	if r.Source != SourceManaged || r.Repo != "o/reviewer" || r.Branch != "dev" || r.AutoUpdate || r.Script != "" ||
		r.Mine.Enabled || r.Mine.MaxAgeDays != 14 || r.Team.MaxAgeDays != 3 || strings.Join(r.Team.Repos, ",") != "o/a" {
		t.Errorf("got %+v", r)
	}
	if got := strings.Join(cfg.Jobs[JobReview].Times, ","); got != "11:00" {
		t.Errorf("review times %q", got)
	}
}

func TestReviewerValidation(t *testing.T) {
	for _, bad := range []ReviewerConfig{
		{Source: "git", Repo: "o/r", Script: "review-cli"},
		{Source: SourceManaged, Repo: "reviewer", Script: "review-cli"},
		{Source: SourceManaged, Repo: "o/r", Script: "../../bin/sh"},
		{Source: SourceManaged, Repo: "o/r"},
	} {
		cfg := defaultConfig()
		team := cfg.Reviewer.Team
		cfg.Reviewer = bad
		cfg.Reviewer.Team = team
		cfg.Reviewer.Command, cfg.Reviewer.MarkerPrefix = "{reviewer} {pr}", "acme"
		if cfg.Reviewer.problem() == nil || cfg.validate() != nil {
			t.Errorf("want a reviewer problem, and a valid config, for %+v", bad)
		}
	}
}

func TestReviewerCLI(t *testing.T) {
	paths := Paths{Reviewer: "/cache/reviewer"}
	cfg := defaultConfig()
	cfg.Reviewer.Script = "review-cli"
	if got := cfg.reviewerCLI(paths); got != "/cache/reviewer/review-cli" {
		t.Errorf("managed: %q", got)
	}
	cfg.Reviewer.Source, cfg.Reviewer.Path = SourceLocal, "/opt/review-cli"
	if got := cfg.reviewerCLI(paths); got != "/opt/review-cli" {
		t.Errorf("local: %q", got)
	}
}

func TestProgramProblem(t *testing.T) {
	dir := t.TempDir()
	prog := dir + "/acme"
	if err := os.WriteFile(prog, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := programProblem("A=1 " + prog + " {pr}"); err != nil {
		t.Errorf("a full path: %v", err)
	}
	if err := programProblem(dir + "/missing {pr}"); err == nil || !strings.Contains(err.Error(), "no reviewer program") {
		t.Errorf("a missing path: %v", err)
	}
	if err := programProblem("sh -c true"); err != nil {
		t.Errorf("a name on PATH: %v", err)
	}
	if err := programProblem("no-such-reviewer-xyz {pr}"); err == nil || !strings.Contains(err.Error(), "not on the worker's PATH") {
		t.Errorf("a name not on PATH: %v", err)
	}
	if err := programProblem("./bin/review {pr}"); err == nil || !strings.Contains(err.Error(), "relative path") {
		t.Errorf("a relative path: %v", err)
	}
	if err := programProblem(`{claude} -p "review {pr}"`); err != nil {
		t.Errorf("{claude} is filled in at run time: %v", err)
	}
}
