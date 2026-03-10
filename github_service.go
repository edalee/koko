package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

var trackedRepos = []string{
	"drumstick2",
	"drumstick-ui",
	"trigon",
	"conductor-bot",
}

type GitHubService struct{}

func NewGitHubService() *GitHubService {
	return &GitHubService{}
}

type ghPRJSON struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	ReviewDecision string `json:"reviewDecision"`
	URL            string `json:"url"`
}

type ghNotificationJSON struct {
	ID      string `json:"id"`
	Unread  bool   `json:"unread"`
	Reason  string `json:"reason"`
	Subject struct {
		Title string `json:"title"`
		Type  string `json:"type"`
		URL   string `json:"url"` // API URL
	} `json:"subject"`
	Repository struct {
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
	UpdatedAt string `json:"updated_at"`
}

// FetchNotifications returns GitHub notifications via the gh CLI.
// filter: "participating", "all" — maps to the GitHub API participating param.
func (g *GitHubService) FetchNotifications(filter string) ([]GitHubNotification, error) {
	url := "/notifications?per_page=50"
	if filter == "participating" {
		url += "&participating=true"
	}

	out, err := exec.Command("gh", "api", url).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api failed: %w", err)
	}

	var raw []ghNotificationJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	var notifications []GitHubNotification
	for _, n := range raw {
		htmlURL := apiToHTMLURL(n.Subject.URL, n.Repository.HTMLURL)
		repo := n.Repository.FullName
		if parts := strings.SplitN(repo, "/", 2); len(parts) == 2 {
			repo = parts[1]
		}
		notifications = append(notifications, GitHubNotification{
			ID:        n.ID,
			Title:     n.Subject.Title,
			Type:      n.Subject.Type,
			Reason:    n.Reason,
			Repo:      repo,
			URL:       htmlURL,
			Unread:    n.Unread,
			UpdatedAt: n.UpdatedAt,
		})
	}

	return notifications, nil
}

// MarkNotificationRead marks a single notification thread as read.
func (g *GitHubService) MarkNotificationRead(threadID string) error {
	err := exec.Command("gh", "api", "--method", "PATCH",
		fmt.Sprintf("/notifications/threads/%s", threadID),
	).Run()
	if err != nil {
		return fmt.Errorf("failed to mark notification read: %w", err)
	}
	return nil
}

// apiToHTMLURL converts a GitHub API URL to a browser URL.
// e.g. "https://api.github.com/repos/org/repo/pulls/123" → "https://github.com/org/repo/pull/123"
func apiToHTMLURL(apiURL, repoHTMLURL string) string {
	if apiURL == "" {
		return repoHTMLURL
	}
	// Strip API prefix
	path := strings.TrimPrefix(apiURL, "https://api.github.com/repos/")
	if path == apiURL {
		return repoHTMLURL
	}
	// "org/repo/pulls/123" → "org/repo/pull/123"
	path = strings.Replace(path, "/pulls/", "/pull/", 1)
	path = strings.Replace(path, "/issues/", "/issues/", 1)
	return "https://github.com/" + path
}

func (g *GitHubService) FetchPRs() ([]GitHubPR, error) {
	var allPRs []GitHubPR
	for _, repo := range trackedRepos {
		out, err := exec.Command("gh", "pr", "list",
			"--repo", "epidemicsound/"+repo,
			"--json", "number,title,author,reviewDecision,url",
			"--limit", "10",
		).Output()
		if err != nil {
			continue
		}
		var prs []ghPRJSON
		if err := json.Unmarshal(out, &prs); err != nil {
			continue
		}
		for _, pr := range prs {
			allPRs = append(allPRs, GitHubPR{
				Repo:           repo,
				Number:         pr.Number,
				Title:          pr.Title,
				Author:         pr.Author.Login,
				ReviewDecision: pr.ReviewDecision,
				URL:            pr.URL,
			})
		}
	}
	return allPRs, nil
}
