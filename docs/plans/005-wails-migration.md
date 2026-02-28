# Plan 005: Wails Desktop App Migration

**Status:** Completed
**Branch:** `feat/wails-migration`

## Context

Koko started as a bubbletea TUI but the vision has evolved: a standalone desktop app (installable via DMG/apt-get) designed around Claude Code sessions, with draggable awareness panels, expandable widgets, and rich styling. A TUI can't deliver drag-and-drop, smooth expand/collapse animations, or modern visual design. Wails (Go backend + web frontend) gives us all of this while reusing our Go PTY and GitHub fetch logic.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  Wails v2 Desktop Window (frameless, custom chrome) │
├─────────────────────────────────────────────────────┤
│  TitleBar (drag region, macOS traffic lights)       │
│  SessionTabBar (+ to create, click to switch, x)    │
├──────────────────────────┬──────────────────────────┤
│                          │  PanelDock (w-72)        │
│  TerminalPane            │  ┌──────────────────┐   │
│  (xterm.js + WebGL)      │  │ ⋮ GitHub    3 PRs│   │
│                          │  │   PR list...     │   │
│  One per session tab,    │  ├──────────────────┤   │
│  hidden when inactive    │  │ ⋮ Slack     6 new│   │
│                          │  │   (collapsed)    │   │
│                          │  ├──────────────────┤   │
│                          │  │ ⋮ Summary        │   │
│                          │  │   3 PRs, 6 msgs  │   │
│                          │  └──────────────────┘   │
│                          │  Drag ⋮ to reorder      │
└──────────────────────────┴──────────────────────────┘
```

**Stack:**
| Layer | Technology |
|-------|-----------|
| Desktop shell | Wails v2 (Go + embedded webview) |
| Frontend | React 19 + TypeScript + Vite |
| Terminal | xterm.js v5 + WebGL addon + FitAddon |
| Drag & drop | @dnd-kit/core + @dnd-kit/sortable |
| UI components | shadcn/ui (Radix primitives) |
| Styling | Tailwind CSS v4 with OKLCH dark theme |
| Icons | Lucide React |
| PTY | creack/pty (reused from TUI) |
| GitHub data | gh CLI (reused from TUI) |

## Go Backend Services

### TerminalManager — PTY session lifecycle

Reuses `creack/pty` patterns from current `pty.go`. Key difference: readLoop emits Wails events instead of signaling bubbletea channels. No VT emulator needed — xterm.js handles rendering.

**Bound methods:**
- `CreateSession(cols, rows int) (string, error)` — spawn PTY, start readLoop goroutine, return session ID
- `Write(sessionID, data string) error` — write user input to PTY (base64 encoded)
- `Resize(sessionID string, cols, rows int) error` — pty.Setsize for SIGWINCH
- `CloseSession(sessionID string) error` — close PTY, kill process

**Events emitted:**
- `pty:data:{sessionID}` — base64-encoded terminal output (replaces outputCh subscription)
- `pty:exit:{sessionID}` — session ended (replaces exitCh)

### GitHubService — PR data

Direct port of current `fetch.go`. Returns `[]GitHubPR` via bound method instead of tea.Cmd.

**Bound methods:**
- `FetchPRs() ([]GitHubPR, error)` — fetches from tracked repos via gh CLI

### App — Lifecycle

- `GetVersion() string`
- Startup/shutdown hooks

## Frontend Components

### useTerminal hook — The core bridge
Replaces the entire bubbletea terminal component + VT emulator + key encoding:
- Creates xterm.js Terminal with WebGL renderer
- `EventsOn("pty:data:{id}")` → `term.write(decoded)` (PTY output)
- `term.onData()` → `Write(id, encoded)` (user input)
- `ResizeObserver` → `FitAddon.fit()` → `Resize(id, cols, rows)` (resize)

### TerminalPane — xterm.js wrapper
One per session. Hidden (not unmounted) when tab is inactive to preserve scrollback and state.

### SessionTabBar — Tab management
Create, switch, close terminal sessions. Each tab = one PTY process (intended for Claude Code sessions).

### PanelDock — Draggable panel container
Right-side dock using `@dnd-kit` SortableContext. Panels reorder via drag handle. Order persists to localStorage.

### PanelWidget — Base expandable card
Click header to toggle collapsed (badge count only) ↔ expanded (top 10 list). Drag handle in header for reorder.

### GitHubPanel / SlackPanel / SummaryPanel
- **Collapsed:** single line with count badge
- **Expanded:** scrollable list of items (PRs, messages, aggregated digest)

## Color Palette — OKLCH Dark Theme

```css
@theme {
  --color-base:              oklch(0.13 0.01 260);
  --color-surface:           oklch(0.16 0.01 260);
  --color-titlebar:          oklch(0.11 0.01 260);
  --color-panel:             oklch(0.14 0.01 260);
  --color-foreground:        oklch(0.93 0.01 260);
  --color-muted-foreground:  oklch(0.55 0.01 260);
  --color-border:            oklch(0.25 0.01 260);
  --color-accent:            oklch(0.65 0.15 250);
  --color-success:           oklch(0.65 0.15 155);
  --color-warning:           oklch(0.75 0.15 80);
  --color-error:             oklch(0.65 0.20 25);
  --color-tab-active:        oklch(0.18 0.01 260);
  --color-tab-hover:         oklch(0.20 0.01 260);
}
```

## Implementation Phases

### Phase 0: Wails Scaffold
Create Wails project structure, React + Vite + Tailwind + shadcn/ui setup. Empty window opens.

### Phase 1: Terminal Backend + xterm.js
Port PTY management to TerminalManager. Connect xterm.js via Wails events. Single session working.

### Phase 2: Session Tabs
Multiple terminal sessions with tab management. Create, switch, close.

### Phase 3: Custom Title Bar + Theme
Frameless window, macOS traffic lights, OKLCH dark theme, app shell layout.

### Phase 4: GitHub Panel
Port fetch logic to GitHubService. Expandable panel with real PR data.

### Phase 5: Panel Dock with Drag-and-Drop
dnd-kit sortable panels. Add Slack + Summary panels. Persist order.

### Phase 6: Cleanup + Packaging
Delete old TUI code. Remove charm deps. DMG packaging.

## Dependencies

**Go (add):**
- `github.com/wailsapp/wails/v2`
- Keep `github.com/creack/pty v1.1.24`

**Go (remove):**
- `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`
- `github.com/charmbracelet/x/vt` + replace directive
- All transitive charm deps

**Frontend (npm):**
- `@xterm/xterm`, `@xterm/addon-fit`, `@xterm/addon-webgl`, `@xterm/addon-web-links`
- `@dnd-kit/core`, `@dnd-kit/sortable`, `@dnd-kit/utilities`
- `tailwindcss@4`, `@radix-ui/*` (via shadcn/ui), `lucide-react`
- `react@19`, `react-dom@19`, `typescript`
