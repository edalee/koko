# Plan 021: PR Detail Overlay Improvements

## Overview

Enhance the full-screen PR detail overlay with better layout, hide/ignore functionality, file changes list, collapsible checks, and richer reviewer context.

## Changes

### 1. Layout Reorder

**Current order:** Header → Branch+stats → Labels → Actions → CI Checks → Description → Mergeable

**New order:** Header (with prominent author) → Actions → Description → File Changes → Reviews → Commits → CI Checks (collapsible) → Labels → Mergeable

**Detail changes in `PRDetailOverlay.tsx`:**

- **Author in header:** Show author prominently — initial badge + name right below the title, not buried in metadata
- **Description first:** Move markdown description to immediately after action buttons
- **File changes:** New section after description (see §3)
- **Collapsible CI checks:** Wrap in disclosure. Auto-collapsed if all passing, expanded if any failing
- **Labels:** Move below checks (less important)

### 2. Hide/Ignore PRs

**Goal:** Eye icon on each PR row in the sidebar list. Clicking hides the PR. Persisted across restarts.

**Backend (`config_service.go` + `types.go`):** ✅ Done
- `HiddenPRs map[string]bool` in `AppConfig` (key: `"repo#number"`)
- `HidePR()`, `UnhidePR()`, `GetHiddenPRs()` on ConfigService

**Frontend (`PRDetailOverlay.tsx`):**
- `EyeOff` icon button on each PR row in the right sidebar list
- On click: call `HidePR(repo, number)`, remove from local state
- Filter hidden PRs from the list before rendering

**Frontend (`RightSidebar.tsx`):**
- PR badge count reflects visible (non-hidden) PRs only

**Frontend (manage hidden):**
- "Show hidden" toggle at bottom of PR list — hidden PRs shown greyed out with `Eye` icon to unhide

### 3. File Changes List

**Goal:** Show actual changed files with per-file +/- counts.

**Backend (`github_service.go`):**
- New bound method: `FetchPRFiles(repo, number)` → `[]PRFile`
- Uses `gh pr view --json files` (lazy-loaded per PR, not bulk)

**Frontend (`PRDetailOverlay.tsx`):**
- "Files changed" section — each row: file icon + path + green `+N` + red `-N`
- Sorted by path, cached in component state

### 4. Additional Reviewer Information

**Merge state reason** — `mergeStateStatus` field tells *why* a PR can't merge:
- BLOCKED (reviews), DIRTY (conflicts), UNSTABLE (CI), BEHIND (base branch)
- Much better than current binary mergeable yes/no

**Reviews & requested reviewers:**
- New bound method: `FetchPRReviews(repo, number)` → `[]PRReview`
- Fetches `latestReviews` + `reviewRequests` from `gh pr view`
- Show reviewer name + decision badge (approved/changes requested/commented)
- Show who still needs to review

**Commits list:**
- New bound method: `FetchPRCommits(repo, number)` → `[]PRCommit`
- Collapsible section with commit messages + authors + short SHA

**Linked issues:**
- Add `closingIssuesReferences` to PR query
- Show linked issue titles with numbers

**Assignees:**
- Add `assignees` to PR query
- Show in header metadata

### 5. Review Skill (Skipped for now)

Pattern identified: adapt the existing design-reviewer agent (`.claude/agents/design-reviewer.md`) for PR code review — iterative loops with configurable depth, posts findings via `gh pr review`. Will implement separately.

## Implementation Order

1. Layout reorder + collapsible checks
2. Hide/ignore PRs (backend done, frontend needed)
3. File changes list (backend + frontend)
4. Additional reviewer info (merge state, reviews, commits, linked issues, assignees)

## New Types ✅ Done

```go
type PRFile struct {
    Path      string `json:"path"`
    Additions int    `json:"additions"`
    Deletions int    `json:"deletions"`
}

type PRReview struct {
    Author      string `json:"author"`
    State       string `json:"state"`       // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
    SubmittedAt string `json:"submittedAt"`
    Body        string `json:"body"`
}

type PRCommit struct {
    SHA     string `json:"sha"`
    Message string `json:"message"`
    Author  string `json:"author"`
    Date    string `json:"date"`
}
```

## Modified Files

- `types.go` — PRFile, PRReview, PRCommit structs ✅
- `config_service.go` — HiddenPRs in AppConfig, HidePR/UnhidePR/GetHiddenPRs ✅
- `github_service.go` — FetchPRFiles, FetchPRReviews, FetchPRCommits, expanded PR query
- `PRDetailOverlay.tsx` — layout reorder, file changes, collapsible checks, reviews, commits
- `RightSidebar.tsx` — hidden PR filtering, badge count
- `App.tsx` — pass hidden PR state
- `frontend/src/types/index.ts` — new TS types
- `frontend/wailsjs/` — auto-generated bindings
