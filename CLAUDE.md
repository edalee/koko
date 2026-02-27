# Koko (Tūī) - Claude Instructions

## Project
A terminal application (TUI) that serves as a unified workspace. Primary use: running Claude Code sessions with integrated GitHub and Slack awareness panels.

**Repo:** `github.com/edalee/koko`
**Language:** Go
**TUI Framework:** Bubble Tea (charmbracelet/bubbletea) + Lip Gloss + Bubbles
**Target OS:** macOS, Linux

## Architecture
- **Main terminal pane** (left) - PTY running zsh, supports Claude Code, vim, executables
- **Collapsible sidebar** (right) - stacked panels: Slack, GitHub, Summary
- **Keyboard-driven** - tmux-inspired keybindings

## Key Dependencies
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - styling
- `github.com/charmbracelet/bubbles` - pre-built components
- `github.com/creack/pty` - PTY for embedded terminal
- Slack Web API - for DM/thread/mention counts
- GitHub REST API (or `gh` CLI) - for PR data and actions

## Conventions
- Follow Go standard project layout
- Use conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
- Keep PRs small and focused
- `make` targets: `build`, `test`, `lint`, `run`

## Design Docs
- Layout and panel specs are in Claude memory (`design.md`)
- Architecture decisions tracked as ADRs in Claude memory (`decisions.md`)
