package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// SearchPR is one row from `gh search prs`.
type SearchPR struct {
	URL        string `json:"url"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	CreatedAt  string `json:"createdAt"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Author struct {
		Login string `json:"login"`
		IsBot bool   `json:"is_bot"` // false even for Dependabot, so Type is checked too
		Type  string `json:"type"`   // "User" or "Bot"
	} `json:"author"`
}

func (p SearchPR) repo() string  { return p.Repository.NameWithOwner }
func (p SearchPR) short() string { return fmt.Sprintf("%s#%d", shortRepo(p.repo()), p.Number) }

func shortRepo(nameWithOwner string) string {
	if i := strings.LastIndexByte(nameWithOwner, '/'); i >= 0 {
		return nameWithOwner[i+1:]
	}
	return nameWithOwner
}

// PRDetail is the part of `gh pr view` that decides whether a PR is ready.
type PRDetail struct {
	Title            string  `json:"title"`
	Body             string  `json:"body"`
	HeadRefName      string  `json:"headRefName"`
	HeadRefOid       string  `json:"headRefOid"`
	IsDraft          bool    `json:"isDraft"`
	Mergeable        string  `json:"mergeable"`        // MERGEABLE, CONFLICTING, UNKNOWN
	MergeStateStatus string  `json:"mergeStateStatus"` // CLEAN, BLOCKED, BEHIND, DIRTY, UNSTABLE, ...
	ReviewDecision   string  `json:"reviewDecision"`   // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED
	Checks           []Check `json:"statusCheckRollup"`
}

// Check is a check run (Status, Conclusion) or a commit status context (State).
type Check struct {
	Name       string `json:"name"`
	Context    string `json:"context"`
	Status     string `json:"status"`     // check runs: COMPLETED, IN_PROGRESS, ...
	Conclusion string `json:"conclusion"` // check runs: SUCCESS, FAILURE, SKIPPED, NEUTRAL, ...
	State      string `json:"state"`      // status contexts: SUCCESS, FAILURE, PENDING, ERROR
}

// blockers lists why an approved PR cannot merge yet. None means ready.
func (d PRDetail) blockers() []string {
	var out []string
	if d.IsDraft {
		out = append(out, "draft")
	}
	if d.ReviewDecision != "APPROVED" {
		out = append(out, "not approved")
	}
	switch d.Mergeable {
	case "CONFLICTING":
		out = append(out, "merge conflicts")
	case "UNKNOWN", "":
		out = append(out, "GitHub has not worked out mergeability yet")
	}
	if d.MergeStateStatus == "BEHIND" {
		out = append(out, "behind the base branch")
	}
	var failing, pending []string
	for _, c := range d.Checks {
		name := c.Name
		if name == "" {
			name = c.Context
		}
		switch {
		case c.State == "FAILURE" || c.State == "ERROR",
			c.Conclusion == "FAILURE" || c.Conclusion == "TIMED_OUT" || c.Conclusion == "CANCELLED" || c.Conclusion == "ACTION_REQUIRED":
			failing = append(failing, name)
		case c.State == "PENDING" || c.State == "EXPECTED",
			c.Status != "" && c.Status != "COMPLETED":
			pending = append(pending, name)
		}
	}
	if len(failing) > 0 {
		out = append(out, "failing checks: "+strings.Join(failing, ", "))
	}
	if len(pending) > 0 {
		out = append(out, "checks still running: "+strings.Join(pending, ", "))
	}
	return out
}

// isBot is true for Dependabot, Renovate and other app accounts.
func isBot(p SearchPR) bool {
	login := strings.ToLower(p.Author.Login)
	return p.Author.IsBot || p.Author.Type == "Bot" || strings.HasPrefix(login, "app/") || strings.HasSuffix(login, "[bot]") ||
		strings.Contains(login, "dependabot") || strings.Contains(login, "renovate")
}

// reviewQueue keeps PRs by people other than me, oldest first.
func reviewQueue(prs []SearchPR, me string) []SearchPR {
	var out []SearchPR
	for _, p := range prs {
		if isBot(p) || strings.EqualFold(p.Author.Login, me) {
			continue
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out
}

func gh(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args[:min(2, len(args))], " "), truncate(strings.TrimSpace(string(ee.Stderr)), 300))
		}
		return nil, fmt.Errorf("gh: %w", err)
	}
	return out, nil
}

func ghJSON(ctx context.Context, v any, args ...string) error {
	out, err := gh(ctx, args...)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, v)
}

const searchFields = "url,number,title,createdAt,repository,author"

func myApprovedPRs(ctx context.Context) ([]SearchPR, error) {
	var prs []SearchPR
	err := ghJSON(ctx, &prs, "search", "prs", "--author", "@me", "--state", "open", "--review", "approved",
		"--json", searchFields, "--limit", "50")
	return prs, err
}

func myOpenPRs(ctx context.Context) ([]SearchPR, error) {
	var prs []SearchPR
	err := ghJSON(ctx, &prs, "search", "prs", "--author", "@me", "--state", "open", "--draft=false",
		"--json", searchFields, "--limit", "50")
	return prs, err
}

// reviewRequests includes requests to my teams, not only to me.
func reviewRequests(ctx context.Context) ([]SearchPR, error) {
	var prs []SearchPR
	err := ghJSON(ctx, &prs, "search", "prs", "--review-requested", "@me", "--state", "open", "--draft=false",
		"--json", searchFields, "--limit", "100")
	return prs, err
}

func prDetail(ctx context.Context, url string) (PRDetail, error) {
	var d PRDetail
	err := ghJSON(ctx, &d, "pr", "view", url, "--json",
		"title,body,headRefName,headRefOid,isDraft,mergeable,mergeStateStatus,reviewDecision,statusCheckRollup")
	return d, err
}

func myLogin(ctx context.Context) (string, error) {
	out, err := gh(ctx, "api", "user", "--jq", ".login")
	return strings.TrimSpace(string(out)), err
}

func slackLink(url, text string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return "<" + url + "|" + r.Replace(text) + ">"
}
