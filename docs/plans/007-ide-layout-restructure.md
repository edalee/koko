# Plan: IDE Layout Restructure

**Status:** Proposed

## Context

The user designed a new IDE-style layout for Koko in Figma and exported reference source code + a screenshot to `docs/references/`. The current layout has a simple title bar + horizontal split (terminal | panel dock). The new design is a full IDE-style workspace with resizable panels, a left session sidebar, and a right icon-bar sidebar.

**Reference files:** `docs/references/Screenshot 2026-03-01 at 14.44.34.png` + `docs/references/src/app/`

## Current vs Target Layout

**Current:**
```
┌─────────────────────────────────────┐
│ TitleBar (KOKO + tabs + icons)      │
├─────────────────────┬───────────────┤
│                     │ PanelDock     │
│  Terminal (full)    │ (GitHub,      │
│                     │  Slack,       │
│                     │  Summary)     │
└─────────────────────┴───────────────┘
```

**Target:**
```
┌─────────────────────────────────────┐
│ Toolbar (plum bg, KOKO, File/Search/Settings, sidebar toggle) │
├─────────────────────────────────────┤
│ SessionTabs (open session tabs)     │
├──────┬─────────────────────┬────────┤
│      │                     │ Icon   │
│ Sess │  Terminal            │ bar + │
│ Side │  (xterm.js, main)   │ Right  │
│ bar  │                     │ Side   │
│      │                     │ bar    │
└──────┴─────────────────────┴────────┘
```

## Key Decisions

1. **Terminal stays in main area** — The reference puts a code editor/chat in the top 70% and a mock terminal in the bottom 30%. Since Koko is terminal-first, the xterm.js terminal fills the entire main content area. The bottom CollapsibleTerminal panel is **deferred** — not needed until we add chat/editor features.

2. **Left sidebar is new** — SessionSidebar with "New Session" button, search, and session cards. Uses `react-resizable-panels` for drag-to-resize.

3. **Right sidebar replaces PanelDock** — Icon bar (Explorer/GitHub/Slack/Email icons) + content panels. Toggleable via toolbar button. Replaces the always-visible PanelDock + @dnd-kit drag reorder.

4. **Color scheme shifts** — From navy OKLCH palette to VS Code dark (`#1e1e1e` base, `#252526` surface) with mint green accents (`#1FF2AB`/`#24A965`) and plum toolbar (`#4A1A33`).

5. **macOS traffic lights** — Keep `Frameless: false` + `TitleBarHiddenInset()`. The toolbar replaces TitleBar but still needs the 72px left spacer for traffic lights.

## New Dependency

**`react-resizable-panels`** — Required by the resizable layout. Used by shadcn/ui's `ResizablePanel`.

```bash
cd frontend && npm install react-resizable-panels
```

## Files to Create

### 1. `frontend/src/components/ui/resizable.tsx`
Thin wrappers around `react-resizable-panels` (from reference `docs/references/src/app/components/ui/resizable.tsx`). Exports `ResizablePanelGroup`, `ResizablePanel`, `ResizableHandle`.

### 2. `frontend/src/components/Toolbar.tsx`
Replaces `TitleBar.tsx`. Plum background (`bg-[#4A1A33]`), KOKO branding with `Code2` icon, File/Search/Settings buttons, sidebar toggle button. Keeps 72px left spacer for macOS traffic lights and `WebkitAppRegion: "drag"`.

### 3. `frontend/src/components/SessionSidebar.tsx`
Left resizable panel. Contains:
- "New Session" button with mint gradient (`from-[#1FF2AB] to-[#24A965]`)
- Search input
- Session cards (active session highlighted with mint gradient bg)
- Session count in collapsible header

Props: `sessions`, `activeSession`, `onSessionSelect`, `onNewSession`, `onDeleteSession`

### 4. `frontend/src/components/RightSidebar.tsx`
Replaces `PanelDock.tsx`. Contains:
- Icon bar (48px wide, `bg-[#252526]`) with module buttons: GitHub, Slack, (later: Explorer, Email)
- Content area showing the active module's panel
- Close button at bottom of icon bar

Modules reuse existing GitHub/Slack panel content but adapted for the new container (no accordion headers, just inline content).

## Files to Modify

### 5. `frontend/src/App.tsx` — Complete layout rewrite
- Remove gradient background, use flat `bg-[#1e1e1e]`
- Structure: `Toolbar` → `SessionTabs` → horizontal `ResizablePanelGroup` (sidebar 15% | main 85%)
- Main area: terminal fills the entire panel (no vertical split for now)
- Right sidebar: conditional 400px panel, toggled via toolbar button
- Remove `TerminalSquare` import and terminal header card
- Remove `PanelDock` and `usePanelState` imports

