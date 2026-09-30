# Koko - Design Document

## Layout (3-panel resizable)

```
┌──────────────────────────────────────────────────────┐
│ ● ● ●             KOKO               [update] [⚙]    │  ← Toolbar (frameless, drag region)
├────────┬──────────────────────────────────┬──────────┤
│ Session│                                  │ Mod │Icon│
│ sidebar│    Terminal (xterm.js v6)        │ ule │bar │
│ 15%    │    full PTY, WebGL               │ con │    │
│ grouped│                                  │ tent│ w-9│
│ by dir │    bg #0f1117, cursor #1FF2AB    │ 19% │    │
│        ├──────────────────────────────────┤     │    │
│        │ Mode switcher · Quick Terminal   │     │    │
└────────┴──────────────────────────────────┴──────────┘
  Left: 14 to 25% (4% collapsed)            Right: 15 to 35% (3% collapsed)
```

**Background:** `#0f1117` base with animated mesh gradient orbs (mint/blue/teal, 12s breathing animation) and SVG noise texture overlay.

## Elevation Order (glass tiers, lightest = most elevated)

1. **Base** — `#0f1117` — deep blue-black app background
2. **Sidebar glass** — `rgba(255,255,255,0.03)` + `backdrop-blur(20px)` — sidebars, session sidebar
3. **Toolbar glass** — `rgba(255,255,255,0.04)` + `backdrop-blur(24px)` — top toolbar
4. **Card glass** — `rgba(255,255,255,0.06)` + `backdrop-blur(12px)` — PR cards, notification cards
5. **Overlay glass** — `rgba(255,255,255,0.08–0.10)` + `backdrop-blur(40px)` — modals, overlay pages

## Color Palette

| Token | Value | Role |
|-------|-------|------|
| Base | `#0f1117` | App background |
| Glass (sidebar) | `rgba(255,255,255,0.03)` | Sidebars |
| Glass (toolbar) | `rgba(255,255,255,0.04)` | Toolbar |
| Glass (card) | `rgba(255,255,255,0.06)` | Cards |
| Glass (overlay) | `rgba(255,255,255,0.08–0.10)` | Modals, overlays |
| Border | `rgba(255,255,255,0.08)` | Ultra-thin borders |
| Accent | `#1FF2AB` | Mint green (active states, cursor) |
| Accent dark | `#24A965` | Mint gradient end |
| Text primary | `rgba(255,255,255,0.92)` | Primary text |
| Text secondary | `rgba(255,255,255,0.55)` | Secondary text |
| Text muted | `rgba(255,255,255,0.35)` | Tertiary / labels |
| Text faint | `rgba(255,255,255,0.20)` | Disabled / hints |
| Success | `oklch(0.65 0.15 155)` | Approved badges |
| Warning | `oklch(0.75 0.15 80)` | Review-needed badges |
| Error | `oklch(0.65 0.2 25)` | Changes-requested badges |
| Merge | `#a78bfa` | Merge button, distinct from approve (mint) |

## Glass Utility Classes

| Class | Background | Blur | Use |
|-------|-----------|------|-----|
| `.glass-toolbar` | `rgba(255,255,255,0.04)` | `24px` | Toolbar |
| `.glass-panel` | `rgba(255,255,255,0.03)` | `20px` | Sidebars |
| `.glass-card` | `rgba(255,255,255,0.06)` | `12px` | Cards |
| `.glass-overlay` | `rgba(255,255,255,0.08–0.10)` | `40px` | Modals |
| `.glow-accent` | — | — | Mint glow shadow on active elements |
| `.inset-highlight` | — | — | Top-edge inset light on cards |

## Design Principles

- **Glassmorphism** — elevation via translucent layers + backdrop-filter blur over mesh gradient
- **Opacity-based text** — hierarchy through white at varying opacity (92/55/35/20%)
- **Gradient borders** — sidebar borders lighter at top, fading down
- **Mint accent** — active states use mint border tint + glow shadow (not gradient fills)
- **Breathing mesh** — 12s ease-in-out animation on background gradient orbs
- **Clean spacing** — generous padding, no cramped elements
- **Token compliance** — use glass utility classes and CSS variables, not raw Tailwind colors. Semantic colours use the `success`, `warning`, `error` and `merge` tokens

