# Koko (Tui) - Claude Instructions

## Project
A desktop application that serves as a unified workspace. Primary use: running Claude Code sessions with integrated GitHub and git awareness panels.

**Repo:** `github.com/edalee/koko`
**Stack:** Wails v2 (Go backend + React frontend)
**Target OS:** macOS, Linux

## Architecture
- **Wails v2 desktop shell** — Go backend + embedded webview
- **Frameless window** — `Frameless: true`, custom traffic lights via `--wails-draggable`, 8px bevel
- **Terminal sessions** — xterm.js v6 + WebGL + SearchAddon, one per tab, connected to PTY via Wails events
- **Right sidebar**: 4 modules, which are file changes (+ CI runs), session context, worktrees and notifications. PRs open in an overlay
- **Left sidebar** — session list grouped by directory, collapsible
- **Remote API** — HTTP/WebSocket on localhost:19876 with Bearer auth
- **MCP server** — JSON-RPC 2.0 over stdio, 8 tools (launched via `koko mcp`)
- **CLI companion** — `koko-cli` (cmd/koko-cli/)
- **koko-worker**: scheduled work jobs (stand-up, focus time, PR reviews by the review worker) in `cmd/koko-worker/`. It is its own Go module and binary, run by a launchd agent (`com.koko.worker`), so jobs run with Koko closed. The app controls it only through `worker.json` and the binary's subcommands. macOS only.

## Key Dependencies
**Go:**
- `github.com/wailsapp/wails/v2` — Desktop app framework
- `github.com/creack/pty` — PTY for terminal sessions
- `github.com/gorilla/websocket` — WebSocket for API server

**Frontend:**
- `react@19`, `react-dom@19` — UI framework
- `@xterm/xterm` v6 + addons (fit, webgl, web-links, search) — Terminal emulator
- `@git-diff-view/react` + `@git-diff-view/shiki` — Code diff viewer
- `tailwindcss@4` — Styling with glassmorphism dark theme
- `lucide-react` — Icons
- `react-markdown` + `rehype-raw` — PR description + comment rendering

## Conventions
- Follow Go standard project layout
- Use conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
- Keep PRs small and focused
- `make` targets: `dev`, `build`, `test`, `lint`, `install-fe`, `build-worker`, `test-worker`

## Project Structure
- `main.go` — Wails entry point + MCP subcommand detection
- `app.go` — App lifecycle, API server start/stop, MCP registration
- `terminal_manager.go` — PTY sessions, slugs, UUID capture, subscriber fan-out, --resume
- `api_server.go` — HTTP/WS API, /interact, PermissionRequest hook
- `mcp_server.go` + `mcp_tools.go` — MCP server, 8 tools
- `github_service.go` — PR fetching, file diffs, reviews, commits, comments, CI status (Wails-bound)
- `git_service.go` — Branch name, file changes, file diffs, repo slug detection
- `process_monitor.go`: child process tree scan for the subagent monitor
- `config_service.go` — Atomic writes, API key, hidden PRs
- `claude_service.go` — Last message extraction, listing a directory's stored conversations, and the file helpers for deleting them
- `conversation_delete.go`: `App.DeleteConversation` and `DeleteConversations`, with the ownership checks (plan 028 step 7a)
- `slack_commands.go` — Slack bot DM command handler
- `types.go` — Shared Go types with JSON tags
- `worker_service.go`: Wails-bound bridge to koko-worker. It edits `worker.json` and calls the binary. Methods pass JSON strings, so `models.ts` stays unchanged.
- `cmd/koko-cli/` — CLI companion
- `cmd/koko-worker/`: koko-worker, a nested Go module. `main.go` (subcommands), `scheduler.go`, `jobs.go`, `claude.go` (`claude -p` runner), `calendar.go`, `github.go`, `jira.go`, `slack.go`, `launchd.go`, `check.go`, `config.go`, `reviewer.go` (the managed reviewer clone)
- `frontend/src/` — React app
  - `components/` — Toolbar, SessionSidebar, RightSidebar, TerminalPane, PRDetailOverlay, CodeViewer, ClaudeModeSwitcher, QuickTerminal, SafeWorkingOverlay, SettingsPanel, SessionDialog, ConversationPicker, WorktreesModule, WorktreeRemovalDialog, etc.
  - `hooks/` — useSessionTabs, useGitHub, useCodeViewer, useNotifications, useSessionActivity, useCI, etc.
  - `components/WorkerSettings.tsx`: the Worker tab in `SettingsPanel.tsx` (on/off, job times, Test and Run now, checks, log)
  - `globals.css` — Glassmorphism dark theme + Tailwind v4
- `build/` — Build assets (Info.plist, app icon)
- `docs/plans/` — Implementation plans (001-029)

## Go Backend Pattern
- Structs bound to Wails via `Bind: []interface{}{...}` in main.go
- Bound structs: `App`, `TerminalManager`, `GitHubService`, `GitService`, `ConfigService`, `ProcessMonitor`, `ClaudeService`, `WorkerService`
- Exported methods on bound structs become callable from frontend
- PTY output sent via `runtime.EventsEmit()` → frontend listens with `EventsOn()`
- Base64 encoding for binary PTY data over Wails IPC

## Frontend Pattern
- TerminalPane: self-contained xterm.js component, one per session
- Hidden (not unmounted) when tab is inactive to preserve scrollback
- Right sidebar: icon bar (collapsed) + module content (expanded)
- PR detail overlay: full-screen with detail panel + PR list sidebar

## Design System
- **Theme**: Glassmorphism dark navy in `frontend/src/globals.css`
- **Base**: `#0f1117` with mesh gradient orbs (mint, blue, teal) + noise texture
- **Glass tiers**: panel (blur 20px) → card (blur 12px) → overlay (blur 40px)
- **Accent**: `#1FF2AB` (mint green) — single accent color
- **Text hierarchy**: opacity-based (92% → 55% → 35%)
- **Borders**: `rgba(255,255,255,0.08)` — ultra-thin
- **Semantic tokens**: `success` (green), `warning` (amber), `error` (red), `merge` (#a78bfa purple) — use these instead of hardcoded Tailwind colors
- **Design memory**: `.claude/agent-memory/design-reviewer/MEMORY.md`
- **`/design` command**: screenshots running app + analyzes against design system

## Design Docs
- Implementation plans in `docs/plans/` (001-029)
- Session memory in Claude memory files
- When a plan is approved, always save it to `docs/plans/` as the first step before any implementation
