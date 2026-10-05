# Changelog

All notable changes to Koko are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

## [0.5.3] - 2026-10-05

### Changed
- **Tono "My PRs"**: reviews only your PRs opened in the last 14 days, a setting in Settings > Worker. It used to review every open PR you ever opened, back to 2022. "Run now" with a PR's URL still reviews any PR
- **Archived repos**: every PR search leaves them out. An archived repo is read-only, so tono could never post there

### Fixed
- **Posting refused for good**: a comment GitHub will never accept, such as on an archived repo or a locked PR, fails at once with one DM. It used to take three tries over a day

## [0.5.2] - 2026-10-05

### Fixed
- **Reinstalling the worker**: `koko-worker install` now waits for the old agent to unload before starting the new one. With a job running, it used to fail with "Bootstrap failed: 5" and leave no agent running

## [0.5.1] - 2026-10-02

### Added
- **Tono review ping**: once a review is on the PR, one Slack line says so, with the verdict and a link to the review. The review itself never goes to Slack
- **Housekeeping in Settings > General**: "Clear history" forgets Koko's closed sessions, and leaves Claude's conversations alone. "Remove worktrees" removes worktrees Koko created for closed sessions. It never forces, and keeps any with uncommitted or ignored files, or still in use (plan 028 step 7b)

### Changed
- **Needs your review**: the stand-up lists only PRs opened by your team. PRs from outside it are counted in one line instead of listed

### Fixed
- **Deleting conversations**: a conversation that a Koko tab may be writing can no longer be deleted (plan 028 step 7a)

## [0.5.0] - 2026-10-02

### Added
- **Conversation picker**: The session dialog lists the conversations Claude has stored for the chosen directory, newest first, with title and last reply. Pick one to reopen it, or start fresh. A conversation open in another tab switches to that tab instead (plan 028)
- **Search Web**: Right-click selected terminal text to search it in the default browser
- **Conversation ownership guard**: A conversation can be open in only one session. Resuming one another session holds is refused, including through the API and MCP (ADR-028)
- **Stable slugs**: A session keeps its slug across restarts, so `koko-1` in the CLI, MCP and Slack always names the same session
- **Delete conversations**: Each row in the conversation picker has a delete button, and "Delete all conversations here" clears a directory. Both confirm first. A conversation a tab holds cannot be deleted (plan 028 step 7)
- **Reload session**: Restart a session's Claude process in the same tab, keeping its slug and conversation. On the sidebar row and in the terminal's right-click menu. Asks first if Claude is still working (plan 028)
- **koko-worker**: a separate binary (`cmd/koko-worker/`) that a launchd agent keeps running. It runs scheduled work jobs on weekdays, with Koko open or closed. macOS only
- **Stand-up job**: a 07:00 Slack DM with today's meetings, your PRs ready to merge with a Jira check, follow-up steps, PRs waiting for your review, and team PRs
- **Stories you can close**: a stand-up section listing your Jira stories, not done, that a PR merged this week or a PR ready to merge covers
- **Focus time job**: at 09:15, books "Focus" blocks in free calendar gaps of 30 minutes or more, through the Google Calendar connector
- **Tono review job**: at 09:30, 12:00 and 14:00, reviews each PR once with the tono CLI, and posts the review as comments on the PR, as you. A PR with nothing to report gets "LGTM 😃⭐😸". Nothing posts for a review that did not finish
- **Tono settings**: "My PRs" reviews every open PR you opened. "Team PRs" reviews the team's recent PRs: those that ask for your review, and every open PR in a repo list you edit in Settings
- **`koko-worker version`**: prints the version the binary was built as. The scheduler logs it at start
- **Worker scheduler**: runs jobs missed while the Mac slept, waits for the internet, and retries other failures 3 times. It books the next Mac wake with `pmset`
- **Worker settings**: a Worker tab in Settings with an on/off switch, job switches and times, "Test", "Run now", connection checks and the log (`WorkerSettings.tsx`, `worker_service.go`)
- **Make targets**: `make build-worker` and `make test-worker`. `make build` puts `koko-worker` inside `Koko.app`
- **Tracked GitHub repos in Settings**: The repos Koko watches for PRs live in config and can be edited in Settings
- **Remove a Koko-created worktree on close**: Closing a session whose worktree Koko created offers to remove it

