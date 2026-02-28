# kōkō is a Tui (Tūī Bird)

A terminal application that serves as a unified workspace for Claude Code sessions, with integrated GitHub and Slack awareness panels.

Built with [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) and [Lip Gloss v2](https://github.com/charmbracelet/lipgloss).

## Features

- **Terminal pane** — Embedded PTY running zsh with full ANSI rendering (supports Claude Code, vim, etc.)
- **Collapsible sidebar** — Stacked panels with click-to-focus and keyboard navigation
- **Keyboard-driven** — tmux-inspired navigation

### Planned

- **Slack panel** — Unread DM, thread, and mention counts at a glance
- **GitHub panel** — Open PR counts per repo with quick actions (merge, approve, label, view)
- **Summary panel** — Aggregated digest of what needs attention

## Requirements

- Go 1.24+
- macOS or Linux

## Getting Started

```bash
make build
make run
```

## Key Bindings

| Key | Action |
|-----|--------|
| `ctrl+\` | Toggle sidebar |
| `ctrl+t` | Focus terminal |
| `ctrl+s` | Focus Slack panel |
| `ctrl+g` | Focus GitHub panel |
| `ctrl+c` | Quit |

## Project Structure

```
cmd/koko/              Entry point
internal/tui/          TUI framework
  root.go              Root model — layout and focus coordination
  messages.go          Custom message types
  styles.go            Shared lipgloss styles
  keymap.go            Key binding constants
  components/
    terminal/          Embedded PTY terminal pane
    sidebar/           Sidebar container
    slack/             Slack awareness panel
    github/            GitHub awareness panel
    summary/           Aggregated summary panel
docs/plans/            Implementation plans
```

## License

[Business Source License 1.1](LICENSE) — source available, non-competing production use allowed. Converts to GPL 2.0+ on 2030-02-27.
