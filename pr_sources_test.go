package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func node(repo string, n int, author string) searchNode {
	var s searchNode
	s.Repository.NameWithOwner = repo
	s.Number = n
	s.URL = "https://github.com/" + repo + "/pull/" + string(rune('0'+n))
	s.Author.Login = author
	s.Author.Typename = "User"
	return s
}

// fakeSearch answers each query by the qualifier it holds.
func fakeSearch(t *testing.T, answers map[string][]searchNode, fail string) *[]string {
	t.Helper()
	var queries []string
	var mu sync.Mutex
	saved := searchPRs
	t.Cleanup(func() { searchPRs = saved })
	searchPRs = func(q string) ([]searchNode, error) {
		mu.Lock()
		queries = append(queries, q)
		sort.Strings(queries)
		mu.Unlock()
		if fail != "" && strings.Contains(q, fail) {
			return nil, errors.New("boom")
		}
		for key, nodes := range answers {
			if strings.Contains(q, key) {
				return nodes, nil
			}
		}
		return nil, nil
	}
	return &queries
}

func TestFetchPRList(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	bot := node("acme/api", 4, "dependabot[bot]")
	bot.Author.Typename = "Bot"
	shared := node("acme/api", 1, "me")
	queries := fakeSearch(t, map[string][]searchNode{
		"author:@me":           {shared},
		"review-requested:@me": {shared, node("acme/web", 2, "alice"), node("other/x", 3, "mallory"), bot},
		"repo:acme/api":        {node("acme/api", 5, "alice")},
	}, "")
	saved := teamMembersFn
	t.Cleanup(func() { teamMembersFn = saved })
	calls := 0
	teamMembersFn = func(string) (map[string]bool, error) {
		calls++
		return map[string]bool{"alice": true, "me": true}, nil
	}

	cfg := defaultPRPanel()
	cfg.Team.Team, cfg.Team.Repos = "acme/platform", []string{"api"}
	var cache memberCache
	list := fetchPRList(cfg, &cache, now)
	var got []string
	for _, p := range list.PRs {
		got = append(got, p.Section+":"+p.Repo)
	}
	// Mine first. The shared PR shows once. Bots and non-members are left out.
	if want := "mine:acme/api team:acme/web team:acme/api"; strings.Join(got, " ") != want || len(list.Errors) != 0 {
		t.Errorf("got %q, errors %v, want %q", got, list.Errors, want)
	}
	all := strings.Join(*queries, "\n")
	if !strings.Contains(all, "author:@me created:>=2026-09-08") || !strings.Contains(all, "created:>=2026-09-24 review-requested:@me") {
		t.Errorf("ages: %q", *queries)
	}
	fetchPRList(cfg, &cache, now.Add(time.Minute))
	if calls != 1 {
		t.Errorf("team members fetched %d times, want once an hour", calls)
	}

	// Without a team, every person's PR shows, but still no bots.
	cfg.Team.Team, cfg.Team.Repos = "", nil
	list = fetchPRList(cfg, &memberCache{}, now)
	if len(list.PRs) != 3 {
		t.Errorf("no team: %d PRs, want 3", len(list.PRs))
	}
}

func TestFetchPRListErrors(t *testing.T) {
	fakeSearch(t, map[string][]searchNode{"review-requested:@me": {node("acme/web", 2, "alice")}}, "author:@me")
	cfg := defaultPRPanel()
	cfg.Team.Repos = []string{"bare"}
	list := fetchPRList(cfg, &memberCache{}, time.Now())
	// A failed source is reported, and the others still show.
	errs := strings.Join(list.Errors, "\n")
	if len(list.PRs) != 1 || len(list.Errors) != 2 || !strings.Contains(errs, "My PRs: boom") || !strings.Contains(errs, `"bare"`) {
		t.Errorf("PRs %d, errors %q", len(list.PRs), list.Errors)
	}
}

func TestFetchPRListChunksRepos(t *testing.T) {
	queries := fakeSearch(t, nil, "")
	cfg := PRPanelConfig{Team: PRTeamScope{Enabled: true, MaxAgeDays: 7}}
	for i := 0; i < 23; i++ {
		cfg.Team.Repos = append(cfg.Team.Repos, "o/r"+string(rune('a'+i)))
	}
	fetchPRList(cfg, &memberCache{}, time.Now())
	// One review-request search, then 23 repos in queries of 10, 10 and 3.
	counts := map[int]int{}
	for _, q := range *queries {
		counts[strings.Count(q, "repo:")]++
	}
	if len(*queries) != 4 || counts[10] != 2 || counts[3] != 1 {
		t.Errorf("queries: %q", *queries)
	}
}

func TestToPRChecks(t *testing.T) {
	var n searchNode
	if err := json.Unmarshal([]byte(`{"statusCheckRollup": {"contexts": {"nodes": [
	  {"__typename": "CheckRun", "name": "go", "status": "COMPLETED", "conclusion": "SUCCESS"},
	  {"__typename": "StatusContext", "context": "ci/legacy", "state": "FAILURE"},
	  {"__typename": "StatusContext", "context": "ci/slow", "state": "PENDING"}]}}}`), &n); err != nil {
		t.Fatal(err)
	}
	want := []PRCheck{{"go", "COMPLETED", "SUCCESS"}, {"ci/legacy", "COMPLETED", "FAILURE"}, {"ci/slow", "IN_PROGRESS", ""}}
	checks := n.toPR(SectionMine).Checks
	if len(checks) != 3 || checks[0] != want[0] || checks[1] != want[1] || checks[2] != want[2] {
		t.Errorf("checks: %+v", checks)
	}
}

func TestGHRepo(t *testing.T) {
	if r, err := ghRepo("acme/api"); err != nil || r != "acme/api" {
		t.Errorf("full name: %q %v", r, err)
	}
	for _, bad := range []string{"api", "/api", "acme/", "a/b/c"} {
		if _, err := ghRepo(bad); err == nil {
			t.Errorf("want an error for %q", bad)
		}
	}
}

func TestPRPanelMigration(t *testing.T) {
	cs := &ConfigService{filePath: filepath.Join(t.TempDir(), "config.json")}
	cs.config.GitHubRepos = []string{"zufolo", "acme/api", " "}
	p := cs.GetPRPanel()
	if strings.Join(p.Team.Repos, ",") != "epidemicsound/zufolo,acme/api" || !p.Mine.Enabled || !p.Team.Enabled {
		t.Errorf("migrated: %+v", p)
	}
	// An empty list meant the old built-in repos, which are gone.
	cs.config.GitHubRepos = nil
	if p := cs.GetPRPanel(); len(p.Team.Repos) != 0 {
		t.Errorf("empty: %+v", p)
	}
	p.Team.Team = "acme/platform"
	if err := cs.SetPRPanel(p); err != nil {
		t.Fatal(err)
	}
	// A config sent back without the PR panel keeps it.
	cfg := cs.GetConfig()
	cfg.PRPanel = nil
	if err := cs.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if got := cs.GetPRPanel(); got.Team.Team != "acme/platform" {
		t.Errorf("after SaveConfig: %+v", got)
	}
}
