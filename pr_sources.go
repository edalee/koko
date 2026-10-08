package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// The PR panel's sources (plan 032). They match the review worker's scopes:
// your PRs, and your team's, which are the PRs that ask for your review and
// every open PR in the team's repos.

// PR sections, in the order the panel shows them. A PR found by more than one
// source shows once, in the first.
const (
	SectionMine = "mine"
	SectionTeam = "team"
)

// prSearchQuery asks for every field the panel shows, for one GraphQL search.
const prSearchQuery = `query($q: String!) {
  search(query: $q, type: ISSUE, first: 100) {
    nodes {
      ... on PullRequest {
        number title url body additions deletions changedFiles
        headRefName baseRefName createdAt updatedAt
        mergeable mergeStateStatus isDraft reviewDecision
        author { __typename login }
        repository { nameWithOwner }
        labels(first: 20) { nodes { name } }
        assignees(first: 10) { nodes { login } }
        statusCheckRollup {
          contexts(first: 100) {
            nodes {
              __typename
              ... on CheckRun { name status conclusion }
              ... on StatusContext { context state }
            }
          }
        }
      }
    }
  }
}`

type searchNode struct {
	Number           int    `json:"number"`
	Title            string `json:"title"`
	URL              string `json:"url"`
	Body             string `json:"body"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	ChangedFiles     int    `json:"changedFiles"`
	HeadRefName      string `json:"headRefName"`
	BaseRefName      string `json:"baseRefName"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
	Mergeable        string `json:"mergeable"`
	MergeStateStatus string `json:"mergeStateStatus"`
	IsDraft          bool   `json:"isDraft"`
	ReviewDecision   string `json:"reviewDecision"`
	Author           struct {
		Typename string `json:"__typename"`
		Login    string `json:"login"`
	} `json:"author"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Assignees struct {
		Nodes []struct {
			Login string `json:"login"`
		} `json:"nodes"`
	} `json:"assignees"`
	StatusCheckRollup *struct {
		Contexts struct {
			Nodes []checkContext `json:"nodes"`
		} `json:"contexts"`
	} `json:"statusCheckRollup"`
}

// checkContext is a check run (name, status, conclusion) or a commit status
// (context, state).
type checkContext struct {
	Typename   string `json:"__typename"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Context    string `json:"context"`
	State      string `json:"state"`
}

// toPR turns a search result into the panel's PR. A commit status (not a
// check run) has a context and a state, so they become its name, and its
// conclusion once it is no longer pending.
func (n searchNode) toPR(section string) GitHubPR {
	pr := GitHubPR{
		Repo: n.Repository.NameWithOwner, Number: n.Number, Title: n.Title, Author: n.Author.Login,
		ReviewDecision: n.ReviewDecision, URL: n.URL, Body: n.Body,
		Additions: n.Additions, Deletions: n.Deletions, ChangedFiles: n.ChangedFiles,
		HeadRef: n.HeadRefName, BaseRef: n.BaseRefName, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
		Mergeable: n.Mergeable, MergeStateStatus: n.MergeStateStatus, IsDraft: n.IsDraft, Section: section,
	}
	for _, l := range n.Labels.Nodes {
		pr.Labels = append(pr.Labels, l.Name)
	}
	for _, a := range n.Assignees.Nodes {
		pr.Assignees = append(pr.Assignees, a.Login)
	}
	if n.StatusCheckRollup != nil {
		for _, c := range n.StatusCheckRollup.Contexts.Nodes {
			check := PRCheck{Name: c.Name, Status: c.Status, Conclusion: c.Conclusion}
			if c.Typename == "StatusContext" {
				check = PRCheck{Name: c.Context, Status: "COMPLETED", Conclusion: c.State}
				if c.State == "PENDING" || c.State == "EXPECTED" {
					check = PRCheck{Name: c.Context, Status: "IN_PROGRESS"}
				}
			}
			pr.Checks = append(pr.Checks, check)
		}
	}
	return pr
}

func (n searchNode) isBot() bool {
	return n.Author.Typename == "Bot" || strings.HasSuffix(n.Author.Login, "[bot]")
}

