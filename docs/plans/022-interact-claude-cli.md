# Plan 022: Fix /interact PTY input and output

## Problem

The `/interact` endpoint had two issues:

1. **Input: Bulk PTY writes don't submit** — Claude Code's Ink TUI uses
   `readline.emitKeypressEvents()` in raw mode. A bulk `ptmx.Write("hello\r")`
   arrives as a single `data` event — the `\r` isn't recognized as a separate
   Return keypress, so the prompt never submits. Bracketed paste also failed.
   `claude -p --resume` was attempted but only works with `-p` mode sessions,
   not interactive PTY sessions.

2. **Output: ANSI stripping loses spaces** — Claude Code's TUI uses cursor-forward
   sequences (`\x1b[nC`) for column alignment (e.g. `git diff --stat`). The old
   regex stripped these, turning "1 file changed" into "1filechanged".

## Solution

### Input: Character-by-character PTY writes

New `WriteKeystrokes()` method writes one byte at a time to ptmx, matching
exactly what xterm.js does when the user types. Each character arrives as its
own stdin read event, so `\r` correctly fires as a Return keypress.

```go
func (tm *TerminalManager) WriteKeystrokes(sessionID string, text string) error {
    // ... lock, clear approval ...
    for _, b := range []byte(text) {
        buf[0] = b
        ptmx.Write(buf)
    }
}
```

### Output: Smart ANSI stripping with cursor-forward → spaces

Two-pass regex approach:
1. Replace `\x1b[nC` (cursor forward) with n spaces
2. Strip all remaining ANSI escape sequences

This preserves column-aligned content spacing without rendering Claude Code's
full TUI chrome (bird logo, spinners, status bar, tips).

A VT terminal emulator (charmbracelet/x/vt) was tried but rendered the entire
screen including all TUI decorations — too much noise.

## What stays unchanged

- PTY sessions (creation, xterm.js display, scrollback, resize, tab switching)
- `/write` endpoint (raw PTY input — keystrokes, y/n, Ctrl+C)
- Subscriber + quiet timer approach for collecting output
- Approval state guard (409 if waiting for tool approval)
- MCP tools (call `/interact` — transparently upgraded)
- Slack `prompt` command (calls `/interact` — transparently upgraded)

## Approaches tried and rejected

| Approach | Why it failed |
|----------|---------------|
| Bulk PTY write with `\r\n` | Text appeared in input but didn't submit |
| Bracketed paste (`\x1b[200~...\x1b[201~`) | Same — text appeared, `\r` didn't submit |
| `claude -p --resume <uuid>` | `--resume` only works with `-p` mode sessions, not PTY |
| 100ms delay between text and `\r` | Untested — replaced by char-by-char approach |
| VT terminal emulator for output | Rendered full TUI including bird/spinner/tips noise |

## Files changed

- `terminal_manager.go` — added `WriteKeystrokes()` method
- `api_server.go` — `/interact` uses `WriteKeystrokes`, smart `stripANSI()` with cursor-forward handling
- `api_server_test.go` — tests for `WriteKeystrokes`, `stripANSI`, approval state guard
