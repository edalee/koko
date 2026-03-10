# kõkõ (Tui)  

A desktop application that serves as a unified workspace for Claude Code sessions, with integrated GitHub and Slack awareness panels.

Built with [Wails v2](https://wails.io/) (Go backend + React frontend).

## Features

- **Terminal sessions** — Multiple PTY sessions via tabbed interface, powered by xterm.js with WebGL rendering
- **GitHub panel** — Live open PR counts from tracked repos with review status badges
- **Slack panel** — Unread DM, thread, and mention counts at a glance
- **Summary panel** — Aggregated digest of what needs attention
- **Draggable panels** — Reorder awareness panels via drag-and-drop, persisted to localStorage
- **Dark theme** — OKLCH color palette designed for extended terminal use

## Requirements

- Go 1.24+
- Node.js 22+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- macOS or Linux

## Getting Started

```bash
# Install frontend dependencies
make install-fe

# Run in development mode (hot reload)
make dev

# Build production .app bundle
make build
```

## Project Structure

```
main.go                    Wails entry point
app.go                     App lifecycle
terminal_manager.go        PTY session management (Wails-bound)
github_service.go          GitHub PR fetching (Wails-bound)
types.go                   Shared Go types
wails.json                 Wails config
build/                     Build assets (Info.plist, icons)
frontend/
  src/
    App.tsx                App shell layout
    globals.css            OKLCH dark theme + Tailwind
    components/
      TitleBar.tsx         Custom title bar with macOS traffic lights
      SessionTabBar.tsx    Tab management (create, switch, close)
      TerminalPane.tsx     xterm.js terminal wrapper
      PanelDock.tsx        Draggable panel container (@dnd-kit)
      GitHubPanel.tsx      Expandable PR list
      SlackPanel.tsx       Slack notifications (mock)
      SummaryPanel.tsx     Aggregated counts
    hooks/
      useSessionTabs.ts    Tab state management
      useGitHub.ts         PR fetching with auto-refresh
      usePanelState.ts     Panel order + expand/collapse
docs/plans/                Implementation plans
```

## License

[Business Source License 1.1](LICENSE) — source available, non-competing production use allowed. Converts to GPL 2.0+ on 2030-02-27.
