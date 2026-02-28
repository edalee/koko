# Koko (Tui) - Claude Instructions

## Project
A desktop application that serves as a unified workspace. Primary use: running Claude Code sessions with integrated GitHub and Slack awareness panels.

**Repo:** `github.com/edalee/koko`
**Stack:** Wails v2 (Go backend + React frontend)
**Target OS:** macOS, Linux

## Architecture
- **Wails v2 desktop shell** — Go backend + embedded webview
- **Terminal sessions** — xterm.js v5 + WebGL, one per tab, connected to PTY via Wails events
- **Panel dock** (right) — draggable panels: GitHub, Slack, Summary
- **Custom title bar** — frameless window with macOS traffic lights

## Key Dependencies
**Go:**
- `github.com/wailsapp/wails/v2` — Desktop app framework
- `github.com/creack/pty` — PTY for terminal sessions

**Frontend:**
- `react@19`, `react-dom@19` — UI framework
- `@xterm/xterm` + addons (fit, webgl, web-links) — Terminal emulator
- `@dnd-kit/core`, `@dnd-kit/sortable` — Drag and drop for panels
- `tailwindcss@4` — Styling with OKLCH dark theme
- `lucide-react` — Icons

## Conventions
- Follow Go standard project layout
- Use conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
- Keep PRs small and focused
- `make` targets: `dev`, `build`, `test`, `lint`, `install-fe`

## Project Structure
- `main.go` — Wails entry point
- `app.go` — App lifecycle
- `terminal_manager.go` — PTY session management (Wails-bound)
- `github_service.go` — GitHub PR fetching (Wails-bound)
- `types.go` — Shared Go types with JSON tags
- `wails.json` — Wails config
- `frontend/src/` — React app
  - `components/` — TitleBar, SessionTabBar, TerminalPane, PanelDock, GitHubPanel, SlackPanel, SummaryPanel
  - `hooks/` — useSessionTabs, useGitHub, usePanelState
  - `globals.css` — OKLCH dark theme + Tailwind
- `build/` — Build assets (Info.plist, app icon)
- `docs/plans/` — Implementation plans

## Go Backend Pattern
- Structs bound to Wails via `Bind: []interface{}{...}` in main.go
- Exported methods on bound structs become callable from frontend
- PTY output sent via `runtime.EventsEmit()` → frontend listens with `EventsOn()`
- Base64 encoding for binary PTY data over Wails IPC

## Frontend Pattern
- TerminalPane: self-contained xterm.js component, one per session
- Hidden (not unmounted) when tab is inactive to preserve scrollback
- PanelDock: @dnd-kit SortableContext for drag-to-reorder panels
- Panel order persisted to localStorage

## Design Docs
- Implementation plans in `docs/plans/`
- Session memory in Claude memory files (`koko.md`)
- When a plan is approved, always save it to `docs/plans/` as the first step before any implementation