### Changed
- **Recent Sessions**: Choosing a closed session now reopens its conversation. It used to start a fresh session and drop it
- **Reconnecting a tab**: Clicking a disconnected tab opens the session dialog with its conversation preselected, so Enter restores it. It used to reconnect silently. A tab with no stored conversation preselects nothing, rather than guessing the newest
- **Worktrees module**: Opening a worktree goes through the session dialog, so a conversation stored for it can be reopened
- **No more `--continue`**: Resuming now needs an explicit conversation id. `--continue` resumed the newest conversation in a directory, which was often another tab's. The API and MCP take `claudeSessionId` to resume, and refuse `resume` without one. `GET /api/sessions` now lists each session's `claudeSessionId` (plan 028 step 8)
- **Session ids are quoted for the shell**: A conversation id reached an `sh -c` script unquoted, so a crafted id could run commands. It is now always a single quoted argument
- **Conversation id capture**: Starts when a line is submitted and is announced by event, instead of one guess five seconds after launch. Refuses rather than guesses when two sessions share a directory
- **Settings panel**: fills the window, with a tab for each section (General, Safe working, GitHub, Worker). The panel is nearly opaque, so it is easier to read
- **Slack settings tab**: hidden. It held the old Slack DM bot, which stays idle while no token is saved

### Fixed
- **Cropped terminals**: A tab that was in the background during a resize kept its old width for good
- **Empty last-message previews**: Previews read a fixed 64KB from the end of the file, which often began inside one large record and missed the message before it
- **Wrong project folder**: Paths containing any non-alphanumeric character mapped to a Claude project folder that did not exist, so no conversation id was ever captured
- **Worktree fields reset**: The new session dialog regenerated its worktree branch name about once a second, overwriting anything typed
- **Close raced the reader**: Closing a live session could call `cmd.Wait` twice at once
- **Settings toggles**: every toggle knob now stays inside its track
- **Stand-up Jira claims**: ticket facts come from Jira itself, and each coverage claim must quote the ticket. A stand-up had credited a ticket with work that neither the ticket nor the PR named. Each ticket now links to Jira, with its status and assignee
- **All-day invites**: a timed invite from midnight to midnight showed as "00:00–00:00" in the stand-up. It now shows as "All day"
- **Release builds**: `Koko.app` from a release now holds `koko-worker`. The release workflow did not build it
- **Missing diffs**: Files committed on the branch, and untracked files, now show a diff

## [0.4.0] - 2026-05-30

### Added
- **Worktrees**: A Worktrees module lists a repo's git worktrees. A new session can create a worktree, so parallel sessions do not share a working tree
- **PR comments**: Read review threads, grouped by file with the diff hunk, and discussion comments in the PR overlay. Reply to either
- **CI status**: GitHub Actions runs for the current branch sit in the File Changes module, with a status dot next to the branch name. The section opens by itself on a failure
- **PR detail overlay**: A full-screen view with the description, author, CI checks, files, reviews and commits, next to the PR list
- **GitHub sidebar**: PRs and notifications moved to the right sidebar. PRs can be hidden and unhidden
- **Frameless window**: Custom traffic lights and an 8px bevel
- **Terminal search**: `Cmd+F` searches the scrollback
- **PR diff viewer**: Click a file in a PR to see its diff in the Code Viewer
- **Raw file viewer**: View a whole file with Shiki syntax highlighting
- **Binary and large files**: The Code Viewer detects binary files and asks before it renders a large one
- **Redraw Terminal**: A menu action that rebuilds a terminal whose WebGL glyphs are corrupt. The glyph cache also flushes by itself to reduce ghosting
- **Code Viewer** — Click file in right sidebar to view GitHub-style split/unified diff with Shiki syntax highlighting
- **Copy enhancements** — `Cmd+C` copies trimmed plain text + HTML, `Cmd+Shift+C` copies as Markdown, right-click context menu
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
- **xterm.js v6** — Upgraded from v5.5.0 to v6.0.0, fixing viewport scroll displacement on Enter key. Removed scroll pinning workaround.
- **Slack integration** — Replaced user token awareness panel with dedicated bot token command handler. Scopes reduced from 5 to 3
- **Approval detection** — PermissionRequest hook replaces terminal output pattern matching (no more false positive amber icons)
- **Session history** — Expanded to 50 entries, not deduplicated by directory. Multiple sessions per directory preserved
- **tailText buffer** — Increased from 2KB to 32KB for richer API output
- **lastMsg extraction** — Handles current Claude Code JSONL format (type:"assistant") in addition to legacy (type:"progress")
- **Keystroke writes**: `/interact` types input one character at a time, because Claude Code ignores a bulk write
- **Semantic colour tokens**: Every component uses the `success`, `warning`, `error` and `merge` tokens instead of raw Tailwind colours

