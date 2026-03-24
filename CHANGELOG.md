# Changelog

All notable changes to Koko are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- **Remote API** — HTTP/WebSocket API server on localhost:19876 with Bearer token auth
- **MCP Server** — `koko mcp` subcommand for Claude Code integration (7 tools: list_sessions, get_session_state, read_output, send_input, create_session, close_session, list_file_changes)
- **Slack Bot** — DM the bot to control sessions (`sessions`, `status`, `send`, `output`, `files`, `help`). Owner-only access via configured Slack member ID
- **CLI Companion** — `koko-cli` binary with `sessions`, `status`, `send`, `output`, `tail` (WebSocket streaming), `files`
- **API Settings** — Settings panel shows API toggle, port, and API key with copy button
- **Terminal subscriber fan-out** — WebSocket and external consumers receive real-time PTY output
- **Go test suite** — 44 tests covering API server, auth middleware, MCP protocol, Slack commands, config persistence, ring buffer, and subscriber mechanism
- **Bot setup guide** — `docs/references/slack-bot-setup.md` with step-by-step Slack app creation

### Changed
- **Slack integration** — Replaced user token awareness panel (DMs, @mentions sidebar) with dedicated bot token command handler. Scopes reduced from 5 to 3 (`im:history`, `im:read`, `chat:write`)
- **Settings panel** — Slack section now shows bot token input + owner ID field instead of user token + test connection

### Removed
- **Slack awareness panel** — `slack_service.go`, `SlackPanel.tsx`, `useSlack.ts` removed (~400 lines)
- **Slack toolbar badge** — No longer shows unread DM count

## [0.1.0] - 2026-03-19

### Added
- **Code Viewer** — Click file in right sidebar to view GitHub-style split/unified diff with Shiki syntax highlighting
- **Copy enhancements** — `Cmd+C` copies trimmed plain text + HTML, `Cmd+Shift+C` copies as Markdown, right-click context menu
- **Session persistence race fix** — Save effect waits for GetSessions to resolve before writing

### Changed
- **xterm.js v6** — Upgraded from v5.5.0 to v6.0.0, fixing viewport scroll displacement on Enter key. Removed scroll pinning workaround.

## [0.0.9] - 2026-03-11

### Added
- **GitHub notifications** — Real notifications via `gh api`, participating/all filter, mark-as-read with optimistic updates
- **PR action buttons** — Approve, merge (squash + delete branch), view on hover
- **Claude mode switcher** — Ask/Auto-edit/Plan buttons below terminal
- **Subagent monitor** — Process tree monitoring for Claude subagents in right sidebar

## [0.0.8] - 2026-03-09

### Added
- **Slack integration** — DMs + mentions via user token, deep linking to Slack app, 60s polling
- **Quick terminal** — `Cmd+`` ` slide-up zsh panel with ring buffer replay
- **Session persistence** — Go backend file storage, `claude --continue` for reconnecting
- **File changes module** — Right sidebar shows git diff with staged/unstaged indicators

## [0.0.7] - 2026-03-08

### Added
- **Glassmorphism restyle** — Glass elevation tiers, mesh gradient orbs, breathing animation
- **Claude sessions** — Named sessions with directory picker, inline rename, recent dirs

## [0.0.6] - 2026-03-02

### Added
- **Pre-commit hooks** — lefthook with Biome lint + typecheck, conventional commit enforcement
- **CI** — GitHub Actions for Go lint/test/build + frontend biome/tsc
- **Design reviewer agent** — `/design` command with Playwright + screencapture hybrid

## [0.0.5] - 2026-03-01

### Added
- **IDE layout** — Resizable 3-panel layout with session sidebar, terminal, right sidebar with icon bar
- **GitHub PRs** — Live PR list with review status badges

## [0.0.4] - 2026-02-28

### Changed
- **Migrated to Wails v2** — From Bubble Tea TUI to Go + React desktop app

## [0.0.1] - 2026-02-27

### Added
- Initial Bubble Tea TUI prototype with terminal and panel dock
