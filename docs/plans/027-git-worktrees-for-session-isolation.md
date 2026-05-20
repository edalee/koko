# Plan 027 — Git Worktrees for Session Isolation

## Problem

Running multiple Claude sessions against the same repository directory
causes them to pollute each other:

- File edits collide (one session's untracked file shows up in the other)
- Build artifacts, `node_modules`, dev-server ports clash
- `git status`, branch checkout, and stage state are shared
- Claude's per-directory memory (`.claude/agent-memory`, CLAUDE.md) is the same
- Subagent / MCP process trees overlap (we monitor by PID under the session)

Worktrees solve the OS-level isolation problem cleanly: each session
gets its own filesystem view of the same repository, on its own branch,
sharing only the underlying `.git` object database.

## Goal

Make worktrees a first-class concept in Koko so users can spin up
parallel sessions on the same repo without manual `git worktree`
ceremony.

## Non-goals

- Replacing branches or providing a branch-switching UI (out of scope)
- Cross-repo worktree management (one repo at a time, scoped to the
  active session's directory)
- Auto-syncing worktrees with remote refs

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Koko frontend                                                   │
│                                                                  │
│  NewSessionDialog ──A── "Create worktree" toggle                │
│         │                                                        │
│         ▼                                                        │
│  SessionSidebar ──B── per-session branch chip + collision dot   │
│                                                                  │
│  RightSidebar ───C── Worktrees module (GitBranch icon)          │
│         │            list / create / remove / open session      │
│         ▼                                                        │
│  ┌───────────────────────────────────────────┐                   │
│  │ Wails bindings (new on GitService):       │                   │
│  │   ListWorktrees(repoDir)                  │                   │
│  │   CreateWorktree(repoDir, path, branch,   │                   │
│  │                  createBranch)            │                   │
│  │   RemoveWorktree(path, force)             │                   │
│  │   GetWorktreeStatus(path)                 │                   │
│  └───────────────────────────────────────────┘                   │
│         │                                                        │
└─────────┼────────────────────────────────────────────────────────┘
          ▼
   `git worktree list --porcelain`
   `git worktree add <path> [-b <branch>] <ref>`
   `git worktree remove [--force] <path>`
```

## Backend

### Go: `git_service.go`

```go
type Worktree struct {
    Path                 string `json:"path"`
    Branch               string `json:"branch"`    // empty if detached
    HeadSHA              string `json:"headSha"`
    IsMain               bool   `json:"isMain"`
    IsDetached           bool   `json:"isDetached"`
    HasUncommittedChanges bool  `json:"hasUncommittedChanges"`
    Prunable             bool   `json:"prunable"` // worktree dir is missing
}

func (gs *GitService) ListWorktrees(repoDir string) ([]Worktree, error)
func (gs *GitService) CreateWorktree(repoDir, path, branch string, createBranch bool) (Worktree, error)
func (gs *GitService) RemoveWorktree(path string, force bool) error
func (gs *GitService) GetWorktreeForDir(dir string) (Worktree, error) // resolves any dir to its worktree
```

Implementation notes:

- `git worktree list --porcelain` is the canonical source; parse the
  `worktree` / `HEAD` / `branch` / `detached` records.
- `HasUncommittedChanges` = result of `git status --porcelain` from the
  worktree path being non-empty.
- `CreateWorktree` runs `git worktree add` from `repoDir`. If
  `createBranch` is true, passes `-b <branch>`. Otherwise just checks
  out the branch at the new path.
- `RemoveWorktree` runs `git worktree remove`; on `--force` failure
  (e.g. main worktree, missing dir), we surface the error to the UI.
- Path defaults: when the UI doesn't supply one, frontend picks
  `<sibling-of-repo>/<repo-name>-<branch-slug>` (kept in the UI to keep
  Go side stateless).

### Types: `types.go`

Add the `Worktree` struct, exported alongside existing GitService types.

### Wails bindings

Regenerated via `wails generate module`; `models.ts` will get the new
`Worktree` class.

## Frontend

### A) NewSessionDialog — Worktree toggle

When the user has picked a directory:

1. Look up whether that directory is currently in use by another
   *active* session via `tabs.some(t => t.directory === directory && t.connected)`.
2. If it is, show a small toggle row below the directory line:

   ```
   ⚠  Another session is using this directory
      [✓] Create as worktree   feat/quick-fix     branch picker ▾
      Worktree path: ~/Projects/koko-feat-quick-fix   (auto, editable)
   ```

3. Branch picker is a free-text field with optional "Create new branch"
   checkbox. Defaults to `<current-branch>-<random-suffix>` with
   create-new enabled, to avoid checkout collision.
4. Path defaults to a sibling of the source repo:
   `<dirname(directory)>/<basename(directory)>-<branch-slug>`.
5. On submit, if the toggle is on:
   - Call `CreateWorktree(directory, path, branch, createBranch)`.
   - Use the returned worktree's path as the session directory.
   - Set the session name default to the branch slug (so it's
     distinguishable from the parent session).
6. The toggle is also offered (defaulted off) when picking any
   directory inside an existing git repo, so the user can always
   start fresh on a new branch.

### B) SessionSidebar — Worktree awareness

Two additions per session row:

1. **Branch chip**: small text like `· feat/api` next to the directory
   name. We already fetch the branch via `useFileChanges(activeTab.directory)`
   for the active session; extend with a new hook
   `useSessionBranches(sessionDirs)` that batches per-directory branch
   lookups (debounced; same data as `GetBranchName`).
2. **Collision dot**: if two visible sessions have the same `directory`,
   show a small amber dot on each (tooltip: "Another session is using
   this directory — consider a worktree").

No layout overhaul; chips render inline in the existing row.

### C) Right-sidebar Worktree manager module

New `SidebarModule = "worktrees"` entry in `RightSidebar.tsx`,
icon = `GitBranch` from lucide-react.

Header:
```
Worktrees · main          [+ New]   ↻
```

Body — list of worktrees for the active session's repo:

```
┌──────────────────────────────────────────────────────┐
│ ● koko                main                  [MAIN]   │
│   ~/Projects/personal/koko                            │
│                                                       │
│ ● koko-feat-api       feat/api  · 3 changes          │
│   ~/Projects/personal/koko-feat-api    [Open] [×]    │
│                                                       │
│ ⚠ koko-old-thing      <prunable — dir missing> [×]   │
└──────────────────────────────────────────────────────┘
```

Per-row actions:
- **Open**: spawns a new Koko session in this worktree (calls
  `createTab` with the worktree path; closes the module).
- **×**: confirms then calls `RemoveWorktree(path, force=false)`. If the
  backend returns "uncommitted changes", offer a "Force remove" prompt.
- **MAIN** badge is read-only — cannot be removed.
- Prunable rows have a single "Prune" button (`git worktree prune`,
  or `--force` remove on missing path).

"+ New" opens a small inline form:
- Branch: text field with autocomplete from `git branch --list`
- Create new branch: checkbox (default off)
- Path: auto-suggested, editable

Polls `ListWorktrees` every 10s while the module is open.

## Data flow

- `useWorktrees(repoDir)` — new hook returning
  `{ worktrees, loading, refresh, create, remove }`.
- The repoDir is resolved from `activeTab.directory` via
  `GetWorktreeForDir` (so picking a worktree shows that worktree's
  siblings, not just the main).
- The Worktree manager module is scoped to the active session; switching
  sessions reloads the list.

## UX rules

- Never auto-create worktrees behind the user's back. The toggle is
  always explicit.
- Never remove the main worktree.
- Always warn on remove with uncommitted changes.
- Path defaults are sibling-of-repo so they don't pollute the source dir.
- Closing a Koko-created worktree's session does *not* auto-remove the
  worktree (deferred to plan 028, "auto-cleanup on session close").

## Testing

- Go: unit-test the `--porcelain` parser with fixtures (main + 2
  worktrees, detached HEAD, prunable, locked).
- Frontend: vitest for the collision-detection logic in
  NewSessionDialog and the worktree branch resolver.
- Manual: create 2 sessions in the same repo, confirm B's collision
  dot fires; create a worktree via C, confirm new session uses it.

## Rollout

1. Land Go methods + types (no UI yet, safe to ship behind the existing
   GitService binding).
2. Land NewSessionDialog toggle (A) — opt-in, no behaviour change for
   users who don't toggle it.
3. Land SessionSidebar enhancements (B) — visual only.
4. Land Worktrees module (C) — new module; users who don't open it pay
   nothing.

## Revert / rollback notes

Each piece is independently revertable:

- **A**: remove the toggle block from `NewSessionDialog.tsx`; the Go
  `CreateWorktree` method can stay unused.
- **B**: remove the branch chip + collision dot from `SessionSidebar.tsx`
  and delete `useSessionBranches`.
- **C**: remove the `"worktrees"` entry from `SidebarModule`, the icon,
  the panel render block, and `useWorktrees`.

The Go additions are pure; leaving them in place after a UI revert
costs nothing.

## Out of scope (future plans)

- **028 — Auto-cleanup on session close**: prompt to `git worktree remove`
  Koko-created worktrees when their session closes cleanly.
- **029 — Branch creation UX**: nicer branch picker with remote
  branch list + tab completion.
- **030 — Worktree templates**: pre-seed common files (`.env.local`)
  into new worktrees.
