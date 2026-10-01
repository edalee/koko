package main

// SessionInfo represents metadata about a running terminal session.
type SessionInfo struct {
	ID   string `json:"id"`   // PTY session ID (transient)
	Slug string `json:"slug"` // human-friendly slug e.g. "koko-1"
	Name string `json:"name"`
	Dir  string `json:"dir"`
	// The Claude conversation this session holds, once captured. API and MCP
	// callers need it to resume a conversation, since resuming takes an id.
	ClaudeSessionID string `json:"claudeSessionId,omitempty"`
}

// SessionRecord is the unified persisted session model.
// Replaces SavedSessionTab and SavedSessionHistory.
type SessionRecord struct {
	Slug            string `json:"slug"`                      // e.g. "koko-1"
	Name            string `json:"name"`                      // display name
	Directory       string `json:"directory"`                 // full path
	ClaudeSessionID string `json:"claudeSessionId,omitempty"` // UUID from Claude Code JSONL
	CreatedAt       int64  `json:"createdAt"`
	ClosedAt        int64  `json:"closedAt,omitempty"`
	Status          string `json:"status"`            // "active", "disconnected", "closed"
	LastMsg         string `json:"lastMsg,omitempty"` // last assistant message snippet
	WorktreePath    string `json:"worktreePath,omitempty"` // the session's worktree, created by Koko or opened from the Worktrees module
	// WorktreeCreated is true only when Koko created the worktree. Settings
	// removes only these in bulk, never one the user made by hand.
	WorktreeCreated bool `json:"worktreeCreated,omitempty"`
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
	IsBinary    bool   `json:"isBinary"`
	LineCount   int    `json:"lineCount"`
}

// FileContentData represents raw file content.
type FileContentData struct {
	Content  string `json:"content"`
	Language string `json:"language"`
	Path     string `json:"path"`
	IsBinary bool   `json:"isBinary"`
}

// GitHubPR represents a pull request from a tracked repository.
type GitHubPR struct {
	Repo           string        `json:"repo"`
	Number         int           `json:"number"`
	Title          string        `json:"title"`
	Author         string        `json:"author"`
	ReviewDecision string        `json:"reviewDecision"`
	URL            string        `json:"url"`
	Body           string        `json:"body"`
	Additions      int           `json:"additions"`
	Deletions      int           `json:"deletions"`
	ChangedFiles   int           `json:"changedFiles"`
	HeadRef        string        `json:"headRef"`
	BaseRef        string        `json:"baseRef"`
	CreatedAt      string        `json:"createdAt"`
	UpdatedAt      string        `json:"updatedAt"`
	Mergeable        string    `json:"mergeable"`
	MergeStateStatus string    `json:"mergeStateStatus"` // BLOCKED, DIRTY, UNSTABLE, BEHIND, etc.
	IsDraft          bool      `json:"isDraft"`
	Labels           []string  `json:"labels"`
	Assignees        []string  `json:"assignees"`
	Checks           []PRCheck `json:"checks"`
}

// PRCheck represents a CI status check on a PR.
type PRCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`     // COMPLETED, IN_PROGRESS, QUEUED
	Conclusion string `json:"conclusion"` // SUCCESS, FAILURE, CANCELLED, etc.
}

// PRFile represents a changed file in a PR.
type PRFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// PRReview represents a review on a PR.
type PRReview struct {
	Author      string `json:"author"`
	State       string `json:"state"`       // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
	SubmittedAt string `json:"submittedAt"`
	Body        string `json:"body"`
}

// PRCommit represents a commit in a PR.
type PRCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

// WorkflowRun represents a GitHub Actions workflow run.
type WorkflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`     // completed, in_progress, queued, requested, waiting
	Conclusion string `json:"conclusion"` // success, failure, cancelled, skipped, timed_out
	Event      string `json:"event"`      // push, pull_request, schedule
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	HTMLURL    string `json:"htmlUrl"`
}

// Worktree represents a git worktree.
type Worktree struct {
	Path                  string `json:"path"`
	Branch                string `json:"branch"`     // empty if detached
	HeadSHA               string `json:"headSha"`
	IsMain                bool   `json:"isMain"`
	IsDetached            bool   `json:"isDetached"`
	HasUncommittedChanges bool   `json:"hasUncommittedChanges"`
	Prunable              bool   `json:"prunable"` // worktree dir is missing
}

// Conversation is one Claude session stored on disk, as offered by the
// session picker.
type Conversation struct {
	UUID       string `json:"uuid"`       // the session file's name
	Title      string `json:"title"`      // Claude's own title, else the first prompt
	Preview    string `json:"preview"`    // last assistant message, truncated
	ModifiedAt int64  `json:"modifiedAt"` // unix millis
	SizeBytes  int64  `json:"sizeBytes"`
}

// DeleteResult reports a bulk conversation delete. Deleted lists the ids, so
// the frontend can drop them from closed-session records that point at them.
type DeleteResult struct {
	Deleted []string `json:"deleted"`
	// Skipped counts conversations in use (by a session, a saved tab or a
	// pending create, or maybe by a running session with no id yet), and
	// files of unknown directory.
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"` // the delete itself failed, so the file is still there
}

// WorktreeCleanup reports a bulk removal of Koko's worktrees. The frontend
// forgets the Removed and Gone paths.
type WorktreeCleanup struct {
	Removed []string          `json:"removed"`
	Gone    []string          `json:"gone"` // folder already deleted, so nothing to remove
	Skipped []SkippedWorktree `json:"skipped"`
}

// SkippedWorktree is a worktree the cleanup left alone, and why.
type SkippedWorktree struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// BranchCI represents CI status for a git branch.
type BranchCI struct {
	Branch string        `json:"branch"`
	Repo   string        `json:"repo"`
	Runs   []WorkflowRun `json:"runs"`
}

// PRComment represents a comment on a PR (review comment or issue comment).
type PRComment struct {
	ID           int64  `json:"id"`
	InReplyToID  int64  `json:"inReplyToId"`
	Author       string `json:"author"`
	AuthorType   string `json:"authorType"`
	Body         string `json:"body"`
	CreatedAt    string `json:"createdAt"`
	HTMLURL      string `json:"htmlUrl"`
	Path         string `json:"path"`
	Line         int    `json:"line"`
	OriginalLine int    `json:"originalLine"`
	Side         string `json:"side"`
	DiffHunk     string `json:"diffHunk"`
	SubjectType  string `json:"subjectType"`
}

// PRCommentThread represents a threaded review comment and its replies.
type PRCommentThread struct {
	Root    PRComment   `json:"root"`
	Replies []PRComment `json:"replies"`
}

// PRCommentsData holds all comments for a PR.
type PRCommentsData struct {
	ReviewThreads []PRCommentThread `json:"reviewThreads"`
	IssueComments []PRComment       `json:"issueComments"`
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
