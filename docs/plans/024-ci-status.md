# Plan 024: CI Status for Active Branches

**Status:** Implemented

## Summary

Shows GitHub Actions CI status for the current branch directly in the File Changes sidebar module. A status dot appears next to the branch name, and a collapsible "CI" section lists recent workflow runs with status icons, names, and timestamps. Clicking a run opens it on GitHub.

## Architecture

- CI status embedded in existing File Changes module (not a separate sidebar module)
- Polls every 60s via `gh run list` (matches PR/notification polling cadence)
- Repo slug detected from `git remote get-url origin` per session directory

## Changes

### Go Backend
- **`types.go`** — Added `WorkflowRun` and `BranchCI` types
- **`git_service.go`** — Added `GetRepoSlug(dir)` — extracts `owner/repo` from git remote (SSH + HTTPS)
- **`github_service.go`** — Added `FetchBranchCI(repoSlug, branch)` — `gh run list --branch` with last 5 runs

### Frontend
- **`types/index.ts`** — Added `WorkflowRun` and `BranchCI` interfaces
- **`hooks/useCI.ts`** — New hook, polls every 60s, takes `(directory, branch)`, calls `GetRepoSlug` → `FetchBranchCI`
- **`components/RightSidebar.tsx`** — CI status dot next to branch name, collapsible `CIRunsSection` below file list (auto-expanded on failures)
- **`App.tsx`** — Wired `useCI` hook, passes `ci` + `ciLoading` to `RightSidebar`

## UI Details

- **Status dot**: green (success), red (failure), amber pulse (in-progress), hidden (no runs)
- **CI section header**: chevron + status icon + "CI" label + run count
- **Each run**: status icon + workflow name (truncated) + external link icon on hover + time ago
- **Auto-expand**: section opens by default if any run is failing or in-progress
- **Click**: opens the GitHub Actions run URL in browser
