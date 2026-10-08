# Architecture Decision Records

## ADR-001: Go + Bubble Tea as initial tech stack
- **Date:** 2026-02-27
- **Status:** Superseded by ADR-005
- **Decision:** Go + Bubble Tea for TUI
- **Outcome:** Initial scaffold worked but hit limitations for rich UI

## ADR-002: Layout with collapsible sidebar
- **Date:** 2026-02-27
- **Status:** Accepted (carried forward to Wails)
- **Decision:** Main terminal pane (left) + panel dock (right) with stacked panels
- **Rationale:** Terminal is primary — should get most screen space. Panels show at-a-glance info.

## ADR-003: Slack integration scope - counts first
- **Date:** 2026-02-27
- **Status:** Accepted
- **Decision:** Start with unread counts only (DMs, Thread Replies, Mentions). Expand later.
- **Rationale:** Keeps scope manageable. Core value is awareness without context switching.

## ADR-004: GitHub integration via gh CLI / API
- **Date:** 2026-02-27
- **Status:** Accepted
- **Decision:** Use GitHub REST API (or `gh` CLI) for PR counts and actions
- **Features:** PR counts per repo, expandable PR list, review status badges

## ADR-005: Migrate from Bubble Tea to Wails v2
- **Date:** 2026-02-28
- **Status:** Accepted
- **Decision:** Wails v2 (Go backend + React frontend) desktop app
- **Rationale:**
  - xterm.js provides a real terminal emulator (vs raw PTY in TUI)
  - React + Tailwind enables richer UI (drag & drop, animations, badges)
  - Wails IPC bridges Go backend to frontend seamlessly
  - Still single binary distribution
  - macOS native features (traffic lights, frameless window)

## ADR-006: macOS app icon with baked-in squircle mask
- **Date:** 2026-03-01
- **Status:** Accepted
- **Decision:** Bake squircle mask directly into appicon.png (80% radius, n=5 superellipse)
- **Rationale:** Wails dev builds don't get macOS auto-mask. Solid background fill prevents white corners.

## ADR-007: IDE layout restructure with resizable panels
- **Date:** 2026-03-01
- **Status:** Accepted
- **Decision:** Restructure from simple TitleBar + Terminal | PanelDock to IDE-style layout with Toolbar, SessionTabs, left SessionSidebar, main terminal, and toggleable RightSidebar with icon bar
- **Rationale:**
  - User-designed Figma reference for more structured workspace
  - Resizable panels (`react-resizable-panels`) give users control over layout
  - Icon-bar sidebar pattern scales to more modules (Explorer, Email, etc.)
  - Mint green accent gradient aligns with modern dark IDE aesthetics
  - Terminal stays primary content (no code editor/chat split yet — deferred)
- **Plan:** `docs/plans/007-ide-layout-restructure.md`

## ADR-008: Pre-commit hooks, linting, and CI
- **Date:** 2026-03-02
- **Status:** Accepted
- **Decision:** lefthook for git hooks, Biome v2 for frontend lint/format, golangci-lint v2 for Go, GitHub Actions CI
- **Rationale:**
  - lefthook: Go-based, fast, simple YAML config — better than husky (Node) or pre-commit (Python)
  - Biome: single tool replaces ESLint + Prettier, very fast
  - golangci-lint v2: standard Go linting (errcheck, govet, staticcheck, unused, ineffassign)
  - Commit-msg hook enforces conventional commits via regex
  - `make check` runs all lints + typecheck locally
- **Key configs:** `lefthook.yml`, `frontend/biome.json`, `.golangci.yml`, `.github/workflows/ci.yml`
- **Note:** Biome v2 changed config schema — `files.ignore` replaced with `files.includes`, needs `css.parser.tailwindDirectives: true` for Tailwind
- **Note:** golangci-lint v2 removed `gosimple` linter (merged into `staticcheck`)
- **Plan:** `docs/plans/008-precommit-lint-ci.md`

## ADR-009: Claude Code PostToolUse hook for Co-Authored-By stripping
- **Date:** 2026-03-02
- **Status:** Accepted
- **Decision:** PostToolUse hook on Bash tool to auto-strip Co-Authored-By lines from commits
- **Config:** `.claude/settings.local.json` hooks section + `.claude/hooks/strip-coauthored.sh`
- **Note:** Hook fires on all Bash tool uses. Config only takes effect after session restart.

## ADR-011: Claude sessions with naming and directory picker
- **Date:** 2026-03-07
- **Status:** Accepted
- **Decision:** Sessions spawn `claude` (not `zsh`) in a user-chosen directory with a custom name. New session dialog with glassmorphism overlay, native macOS directory picker, recent directories (localStorage), random name generator (adjective-noun), inline rename in sidebar.
- **Rationale:**
  - Koko is a dedicated Claude Code launcher, not a general terminal
  - Users need per-project sessions with meaningful names
  - Native directory dialog provides familiar UX
  - Recent dirs reduce friction for repeat projects
  - No auto-created session on startup — explicit creation only
