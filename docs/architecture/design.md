# Koko - Design Document

## Layout (3-panel resizable)

```
┌───────────────────────────────────────────────────┐
│ ● ● ●  w-18  KOKO  File  Search  Settings    [⊞] │  ← Toolbar (glass, blur(24px))
├───────────────────────────────────────────────────┤
│ ▾ Open Sessions (2)                               │  ← SessionTabs (collapsible)
│  Session 1 ×  │  Session 2 ×                      │  ← mint border active, plain inactive
├──────┬────────────────────────────────┬───────────┤
│ Sess │                                │ Icon │ Mod │
│ Side │    Terminal (xterm.js)          │ bar  │ ule │
│ bar  │    full PTY, WebGL             │ w-9  │ con │
│ 15%  │                                │ 3%   │tent │
│      │    bg #0f1117                  │      │ 300 │
│      │    fg rgba(255,255,255,0.92)   │ glass│ px+ │
│      │    cursor #1FF2AB             │panel │     │
└──────┴────────────────────────────────┴───────────┘
  ResizablePanelGroup (horizontal)        RightSidebar
  min 12% / max 25% for sidebar           collapsed 3% (w-9 icon bar)
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
- **Token compliance** — use glass utility classes and CSS variables, not raw Tailwind colors

## Components

### Toolbar
- `.glass-toolbar`, `border-b border-white/8`, drag region via `WebkitAppRegion: "drag"`
- macOS traffic light spacer (`w-18`)
- KOKO SVG logo (`h-4 w-auto`)
- Notification icons: GitHub PRs, Slack, Mail — all `p-2 rounded-md`, active: `text-accent bg-white/10`
- Settings: icon-only (no text label), same pattern as notification icons, separated by `w-px h-4 bg-white/10` divider
- Badge dots via `NotificationBadge` component at `-top-1 -right-1`

### SessionTabs
- Collapsible header: chevron + "Open Sessions (N)" in secondary text
- Active tab: mint border tint + `.glow-accent` shadow
- Inactive tab: `hover:bg-white/5`
- Close button (X) per tab

### SessionSidebar
- `.glass-panel`, `border-r border-white/8`, resizable 12–25% width
- "New Session" button: mint accent styled
- Search input with icon
- Session list: Terminal icon + title, active state with mint border + glow

### RightSidebar
- `.glass-panel`, `border-l border-white/8`, toggleable
- Collapsed: 3% width, `w-9` icon bar
- Icon bar: module buttons (FileCode2, Bot), collapse toggle at bottom
- Active module: mint accent icon + `bg-white/[0.08]`
- Module content panel: full-height, scrollable

### File Changes Module (RightSidebar)
- Header: "File Changes" title + branch name + file count + refresh button
- File list: icon + filename + directory path, hover reveals status badge
- Color coding: staged=green (all statuses), unstaged: added=green, modified=orange, deleted=red, renamed=blue
- Partially staged files appear twice (staged + unstaged entries)
- Polls every 5 seconds via `useFileChanges` hook
- Empty states: "No changes on this branch" / "No active session"

### Subagents Module (RightSidebar)
- Header: "Subagents" title + "N agents · M MCPs" summary (accent/blue colors) + refresh button
- Subagents: sparkle icon + green dot + command name + elapsed time + child process count
- MCP Servers: dedicated section with blue plug icon header "MCP Servers (N)", blue dot + name + elapsed time
  - Friendly names extracted from command line (nolo, docker mcp, npx package names)
- Tools: wrench icon, de-emphasized text
- Infrastructure: collapsible section with cog icon (caffeinate, docker-credential)
- Green dot on Bot icon in sidebar icon bar when subagents active
- Polls every 3 seconds via `useSubagents` hook
- Empty states: "No active session" / "No active subagents"

### QuickTerminal
- `Cmd+`` ` toggles slide-up panel (35% height) from bottom
- Spawns `zsh -l` in active session directory
- Glass toolbar header: Terminal icon + "Terminal" + directory path + close/minimize buttons
- Slide-up/slide-down CSS animations (200ms/150ms cubic-bezier)
- Ring buffer (64KB) on Go side for prompt replay on late mount
- Separate from Claude sessions — utility shell for git, make, etc.

### TerminalPane
- Full-size xterm.js v6, no wrapper card
- Theme: bg `#0f1117`, fg `rgba(255,255,255,0.92)`, cursor `#1FF2AB`
- WebGL addon for performance
- Hidden (not unmounted) when tab inactive — preserves scrollback
- Calls `ReplayBuffer()` on mount to replay any missed PTY output
- Copy: Cmd+C (trimmed plain text + HTML), Cmd+Shift+C (Markdown), right-click context menu
- SerializeAddon for HTML serialization of terminal selections

### CodeViewer
- Full-screen overlay (reuses OverlayPage pattern) — opens on file click in right sidebar
- `@git-diff-view/react` with Shiki syntax highlighting
- Split / Unified toggle in header bar
- File path, status badge (A/M/D + staged indicator), +/- stats
- Dark theme CSS variables: mint green additions, red deletions, subtle hunk headers
- Escape to close
- Go backend: `GetFileDiff(dir, path, staged)` returns old/new content + hunks

### GitHubPanel
- Real PR data via Go backend `github_service.go`
- PR cards use `.glass-card` + `.inset-highlight` + `rounded-xl`
- Review badges: approved (green), changes (red), review needed (yellow)
- Clickable cards open PR in browser

### SlackPanel
- Real Slack API via user token (`xoxp-`): DMs + mentions + thread replies
- DMs: `conversations.list` + `conversations.history`, scoped to last hour, only where other person spoke last
- Mentions/threads: `search.messages` with `<@selfID>` query (requires `search:read` scope)
- DMs and mentions fetched in parallel goroutines
- Props-based: receives messages, loading, configured state from `useSlack` hook
- Unconfigured state: icon + "Connect Slack" message + "Open Settings" button
- Message cards: `.glass-card`, DM icon (mint) vs @mention icon (blue), sender, channel, preview, time ago
- Click opens `slack://` deep link to native Slack app
- Polls every 60 seconds
- Required scopes: `im:history`, `im:read`, `users:read`, `search:read`

### SettingsPanel
- Slack token input with show/hide toggle (Eye/EyeOff icons)
- Save Token button with loading/saved feedback states
- Test Connection button (calls `TestConnection()`, shows user@team)
- Debug API Responses link (dev-only, calls `DebugFetch()`)
- Results in `<pre>` with green=success / red=error color coding

### MailPanel
- Notification/mail overlay page
- Uses `.glass-card` for message items
- Currently mock data

### OverlayPage
- Full-panel overlay using `.glass-overlay`
- Slides over right sidebar content

### NotificationBadge
- Unread count indicator on icon bar buttons

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

- `Frameless: false` — required for macOS traffic lights
- `TitleBarHiddenInset()` — hides title text, shows traffic lights inset
- `BackgroundColour: {R: 15, G: 17, B: 23}` — matches base `#0f1117`
- `WebviewIsTransparent: true`

## App Icon

- Source: `build/koko-bird-colour.png` — colourful tui bird
- Wails generates `.icns` from this
- Baked-in squircle mask for macOS dev builds
