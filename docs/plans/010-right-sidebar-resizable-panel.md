# Plan 010: Right Sidebar Resizable Panel + GitHub PR Links

**Status:** Completed

## Context

The right sidebar used an absolute-positioned overlay that appeared/disappeared on toggle. This plan converts it to a resizable panel (matching the left sidebar pattern) and adds clickable GitHub PR links.

## Changes

### GitHub PR Links (Go + Frontend)
- Added `URL string` field to `GitHubPR` struct in `types.go`
- Added `url` to `ghPRJSON` struct and `--json` query in `github_service.go`
- Added `url: string` to `GitHubPR` TypeScript interface
- PR cards in `GitHubPanel.tsx` open the PR URL in the system browser via `BrowserOpenURL`

### Right Sidebar as Resizable Panel
- **App.tsx**: Replaced 2-panel + absolute overlay with 3-panel layout: `[Left 15%] | [Main 66%] | [Right 19%/4%]`
- **RightSidebar.tsx**: Added `isCollapsed`/`onToggleCollapse` props; icon bar always visible, module content only when expanded; icon clicks auto-expand; chevron toggle at bottom
- **Deleted** `RightSidebarToggle.tsx` (replaced by inline chevron in RightSidebar)

### Layout
```
Collapsed:  [Left Sidebar 15%] | [Main Content ~81%] | [Icon Bar 4%]
Expanded:   [Left Sidebar 15%] | [Main Content 66%]  | [Right Sidebar 19%]
```

## Verification
1. Right sidebar starts collapsed (icon bar on right edge)
2. Click module icon → sidebar expands showing that module
3. Drag resize handle between main and right sidebar
4. Click chevron → collapses back to icon bar
5. Click a GitHub PR card → opens PR URL in system browser
6. Left sidebar collapse/expand still works independently
7. Terminal refits when panels resize