- **Plan:** `docs/plans/012-claude-sessions-naming-directory.md`

## ADR-010: Design reviewer agent with Playwright + screencapture
- **Date:** 2026-03-02
- **Status:** Accepted
- **Decision:** Upgrade `/design` slash command from inline prompt to a proper agent definition with hybrid capture (native screencapture + Playwright MCP), reference design comparison, and persistent memory
- **Rationale:**
  - Inline prompt had stale component paths and no memory across sessions
  - Playwright enables interactive review (click sidebars, resize viewport, check console/network)
  - Reference design comparison (`docs/references/`) provides objective baseline
  - Agent memory tracks known issues across reviews (avoids re-reporting fixed items)
  - Severity-rated output (Critical/Major/Minor) prioritizes fixes
- **Files:** `.claude/agents/design-reviewer.md`, `.claude/agent-memory/design-reviewer/MEMORY.md`, `.claude/commands/design.md`
- **Plan:** `docs/plans/009-design-reviewer-agent.md` (not yet written)

## ADR-013: Glassmorphism restyle
- **Date:** 2026-03-08
- **Status:** Accepted
- **Decision:** Replace hex-based VS Code dark theme with glassmorphism design system — translucent glass layers over animated mesh gradient background
- **Rationale:**
  - Creates depth and visual hierarchy through glass elevation tiers (panel/toolbar/card/overlay)
  - Mesh gradient orbs with breathing animation add life without distraction
  - Opacity-based text hierarchy (92/55/35/20%) is more flexible than named gray hex values
  - SVG noise texture overlay adds tactile grain
  - Mint accent kept, plum removed entirely
- **Key changes:** CSS glass utility classes, `#root::before` mesh gradient, `#root::after` noise texture, all components updated to glass tokens
- **Plan:** `docs/plans/013-glassmorphism-restyle.md`

## ADR-014: Session persistence and resume with --continue
- **Date:** 2026-03-08
- **Status:** Superseded by ADR-036
- **Decision:** Persist session tabs to localStorage, mark as disconnected on app restart/session exit, reconnect with `claude --continue` flag
- **Rationale:**
  - Users lose context when app restarts — persistence preserves session list
  - `claude --continue` resumes the last Claude session in that directory
  - Disconnected sessions show "Click to resume" label, auto-reconnect on click
  - Keyboard shortcuts (Cmd+N/W/1-9) for power users

## ADR-015: Git file changes module in right sidebar
- **Date:** 2026-03-08
- **Status:** Accepted
- **Decision:** Right sidebar "File Changes" module shows git diff for active session's directory — staged (green) vs unstaged (orange), polls every 5s
- **Rationale:**
  - Claude Code sessions modify files — seeing changes at a glance avoids switching to terminal for `git status`
  - Staged/unstaged distinction matches VS Code mental model (green = ready, orange = working)
  - Partially staged files appear twice (once green, once orange)
  - Branch name + file count in header
- **Go backend:** `git_service.go` — `GetFileChanges(dir)`, `GetBranchName(dir)`, parses `git status --porcelain` XY columns
- **Frontend:** `useFileChanges` hook with 5s polling, `RightSidebar` renders file list

## ADR-016: Slack integration with user token and config persistence
- **Date:** 2026-03-09
- **Status:** Superseded by ADR-027
- **Decision:** Real Slack DM + @mention fetching using a user token (`xoxp-`/`xoxe.xoxp-`), stored in `~/.config/koko/config.json` with `0600` permissions. Settings overlay for token configuration with test/debug tools.
- **Rationale:**
  - User token scopes: `im:history`, `im:read`, `users:read`, `search:read`, `chat:write`
  - Slack deprecated broad `search:read` — granular `search:read.im`/`search:read.mpim` only cover DMs/group DMs (redundant with direct conversations API)
  - Channel @mentions dropped for v1 — would need Events API (requires webhook server) or `channels:history` polling
  - Config file uses restrictive permissions since it contains secrets
  - Settings overlay follows glassmorphism patterns with token show/hide toggle
  - DMs fetched via `conversations.list` + `conversations.history`
  - Deep linking: `slack://channel?team=T&id=C&message=ts` opens native Slack app
  - Frontend polls every 30 seconds via `useSlack` hook
- **Go backend:** `slack_service.go` (API client), `config_service.go` (persistence)
- **Frontend:** `SlackPanel.tsx`, `SettingsPanel.tsx`, `useSlack.ts`

