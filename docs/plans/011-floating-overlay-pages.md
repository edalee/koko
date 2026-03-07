# Plan 011: Floating Overlay Pages with Toolbar Notification Badges

## Context

The right sidebar panel approach (icon bar + sliding content) feels like a traditional IDE layout. The user wants a more modern, stylish UI where status icons live in the toolbar with colorful notification badges, and clicking them reveals floating glassmorphism overlay pages with smooth animations. This also adds a new Mail module (mock data).

## Architecture Change

```
Before:  [Toolbar: KOKO ... Settings]
         [SessionSidebar | Terminal | RightSidebar(Explorer/GitHub/Slack icons+panel)]

After:   [Toolbar: KOKO ... 🔀(3) 💬(6) 📧(2) ⚙ Settings]
         [SessionSidebar | Terminal | RightSidebar(file changes / subagent panels)]
                    [ Floating Overlay (glassmorphism, centered) ]
```

3-panel layout stays, but the right sidebar is repurposed for **file changes and subagent panels** (not GitHub/Slack/Mail). The notification modules (GitHub PRs, Slack DMs, Mail) move to floating overlays triggered from toolbar icons.

## Files Created

### `frontend/src/components/OverlayPage.tsx`
Animated glassmorphism container:
- **Backdrop**: `fixed`, `bg-black/40`, `backdrop-blur-sm`, fade-in
- **Card**: centered, `w-[520px] max-h-[70vh]`, glass background, `backdrop-blur-2xl`, `rounded-2xl`, `border border-white/[0.08]`, `shadow-2xl`
- **Header**: icon + title + close (X) button
- **Content**: scrollable area for module content
- **Animation**: enter = scale(0.95)+translateY(8px)+opacity→1, exit = reverse. Spring-like easing.
- **Close**: click backdrop, press Escape, or click X
- **State machine**: `closed` → `open` → `closing` → `closed` (exit animation completes before unmount)

### `frontend/src/components/NotificationBadge.tsx`
Small badge overlay for toolbar icons:
- Absolute positioned (`-top-1 -right-1`), `min-w-[18px] h-[18px] rounded-full`
- Per-module colors: GitHub=accent, Slack=`#E01E5A`, Mail=`#569CD6`
- Pop animation on mount (`scale(0)→scale(1.2)→scale(1)`)
- Hidden when count is 0

### `frontend/src/components/MailPanel.tsx`
Mock mail module (mirrors SlackPanel pattern):
- Mock data: GitHub notifications, Jira assignments, deploy alerts
- Card design: Mail icon, sender, subject, time, unread dot indicator

### `frontend/src/hooks/useOverlay.ts`
Overlay state management:
- `activeOverlay: "github" | "slack" | "mail" | null`
- `toggleOverlay(module)` — opens or closes
- Escape key listener to close

## Files Modified

### `frontend/src/App.tsx`
- Keep 3-panel layout (SessionSidebar | Terminal | RightSidebar)
- Lift `useGitHub()` to App level (toolbar needs PR count)
- Add `useOverlay()` hook
- Render 3 `<OverlayPage>` components (GitHub, Slack, Mail) as floating layers over the panel group
- Pass toggle callbacks + counts to Toolbar
- RightSidebar stays but with repurposed content (file changes / subagents)

### `frontend/src/components/Toolbar.tsx`
- Add 3 icon buttons between spacer and Settings: GitPullRequest, MessageSquare, Mail
- Each wrapped with NotificationBadge showing count
- Active state (overlay open): `text-accent bg-white/10`
- Default: `text-muted-foreground hover:text-white hover:bg-white/5`
- All buttons need `WebkitAppRegion: "no-drag"`

### `frontend/src/components/GitHubPanel.tsx`
- Accept `{ prs, loading, refresh }` as props (remove internal `useGitHub()`)
- Remove panel header (overlay shell provides title)
- Adjust wrapper from sidebar layout to overlay content layout

### `frontend/src/components/SlackPanel.tsx`
- Remove panel header (overlay provides it)
- Adjust layout for overlay context
- Export `useSlackCount()` for toolbar badge

### `frontend/src/components/RightSidebar.tsx`
Repurposed — remove GitHub/Slack/Explorer module switching, replace with:
- **File Changes panel**: show files modified in the current session (placeholder/mock for now)
- **Subagent panel**: show running/completed subagent tasks (placeholder/mock for now)
- Keep the icon bar but swap icons: `FileCode2` (file changes), `Bot` (subagents)
- Keep the collapsible behavior and resizable panel integration

### `frontend/src/globals.css`
Added tokens and keyframes:
- `--color-glass`, `--color-glass-border`, `--color-badge-slack`, `--color-badge-mail`
- `@keyframes overlay-in/out`, `backdrop-in/out`, `badge-pop`

## Animation Summary

| Animation | Duration | Easing | Effect |
|-----------|----------|--------|--------|
| Backdrop fade | 200ms | ease-out | opacity 0→1 |
| Card enter | 200ms | cubic-bezier(0.16,1,0.3,1) | scale+translateY+opacity |
| Card exit | 150ms | same | reverse |
| Badge pop | 300ms | spring bezier(0.34,1.56,0.64,1) | scale bounce |

## No New Dependencies

All achievable with Tailwind v4 utilities, CSS keyframes, lucide-react icons (Mail already available), and React 19.

## Verification

1. `make dev` — app starts, 3-panel layout (left sidebar + terminal + right sidebar)
2. Toolbar shows 3 notification icons with colored badges
3. Click GitHub icon → overlay slides in with glassmorphism, shows PR cards
4. Click Slack icon → GitHub overlay closes, Slack overlay opens
5. Click backdrop or press Escape → overlay closes with exit animation
6. Terminal regains focus after overlay closes
7. Badge counts display correctly, hidden when 0
8. Right sidebar shows file changes / subagent panels (placeholder content)
9. Right sidebar collapse/expand and resize still works
10. macOS traffic lights still work, drag region unaffected
11. Panel resizing still works (all 3 panels)
