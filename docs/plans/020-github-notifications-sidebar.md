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

## Remaining Work
- **Compact PR sidebar list** — replace card layout with one-line-per-PR list (title + repo#number, click → overlay). Current cards are too dense at 29 PRs.
- **Markdown rendering** — PR body is raw text. Need react-markdown or lightweight renderer for headings, lists, code blocks, bold/italic.

## Completed
- Toolbar: removed GitHub + Notifications buttons
- Right sidebar: 4 modules (files, context, PRs, notifications) with badge counts
- Notifications: mark-all-as-read (CheckCheck icon, PUT /notifications)
- PR detail overlay: full-screen with description, CI, actions, PR list column
- Expanded PR data from gh CLI (body, additions, deletions, checks, labels, etc.)
- GitHubPR type expanded in Go + TypeScript
- Background fix: disconnected overlay uses bg-base/60 with backdrop-blur
- lastMsg extraction: handles current + legacy JSONL formats

## Modified Files
- `Toolbar.tsx` — simplified to logo + settings
- `RightSidebar.tsx` — 4 modules with badge counts
- `GitHubPanel.tsx` — onPRClick callback, will become compact list
- `NotificationsPanel.tsx` — mark-all-as-read
- `App.tsx` — PRDetailOverlay wired, overlays removed from toolbar
- `PRDetailOverlay.tsx` — new full-screen PR detail
- `useOverlay.ts` — reduced to "settings" only
- `github_service.go` — expanded PR fields
- `types.go` — GitHubPR + PRCheck types
