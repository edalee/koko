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
	Mergeable        string `json:"mergeable"`
	MergeStateStatus string `json:"mergeStateStatus"`
	IsDraft          bool   `json:"isDraft"`
	Labels           []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
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

// FetchPRFiles returns the list of changed files for a specific PR.
func (g *GitHubService) FetchPRFiles(repo string, number int) ([]PRFile, error) {
	out, err := exec.Command("gh", "pr", "view",
		"--repo", "epidemicsound/"+repo,
		"--json", "files",
		fmt.Sprintf("%d", number),
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}

	var result struct {
		Files []struct {
			Path      string `json:"path"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	var files []PRFile
	for _, f := range result.Files {
		files = append(files, PRFile{Path: f.Path, Additions: f.Additions, Deletions: f.Deletions})
	}
	return files, nil
}

// FetchPRFileDiff returns diff data for a single file in a PR, suitable for the code viewer.
func (g *GitHubService) FetchPRFileDiff(repo string, number int, filePath string) (FileDiffData, error) {
	fullRepo := "epidemicsound/" + repo
	prNum := fmt.Sprintf("%d", number)
	lang := inferLanguage(filePath)

	result := FileDiffData{
		OldFileName: filePath,
		NewFileName: filePath,
		Language:    lang,
	}

	// Get unified diff for the entire PR, then extract hunks for this file
	diffOut, err := exec.Command("gh", "pr", "diff", "--repo", fullRepo, prNum).Output()
	if err != nil {
		return result, fmt.Errorf("gh pr diff failed: %w", err)
	}

	result.Hunks = extractFileHunks(string(diffOut), filePath)

	// Count additions/deletions from hunks
	for _, line := range strings.Split(result.Hunks, "\n") {
		if len(line) > 0 && line[0] == '+' && !strings.HasPrefix(line, "+++") {
			result.Additions++
		} else if len(line) > 0 && line[0] == '-' && !strings.HasPrefix(line, "---") {
			result.Deletions++
		}
	}

	// Get base branch for old content
	prInfo, err := exec.Command("gh", "pr", "view", "--repo", fullRepo, "--json", "baseRefName,headRefName", prNum).Output()
	if err == nil {
		var info struct {
			BaseRefName string `json:"baseRefName"`
			HeadRefName string `json:"headRefName"`
		}
		if json.Unmarshal(prInfo, &info) == nil {
			// Old content from base branch
			if old, err := exec.Command("gh", "api",
				fmt.Sprintf("repos/%s/contents/%s", fullRepo, filePath),
				"-H", "Accept: application/vnd.github.raw",
				"--jq", ".",
				"-X", "GET",
				"-f", "ref="+info.BaseRefName,
			).Output(); err == nil {
				result.OldContent = string(old)
			}

			// New content from head branch
			if newC, err := exec.Command("gh", "api",
				fmt.Sprintf("repos/%s/contents/%s", fullRepo, filePath),
				"-H", "Accept: application/vnd.github.raw",
				"--jq", ".",
				"-X", "GET",
				"-f", "ref="+info.HeadRefName,
			).Output(); err == nil {
				result.NewContent = string(newC)
			}
		}
	}

	return result, nil
}

// extractFileHunks extracts the unified diff hunks for a specific file from a full PR diff.
func extractFileHunks(fullDiff, filePath string) string {
	lines := strings.Split(fullDiff, "\n")
	var hunks []string
	inFile := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if inFile {
				break // reached next file
			}
			inFile = strings.HasSuffix(line, " b/"+filePath)
			continue
		}
		if inFile {
			hunks = append(hunks, line)
		}
	}

	return strings.Join(hunks, "\n")
}

// FetchPRReviews returns reviews and requested reviewers for a specific PR.
func (g *GitHubService) FetchPRReviews(repo string, number int) ([]PRReview, error) {
	out, err := exec.Command("gh", "pr", "view",
		"--repo", "epidemicsound/"+repo,
		"--json", "latestReviews,reviewRequests",
		fmt.Sprintf("%d", number),
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}

	var result struct {
		LatestReviews []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			State       string `json:"state"`
			SubmittedAt string `json:"submittedAt"`
			Body        string `json:"body"`
		} `json:"latestReviews"`
		ReviewRequests []struct {
			Login string `json:"login"`
			Name  string `json:"name"`
			Slug  string `json:"slug"`
		} `json:"reviewRequests"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	var reviews []PRReview
	for _, r := range result.LatestReviews {
		reviews = append(reviews, PRReview{
			Author:      r.Author.Login,
			State:       r.State,
			SubmittedAt: r.SubmittedAt,
			Body:        r.Body,
		})
	}
	// Add pending reviewers as PENDING state
	for _, rr := range result.ReviewRequests {
		name := rr.Login
		if name == "" {
			name = rr.Slug // team review request
		}
		reviews = append(reviews, PRReview{
			Author: name,
			State:  "PENDING",
		})
	}
	return reviews, nil
}

// FetchPRCommits returns the commit list for a specific PR.
func (g *GitHubService) FetchPRCommits(repo string, number int) ([]PRCommit, error) {
	out, err := exec.Command("gh", "pr", "view",
		"--repo", "epidemicsound/"+repo,
		"--json", "commits",
		fmt.Sprintf("%d", number),
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}

	var result struct {
		Commits []struct {
			OID     string `json:"oid"`
			Authors []struct {
				Login string `json:"login"`
				Name  string `json:"name"`
			} `json:"authors"`
			MessageHeadline string `json:"messageHeadline"`
			CommittedDate   string `json:"committedDate"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	var commits []PRCommit
	for _, c := range result.Commits {
		author := ""
		if len(c.Authors) > 0 {
			author = c.Authors[0].Login
			if author == "" {
				author = c.Authors[0].Name
			}
		}
		commits = append(commits, PRCommit{
			SHA:     c.OID,
			Message: c.MessageHeadline,
			Author:  author,
			Date:    c.CommittedDate,
		})
	}
	return commits, nil
}

func (g *GitHubService) FetchPRs() ([]GitHubPR, error) {
	var allPRs []GitHubPR
	for _, repo := range trackedRepos {
		out, err := exec.Command("gh", "pr", "list",
			"--repo", "epidemicsound/"+repo,
			"--json", "number,title,author,reviewDecision,url,body,additions,deletions,changedFiles,headRefName,baseRefName,createdAt,updatedAt,mergeable,mergeStateStatus,isDraft,labels,statusCheckRollup,assignees",
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
			var assignees []string
			for _, a := range pr.Assignees {
				assignees = append(assignees, a.Login)
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
				Repo:             repo,
				Number:           pr.Number,
				Title:            pr.Title,
				Author:           pr.Author.Login,
				ReviewDecision:   pr.ReviewDecision,
				URL:              pr.URL,
				Body:             pr.Body,
				Additions:        pr.Additions,
				Deletions:        pr.Deletions,
				ChangedFiles:     pr.ChangedFiles,
				HeadRef:          pr.HeadRefName,
				BaseRef:          pr.BaseRefName,
				CreatedAt:        pr.CreatedAt,
				UpdatedAt:        pr.UpdatedAt,
				Mergeable:        pr.Mergeable,
				MergeStateStatus: pr.MergeStateStatus,
				IsDraft:          pr.IsDraft,
				Labels:           labels,
				Assignees:        assignees,
				Checks:           checks,
			})
		}
	}
	return allPRs, nil
}