## Components

### Toolbar
- `.glass-toolbar`, `border-b border-border`. The whole bar is a drag region through `--wails-draggable: drag`
- Custom traffic lights (close, minimise, maximise) at the left. They fade in on hover
- Maximise saves the window bounds, then fills the screen. This avoids the macOS two-step zoom animation
- KOKO SVG logo, centred (`h-4 w-auto`)
- Right side: an update pill (mint, links to the release) when a new version exists, then the Settings button
- Interactive controls opt out of dragging with `--wails-draggable: no-drag`

### SessionSidebar
- `.glass-panel`, resizable 14 to 25% width. Collapses to a 4% icon strip
- "New Session" button with a `⌘N` hint, and a search input
- Sessions group under a directory header when two or more share a directory. Headers collapse and show a count
- Each row: status dot, name or slug, `⌘1` to `⌘9` hint on hover, reload and close buttons on hover
- An amber pulse marks a session waiting for tool approval
- A warning marks a directory that two sessions share, and suggests a worktree

### TerminalPane
- Full-size xterm.js v6, no wrapper card
- Theme: bg `#0f1117`, fg `rgba(255,255,255,0.92)`, cursor `#1FF2AB`
- WebGL, fit, web-links, search and serialize addons
- Hidden (not unmounted) when its tab is inactive, so scrollback survives
- Re-fits when its tab becomes active, so a resize in the background does not crop it
- Calls `ReplayBuffer()` on mount to replay missed PTY output
- `Cmd+F` searches the scrollback
- A disconnected session shows a context card with its last reply, its directory and "Click to reconnect"
- Right-click menu:

| Item | Shortcut | Notes |
|------|----------|-------|
| Copy | `⌘C` | Trimmed plain text plus HTML |
| Copy as Markdown | `⇧⌘C` | |
| Paste | `⌘V` | |
| Search Web | | Opens the selection in the default browser |
| Select All | `⌘A` | |
| Redraw Terminal | | Rebuilds the WebGL glyph cache |
| Clear Terminal | | |
| Reload Session | | Only for Claude sessions |

### ClaudeModeSwitcher
- Buttons below the terminal for Claude Code's modes
- Sends Shift+Tab to the PTY. The cycle order is `auto → plan → ask`, with 80ms between writes

### QuickTerminal
- `Cmd+`` ` toggles a slide-up panel (35% height) from the bottom
- One per session, keyed by the tab's creation time. Spawns `zsh -l` in the session's directory
- Glass header: Terminal icon, "Terminal", directory path, close and minimise buttons
- Ring buffer (64KB) on the Go side replays the prompt on a late mount

### RightSidebar
- `.glass-panel`, resizable 15 to 35% width. Collapses to a `w-9` icon bar
- Icon bar buttons: File Changes, Session Context, Worktrees, Pull Requests, Notifications
- Active module: mint accent icon + `bg-white/[0.08]`
- Pull Requests opens the PR detail overlay instead of a module
- Badges on the icon bar come from `NotificationBadge`

### File Changes module
- Header: branch name with a CI status dot, file count, refresh button
- File list: icon, file name, directory, and a status badge on hover
- Colour: staged is green. Unstaged: added green, modified orange, deleted red, renamed blue
- A partly staged file appears twice (staged and unstaged)
- Shows files committed on the branch and untracked files, not only the working tree
- CI runs: a collapsible list of GitHub Actions runs for the branch. It opens by itself on a failure. A click opens the run on GitHub
- Polls files every 5s and CI every 60s (`useFileChanges`, `useCI`)

### Session Context module
- Running subagents from the session's process tree (`useSubagents`, every 5s)
- MCP Servers: configured servers and their status
- Agents: built-in Claude agents and their models
- Commands: project, global and plugin slash commands. A click types the command into the terminal

