package main

import (
	"encoding/json"
	"os/exec"
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
