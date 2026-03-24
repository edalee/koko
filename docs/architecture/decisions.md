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
- **Status:** Accepted
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
- **Status:** Accepted
- **Decision:** Real Slack DM + @mention fetching using a user token (`xoxp-`/`xoxe.xoxp-`), stored in `~/.config/koko/config.json` with `0600` permissions. Settings overlay for token configuration with test/debug tools.
- **Rationale:**
  - Bot token scopes: `im:history`, `im:read`, `users:read`
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
- **Status:** Accepted (updates ADR-016)
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