### Worktrees module
- Lists the repo's git worktrees, with branch, uncommitted changes and prunable state
- Open a worktree (through the session dialog) or remove it
- Polls every 10s (`useWorktrees`)

### Notifications module
- GitHub notifications, participating or all
- Mark one or all as read, with optimistic updates. Polls every 60s

### PRDetailOverlay
- Full-screen overlay (`z-50`) with a PR list sidebar and a detail panel
- PRs come from the tracked repos. They poll every 60s and can be hidden
- Detail: author, actions (approve, merge in `merge` purple, view), description as markdown with raw HTML, files, reviews, commits, collapsible CI checks, labels and merge state
- Comments: review threads grouped by file with the diff hunk, and discussion comments. Both take replies
- A file click opens the CodeViewer with the PR diff
- Opens in 100ms and closes in 75ms

### CodeViewer
- Full-screen overlay at `z-[60]`, so it sits above the PR overlay
- `@git-diff-view/react` with Shiki highlighting. Split or unified view, and wrap
- Raw mode shows the whole file with Shiki
- Binary files show a notice. A file over 10,000 lines asks before it renders
- `[` and `]`, or `↑` and `↓`, switch files. Escape closes
- Backend: `GetFileDiff`, `GetFileContent`, and `FetchPRFileDiff` for PRs. The diff must keep its `---` and `+++` headers

### SessionDialog
- One dialog, three modes: new, reconnect and worktree. It remounts on each open
- New: name, directory (with recent directories), optional new worktree, then the conversation picker
- Reconnect: opens with the tab's conversation preselected, so Enter restores it. A tab with no stored conversation preselects nothing
- Worktree: opens for a worktree from the Worktrees module
- Recent Sessions: closed sessions with their last reply. Choosing one reopens its conversation
- `role="dialog"`, `aria-modal`. Focus goes to the name field, or to the panel in reconnect mode

### ConversationPicker
- Lists the conversations Claude stored for the directory, newest first, with title and last reply
- Native radio inputs, with "New conversation" first
- A conversation another tab holds switches to that tab instead of opening twice

### ConfirmDialog
- Used for reload of a busy session and other destructive actions
- Focus goes to Cancel when the action is destructive, else to Confirm
- Escape cancels. Enter acts only on the focused button

### WorktreeRemovalDialog
- Opens when a session closes whose worktree Koko created. Offers to remove it

### SettingsPanel
- Fills the window as a nearly opaque overlay (`OverlayPage`, `.glass-overlay`)
- Tabs:

| Tab | Contents |
|-----|----------|
| General | Remote API: on/off, port, API key with copy button |
| Safe working | Quiet hours, break cycle and presets (90/15, 60/10, 45/5) |
| GitHub | Tracked repos, hidden PRs |
| Worker | `WorkerSettings.tsx`: on/off, job times, Test and Run now, connection checks, log |

- The Slack tab is hidden until the Slack DM bot is dropped or rebuilt (plan 029)

### SafeWorkingOverlay
- Full-screen overlay for quiet hours, with a 30-minute delay button
- Break screen with "Skip this break". Checks every 30s

### Resizable (ui/resizable.tsx)
- Wraps `react-resizable-panels` (Group, Panel, Separator)
- Handle: `w-px bg-white/8 hover:bg-accent/50`

## Mesh Gradient Background

- Three animated orbs: mint (`#1FF2AB`), blue (`#3B82F6`-ish), teal
- Positioned via absolute/fixed, blurred heavily (`blur(100px)+`)
- 12-second `ease-in-out` breathing animation (scale + translate)
- SVG noise texture overlay for grain effect
- Lives behind all glass layers

## Wails Config

- `Frameless: true`: Koko draws its own traffic lights
- 8px `border-radius` on `html, body, #root` gives the window its bevel
- `BackgroundColour: {R: 15, G: 17, B: 23}` matches base `#0f1117`
- `WebviewIsTransparent: true`
- Default size 1280×800, minimum 800×600

## App Icon

- Source: `build/appicon.png`, the colourful tui bird
- Wails generates the `.icns` from it