## ADR-017: Quick terminal with slide-up panel
- **Date:** 2026-03-09
- **Status:** Accepted
- **Decision:** `Cmd+`` ` toggles a slide-up zsh panel (35% height) at the bottom of the main content area. Spawns `zsh -l` in the active session's directory.
- **Rationale:**
  - Users need a quick shell for git, make, etc. without leaving Koko
  - Separate from Claude sessions (zsh, not claude)
  - Ring buffer (64KB) on Go side solves prompt-not-rendering race (PTY emits before frontend mounts)
  - `ReplayBuffer()` method replays buffered output on TerminalPane mount
  - Slide-up/down CSS animations for polish
- **Files:** `QuickTerminal.tsx`, `terminal_manager.go` (`CreateShellSession`, `ReplayBuffer`, `ringBuffer`)

## ADR-018: Subagent process monitor with MCP detection
- **Date:** 2026-03-09
- **Status:** Accepted
- **Decision:** Monitor Claude's child process tree to show subagent activity and connected MCP servers in the right sidebar Agents module
- **Rationale:**
  - Claude Code subagents are child processes of the main `claude` PID
  - `cmd.Process.Pid` gives us the claude PID (exec replaces shell)
  - `pgrep -P <pid>` + `ps` reveals children: subagents, MCP servers, tools
  - Four-way classification: `claude` = subagent, `nolo`/`docker mcp`/`npx @` = mcp, `caffeinate` = infrastructure, rest = tool
  - MCP servers get friendly names extracted from command line (e.g., "nolo", "docker mcp", package name for npx-based)
  - No structured API from Claude Code yet — process monitoring is the current best approach
  - When Claude Code adds agent lifecycle events, upgrade to event-driven monitoring
- **Go backend:** `process_monitor.go` (`GetChildProcesses`), `terminal_manager.go` (`GetSessionPID`)
- **Frontend:** `useSubagents.ts` (3s polling), `RightSidebar.tsx` (subagents/MCPs/infra sections)
- **Plan:** `docs/plans/014-subagent-monitor.md`

## ADR-019: GitHub notifications replacing mock mail panel
- **Date:** 2026-03-11
- **Status:** Accepted
- **Decision:** Replace mock MailPanel with real GitHub notifications via `gh api /notifications`. Server-side `participating=true` filtering, client-side sub-filters (review/mentioned/all), mark-as-read with optimistic updates.
- **Rationale:**
  - MailPanel was mock data with no real integration
  - GitHub notifications API provides review requests, mentions, CI activity — exactly what developers need
  - Server-side `participating=true` matches github.com behavior (50 items vs 5000+ unfiltered)
  - Mark-as-read via `PATCH /notifications/threads/{id}` with optimistic UI update + refresh
  - No pagination needed — small focused lists are more useful than exhaustive ones

## ADR-020: PR action buttons (approve, merge, view)
- **Date:** 2026-03-11
- **Status:** Accepted
- **Decision:** Add hover-revealed action buttons on PR cards: Approve (`gh pr review --approve`), Merge (`gh pr merge --squash --delete-branch`), View (open in browser)
- **Rationale:**
  - Quick PR triage without leaving Koko or opening browser
  - Squash merge + delete branch is the team's default merge strategy
  - Approve disabled when already approved, spinner during async operations
  - Auto-refreshes PR list after actions complete

