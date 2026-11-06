package main

import (
	"testing"
	"time"
)

func TestRunNowLimit(t *testing.T) {
	cfg := `{"reviewer": {"runs": [
		{"name": "daily"},
		{"name": "weekly", "target": "repos", "repos": ["acme/api", "acme/web", "acme/app", "acme/cli", "acme/docs"]},
		{"name": "slow", "target": "repos", "repos": ["acme/api", "acme/web", "acme/app"], "timeoutMinutes": 90},
		{"name": "small", "target": "repos", "repos": ["acme/api"]}
	]}}`
	for job, want := range map[string]time.Duration{
		"review:weekly": 5*time.Hour + 10*time.Minute,
		"review:slow":   4*time.Hour + 40*time.Minute,
		"review:small":  4 * time.Hour, // never less than 4 hours
		"review:daily":  4 * time.Hour, // a PRs run
		"review":        4 * time.Hour,
		"standup":       4 * time.Hour,
		"review:gone":   4 * time.Hour,
	} {
		if got := runNowLimit(cfg, job); got != want {
			t.Errorf("%s: %s, want %s", job, got, want)
		}
	}
	if got := runNowLimit("not json", "review:weekly"); got != 4*time.Hour {
		t.Errorf("bad JSON: %s", got)
	}
}

func TestCleanPath(t *testing.T) {
	dir := t.TempDir()
	out := "Welcome!\n\n<<PATH>>" + dir + ":/usr/bin:" + dir + "::/no/such/dir:/bin<<END>>\n"
	got, err := cleanPath(out)
	if want := dir + ":/usr/bin:/bin"; err != nil || got != want {
		t.Errorf("cleanPath = %q, %v, want %q", got, err, want)
	}
	if _, err := cleanPath("no markers"); err == nil {
		t.Error("want an error without the markers")
	}
}
