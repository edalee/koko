package main

// SessionInfo represents metadata about a running terminal session.
type SessionInfo struct {
	ID   string `json:"id"`   // PTY session ID (transient)
	Slug string `json:"slug"` // human-friendly slug e.g. "koko/1"
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

// SessionRecord is the unified persisted session model.
// Replaces SavedSessionTab and SavedSessionHistory.
type SessionRecord struct {
	Slug            string `json:"slug"`                      // e.g. "koko/1"
	Name            string `json:"name"`                      // display name
	Directory       string `json:"directory"`                 // full path
	ClaudeSessionID string `json:"claudeSessionId,omitempty"` // UUID from Claude Code JSONL
	CreatedAt       int64  `json:"createdAt"`
	ClosedAt        int64  `json:"closedAt,omitempty"`
	Status          string `json:"status"`          // "active", "disconnected", "closed"
	LastMsg         string `json:"lastMsg,omitempty"` // last assistant message snippet
}

// SessionsData holds all persisted session state.
type SessionsData struct {
	Sessions   []SessionRecord `json:"sessions"`   // all sessions (active + disconnected + closed)
	RecentDirs []string        `json:"recentDirs"` // for new session dialog
	// Legacy fields for migration
	Tabs    []SavedSessionTab     `json:"tabs,omitempty"`
	History []SavedSessionHistory `json:"history,omitempty"`
}

// SavedSessionTab is the legacy persisted session tab (pre-019).
type SavedSessionTab struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Directory string `json:"directory"`
	CreatedAt int64  `json:"createdAt"`
}

// SavedSessionHistory is the legacy closed session entry (pre-019).
type SavedSessionHistory struct {
	Name        string `json:"name"`
	Directory   string `json:"directory"`
	CreatedAt   int64  `json:"createdAt"`
	ClosedAt    int64  `json:"closedAt"`
	LastMessage string `json:"lastMessage,omitempty"`
}

// ProcessInfo represents a child process of a Claude session.
type ProcessInfo struct {
	PID       int    `json:"pid"`
	Command   string `json:"command"`
	FullCmd   string `json:"fullCmd"`
	Type      string `json:"type"`
	Elapsed   string `json:"elapsed"`
	ElapsedMs int64  `json:"elapsedMs"`
	Children  int    `json:"children"`
}

// MCPServer represents a configured MCP server and its connection status.
type MCPServer struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Status  string `json:"status"`
}

// AgentInfo represents a built-in Claude agent.
type AgentInfo struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// CommandInfo represents a slash command or custom agent definition.
type CommandInfo struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// FileDiffData represents diff data for a single file.
type FileDiffData struct {
	OldFileName string `json:"oldFileName"`
	OldContent  string `json:"oldContent"`
	NewFileName string `json:"newFileName"`
	NewContent  string `json:"newContent"`
	Hunks       string `json:"hunks"`
	Language    string `json:"language"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
}

// FileContentData represents raw file content.
type FileContentData struct {
	Content  string `json:"content"`
	Language string `json:"language"`
	Path     string `json:"path"`
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
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Repo      string `json:"repo"`
	URL       string `json:"url"`
	Unread    bool   `json:"unread"`
	UpdatedAt string `json:"updatedAt"`
}
