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
	Body           string `json:"body"`
	Additions      int    `json:"additions"`
	Deletions      int    `json:"deletions"`
	ChangedFiles   int    `json:"changedFiles"`
	HeadRefName    string `json:"headRefName"`
	BaseRefName    string `json:"baseRefName"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	Mergeable      string `json:"mergeable"`
	IsDraft        bool   `json:"isDraft"`
	Labels         []struct {
		Name string `json:"name"`
	} `json:"labels"`
	StatusCheckRollup []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"statusCheckRollup"`
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

// MarkAllNotificationsRead marks all notifications as read.
func (g *GitHubService) MarkAllNotificationsRead() error {
	err := exec.Command("gh", "api", "--method", "PUT", "/notifications").Run()
	if err != nil {
		return fmt.Errorf("failed to mark all notifications read: %w", err)
	}
	return nil
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

// ApprovePR approves a pull request via gh CLI.
func (g *GitHubService) ApprovePR(repo string, number int) error {
	err := exec.Command("gh", "pr", "review",
		"--repo", "epidemicsound/"+repo,
		"--approve",
		fmt.Sprintf("%d", number),
	).Run()
	if err != nil {
		return fmt.Errorf("failed to approve PR: %w", err)
	}
	return nil
}

// MergePR merges a pull request via gh CLI using squash merge.
func (g *GitHubService) MergePR(repo string, number int) error {
	err := exec.Command("gh", "pr", "merge",
		"--repo", "epidemicsound/"+repo,
		"--squash",
		"--delete-branch",
		fmt.Sprintf("%d", number),
	).Run()
	if err != nil {
		return fmt.Errorf("failed to merge PR: %w", err)
	}
	return nil
}

func (g *GitHubService) FetchPRs() ([]GitHubPR, error) {
	var allPRs []GitHubPR
	for _, repo := range trackedRepos {
		out, err := exec.Command("gh", "pr", "list",
			"--repo", "epidemicsound/"+repo,
			"--json", "number,title,author,reviewDecision,url,body,additions,deletions,changedFiles,headRefName,baseRefName,createdAt,updatedAt,mergeable,isDraft,labels,statusCheckRollup",
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
			var labels []string
			for _, l := range pr.Labels {
				labels = append(labels, l.Name)
			}
			var checks []PRCheck
			for _, c := range pr.StatusCheckRollup {
				checks = append(checks, PRCheck{
					Name:       c.Name,
					Status:     c.Status,
					Conclusion: c.Conclusion,
				})
			}
			allPRs = append(allPRs, GitHubPR{
				Repo:           repo,
				Number:         pr.Number,
				Title:          pr.Title,
				Author:         pr.Author.Login,
				ReviewDecision: pr.ReviewDecision,
				URL:            pr.URL,
				Body:           pr.Body,
				Additions:      pr.Additions,
				Deletions:      pr.Deletions,
				ChangedFiles:   pr.ChangedFiles,
				HeadRef:        pr.HeadRefName,
				BaseRef:        pr.BaseRefName,
				CreatedAt:      pr.CreatedAt,
				UpdatedAt:      pr.UpdatedAt,
				Mergeable:      pr.Mergeable,
				IsDraft:        pr.IsDraft,
				Labels:         labels,
				Checks:         checks,
			})
		}
	}
	return allPRs, nil
}
