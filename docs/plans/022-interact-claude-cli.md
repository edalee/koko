# Plan 022: Replace PTY-based /interact with `claude -p --resume`

## Problem

The `/interact` endpoint writes text to the PTY and waits for output to settle.
This approach has multiple issues:

1. **Bracketed paste mode** — Claude Code's TUI enables bracketed paste. Raw PTY
   writes appear in the input buffer but don't execute. Wrapping in paste sequences
   and splitting writes doesn't reliably submit.
2. **Quiet timer races** — The phase-1/phase-2 quiet timer fires during gaps between
   Claude's tool calls, returning partial responses.
3. **ANSI stripping** — PTY output is full of escape codes that must be stripped,
   losing formatting context.

## Solution

Replace PTY write + subscriber + quiet timer with `claude -p --resume <uuid>`:

```
POST /api/sessions/:id/interact {"text": "check the tests"}

→ exec: claude -p "check the tests" --resume <uuid> --output-format json
→ parse JSON result
→ return {"output": "...", "state": "idle"}
```

## What stays unchanged

- PTY sessions (creation, xterm.js display, scrollback, resize, tab switching)
- Session recovery (`--resume` / `--continue` on reconnect)
- `/write` endpoint (raw PTY input — keystrokes, y/n, Ctrl+C)
- Approval indicator (PermissionRequest hook still fires for PTY session)
- MCP tools (call `/interact` — transparently upgraded)
- Slack `prompt` command (calls `/interact` — transparently upgraded)

## Implementation

### `api_server.go` — `handleSessionInteract`

Replace the entire subscriber/quiet-timer machinery with:

1. Check `state == "idle"` (reject if approval or not ready)
2. Get `claudeSessionID` from TerminalManager
3. `exec.CommandContext` with timeout:
   ```
   claude -p "<text>" --resume <uuid> --output-format json --bare
   ```
4. Parse JSON response → extract `result` field
5. Return to caller

Environment: `resolveClaudePath()` for binary, `tm.buildEnv()` for PATH.

### Request/response contract (unchanged for callers)

```json
// Request
{"text": "what is the status?", "timeout_ms": 120000}

// Response — 200
{"output": "Here's the status...", "state": "idle"}

// Response — 409
{"error": "session is waiting for approval"}
{"error": "session not ready — Claude session ID not yet captured"}
```

### Deleted code

- Phase-1/phase-2 quiet timer (~80 lines)
- Subscriber channel setup/teardown
- ANSI strip regex usage in interact
- Bracketed paste logic

## Risks and mitigations

| Risk | Mitigation |
|------|------------|
| Concurrent JSONL writes (PTY Claude idle but holding file) | Gate on `state == "idle"`, return 409 if not |
| PTY display divergence (Slack exchange not visible in terminal) | By design. Claude has full context from JSONL |
| No UUID yet (fresh session) | Return 409. Caller retries |
| Tool approvals block `-p` process | `exec.CommandContext` timeout kills it. Consider `--permission-mode acceptEdits` |
| Binary/env resolution | Reuse `resolveClaudePath()` + `tm.buildEnv()` |

## Verification

```bash
# Direct test
curl -X POST http://127.0.0.1:19876/api/sessions/koko-1/interact \
  -H "Authorization: Bearer <key>" \
  -H "Content-Type: application/json" \
  -d '{"text":"what files have changed?"}'

# Slack
prompt koko-1 what files have changed?

# Check logs
tail -f ~/Library/Application\ Support/koko/koko.log | grep interact
```

## Files changed

- `api_server.go` — rewrite `handleSessionInteract`
- `api_server_test.go` — update interact tests
