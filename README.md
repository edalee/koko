# kōkō is a Tui (Tūī Bird)

A terminal application that serves as a unified workspace for Claude Code sessions, with integrated GitHub and Slack awareness panels.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features (planned)

- **Terminal pane** — Full PTY running zsh, Claude Code, vim, or any CLI tool
- **Slack panel** — Unread DM, thread, and mention counts at a glance
- **GitHub panel** — Open PR counts per repo with quick actions (merge, approve, label, view)
- **Summary panel** — Aggregated digest of what needs attention
- **Keyboard-driven** — tmux-inspired navigation, collapsible sidebar

## Requirements

- Go 1.22+
- macOS or Linux

## Getting Started

```bash
make build
./koko
```

## License

MIT
