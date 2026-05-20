package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitService provides git information for session directories.
type GitService struct{}

// NewGitService creates a new GitService.
func NewGitService() *GitService {
	return &GitService{}
}

// FileChange represents a changed file in the current git branch.
type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"` // "added", "modified", "deleted", "renamed"
	Staged bool   `json:"staged"`
}

// GetFileChanges returns files changed in the current branch compared to the default branch.
func (gs *GitService) GetFileChanges(dir string) ([]FileChange, error) {
	if dir == "" {
		return nil, nil
	}

	// Always include uncommitted changes (staged + unstaged)
	uncommitted, _ := gs.getUncommittedChanges(dir)

	// Find the merge base with the default branch
	base, err := gs.findBaseBranch(dir)
	if err != nil {
		return uncommitted, nil
	}

	// Get diff against the base branch
	mergeBase, err := gs.runGit(dir, "merge-base", base, "HEAD")
	if err != nil {
		return uncommitted, nil
	}

	// Committed changes vs base
	diffOutput, err := gs.runGit(dir, "diff", "--name-status", strings.TrimSpace(mergeBase), "HEAD")
	if err != nil {
		return uncommitted, nil
	}

	changes := gs.parseDiffOutput(diffOutput)

	return gs.mergeChanges(changes, uncommitted), nil
}

// GetBranchName returns the current branch name for the given directory.
func (gs *GitService) GetBranchName(dir string) (string, error) {
	out, err := gs.runGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetRepoSlug returns the owner/repo slug from the git remote origin URL.
// e.g. "epidemicsound/drumstick2" from "git@github.com:epidemicsound/drumstick2.git"
func (gs *GitService) GetRepoSlug(dir string) (string, error) {
	out, err := gs.runGit(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(out)
	// SSH: git@github.com:owner/repo.git
	if idx := strings.Index(url, ":"); idx != -1 && !strings.Contains(url[:idx], "/") {
		url = url[idx+1:]
	}
	// HTTPS: https://github.com/owner/repo.git
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimPrefix(url, "http://github.com/")
	url = strings.TrimSuffix(url, ".git")
	return url, nil
}

func (gs *GitService) findBaseBranch(dir string) (string, error) {
	// Try common default branch names
	for _, branch := range []string{"main", "master"} {
		_, err := gs.runGit(dir, "rev-parse", "--verify", branch)
		if err == nil {
			return branch, nil
		}
	}
	// Try remote defaults
	for _, branch := range []string{"origin/main", "origin/master"} {
		_, err := gs.runGit(dir, "rev-parse", "--verify", branch)
		if err == nil {
			return branch, nil
		}
	}
	return "", exec.ErrNotFound
}

func (gs *GitService) getUncommittedChanges(dir string) ([]FileChange, error) {
	out, err := gs.runGit(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	return gs.parseStatusOutput(out), nil
}

func (gs *GitService) runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (gs *GitService) parseDiffOutput(output string) []FileChange {
	var changes []FileChange
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		status := gs.normalizeStatus(parts[0])
		path := parts[1]
		// Handle renames: R100\told\tnew
		if strings.HasPrefix(parts[0], "R") && len(parts) > 1 {
			tabParts := strings.SplitN(line, "\t", 3)
			if len(tabParts) == 3 {
				path = tabParts[2]
			}
		}
		changes = append(changes, FileChange{Path: path, Status: status})
	}
	return changes
}

func (gs *GitService) parseStatusOutput(output string) []FileChange {
	var changes []FileChange
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if len(line) < 4 {
			continue
		}
		x := line[0] // staged (index) status
		y := line[1] // unstaged (working tree) status
		path := strings.TrimSpace(line[3:])

		// Untracked files
		if x == '?' {
			changes = append(changes, FileChange{Path: path, Status: "added", Staged: false})
			continue
		}

		// Staged change (index column)
		if x != ' ' {
			changes = append(changes, FileChange{Path: path, Status: gs.charToStatus(x), Staged: true})
		}

		// Unstaged change (working tree column)
		if y != ' ' {
			changes = append(changes, FileChange{Path: path, Status: gs.charToStatus(y), Staged: false})
		}
	}
	return changes
}

func (gs *GitService) charToStatus(c byte) string {
	switch c {
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	default:
		return "modified"
	}
}

func (gs *GitService) normalizeStatus(s string) string {
	switch {
	case strings.HasPrefix(s, "A"):
		return "added"
	case strings.HasPrefix(s, "D"):
		return "deleted"
	case strings.HasPrefix(s, "R"):
		return "renamed"
	default:
		return "modified"
	}
}

// langMap maps file extensions to language names for syntax highlighting.
var langMap = map[string]string{
	".ts": "typescript", ".tsx": "tsx", ".js": "javascript", ".jsx": "jsx",
	".go": "go", ".py": "python", ".rs": "rust", ".java": "java",
	".kt": "kotlin", ".rb": "ruby", ".sh": "bash", ".bash": "bash",
	".css": "css", ".scss": "scss", ".html": "html", ".json": "json",
	".yaml": "yaml", ".yml": "yaml", ".toml": "toml", ".md": "markdown",
	".sql": "sql", ".proto": "protobuf", ".dockerfile": "dockerfile",
	".xml": "xml", ".svg": "xml", ".graphql": "graphql",
	".c": "c", ".cpp": "cpp", ".h": "c", ".hpp": "cpp",
	".swift": "swift", ".zig": "zig", ".lua": "lua",
}

func inferLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	// Handle Dockerfile without extension
	base := strings.ToLower(filepath.Base(path))
	if base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") {
		return "dockerfile"
	}
	if base == "makefile" {
		return "makefile"
	}
	return ""
}