### Fixed
- **Session persistence race** — Save effect waits for GetSessions to resolve before writing
- **Unicode paste** — `btoa()` crashes on chars > U+00FF (em dashes, smart quotes). Now uses TextEncoder for UTF-8 safe base64
- **Ghost amber icons** — False positive approval detection from broad pattern matching against 32KB buffer
- **Slug URL collision** — Changed separator from `/` to `-` (koko/1 → koko-1) to avoid API route conflicts
- **PR descriptions**: Raw HTML (details, summary, images) now renders
- **Terminal links**: URLs in a terminal are clickable

### Removed
- **Slack awareness panel** — `slack_service.go`, `SlackPanel.tsx`, `useSlack.ts` removed (~400 lines)
- **Slack toolbar badge** — No longer shows unread DM count

## [0.3.0] - 2026-03-15

### Changed
- **Install docs**: Document the macOS XProtect limitation and recommend building from source

## [0.2.9] - 2026-03-15

### Fixed
- **Homebrew**: Wrap `Koko.app` in a versioned directory inside the tar.gz

## [0.2.8] - 2026-03-15

### Fixed
- **Homebrew**: Switch from a Cask to a Formula, to avoid XProtect removing the binary

## [0.2.7] - 2026-03-15

### Fixed
- **Homebrew**: A pkg postinstall script clears the quarantine flag

## [0.2.6] - 2026-03-15

### Changed
- **Install docs**: Describe the pkg-based Homebrew Cask

## [0.2.5] - 2026-03-15

### Fixed
- **Homebrew**: Switch to a pkg installer, to avoid the quarantined binary

## [0.2.4] - 2026-03-15

### Fixed
- **Homebrew**: Ad-hoc codesign before zipping, so Gatekeeper keeps the binary

## [0.2.3] - 2026-03-15

### Fixed
- **Homebrew**: Package the Cask as a zip and drop codesigning

## [0.2.2] - 2026-03-15

### Added
- **Plugin skills**: The session context sidebar lists skills from plugins

## [0.2.1] - 2026-03-14

### Added
- **Edit menu**: A macOS Edit menu, so `Cmd+C` and `Cmd+V` work in the terminal
- **Homebrew tap**: The release workflow updates the tap

### Fixed
- **Scroll jumps**: The Claude mode switcher no longer shifts the layout
- **DMG path**: The DMG and Cask use `Koko.app`, to avoid unicode path problems

## [0.2.0] - 2026-03-14

### Added
- **Session context panel**: Session history and directory-based session names
- **Approval icons**: A session icon shows when Claude waits for approval
- **Quiet hours delay**: A button on the quiet hours overlay delays it by 30 minutes

### Fixed
- **Quick terminal**: One per session, and scroll position is kept

## [0.1.2] - 2026-03-12

### Fixed
- **Session switching**: Click to select a session, click again to reconnect. Per-tab quick terminal and debounced resize

## [0.1.1] - 2026-03-11

### Added
- **Shortcut hints**: The session sidebar shows keyboard shortcuts
- **README and landing page**: Screenshots and a GitHub Pages site

## [0.1.0] - 2026-03-11

### Added
- **Release workflow**: Builds a macOS DMG and a Linux AppImage
- **Update check**: The toolbar shows when a new version exists
- **Safe working**: Quiet hours with an overlay
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
