package main

// SessionInfo represents metadata about a terminal session.
type SessionInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

// ProcessInfo represents a child process of a Claude session.
type ProcessInfo struct {
	PID       int    `json:"pid"`
	Command   string `json:"command"`   // short display name
	FullCmd   string `json:"fullCmd"`   // full command line
	Type      string `json:"type"`      // "subagent", "mcp", "infrastructure", "tool"
	Elapsed   string `json:"elapsed"`   // human-readable duration
	ElapsedMs int64  `json:"elapsedMs"` // for sorting
	Children  int    `json:"children"`  // number of grandchild processes
}

// MCPServer represents a configured MCP server and its connection status.
type MCPServer struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Status  string `json:"status"` // "connected", "auth_needed", "error"
}

// AgentInfo represents a built-in Claude agent.
type AgentInfo struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// CommandInfo represents a slash command or custom agent definition.
type CommandInfo struct {
	Name        string `json:"name"`
	Source      string `json:"source"` // "project", "global"
	Type        string `json:"type"`   // "command", "agent"
	Description string `json:"description"`
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

// GitHubNotification represents a GitHub notification.
type GitHubNotification struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`   // "PullRequest", "Issue", "Release", etc.
	Reason    string `json:"reason"` // "review_requested", "mention", "subscribed", etc.
	Repo      string `json:"repo"`
	URL       string `json:"url"` // HTML URL for opening in browser
	Unread    bool   `json:"unread"`
	UpdatedAt string `json:"updatedAt"`
}