// isBinaryData checks if data contains null bytes in the first 8KB (same heuristic as git).
func isBinaryData(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	return bytes.ContainsRune(data[:n], 0)
}

// GetFileDiff returns the diff data for a file including old/new content and unified diff hunks.
func (gs *GitService) GetFileDiff(dir, path string, staged bool) (FileDiffData, error) {
	if dir == "" || path == "" {
		return FileDiffData{}, nil
	}

	lang := inferLanguage(path)
	result := FileDiffData{
		OldFileName: path,
		NewFileName: path,
		Language:    lang,
	}

	// Get the unified diff hunks
	var hunks string
	var err error
	if staged {
		hunks, err = gs.runGit(dir, "diff", "--cached", "--", path)
	} else {
		hunks, err = gs.runGit(dir, "diff", "--", path)
	}
	if err != nil {
		// Might be a new untracked file — no diff available
		hunks = ""
	}
	result.Hunks = hunks

	// Count additions/deletions from hunks
	for _, line := range strings.Split(hunks, "\n") {
		if len(line) > 0 && line[0] == '+' && !strings.HasPrefix(line, "+++") {
			result.Additions++
		} else if len(line) > 0 && line[0] == '-' && !strings.HasPrefix(line, "---") {
			result.Deletions++
		}
	}

	// Get old content
	if staged {
		// For staged files, old content is from HEAD
		old, err := gs.runGit(dir, "show", "HEAD:"+path)
		if err != nil {
			// New file — no HEAD version
			result.OldContent = ""
		} else {
			result.OldContent = old
		}
	} else {
		// For unstaged files, old content is from the index
		old, err := gs.runGit(dir, "show", ":"+path)
		if err != nil {
			// File might not be in the index (untracked), try HEAD
			old, err = gs.runGit(dir, "show", "HEAD:"+path)
			if err != nil {
				result.OldContent = ""
			} else {
				result.OldContent = old
			}
		} else {
			result.OldContent = old
		}
	}

	// Get new content from working tree
	fullPath := filepath.Join(dir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		// Deleted file
		result.NewContent = ""
	} else {
		if isBinaryData(data) {
			result.IsBinary = true
			result.OldContent = ""
			result.NewContent = ""
			result.Hunks = ""
			return result, nil
		}
		result.NewContent = string(data)
	}

	// Count lines in the larger of old/new content
	oldLines := strings.Count(result.OldContent, "\n")
	newLines := strings.Count(result.NewContent, "\n")
	if oldLines > newLines {
		result.LineCount = oldLines
	} else {
		result.LineCount = newLines
	}

	return result, nil
}

// GetFileContent returns the raw content of a file with language inference.
func (gs *GitService) GetFileContent(dir, path string) (FileContentData, error) {
	if dir == "" || path == "" {
		return FileContentData{}, nil
	}

	fullPath := filepath.Join(dir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return FileContentData{}, err
	}

	if isBinaryData(data) {
		return FileContentData{
			Language: inferLanguage(path),
			Path:     path,
			IsBinary: true,
		}, nil
	}

	return FileContentData{
		Content:  string(data),
		Language: inferLanguage(path),
		Path:     path,
	}, nil
}

