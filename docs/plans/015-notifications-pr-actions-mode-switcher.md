# Plan 015: GitHub Notifications, PR Actions, and Claude Mode Switcher

## Context

Three enhancements to improve awareness and control within Koko:
1. Replace mock MailPanel with real GitHub notifications
2. Add approve/merge/view action buttons to PR cards
3. Add Claude Code permission mode switcher below terminal

## Changes

### 1. GitHub Notifications (replaces MailPanel)

**Go backend (`github_service.go`):**
- `FetchNotifications(filter string)` — calls `gh api /notifications?per_page=50`, adds `&participating=true` when filter is "participating"
- `MarkNotificationRead(threadID string)` — calls `PATCH /notifications/threads/{id}`
- `apiToHTMLURL()` — converts GitHub API URLs to browser HTML URLs

**Go types (`types.go`):**
- `GitHubNotification` struct: ID, Title, Type, Reason, Repo, URL, Unread, UpdatedAt

**Frontend hook (`useNotifications.ts`):**
- Two-tier filtering: server-side `participating`/`all` filter, client-side sub-filter (all/review/mentioned)
- 60s polling interval
- `markRead` callback with optimistic update + post-action refresh

**Frontend component (`NotificationsPanel.tsx`):**
- Source filter tabs: Participating / All (triggers server-side refetch)
- Sub-filter tabs: All / Review / Mentioned (client-side filtering by reason)
- Glass cards with type icons (PR/Release/Issue), reason badges, unread dots, timeAgo
- Hover reveals checkmark button to mark as read without opening the link

**Wiring:**
- `useOverlay.ts`: `"mail"` → `"notifications"` in OverlayModule type
- `Toolbar.tsx`: Bell icon replaces Mail, `notifCount` prop replaces `mailCount`
- `App.tsx`: useNotifications hook wired to NotificationsPanel
- `MailPanel.tsx`: deleted

### 2. PR Action Buttons

**Go backend (`github_service.go`):**
- `ApprovePR(repo string, number int)` — calls `gh pr review --approve`
- `MergePR(repo string, number int)` — calls `gh pr merge --squash --delete-branch`

**Frontend (`GitHubPanel.tsx`):**
- Hover-revealed action bar on each PR card:
  - **Approve** (green check) — disabled if already approved, spinner while busy
  - **Merge** (purple merge icon) — squash merge with branch delete, spinner while busy
  - **View** (external link) — opens in browser
- Card no longer auto-opens URL on click — View button handles it
- Auto-refreshes PR list after approve/merge

### 3. Claude Mode Switcher

**Frontend (`ClaudeModeSwitcher.tsx`):**
- Slim bar below each terminal session
- Three mode buttons: Ask / Auto-edit / Plan
- Sends `Shift+Tab` escape sequence (`\x1b[Z`) to PTY to cycle modes
- Calculates steps needed to reach target mode from current mode
- Shows `⇧⇥ to cycle` hint
- Mode state tracked locally (not synced from Claude output)

**Wiring (`App.tsx`):**
- ClaudeModeSwitcher rendered below each TerminalPane in a flex column layout

### 4. App Icon Update

- New tui bird SVG (`build/appIcon.svg`) as source
- Generated `build/appicon.png` (1024x1024) via ImageMagick for Wails `.icns` build
- Old bird assets removed (`koko-bird-colour.png/svg`, `koko-bird-mute.png/svg`)

## Files Summary

| Action | File | What |
|--------|------|------|
| Modify | `github_service.go` | Add notifications + PR approve/merge methods |
| Modify | `types.go` | Add GitHubNotification struct |
| Delete | `frontend/src/components/MailPanel.tsx` | Replaced by NotificationsPanel |
| Create | `frontend/src/components/NotificationsPanel.tsx` | Real GitHub notifications UI |
| Create | `frontend/src/hooks/useNotifications.ts` | Notification fetching + mark-read hook |
| Modify | `frontend/src/components/GitHubPanel.tsx` | Add approve/merge/view action buttons |
| Create | `frontend/src/components/ClaudeModeSwitcher.tsx` | Permission mode switcher |
| Modify | `frontend/src/App.tsx` | Wire notifications, mode switcher |
| Modify | `frontend/src/components/Toolbar.tsx` | Bell icon, notifCount |
| Modify | `frontend/src/hooks/useOverlay.ts` | mail → notifications |
| Replace | `build/appicon.png` | New tui bird icon (1024x1024) |
| Add | `build/appIcon.svg` | Source SVG for app icon |

## Status: Completed
