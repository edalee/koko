package main

import (
	"os"
	"path/filepath"
	"strings"
)

// RemoveWorktrees removes worktrees Koko created for sessions that have since
// closed (plan 028 step 7b). It never forces.
//
// It skips a worktree that a session or a saved tab uses, one with
// uncommitted or ignored files, and any path that is not a linked worktree,
// such as a repo's main checkout. Ignored files count because a plain
// `git worktree remove` deletes them, and they are often a local .env or
// settings file. The user removes a skipped one from the Worktrees module,
// which offers the force path with a warning.
func (a *App) RemoveWorktrees(paths []string) WorktreeCleanup {
	out := WorktreeCleanup{Removed: []string{}, Gone: []string{}, Skipped: []SkippedWorktree{}}
	skip := func(path, reason string) {
		out.Skipped = append(out.Skipped, SkippedWorktree{Path: path, Reason: reason})
	}
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Git may still list it, so its branch reads as checked out
			// until `git worktree prune` runs in the repo.
			out.Gone = append(out.Gone, path)
			continue
		}
		// Checked per path, not once up front, so a session opened while
		// earlier worktrees were being removed still counts.
		if usedBy(path, a.dirsInUse()) {
			skip(path, "in use by a session")
			continue
		}
		if !a.isLinkedWorktree(path) {
			skip(path, "not a linked worktree")
			continue
		}
		if status, err := a.git.runGit(path, "status", "--porcelain", "--ignored"); err != nil {
			skip(path, "could not read its git status")
			continue
		} else if strings.TrimSpace(status) != "" {
			skip(path, "has uncommitted or ignored files")
			continue
		}
		if err := a.git.RemoveWorktree(path, false); err != nil {
			skip(path, "git refused: "+err.Error())
			continue
		}
		out.Removed = append(out.Removed, path)
	}
	return out
}

// dirsInUse returns the directories of running sessions and of saved tabs,
// connected or not. A disconnected tab reconnects into its directory, so
// removing that directory would leave the tab nowhere to reconnect.
//
// A session whose Claude has exited is not counted. Closing a disconnected
// tab leaves its dead entry in the TerminalManager, and that entry would
// otherwise block removal until Koko restarts.
func (a *App) dirsInUse() []string {
	var dirs []string
	a.tm.mu.Lock()
	for _, s := range a.tm.sessions {
		if s.alive() {
			dirs = append(dirs, s.dir)
		}
	}
	a.tm.mu.Unlock()
	for _, r := range a.cfg.GetSessions().Sessions {
		if r.Status == "active" || r.Status == "disconnected" {
			dirs = append(dirs, r.Directory, r.WorktreePath)
		}
	}
	return dirs
}

// usedBy reports whether any of dirs is path or lies inside it.
func usedBy(path string, dirs []string) bool {
	root := normaliseDir(path) + string(filepath.Separator)
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if strings.HasPrefix(normaliseDir(d)+string(filepath.Separator), root) {
			return true
		}
	}
	return false
}

// isLinkedWorktree reports whether path is the top of a linked worktree. Its
// git dir then differs from the common dir, which belongs to the main
// checkout.
func (a *App) isLinkedWorktree(path string) bool {
	out, err := a.git.runGit(path, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-dir", "--git-common-dir")
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		return false
	}
	top, gitDir, commonDir := lines[0], lines[1], lines[2]
	return sameDir(top, path) && !sameDir(gitDir, commonDir)
}
