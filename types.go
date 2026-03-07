package main

// SessionInfo represents metadata about a terminal session.
type SessionInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

// GitHubPR represents a pull request from a tracked repository.
type GitHubPR struct {
	Repo           string `json:"repo"`
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	ReviewDecision string `json:"reviewDecision"`
	URL            string `json:"url"`
}