### 6. `frontend/src/components/SessionTabBar.tsx` → Restyle as `SessionTabs.tsx`
- Rename file to `SessionTabs.tsx`
- Match reference: `bg-black/20` header, `border-white/10` dividers
- Active tab: mint gradient background (`from-[#1FF2AB]/30 to-[#24A965]/30`)
- Close button on tabs
- Collapsible with chevron + session count badge

### 7. `frontend/src/globals.css` — New color tokens
Replace OKLCH palette with VS Code dark + mint accents:
```css
@theme {
  --color-base:              #1e1e1e;
  --color-surface:           #252526;
  --color-toolbar:           #4A1A33;
  --color-foreground:        #cccccc;
  --color-muted-foreground:  #888888;
  --color-border:            rgba(255, 255, 255, 0.1);
  --color-accent:            #1FF2AB;
  --color-accent-dark:       #24A965;
  --color-success:           oklch(0.65 0.15 155);
  --color-warning:           oklch(0.75 0.15 80);
  --color-error:             oklch(0.65 0.20 25);
}
```

### 8. `frontend/src/components/TerminalPane.tsx` — Update theme colors
- `background`: `#1e1e1e` (matches base)
- `foreground`: `#cccccc`
- `cursor`: `#1FF2AB` (mint accent)
- `selectionBackground`: `rgba(255, 255, 255, 0.1)`

### 9. `frontend/src/components/GitHubPanel.tsx` — Adapt for right sidebar
- Remove accordion expand/collapse (always expanded in sidebar)
- Remove `ChevronRight` toggle
- Style with `bg-white/5` cards and `border-white/10` borders (matching reference)
- Keep existing `useGitHub` hook and PR data

### 10. `frontend/src/components/SlackPanel.tsx` — Adapt for right sidebar
- Same treatment as GitHubPanel — remove accordion, match reference card style

### 11. `main.go` — Update startup background
- `BackgroundColour`: `{R: 30, G: 30, B: 30, A: 1}` (matches `#1e1e1e`)

### 12. `frontend/src/hooks/useSessionTabs.ts` — No changes needed
Existing hook provides all the state management needed.

### 13. `frontend/src/types/index.ts` — Keep as-is
`SessionTab` and `GitHubPR` types are sufficient.

## Files to Remove

### `frontend/src/components/TitleBar.tsx`
Replaced by `Toolbar.tsx`.

### `frontend/src/components/SummaryPanel.tsx`
Subsumed into RightSidebar modules. Can be added back later as a module.

### `frontend/src/hooks/usePanelState.ts`
No longer needed — PanelDock is gone, right sidebar uses simple `useState`.

## Implementation Order

1. **Install dependency**: `npm install react-resizable-panels`
2. **Create `ui/resizable.tsx`** — foundation for all resizable panels
3. **Create `Toolbar.tsx`** — new top bar (replaces TitleBar)
4. **Create `SessionSidebar.tsx`** — left panel
5. **Restyle `SessionTabBar.tsx` → `SessionTabs.tsx`** — tab bar below toolbar
6. **Create `RightSidebar.tsx`** — right panel with icon bar + GitHub/Slack content
7. **Adapt `GitHubPanel.tsx` + `SlackPanel.tsx`** — simpler card style for sidebar
8. **Rewrite `App.tsx`** — wire everything together with ResizablePanelGroup
9. **Update `globals.css`** — new color tokens
10. **Update `TerminalPane.tsx`** — terminal theme colors
11. **Update `main.go`** — startup background color
12. **Clean up** — remove TitleBar.tsx, SummaryPanel.tsx, usePanelState.ts, PanelDock.tsx

## Verification

1. `make dev` — app opens with dark `#1e1e1e` background
2. Plum toolbar visible at top with KOKO branding and traffic lights
3. Session tabs row below toolbar showing active sessions
4. Left sidebar shows session list, resizable by dragging the handle
5. "New Session" button has mint green gradient
6. Terminal fills the main content area with proper xterm.js rendering
7. Right sidebar toggles open/closed via toolbar button
8. Right sidebar icon bar shows GitHub/Slack module tabs
9. GitHub PRs display in right sidebar when GitHub module selected
10. Resizing panels works smoothly without layout jumps
11. Terminal refits correctly when panels resize
