# Koko Roadmap

## Backlog

Features planned for future implementation, roughly prioritized.

### Code Viewer Enhancements
- [x] **Keyboard navigation** — `↑`/`↓` or `[`/`]` to switch between files in the diff viewer
- [x] **Raw file viewer** — view unmodified files with syntax highlighting (not just diffs)
- [x] **Large file handling** — line count detection + gated loading for 10k+ line diffs
- [x] **Binary file detection** — show "Binary file" message instead of attempting diff
- [ ] **Inline comments** — annotate diff lines (future: code review workflow)

### Terminal
- [x] **Search scrollback** — `Cmd+F` to search terminal output (xterm.js SearchAddon)
- [ ] **Multi-file diff navigation** — PR-style "Files changed" view across commits
- [ ] **Blame view** — git blame overlay for file viewer

### Slack
- [ ] **Channel mentions** — monitor specific channels for keywords/mentions (needs `channels:history` scope)
- [ ] **Reaction support** — quick-react to messages from the bot DM
- [ ] **Real unread tracking** — if Slack API ever exposes unread state for user tokens

### Sessions
- [x] **Conversation picker**: choose a new conversation or one of those stored for a directory (plan 028, steps 1 to 4)
- [x] **Route every entry point through the picker**: worktrees module and disconnected tabs. A tab with no stored conversation preselects nothing (plan 028 step 5, D1)
- [ ] **Reload session**: restart the Claude process in place, keeping the tab, slug and conversation (plan 028 step 6)
- [ ] **Delete conversations**: TODO. Per-row delete in the picker, and "clear all conversations here". Deletes Claude's session file, so it confirms first and skips any conversation a tab holds (plan 028 step 7). Until then, delete `~/.claude/projects/<folder>/<id>.jsonl` by hand
- [ ] **Clear session history**: TODO. Drops Koko's closed-session records only, from Settings (plan 028 step 7)
- [ ] **Remove Koko's worktrees**: TODO. From Settings, never forced, skipping any with uncommitted changes (plan 028 step 7)
- [ ] **Drop `--continue`**: resume by explicit conversation id only, once the picker covers every entry point (plan 028 step 8)
- [ ] **Session context polish** — MCP servers, agents, commands panel refinements
- [ ] **Session grouping** — group sessions by project/directory
- [ ] **Session export** — export terminal scrollback as text/markdown/HTML

### GitHub
- [x] **PR diff viewer** — view PR diffs directly in Koko (reuse code viewer)
- [x] **PR comments** — read and reply to PR review comments
- [x] **CI status** — show GitHub Actions status for active branches

### Design & UX
- [ ] **Themes** — light theme, custom accent colors
- [ ] **Font settings** — configurable terminal font and size
- [ ] **Window management** — remember window size/position across restarts
- [ ] **Notification sounds** — optional audio alerts for Slack DMs and PR reviews

### Infrastructure
- [ ] **Auto-update** — check for new versions and prompt to update
- [ ] **Linux support** — test and fix Linux-specific issues
- [ ] **Homebrew distribution** — `brew install koko`
- [ ] **Remote API enhancements** — file diffs, GitHub PRs, notifications endpoints

## Completed

See `docs/plans/` for detailed implementation plans of completed features (001-029).
