# Plan 001: Initial Scaffold

**Status:** Completed
**Date:** 2026-02-27

## Context

Fresh Go repo with only `go.mod`, `README.md`, `CLAUDE.md`, `.gitignore`. Need a minimal working skeleton that compiles, runs, and shows the basic layout framework with component composition in place.

## File Structure

```
koko/
├── cmd/koko/main.go
├── internal/
│   └── tui/
│       ├── root.go
│       ├── messages.go
│       ├── styles.go
│       ├── keymap.go
│       └── components/
│           ├── terminal/model.go
│           ├── sidebar/model.go
│           ├── slack/model.go
│           ├── github/model.go
│           └── summary/model.go
├── Makefile
├── go.mod
├── go.sum
├── README.md
├── CLAUDE.md
└── .gitignore
```

## What each file does

### `cmd/koko/main.go`
- Creates root model, starts `tea.NewProgram`

### `internal/tui/root.go`
- Root model holds: terminal, sidebar, focus state, dimensions, sidebar visible flag
- `Init()` returns nil (no initial command)
- `Update()` handles global keys (quit, toggle sidebar, switch focus), delegates to focused child
- `View()` returns `tea.View` with `AltScreen = true`; uses lipgloss `JoinHorizontal` of terminal + sidebar (if visible), plus status bar at bottom

### `internal/tui/messages.go`
- `FocusPanel` type with constants (FocusTerminal, FocusSlack, FocusGitHub, FocusSummary)
- `FocusChangedMsg` for future use

### `internal/tui/styles.go`
- Status bar style (dim foreground on dark background)

### `internal/tui/keymap.go`
- `ctrl+\` toggle sidebar
- `ctrl+t` focus terminal
- `ctrl+g` focus GitHub panel
- `ctrl+s` focus Slack panel
- `ctrl+c` quit

### `internal/tui/components/terminal/model.go`
- Placeholder that renders "Terminal" header + info text
- Active/inactive border based on focus

### `internal/tui/components/sidebar/model.go`
- Container that stacks Slack, GitHub, Summary panels vertically
- Distributes available height between panels (equal thirds)
- Exported child fields (Slack, GitHub, Summary) for root focus management

### `internal/tui/components/slack/model.go`
- Static placeholder showing mock counts (DMs: 3, Threads: 1, Mentions: 2)

### `internal/tui/components/github/model.go`
- Static placeholder showing mock PR counts per repo

### `internal/tui/components/summary/model.go`
- Static placeholder with aggregated info

### `Makefile`
- `build`, `run`, `test`, `lint`, `clean` targets

## Component Pattern

Each component exports a `Model` struct with:
- `New()` constructor
- `Init() tea.Cmd`
- `Update(tea.Msg) (Model, tea.Cmd)` (concrete return type, not tea.Model)
- `View() string` (returns string, only root returns `tea.View`)
- `SetSize(width, height int) Model`
- `SetFocus(focused bool) Model`

Root model owns all children as struct fields (not interfaces). Root broadcasts dimensions via `SetSize()`. Focus tracked in root; only focused component receives key messages. Children return `tea.Cmd` which root batches.

## Layout Calculation

- Sidebar width: 30 chars fixed (when visible)
- Terminal width: total width - sidebar width (or full width when sidebar hidden)
- Status bar: 1 line at bottom
- Content area: height - 1 (status bar)

## Key Bindings

| Key | Action |
|-----|--------|
| `ctrl+\` | Toggle sidebar visibility |
| `ctrl+t` | Focus terminal pane |
| `ctrl+s` | Focus Slack panel (auto-opens sidebar) |
| `ctrl+g` | Focus GitHub panel (auto-opens sidebar) |
| `ctrl+c` | Quit |

## Dependencies

- `charm.land/bubbletea/v2` v2.0.0
- `charm.land/lipgloss/v2` v2.0.0

**Note:** Bubbletea and Lipgloss v2 moved from `github.com/charmbracelet/*` to `charm.land/*` module paths.

## Key v2 API Differences (from v1)

| Aspect | v1 | v2 |
|--------|----|----|
| Import path | `github.com/charmbracelet/bubbletea` | `charm.land/bubbletea/v2` |
| `Init()` | `Init() tea.Cmd` | `Init() tea.Cmd` (same) |
| `View()` | `View() string` | `View() tea.View` |
| Alt screen | `tea.WithAltScreen()` option | `view.AltScreen = true` field |
| Key press msg | `tea.KeyMsg` | `tea.KeyPressMsg` |
| View creation | return string | `tea.NewView(string)` |

## Verification

1. `make build` compiles without errors
2. `make run` launches the TUI in alt screen
3. `ctrl+\` toggles sidebar on/off
4. `ctrl+t/g/s` switches focus (visible via border highlight)
5. Window resize reflows layout correctly
6. `ctrl+c` exits cleanly
