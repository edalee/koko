# Changelog

All notable changes to Koko are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- **Conversation picker**: The session dialog lists the conversations Claude has stored for the chosen directory, newest first, with title and last reply. Pick one to reopen it, or start fresh. A conversation open in another tab switches to that tab instead (plan 028)
- **Search Web**: Right-click selected terminal text to search it in the default browser
- **Conversation ownership guard**: A conversation can be open in only one session. Resuming one another session holds is refused, including through the API and MCP (ADR-028)
- **Stable slugs**: A session keeps its slug across restarts, so `koko-1` in the CLI, MCP and Slack always names the same session
- **Reload session**: Restart a session's Claude process in the same tab, keeping its slug and conversation. On the sidebar row and in the terminal's right-click menu. Asks first if Claude is still working (plan 028)
- **Remote API** — HTTP/WebSocket API server on localhost:19876 with Bearer token auth
- **MCP Server** — `koko mcp` subcommand, 8 tools including `interact` (send+receive in one call with output settle detection)
- **Slack Bot** — DM the bot to control sessions. Owner-only access via Slack member ID
- **CLI Companion** — `koko-cli` binary with `sessions`, `status`, `send`, `output`, `tail` (WebSocket streaming), `files`
- **Session identity** — Directory-scoped slugs (koko-1, drumstick-2) replace transient session-N IDs. Slugs work across all clients (API, MCP, Slack, CLI)
- **Claude UUID recovery** — Captures Claude Code session UUID from JSONL files. `claude --resume <uuid>` for precise reconnects instead of `--continue`
- **Session context card** — Disconnected sessions show glass card with last assistant message, directory path, and "Click to reconnect" over semi-transparent mesh gradient
- **Grouped sidebar** — Sessions grouped under directory headers when multiple sessions share a directory, flat otherwise. Collapsible with status dots and count badges
- **Atomic session writes** — Write-ahead with .bak backup for crash recovery
- **`/interact` endpoint** — Send text and block until output settles (configurable quiet_ms/timeout_ms). Returns ANSI-stripped response
- **PermissionRequest hook** — HTTP callback from Claude Code for deterministic approval detection. Replaces fragile terminal pattern matching
- **Go test suite** — 44+ tests covering API, auth, MCP, Slack commands, config, terminal manager
- **CHANGELOG.md** — Version history

### Changed
- **Recent Sessions**: Choosing a closed session now reopens its conversation. It used to start a fresh session and drop it
- **Reconnecting a tab**: Clicking a disconnected tab opens the session dialog with its conversation preselected, so Enter restores it. It used to reconnect silently. A tab with no stored conversation preselects nothing, rather than guessing the newest
- **Worktrees module**: Opening a worktree goes through the session dialog, so a conversation stored for it can be reopened
- **No more `--continue`**: Resuming now needs an explicit conversation id. `--continue` resumed the newest conversation in a directory, which was often another tab's. The API and MCP `resume` flag carries no id, so it now starts a fresh conversation (plan 028 step 8)
- **Conversation id capture**: Starts when a line is submitted and is announced by event, instead of one guess five seconds after launch. Refuses rather than guesses when two sessions share a directory
- **Slack integration** — Replaced user token awareness panel with dedicated bot token command handler. Scopes reduced from 5 to 3
- **Approval detection** — PermissionRequest hook replaces terminal output pattern matching (no more false positive amber icons)
- **Session history** — Expanded to 50 entries, not deduplicated by directory. Multiple sessions per directory preserved
- **tailText buffer** — Increased from 2KB to 32KB for richer API output
- **lastMsg extraction** — Handles current Claude Code JSONL format (type:"assistant") in addition to legacy (type:"progress")

### Fixed
- **Cropped terminals**: A tab that was in the background during a resize kept its old width for good
- **Empty last-message previews**: Previews read a fixed 64KB from the end of the file, which often began inside one large record and missed the message before it
- **Wrong project folder**: Paths containing any non-alphanumeric character mapped to a Claude project folder that did not exist, so no conversation id was ever captured
- **Worktree fields reset**: The new session dialog regenerated its worktree branch name about once a second, overwriting anything typed
- **Close raced the reader**: Closing a live session could call `cmd.Wait` twice at once
- **Unicode paste** — `btoa()` crashes on chars > U+00FF (em dashes, smart quotes). Now uses TextEncoder for UTF-8 safe base64
- **Ghost amber icons** — False positive approval detection from broad pattern matching against 32KB buffer
- **Slug URL collision** — Changed separator from `/` to `-` (koko/1 → koko-1) to avoid API route conflicts

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