## ADR-021: Claude Code permission mode switcher
- **Date:** 2026-03-11
- **Status:** Accepted
- **Decision:** Slim bar below terminal with Ask/Auto-edit/Plan mode buttons. Sends `Shift+Tab` (`\x1b[Z`) escape sequence to PTY to cycle Claude Code's permission modes.
- **Rationale:**
  - Claude Code's Shift+Tab cycling is not discoverable in a desktop app context
  - Visual mode indicator makes current state visible at a glance
  - Sending escape sequences via existing PTY write path — no new backend methods needed
  - Mode state tracked locally (cannot reliably parse Claude's TUI output)
  - Caveat: manual Shift+Tab in terminal desynchs UI state — acceptable tradeoff for v1

## ADR-022: xterm.js v6 upgrade and scroll fix
- **Date:** 2026-03-18
- **Status:** Accepted
- **Decision:** Upgrade xterm.js from 5.5.0 to 6.0.0 to fix viewport scroll displacement on Enter key
- **Rationale:**
  - The viewport would scroll up when pressing Enter in Claude's TUI (alternate buffer)
  - Root cause: unknown WebKit/xterm.js interaction displacing `.xterm-viewport` scrollTop during key events
  - xterm.js 6.0.0 includes: #5390/#5411 (alt buffer scroll teleport fix), #5437 (prevent page scroll in alt buffer), #5096 (complete viewport rewrite using VS Code's scrollbar), #5127 (scroll dimensions on buffer switch)
  - Also removed `@xterm/addon-canvas` (deleted in v6 — use WebGL or DOM renderer)
  - An `onWriteParsed` + `onScroll` scroll pinning workaround was tried but blocked intentional scrolling; removed after v6 proved sufficient
- **Breaking changes handled:** canvas addon removal (not used), `windowsMode`/`fastScrollModifier` removal (not used)

## ADR-023: Terminal copy enhancements
- **Date:** 2026-03-18
- **Status:** Accepted
- **Decision:** Enhanced copy with whitespace cleanup, Markdown export, and right-click context menu
- **Rationale:**
  - xterm.js pads lines to terminal width with spaces — `Cmd+C` now trims trailing whitespace and collapses runs of 2+ spaces
  - `Cmd+Shift+C` copies selection as Markdown (converts HTML formatting via SerializeAddon)
  - `Cmd+C` also puts HTML on clipboard (via `text/html` MIME type) for rich paste into Slack/Docs
  - Right-click context menu replaces native WebKit menu (can't extend native): Copy, Copy as Markdown, Paste, Select All, Clear Terminal
  - `attachCustomKeyEventHandler` intercepts `Cmd+Shift+C` before xterm.js swallows it
- **Packages:** `@xterm/addon-serialize` for `serializeAsHTML()`

## ADR-024: Slack mentions/threads and unread scoping
- **Date:** 2026-03-18
- **Status:** Superseded by ADR-027 (updated ADR-016)
- **Decision:** Upgraded Slack from bot token DMs-only to user token with mentions, threads, and time-scoped inbox
- **Changes from ADR-016:**
  - User token (`xoxp-`) instead of bot token — sees all user's DMs
  - Added `search:read` scope for `search.messages` API (mentions + thread replies)
  - DMs scoped to last hour, only where other person spoke last (inbox model)
  - Mentions/threads fetched via `search.messages` with `<@selfID>` query
  - DMs and mentions fetched in parallel goroutines
  - Polling interval: 60s (was 30s)
  - Slack API doesn't expose unread counts for user tokens — abandoned `conversations.info` approach

## ADR-025: Code viewer with @git-diff-view/react
- **Date:** 2026-03-19
- **Status:** Accepted
- **Decision:** Click file in right sidebar → full-screen diff overlay with GitHub-style rendering
- **Rationale:**
  - `@git-diff-view/react` provides GitHub-faithful diff rendering with split/unified views
  - `@git-diff-view/shiki` adds VS Code-quality syntax highlighting (TextMate grammars)
  - Pure CSS import (`diff-view-pure.css`) avoids Tailwind conflicts
  - Custom CSS variables match Koko's dark palette (mint green additions, red deletions)
  - Go backend: `GetFileDiff(dir, path, staged)` handles staged/unstaged/new/deleted files
  - Overlay pattern reuses existing glassmorphism OverlayPage component
- **Packages:** `@git-diff-view/react`, `@git-diff-view/shiki`
- **Plan:** `docs/plans/016-code-viewer.md`

## ADR-012: TerminalPane stability — onExit ref pattern
- **Date:** 2026-03-08
- **Status:** Accepted
- **Decision:** Store `onExit` callback in a ref instead of useEffect dependency to prevent terminal re-initialization
- **Rationale:**
  - `onExit` is `() => handleSessionExit(tab.id)` — new function every render
  - Having it in `useCallback`/`useEffect` deps caused terminal dispose + re-create on every parent render
  - This killed the xterm.js renderer (`this._renderer.value.dimensions` error)
  - Ref pattern keeps terminal stable for the lifetime of `sessionId`
- **Debugging notes:**
  - `CLAUDECODE` env var must be stripped — Claude Code refuses to start inside another session
  - GUI apps on macOS have minimal PATH — use `zsh -l -c "exec claude"` to get login shell PATH
  - WebGL addon needs `onContextLoss` handler or renderer crashes silently

## ADR-026: Remote access via HTTP API, MCP, Slack commands, and CLI
- **Date:** 2026-03-24
- **Status:** Accepted
- **Decision:** Expose Koko's session management over HTTP/WebSocket API on localhost, with MCP server for Claude Code integration, Slack bot DM commands, and a standalone CLI companion binary.
- **Rationale:**
  - Koko only worked through the local webview — no way to interact from other tools/devices
  - HTTP API (`api_server.go`) on port 19876, Bearer token auth (auto-generated 32-byte hex key)
  - WebSocket streaming (`/api/sessions/:id/stream`) for real-time PTY output to external consumers
  - Subscriber fan-out in `readLoop` — non-blocking channel send to all subscribers, slow consumers drop data
  - MCP server (`koko mcp` subcommand) communicates via stdio JSON-RPC, proxies to HTTP API
  - Auto-registers in `~/.claude/settings.json` on app startup
  - Slack commands poll bot DMs every 5s, respond via `chat.postMessage` (needs `chat:write` scope)
  - CLI companion (`cmd/koko-cli/`) reads `cli.json` for API host/key, supports `tail` via WebSocket
  - API listens only on `127.0.0.1` (localhost) — not exposed to network
  - `cli.json` written by app for CLI/MCP to discover connection details
- **Performance considerations:**
  - Subscriber channels buffered to 256 messages — prevents backpressure from blocking PTY reads
  - Non-blocking fan-out (`select/default`) drops data for slow consumers rather than blocking
  - WebSocket handler has a read goroutine to detect client disconnect promptly
  - Slack polling at 5s intervals to avoid rate limits
- **Security:**
  - API: localhost-only + Bearer token auth (auto-generated 32-byte hex key)
  - Slack bot: `SlackOwnerID` config field restricts command processing to a single Slack user — messages from anyone else silently dropped. Without this, any workspace member could read session output and send input.
  - MCP: inherits API auth via `cli.json`
- **Files:** `api_server.go`, `mcp_server.go`, `mcp_tools.go`, `slack_commands.go`, `cmd/koko-cli/`
- **Tests:** `api_server_test.go`, `mcp_server_test.go`, `slack_commands_test.go`, `config_service_test.go`, `terminal_manager_test.go`
- **Plan:** `docs/plans/017-remote-api.md`

## ADR-027: Slack bot simplification — remove awareness panel
- **Date:** 2026-03-25
- **Status:** Accepted (supersedes ADR-016, ADR-024)
- **Decision:** Remove the Slack awareness panel (DMs, @mentions sidebar). Keep only the bot command handler with a dedicated bot token (`xoxb-`). Owner-only access via `SlackOwnerID` config field.
- **Rationale:**
  - Awareness panel used a user token (`xoxp-`) which made the "bot" indistinguishable from the user — every DM got treated as a command
  - Bot token gives Koko its own Slack identity — users DM the bot intentionally
  - No prefix needed (was `koko <cmd>`) since the bot's DM channel is purpose-built
  - Scopes reduced from 5 (`im:history`, `im:read`, `users:read`, `search:read`, `chat:write`) to 3 (`im:history`, `im:read`, `chat:write`)
  - `SlackOwnerID` prevents other workspace members from controlling sessions — silently drops non-owner messages
  - Removed ~400 lines: `slack_service.go`, `SlackPanel.tsx`, `useSlack.ts`
- **Setup:** `docs/references/slack-bot-setup.md`
- **Plan:** `docs/plans/018-slack-bot-simplification.md`

## ADR-028: One conversation, one owner
- **Date:** 2026-09-27
- **Status:** Accepted
- **Decision:** A Claude conversation is held by at most one Koko session. `CreateSessionWithOpts` refuses with `ErrConversationBusy` when another session holds the requested conversation. Every path goes through `uuidClaimedLocked`: an explicit `--resume` and the capture detector.
- **Rationale:**
  - Two Claude processes writing one conversation file corrupt it.
  - A disabled row in the picker is not enforcement. The API and the MCP server reach the create path too.
  - Failing to capture a conversation id is visible. Capturing the wrong one silently resumes someone else's conversation later. So capture refuses when ownership is unclear, rather than guessing.
- **Consequences:**
  - A dead session still owns its conversation, because reconnecting its tab resumes it. Only the session named in `Replaces` is exempt, and it is closed before its successor starts.
  - A create reserves the conversation before its PTY starts, keyed by the create, so two racing creates cannot both pass the check.
  - There is no `--continue`. It resumed the newest conversation in a directory, which could be held by another tab, so plan 028 step 8 removed it. The API and MCP take a conversation id to resume, and refuse `resume` without one rather than start fresh behind the caller's back. `GET /api/sessions` lists each session's id so callers can find one.
  - The picker marks a held conversation with its tab's slug, and choosing it switches to that tab.
  - Deleting a conversation follows the same rule, and adds two holders the create path does not need (plan 028 step 7a). A saved tab holds its conversation, though it has no session until it reconnects. A running session with no id yet may hold any conversation in its directory that changed since it started.
- **Plan:** `docs/plans/028-session-recovery-and-picker.md`

## ADR-029: koko-worker as a separate binary under launchd
- **Date:** 2026-09-30
- **Status:** Accepted
- **Decision:** Scheduled work jobs run in `koko-worker`, a separate binary in its own Go module (`cmd/koko-worker/`). A launchd agent keeps it running. The jobs do not run as goroutines in the Koko app.
- **Rationale:**
  - Jobs must run with the Koko window closed.
  - launchd restarts the worker if it stops (`RunAtLoad`, `KeepAlive`, `ThrottleInterval` 30).
  - `koko-worker install` copies the binary to `~/Library/Application Support/koko/worker/bin/`. A rebuild or a moved `Koko.app` then never breaks the agent.
  - The plist sets its own PATH, because launchd's default PATH has no `gh` or `claude`.
  - The agent has its own label, `com.koko.worker`. It never calls Koko's API or touches Koko's sessions.
  - A nested module, like `koko-cli`, keeps the worker's tests and lint apart from the app. `make test-worker` runs its tests, and `make test` calls it.
  - The app finds the binary next to its own (inside `Koko.app`), then in `build/bin` for `make dev`, then at the agent's copy.
  - If the running agent uses an older copy than the one next to Koko, `WorkerService.Status` reinstalls it. `install --if-idle` skips this while a job runs.
- **Trade-off:** macOS only, because the worker depends on launchd, `pmset` and `caffeinate`.
- **Files:** `cmd/koko-worker/main.go`, `cmd/koko-worker/launchd.go`, `worker_service.go`, `Makefile`
- **Plan:** `docs/plans/029-koko-worker.md`

## ADR-030: One worker agent with its own scheduler and state on disk
- **Date:** 2026-09-30
- **Status:** Accepted
- **Decision:** One launchd agent runs `koko-worker serve`, which has its own scheduler. There is no launchd agent per job. Run state lives in `worker/state.json`.
- **Rationale:**
  - The scheduler reloads `worker.json` every 30 seconds. A time change in the UI needs no plist rewrite.
  - It runs any of today's slots that have come round and not run. A job missed while the Mac slept runs on wake.
  - One run covers every missed slot of a job. After a long sleep, tono runs once for 09:30 and 12:00. A slot from an earlier day is dropped.
  - `state.json` records each slot's outcome, because a launchd restart loses memory. It is changed under a file lock, so the scheduler and "Run now" never overwrite each other.
  - `TimesAdded` records when each run time first appeared. A slot counts only if its time was set before the slot came round. A time set earlier than now, or a job switched on after its time, waits for the next day.
  - Each job has a lock file, shared by the scheduler and "Run now". A "Run now" is recorded against today's slots, so the scheduler does not repeat it.
  - Before a run, the worker checks that Slack answers. Without internet, nothing runs and nothing is recorded, so the next tick tries again.
  - Any other failure gets 3 tries, 5 minutes apart, then one warning DM.
  - `caffeinate` stops idle sleep while jobs run.
- **Files:** `cmd/koko-worker/scheduler.go`, `cmd/koko-worker/main.go`
- **Tests:** `cmd/koko-worker/worker_test.go`
- **Plan:** `docs/plans/029-koko-worker.md`

## ADR-031: The worker has its own config file
- **Date:** 2026-09-30
- **Status:** Accepted
- **Decision:** Worker settings live in `~/Library/Application Support/koko/worker.json`, not in Koko's `config.json`.
- **Rationale:**
  - Koko's `SaveConfig` rewrites `config.json` from its own struct. It would drop a `worker` key it does not know.
  - A separate file needs no change to `config_service.go` or `types.go`.
  - The Koko UI writes the file, with mode 0600, through `WorkerService.SaveConfig`. The worker only reads it.
  - The worker restores the default for any empty value, so an empty text box in the UI never wipes a default.
  - The worker has its own Slack bot token and user ID in this file. Koko's own Slack settings are not used.
  - `WorkerService` passes JSON strings, not structs. The Wails bindings stay in their own files, and the shared `models.ts` does not change.
- **Files:** `cmd/koko-worker/config.go`, `worker_service.go`
- **Plan:** `docs/plans/029-koko-worker.md`

## ADR-032: Worker jobs run Claude headless with tight allowlists
- **Date:** 2026-09-30
- **Status:** Accepted
- **Decision:** Jobs that need judgement or a claude.ai connector run `claude -p`. Go does the rest: PR lists through `gh`, calendar gaps, Slack layout.
- **Rationale:**
  - The prompt goes on stdin. The tool flags take several values and would swallow a prompt passed as an argument.
  - `--permission-mode dontAsk` refuses any tool not on the allowlist, so a run never waits for an answer.
  - `--no-session-persistence` keeps worker runs out of Koko's conversation list.
  - `--setting-sources ""` plus a worker settings file with `disableAllHooks` excludes your user allow rules, hooks and plugins.
  - Each call passes its own allowlist, for example only `list_events` or only `create_event`. A fixed deny list (`denyWrites`) refuses file writes, GitHub writes, Jira writes and any calendar change other than create.
  - Go reads the raw tool results from `--output-format stream-json`. Claude does not retype event data or links.
  - Go builds the PR lists itself, so the links are exact. Claude only judges Jira ticket coverage and writes follow-up steps.
- **Files:** `cmd/koko-worker/claude.go`, `cmd/koko-worker/jobs.go`
- **Plan:** `docs/plans/029-koko-worker.md`

## ADR-033: Calendar through the Google Calendar connector
- **Date:** 2026-09-30
- **Status:** Accepted
- **Decision:** The worker reads and writes the calendar through the claude.ai Google Calendar connector. It does not use EventKit (the macOS calendar framework).
- **Rationale:**
  - EventKit under launchd needs a signed helper binary and a calendar permission. The connector needs neither.
  - Focus time runs in three steps: Claude lists today's events, Go finds the gaps, Claude creates one event per gap.
  - The gap finder is a pure Go function, with unit tests.
  - Focus blocks are plain busy events titled "Focus", tagged `[koko-worker:focus]` in the description. They count as busy, so a second run books nothing.
  - They are not Google's "Focus time" type, which can decline new invites by itself.
- **Files:** `cmd/koko-worker/calendar.go`
- **Plan:** `docs/plans/029-koko-worker.md`

## ADR-034: The reviewer stays read-only, and the worker posts
- **Date:** 2026-09-30
- **Status:** Accepted. Amended on 2026-10-01 and 2026-10-05, see the end of this ADR
- **Decision:** The review job calls the reviewer CLI as `tono <number> --all -l high -R <repo>` and never passes `-c`. Extra guards stop the reviewer's Claude from posting to GitHub. The worker's own Go code posts the review, as the 2026-10-01 amendment says.
- **Rationale:**
  - The review runs in a cache clone under `~/.cache/koko-worker/repos`, at the PR head. Your working clones are never touched.
  - `TONO_CLAUDE` points to a wrapper. It adds `--no-session-persistence` and denies the posting commands.
  - Read-only `gh` and `git` stand-ins go first on tono's PATH. They refuse any command that changes GitHub, including `gh api` calls that send data, and any `git push`. A deny list alone matches only command prefixes.
  - A review is capped at 45 minutes. On timeout the whole process group gets SIGTERM, so tono removes its own lock.
  - One review per PR: a PR reviewed once, or one that already has a tono comment, is not reviewed again.
  - Results are keyed `repo#number@sha`, so the stand-up can say when a PR has new commits since its review.
- **Amendments:**
  - 2026-10-01: the worker's own Go code posts each pass's draft PR comment, or an LGTM. Tono's Claude stays read-only, so the guards above still hold.
  - 2026-10-05: the job is now the review worker, `review`. The reviewer CLI is a managed clone of `reviewer.repo` at `reviewer.branch` under `~/.cache/koko-worker/reviewer`, fast-forwarded before each run, or a local path. Your own tonometer checkout no longer decides which code runs.
- **Files:** `cmd/koko-worker/jobs.go`, `cmd/koko-worker/reviewer.go`
- **Plan:** `docs/plans/029-koko-worker.md`

## ADR-035: Mac wake through pmset and a sudoers rule
- **Date:** 2026-09-30
- **Status:** Accepted
- **Decision:** The worker books the next Mac wake with `sudo -n /usr/bin/pmset schedule wake`, two minutes before the next job.
- **Rationale:**
  - launchd cannot wake a sleeping Mac. `pmset` can, but it needs root.
  - A sudoers rule allows only `pmset schedule wake` and `pmset schedule cancel wake`, with no password. You add it once. The wake check shows the exact line.
  - `sudo -n` fails at once if the rule is missing, so the worker never waits for a password.
  - The worker keeps one wake booked. If the times change, it cancels the old wake first. `uninstall` cancels any booked wake.
  - The wake is booked even while offline.
- **Open question:** a MacBook with the lid closed may wake only briefly, or only on power. This is not tested yet.
- **Files:** `cmd/koko-worker/launchd.go`, `cmd/koko-worker/check.go`
- **Plan:** `docs/plans/029-koko-worker.md`

## ADR-036: Sessions stored by the backend, resumed by conversation id
- **Date:** 2026-03-26
- **Status:** Accepted (supersedes ADR-014)
- **Decision:** Koko stores its tabs and closed-session history in `sessions.json`, through `ConfigService`. A session reopens its Claude conversation with `claude --resume <id>`, using the tab's stored id or one chosen in the session dialog. There is no `--continue`: plan 028 step 8 removed it.
- **Rationale:**
  - `SaveSessions` writes to `sessions.json.new`, moves the old file to `sessions.json.bak`, then renames. A crash mid-write never leaves a half-written file.
  - If `sessions.json` is missing or corrupt, `GetSessions` reads `sessions.json.bak`.
  - The file lives in the user config directory, under `koko/`, with mode 0600. With neither file present, `GetSessions` migrates old data from WebKit localStorage.
  - `--continue` opens the newest conversation in a directory. With two sessions in one directory, that is often the other session's conversation.
  - Koko reads the conversation id from Claude's session files after the first submitted line, and stores it on the tab.
  - Each session has a slug (`koko-1`), which the API, MCP, Slack bot and CLI use. A recovered session keeps its slug.
  - At startup, `app.go` seeds the slug counters from `sessions.json` before the frontend can create a session. A new session then never takes a saved tab's slug.
- **Consequences:**
  - A disconnected tab opens the session dialog with its conversation preselected. It no longer reconnects silently.
  - The API and MCP take a conversation id to resume, and refuse `resume` without one rather than start fresh behind the caller's back.
- **Files:** `config_service.go`, `terminal_manager.go`, `app.go`, `frontend/src/hooks/useSessionTabs.ts`
- **Tests:** `config_service_test.go`, `slug_test.go`, `terminal_manager_test.go`
- **Plans:** `docs/plans/019-session-identity-and-recovery.md`, `docs/plans/028-session-recovery-and-picker.md`

## ADR-037: Approval detection through Claude's PermissionRequest hook
- **Date:** 2026-03-26
- **Status:** Accepted
- **Decision:** Koko learns that Claude waits for tool approval from Claude Code's `PermissionRequest` hook. It does not match patterns in the terminal output.
- **Rationale:**
  - Pattern matching against the output buffer gave false amber icons.
  - At startup, `installPermissionHook` writes an HTTP hook to `~/.claude/settings.json`. It points at `http://127.0.0.1:<port>/api/hooks/permission-request`, with a 5 second timeout.
  - The endpoint needs no Bearer token, because the local Claude process has none. The API server listens on `127.0.0.1` only.
  - The handler matches the session by Claude's conversation id, then by working directory. It marks that session as waiting for approval.
  - It returns `{}`, so Claude still shows its own prompt. A decision in the reply would approve or deny without the user.
  - The flag clears when input reaches the session, through `Write` or `WriteKeystrokes`.
  - `GetSessionState` returns `"approval"` or `"idle"`. The sidebar pulse, reload's confirmation and the API read it. The MCP `get_session_state` tool reaches it through the API.
- **Consequences:** Koko replaces any other `PermissionRequest` hook entry in `~/.claude/settings.json` with its own.
- **Files:** `app.go`, `api_server.go`, `terminal_manager.go`, `frontend/src/hooks/useSessionActivity.ts`

## ADR-038: Git worktrees for parallel sessions
- **Date:** 2026-05-20
- **Status:** Accepted
- **Decision:** Koko can start a session in a new git worktree, so parallel sessions on one repo do not share files, branch or index. It drives plain `git worktree` commands through `GitService`.
- **Rationale:**
  - `CreateWorktree`, `ListWorktrees`, `RemoveWorktree` and `PruneWorktrees` wrap `git worktree add`, `list --porcelain`, `remove` and `prune`.
  - The session dialog proposes a new branch and a sibling directory, both with the same random suffix. The toggle starts on when another tab already uses the directory.
  - The sidebar shows each session's branch, and an amber dot when two sessions share a directory.
  - The Worktrees module lists the repo's worktrees and flags uncommitted changes. Opening one goes through the session dialog.
  - A tab records its worktree (`worktreePath`), and whether Koko created it (`worktreeCreated`). Closing that tab asks whether to remove it.
  - A closed session keeps the path of a worktree Koko created and the user kept. Settings > General removes these in bulk, never forced. It skips any with uncommitted or ignored files, any a session or saved tab uses, and any path that is not a linked worktree (plan 028 step 7b).
  - Removal tries without `--force` first. Only when git refuses does the dialog offer a forced removal. This protects uncommitted work.
- **Files:** `git_service.go`, `frontend/src/components/SessionDialog.tsx`, `frontend/src/components/WorktreesModule.tsx`, `frontend/src/components/WorktreeRemovalDialog.tsx`, `frontend/src/hooks/useWorktrees.ts`, `frontend/src/hooks/useSessionBranches.ts`, `worktree_cleanup.go`, `frontend/src/components/HousekeepingSettings.tsx`
- **Plan:** `docs/plans/027-git-worktrees-for-session-isolation.md`

## ADR-039: A reviewer contract for the review worker
- **Date:** 2026-10-08
- **Status:** Accepted
- **Decision:** The review worker runs any reviewer CLI that follows a small contract, not only tono. The worker fills in `reviewer.args` and passes `REVIEW_RESULT`, `REVIEW_PR`, `REVIEW_REPO`, `REVIEW_PR_URL`, `REVIEW_SHA` and `REVIEW_CLAUDE`. The reviewer exits 0, or 2 for nothing to review, and writes its comments as JSON to `REVIEW_RESULT`. It never posts. The worker posts each comment as written.
- **Rationale:**
  - Two tono changes, LGTM drafts and skipping a code review already clean at the head commit, broke a worker that read tono's private logs. A contract keeps the reviewer's internals out of the worker.
  - The reviewer writes its own LGTM, so a clean review says what it checked, and the worker invents nothing.
  - A skipped pass is a pass with no comment, never a failure.
  - The marker prefix (`reviewer.markerPrefix`) is how the worker knows a PR is reviewed, so it is configurable with the reviewer.
  - Until tono writes a result file, the `tono-logs` adapter reads its logs as before. It reads `**LGTM!**` as ready, posts one LGTM instead of three, and treats a code review skipped for an LGTM on the head commit as skipped.
- **Files:** `cmd/koko-worker/review_contract.go`, `cmd/koko-worker/jobs.go`, `cmd/koko-worker/config.go`, `frontend/src/components/WorkerSettings.tsx`
- **Plan:** `docs/plans/030-reviewer-contract.md`
