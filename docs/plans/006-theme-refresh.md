# Plan 006: Theme Refresh + Gradient Session Tabs

**Status:** Completed

## Context

Refreshed Koko's dark theme using a 5-color palette (`#C2E3EA`, `#747374`, `#6C3449`, `#1F3860`, `#72869C`) and added gradient-styled session tabs. The design direction targets a modern dark dashboard aesthetic: dark card surfaces with subtle elevation, clean spacing, and minimal borders.

## Color Mapping

| Hex | Name | OKLCH | Theme role |
|-----|------|-------|------------|
| — | Near-black | `oklch(0.10 0.02 260)` | `base` — app background (darkest) |
| — | Almost-black navy | `oklch(0.13 0.03 260)` | `titlebar` — title bar |
| `#1F3860` | Deep navy | `oklch(0.22 0.05 260)` | `surface` — terminal bg, cards |
| — | Mid navy | `oklch(0.25 0.04 260)` | `panel` — panel dock bg |
| `#C2E3EA` | Icy blue | `oklch(0.89 0.03 210)` | `foreground` — primary text |
| `#72869C` | Steel blue | `oklch(0.58 0.04 250)` | `muted-foreground` — secondary text |
| — | Dim steel | `oklch(0.35 0.03 250)` | `border` |
| `#6C3449` | Dark plum | `oklch(0.45 0.14 350)` | `accent` — active elements |
| — | Plum tint | `oklch(0.30 0.08 350)` | `tab-active` — gradient start |
| — | Light plum | `oklch(0.35 0.06 350)` | `tab-hover` |
| `#747374` | Neutral gray | — | Reference only (informed elevation levels) |

## Elevation Order (dark → light)

1. `base` — near-black (app background, CSS gradient)
2. `titlebar` — almost-black navy
3. `panel` — dark navy (panel dock)
4. `surface` — navy #1F3860 (terminals, cards, overlays)

## Changes Made

### `frontend/src/globals.css` — Theme tokens
Replaced all OKLCH values with the new palette. Added `tab-active` and `tab-hover` plum-tinted tokens.

### `frontend/src/App.tsx` — Root layout
- Replaced `bg-base` with CSS gradient background: `linear-gradient(180deg, oklch(0.10 0.02 260), oklch(0.18 0.04 260))`
- Removed `border border-border rounded-xl overflow-hidden` (was clipping macOS traffic lights)
- Added `p-2` padding around terminal area for breathing room

### `frontend/src/components/SessionTabBar.tsx` — Gradient tabs
- Active tab: `linear-gradient(135deg, oklch(0.40 0.12 350), oklch(0.28 0.06 260))` — plum→navy diagonal
- Flat tabs: removed `rounded-lg` bevels
- Centered text: `justify-center` + `min-w-32 px-8`
- Close button: absolutely positioned (`absolute right-1.5`) so text stays centered
- Taller tabs: `h-8` for generous internal space
- Inactive: transparent with steel blue text
- Hover: `bg-tab-hover` plum tint

### `frontend/src/components/TerminalPane.tsx` — xterm.js theme
- `background`: `#1F3860` (navy, matches surface)
- `foreground`: `#C2E3EA` (icy blue)
- `cursor`: `#8A4460` (brighter plum)
- `selectionBackground`: `#72869C40` (steel blue + alpha)
- Updated ANSI colors: `black`→navy, `white`→icy blue, `brightBlack`→steel blue

### `main.go` — Wails config
- `BackgroundColour`: `{R: 14, G: 16, B: 24}` (near-black, prevents startup flash)
- `Frameless: false` — **critical fix**: `Frameless: true` hides macOS traffic lights even with `TitleBarHiddenInset()`. Setting to `false` and relying on `TitleBarHiddenInset()` alone gives frameless look with traffic lights.

### `frontend/src/components/TitleBar.tsx` — Clean title bar
- Removed `border-b border-border` — relies on bg-titlebar color difference instead

### `frontend/src/components/PanelDock.tsx` — Panel dock spacing
- Removed `border-l border-border` — relies on bg-panel color difference
- Increased padding: `p-2` → `p-3`
- Increased gap: `space-y-2` → `space-y-3`

### `frontend/src/components/GitHubPanel.tsx`, `SlackPanel.tsx`, `SummaryPanel.tsx` — Panel cards
- Removed `rounded-lg border border-border` — uses bg-surface elevation on bg-panel
- Softened internal dividers: `border-border` → `border-border/30` (30% opacity)

## Key Learnings

1. **`Frameless: true` vs `TitleBarHiddenInset()`**: On macOS, `Frameless: true` completely removes native window chrome including traffic lights. `TitleBarHiddenInset()` alone provides the frameless look while preserving traffic lights.

2. **`rounded-xl overflow-hidden` clips traffic lights**: macOS renders traffic lights in the native layer. CSS `overflow-hidden` with border-radius on the root container clips them.

3. **Wails hot reload**: Vite picks up CSS/TSX changes, but Tailwind class changes can be unreliable. Go changes always require full restart (`pkill -f "wails dev"` then `~/go/bin/wails dev`).

4. **Tab text centering**: With flex items, `justify-center` only works if the container has extra space. Close buttons must be absolutely positioned to avoid pushing text off-center. `min-w` ensures the container is wider than content.

## Tooling Updates

### `.claude/commands/design.md`
Rewrote the `/design` command for the Wails era. Now spawns a subagent that screenshots the running app, reads all component source, analyzes against the design system, and returns structured change proposals.

### Design memory (`memory/design.md`)
Updated from old Bubble Tea TUI layout to current Wails architecture with full color palette, component reference, and styling conventions.
