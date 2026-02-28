# Plan 003: Embedded PTY Terminal

**Status:** Completed
**Date:** 2026-02-28

## Context

The terminal pane is currently a static placeholder. To make koko usable, it needs to run a real shell (zsh) inside the terminal pane, with full ANSI rendering support so TUI apps like Claude Code and vim work correctly.

## Architecture

Three layers working together:

```
Input:  tea.KeyPressMsg → keyToBytes() → pty master fd → zsh
Output: zsh → pty master fd → goroutine → vt.Write() → vt.Render() → View()
```

**Libraries:**
- `github.com/creack/pty` v1.1.24 — PTY creation, resize (SIGWINCH)
- `github.com/charmbracelet/x/vt` — VT terminal emulator from Charm ecosystem. Parses ANSI, maintains 2D cell grid, `Render()` returns styled ANSI string

**Why `x/vt`** over vt10x or bubbleterm:
- Same ultraviolet key constants as bubbletea (values identical, eases key encoding)
- `Render()` returns ANSI strings directly usable in lipgloss
- Active Charm ecosystem development
- Note: experimental (no tagged releases, pseudo-version only) — acceptable for personal project

## Key Design Decisions

### Shared mutable state behind a pointer

Bubble Tea uses value-copy semantics — `Update()` returns a new `Model`. But PTY state (file descriptors, goroutines, VT emulator) can't be copied. Solution: heap-allocate shared state:

```go
type ptyState struct {
    ptmx     *os.File
    cmd      *exec.Cmd
    vt       *vt.Emulator
    mu       sync.Mutex   // protects vt (goroutine writes, View reads)
    outputCh chan struct{} // signals new PTY output (buffered, 1)
    exitCh   chan error    // signals shell exit (buffered, 1)
}

type Model struct {
    width, height int
    focused       bool
    state         *ptyState // shared across value copies
}
```

### Async bridge: goroutine → Bubble Tea

Continuous Cmd subscription pattern:
1. `Init()` returns `startShell` Cmd → starts PTY, goroutine, returns `outputMsg`
2. `Update(outputMsg)` returns `waitForOutput()` Cmd (blocks on channel)
3. Goroutine reads PTY → writes to VT → signals channel (non-blocking select/default)
4. `waitForOutput()` unblocks → returns `outputMsg` → go to step 2

Same pattern for exit notification via `exitCh`.

### Key encoding

`tea.KeyPressMsg` → raw bytes for PTY. Replicate the encoding logic from `vt/key.go:SendKey()` but write to a string (not VT pipe). Both packages use identical ultraviolet constant values (`tea.KeyUp == uv.KeyUp`, `tea.ModCtrl == uv.ModCtrl`), so the switch-case translates directly.

### Shell startup

Start with default 80x24 size in `Init()`, resize on first `WindowSizeMsg`. Simpler than deferring startup until we know the real size.

### Cursor rendering

Skip in v1 — shell apps (zsh, vim, Claude Code) draw their own cursors. The VT emulator tracks cursor position for future use.

## Changes

### New files

| File | Purpose |
|------|---------|
| `internal/tui/components/terminal/pty.go` | `ptyState` struct, `startShell()` Cmd, `readLoop()` goroutine, `waitForOutput()`/`waitForExit()` Cmds, `Close()` |
| `internal/tui/components/terminal/keyencode.go` | `keyToBytes(tea.KeyPressMsg) string` — key-to-raw-bytes encoding |
| `internal/tui/components/terminal/messages.go` | `outputMsg struct{}`, `exitedMsg struct{ Err error }` |

### Modified files

| File | Changes |
|------|---------|
| `internal/tui/components/terminal/model.go` | Add `state *ptyState` field. Rewrite `Init()`, `Update()`, `View()`, `SetSize()`. Add `Close()`. |
| `internal/tui/root.go` | `Init()` → return `m.terminal.Init()`. Route terminal messages. Call `terminal.Close()` on quit. |
| `go.mod` | Add `github.com/creack/pty`, `github.com/charmbracelet/x/vt`. Replace directive for patched x/vt. |
| `_patches/vt/screen.go` | Fix `Buffer.Touched` → `Screen.touched` field (ultraviolet API break). |
| `_patches/vt/csi_mode.go` | Same fix — `e.scr.buf.Touched` → `e.scr.touched`. |

### Data flow detail

**Update()** handles:
- `tea.KeyPressMsg` → encode to bytes → write to PTY master fd
- `outputMsg` → re-subscribe to output + exit channels via `tea.Batch`
- `exitedMsg` → mark state as exited

**View()** reads:
- Lock mutex → `vt.Render()` → unlock → render inside border box

**SetSize()** propagates:
- Resize VT emulator grid (inner size = total - 2 for border)
- `pty.Setsize()` sends SIGWINCH to child process

**readLoop() goroutine:**
- Reads PTY master in 32KB chunks
- Feeds to `vt.Write()` under mutex
- Signals `outputCh` with select/default (coalesces rapid output)

### Key encoding (`keyencode.go`)

Mirror the switch-case from `vt/key.go:SendKey()`, using `tea.*` constants:
- Ctrl+a-z → `\x01`-`\x1a`
- Enter → `\r`, Tab → `\t`, Backspace → `\x7f`, Escape → `\x1b`
- Arrow keys → `\x1b[A/B/C/D`
- F1-F12 → standard VT sequences
- Printable chars → `key.Text` or `string(key.Code)`
- Alt modifier → prepend `\x1b`

## Implementation Notes

**x/vt compatibility patch:** The upstream `x/vt` module (pseudo-version `20260225`) references `uv.Buffer.Touched`, a field that was moved to `uv.RenderBuffer` in the ultraviolet version required by bubbletea v2 (`20260205`). A local patch in `_patches/vt/` moves the `Touched` tracking to a standalone field on `Screen`. This doesn't affect `Render()` (which renders all lines unconditionally) — only the `Draw()` optimization path. The `replace` directive in `go.mod` points to this local copy.

**No application cursor mode awareness:** `keyToBytes()` always sends normal-mode arrow key sequences (`\x1b[A`) rather than querying the VT emulator's cursor keys mode. This is acceptable for initial use — most shells work fine with normal mode sequences.

## Known Limitations

**Intercepted keys:** ctrl+t, ctrl+s, ctrl+g, ctrl+\\, ctrl+c are captured as global hotkeys and never reach the shell. This means:
- ctrl+c can't send SIGINT to shell processes
- ctrl+s can't be used for forward search in bash/zsh
- ctrl+t can't transpose characters

Future improvement: tmux-style prefix key or double-tap pattern.

## Verification

1. `go build ./...` compiles with new deps
2. `make run` → terminal pane shows zsh prompt (not placeholder)
3. Type `echo hello` + Enter → see output
4. Type `ls` → see colored directory listing (ANSI colors work)
5. Arrow keys work (command history, cursor movement)
6. Resize window → shell reflows correctly
7. ctrl+t/s/g still switch focus (not sent to shell)
8. ctrl+\\ still toggles sidebar
9. ctrl+c still quits koko
10. `/design` snapshot test → layout dimensions still correct
11. Run `vim` inside terminal → basic functionality works