// searchPRs runs one GitHub search for PRs. It is a variable so tests can
// replace it.
var searchPRs = func(query string) ([]searchNode, error) {
	out, err := exec.Command("gh", "api", "graphql", "-f", "q="+query, "-f", "query="+prSearchQuery,
		"--jq", ".data.search.nodes").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	var nodes []searchNode
	if err := json.Unmarshal(out, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// teamMembersFn lists an "org/team-slug" team's logins. Tests replace it.
var teamMembersFn = func(team string) (map[string]bool, error) {
	org, slug, _ := strings.Cut(team, "/")
	out, err := exec.Command("gh", "api", "--paginate", fmt.Sprintf("orgs/%s/teams/%s/members", org, slug), "--jq", ".[].login").Output()
	if err != nil {
		return nil, fmt.Errorf("team %s: %v", team, err)
	}
	members := map[string]bool{}
	for _, login := range strings.Fields(string(out)) {
		members[strings.ToLower(login)] = true
	}
	return members, nil
}

// memberCache keeps each team's members for an hour, because the panel
// refreshes every minute.
type memberCache struct {
	mu      sync.Mutex
	team    string
	members map[string]bool
	at      time.Time
}

func (c *memberCache) get(team string, now time.Time) (map[string]bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.team == team && c.members != nil && now.Sub(c.at) < time.Hour {
		return c.members, nil
	}
	members, err := teamMembersFn(team)
	if err != nil {
		return nil, err
	}
	c.team, c.members, c.at = team, members, now
	return members, nil
}

// maxReposPerQuery keeps a repos search under GitHub's query length limit.
const maxReposPerQuery = 10

// fetchPRList collects the panel's PRs from the sources cfg turns on. A
// failed source is reported in Errors, and the others still show.
func fetchPRList(cfg PRPanelConfig, members *memberCache, now time.Time) PRList {
	list := PRList{PRs: []GitHubPR{}}
	seen := map[string]bool{}
	add := func(section string, nodes []searchNode, keep func(searchNode) bool) {
		for _, n := range nodes {
			if seen[n.URL] || (keep != nil && !keep(n)) {
				continue
			}
			seen[n.URL] = true
			list.PRs = append(list.PRs, n.toPR(section))
		}
	}
	since := func(days int) string { return now.AddDate(0, 0, -days).Format("2006-01-02") }

	if cfg.Mine.Enabled {
		nodes, err := searchPRs(fmt.Sprintf("is:pr is:open archived:false author:@me created:>=%s", since(cfg.Mine.MaxAgeDays)))
		if err != nil {
			list.Errors = append(list.Errors, "My PRs: "+err.Error())
		}
		add(SectionMine, nodes, nil)
	}
	if !cfg.Team.Enabled {
		return list
	}

	var team map[string]bool
	if cfg.Team.Team != "" {
		m, err := members.get(cfg.Team.Team, now)
		if err != nil {
			list.Errors = append(list.Errors, "Team PRs: "+err.Error())
			return list
		}
		team = m
	}
	keep := func(n searchNode) bool {
		return !n.isBot() && (team == nil || team[strings.ToLower(n.Author.Login)])
	}
	base := fmt.Sprintf("is:pr is:open archived:false draft:false created:>=%s", since(cfg.Team.MaxAgeDays))
	nodes, err := searchPRs(base + " review-requested:@me")
	if err != nil {
		list.Errors = append(list.Errors, "Review requests: "+err.Error())
	}
	add(SectionTeam, nodes, keep)
	repos, bad := teamRepos(cfg.Team)
	for _, r := range bad {
		list.Errors = append(list.Errors, fmt.Sprintf("Team PRs: repo %q, want owner/repo", r))
	}
	for i := 0; i < len(repos); i += maxReposPerQuery {
		chunk := repos[i:min(i+maxReposPerQuery, len(repos))]
		nodes, err := searchPRs(base + " repo:" + strings.Join(chunk, " repo:"))
		if err != nil {
			list.Errors = append(list.Errors, "Team repos: "+err.Error())
		}
		add(SectionTeam, nodes, keep)
	}
	return list
}

// teamRepos is the team's repos as "owner/repo". A bare name means the team's
// org. Without a team, a bare name is bad.
func teamRepos(t PRTeamScope) (repos, bad []string) {
	org, _, _ := strings.Cut(t.Team, "/")
	for _, r := range t.Repos {
		r = strings.TrimSpace(r)
		switch {
		case r == "":
		case strings.Contains(r, "/"):
			repos = append(repos, r)
		case org != "":
			repos = append(repos, org+"/"+r)
		default:
			bad = append(bad, r)
		}
	}
	return repos, bad
}

// ghRepo checks that repo is "owner/repo" before it goes to gh.
func ghRepo(repo string) (string, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("repo %q, want owner/repo", repo)
	}
	return repo, nil
}
