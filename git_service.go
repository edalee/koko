package main

import (
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
		result.NewContent = string(data)
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
