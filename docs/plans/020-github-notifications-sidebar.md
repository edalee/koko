# Plan 020: Move GitHub & Notifications to Right Sidebar

## Changes

### 1. Remove from Toolbar
Remove the GitHub PR and Notifications overlay buttons from the toolbar header. Settings stays.

### 2. Add to Right Sidebar Icon Bar
Add two new module icons to the right sidebar icon bar:
- **GitPullRequest** icon → PRs module
- **Bell** icon → Notifications module

With notification badges showing counts (same as current toolbar badges).

### 3. Notifications — Mark All as Read
Add a "Mark all as read" button next to the refresh button in the notifications header. Calls `MarkNotificationRead` for each unread notification.

### 4. PR Detail View — Full-Screen
When clicking a PR card, open a full-screen overlay (reuse OverlayPage pattern) with:

**Left panel (detail):**
- PR title + number
- Author + branch (head → base)
- Review status badge
- PR description (rendered markdown or plain text)
- Labels
- CI status checks (pass/fail/pending)
- +additions / -deletions stats
- Created/updated timestamps
- Action buttons: Approve, Merge, View in browser

**Right column (PR list):**
- Scrollable list of all open PRs
- Active PR highlighted
- Click to switch detail view

**File changes section:**
- List of changed files with +/- counts
- Click to open in CodeViewer (reuse existing diff viewer)

### 5. Enhanced PR Data from gh CLI
Expand `FetchPRs` to include:
```
body, additions, deletions, changedFiles, labels, headRefName,
baseRefName, createdAt, updatedAt, mergeable, isDraft, statusCheckRollup
```

### Background
The image background (mesh gradient orbs) should be visible behind the sidebar modules since they use glass transparency. If not showing, the glass-panel CSS may need the backdrop-filter to be properly layered.

## Modified Files
- `Toolbar.tsx` — remove GitHub + Notifications buttons
- `RightSidebar.tsx` — add PR and Notifications modules to icon bar
- `GitHubPanel.tsx` — adapt for sidebar module layout
- `NotificationsPanel.tsx` — add mark-all-as-read, adapt for sidebar
- `App.tsx` — remove overlay wrappers for GitHub/Notifications, pass data to RightSidebar
- `useOverlay.ts` — remove "github" and "notifications" from OverlayModule type
- `github_service.go` — fetch expanded PR fields
- `types.go` — expand GitHubPR type with new fields
- New: `PRDetailOverlay.tsx` — full-screen PR detail view with file list