// mergeChanges combines committed and uncommitted changes, deduplicating by path.
func (gs *GitService) mergeChanges(committed, uncommitted []FileChange) []FileChange {
	seen := make(map[string]bool)
	var result []FileChange
	for _, c := range committed {
		seen[c.Path] = true
		result = append(result, c)
	}
	for _, c := range uncommitted {
		if !seen[c.Path] {
			result = append(result, c)
		}
	}
	return result
}

// ListWorktrees returns all worktrees for the repository containing dir.
// dir may be any path inside the repo (main worktree or any worktree).
func (gs *GitService) ListWorktrees(dir string) ([]Worktree, error) {
	if dir == "" {
		return nil, nil
	}
	out, err := gs.runGit(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	worktrees := parseWorktreePorcelain(out)
	// Annotate each worktree with HasUncommittedChanges
	for i := range worktrees {
		if worktrees[i].Prunable {
			continue
		}
		status, err := gs.runGit(worktrees[i].Path, "status", "--porcelain")
		if err == nil && strings.TrimSpace(status) != "" {
			worktrees[i].HasUncommittedChanges = true
		}
	}
	return worktrees, nil
}

// CreateWorktree creates a new worktree at path checked out to branch.
// If createBranch is true, branch is created as a new branch from HEAD.
// repoDir must be any path inside the repository.
func (gs *GitService) CreateWorktree(repoDir, path, branch string, createBranch bool) (Worktree, error) {
	if repoDir == "" || path == "" {
		return Worktree{}, exec.ErrNotFound
	}
	args := []string{"worktree", "add"}
	if createBranch {
		args = append(args, "-b", branch, path)
	} else {
		args = append(args, path, branch)
	}
	if _, err := gs.runGit(repoDir, args...); err != nil {
		return Worktree{}, err
	}
	// Look up the new worktree to return its full state
	worktrees, err := gs.ListWorktrees(repoDir)
	if err != nil {
		return Worktree{}, err
	}
	absPath, _ := filepath.Abs(path)
	for _, wt := range worktrees {
		if wt.Path == absPath || wt.Path == path {
			return wt, nil
		}
	}
	// Fallback: synthesise from inputs
	return Worktree{Path: path, Branch: branch}, nil
}

// RemoveWorktree removes a worktree at path. force=true passes --force,
// which is needed when the worktree has uncommitted changes or is locked.
// The main worktree cannot be removed.
func (gs *GitService) RemoveWorktree(path string, force bool) error {
	if path == "" {
		return exec.ErrNotFound
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	// Run from inside the worktree itself (or its parent if dir is gone)
	runDir := path
	if _, err := os.Stat(path); err != nil {
		runDir = filepath.Dir(path)
	}
	_, err := gs.runGit(runDir, args...)
	return err
}

// PruneWorktrees runs `git worktree prune` to remove administrative
// records for worktrees whose directories no longer exist.
func (gs *GitService) PruneWorktrees(dir string) error {
	if dir == "" {
		return exec.ErrNotFound
	}
	_, err := gs.runGit(dir, "worktree", "prune")
	return err
}

// GetWorktreeForDir resolves any directory inside a repo to its worktree info.
func (gs *GitService) GetWorktreeForDir(dir string) (Worktree, error) {
	if dir == "" {
		return Worktree{}, exec.ErrNotFound
	}
	// `git rev-parse --show-toplevel` gives the worktree root
	out, err := gs.runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Worktree{}, err
	}
	root := strings.TrimSpace(out)
	worktrees, err := gs.ListWorktrees(root)
	if err != nil {
		return Worktree{}, err
	}
	for _, wt := range worktrees {
		if wt.Path == root {
			return wt, nil
		}
	}
	return Worktree{Path: root}, nil
}

// parseWorktreePorcelain parses the output of `git worktree list --porcelain`.
// Records are separated by blank lines. Each record has lines like:
//
//	worktree /path/to/main
//	HEAD 1234abcd...
//	branch refs/heads/main
//
// detached worktrees have `detached` instead of `branch`. Missing dirs
// have `prunable <reason>`.
func parseWorktreePorcelain(output string) []Worktree {
	var worktrees []Worktree
	var cur Worktree
	hasAny := false
	flush := func() {
		if !hasAny {
			return
		}
		worktrees = append(worktrees, cur)
		cur = Worktree{}
		hasAny = false
	}
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			flush()
			continue
		}
		hasAny = true
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.HeadSHA = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			cur.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "detached":
			cur.IsDetached = true
		case strings.HasPrefix(line, "prunable"):
			cur.Prunable = true
		}
	}
	flush()
	// The first entry from `git worktree list` is always the main worktree
	if len(worktrees) > 0 {
		worktrees[0].IsMain = true
	}
	return worktrees
}
