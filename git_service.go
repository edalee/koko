package main

import (
	"os/exec"
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
