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
- `charm.land/bubbletea/v2` - TUI framework (v2)
- `charm.land/lipgloss/v2` - styling (v2)
- `github.com/charmbracelet/bubbles` - pre-built components (when needed)
- `github.com/creack/pty` - PTY for embedded terminal
- Slack Web API - for DM/thread/mention counts
- GitHub REST API (or `gh` CLI) - for PR data and actions

## Conventions
- Follow Go standard project layout
- Use conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
- Keep PRs small and focused
- `make` targets: `build`, `test`, `lint`, `run`

## Project Structure
- `cmd/koko/` - entry point
- `internal/tui/` - root model, messages, styles, keymap
- `internal/tui/components/` - terminal, sidebar, slack, github, summary
- `docs/plans/` - implementation plans

## Component Pattern
- Each component: `New()`, `Init() tea.Cmd`, `Update(msg) (Model, tea.Cmd)`, `View() string`, `SetSize()`, `SetFocus()`
- Only root returns `tea.View` (v2 requirement); children return `string`
- Root owns children as concrete struct fields (not interfaces)
- No circular imports: components define their own styles inline

## Design Docs
- Implementation plans in `docs/plans/`
- Session memory in Claude memory files (`koko.md`)
