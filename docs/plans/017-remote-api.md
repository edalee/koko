# Plan 017: Remote Access — API Server, MCP, Slack Commands, CLI

## Overview
Exposes Koko's functionality via HTTP/WebSocket API, enabling remote access through Claude Code (MCP), Slack DMs, and a CLI companion.

## Architecture
- `api_server.go` — HTTP/WS API on port 19876, Bearer token auth
- `mcp_server.go` + `mcp_tools.go` — stdio MCP server (`koko mcp` subcommand)
- `slack_commands.go` — Polls bot DMs for commands, responds via chat.postMessage
- `cmd/koko-cli/` — Standalone CLI companion binary

## API Endpoints
| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/health` | Health check (no auth) |
| GET | `/api/sessions` | List sessions |
| GET | `/api/sessions/:id` | Session detail + state |
| POST | `/api/sessions` | Create session |
| POST | `/api/sessions/:id/write` | Send text input to PTY |
| GET | `/api/sessions/:id/output` | Read recent output |
| GET | `/api/sessions/:id/state` | Get approval/idle state |
| DELETE | `/api/sessions/:id` | Close session |
| WS | `/api/sessions/:id/stream` | Stream PTY output |
| GET | `/api/files?dir=` | Git file changes |

## MCP Tools
list_sessions, get_session_state, read_output, send_input, create_session, close_session, list_file_changes

## Slack Commands
All commands require a `koko` prefix to avoid responding to normal DMs:
`koko sessions`, `koko status`, `koko send`, `koko files`, `koko output`, `koko help`

## CLI Commands
sessions, status, send, output, tail, files

## Key Changes
- `terminal_manager.go` — subscriber fan-out for API/WebSocket consumers
- `config_service.go` — API key generation, port config, CLI config file
- `app.go` — API server lifecycle, MCP server auto-registration
- `main.go` — MCP subcommand detection, service wiring
- `SettingsPanel.tsx` — API toggle, port, key display with copy

## Performance Notes

### Subscriber fan-out is lossy for slow consumers
The `select/default` pattern in `readLoop` drops PTY chunks when a subscriber's channel buffer (256 slots) is full. This is the correct tradeoff — PTY reads must never block — but a slow `koko-cli tail` client will miss data.

**If this becomes a problem:** Add a per-subscriber ring buffer so slow consumers get the most recent N bytes rather than gaps. Or increase channel buffer size.

### Session output window is 2KB
`ReadOutput` returns the last ~2KB of ANSI-stripped text from `tailText`. For sessions with heavy output, this is a narrow window for MCP/CLI consumers.

**If more history is needed:** Enable the `buf` ring buffer (currently only on QuickTerminal sessions at 64KB) on Claude sessions too, or write output to a temp file for full-history access.

### Slack command polling — IM channel caching
`conversations.list` is called once at init and refreshed every 5 minutes (not every poll cycle). Each poll calls `conversations.history` once per cached IM channel. With N active DM threads, that's N API calls per 5-second cycle.

**Rate limit math:** Slack Tier 3 allows ~50 requests/minute. With 10 DM channels, that's 10 calls/5s = 120/min — over limit. For heavy DM users, increase poll interval or switch to Slack Events API (requires webhook server).

### No request body size limits on API
POST endpoints use `json.NewDecoder(r.Body).Decode()` without size caps. A client could send an arbitrarily large payload.

**Mitigation:** API only listens on 127.0.0.1 and requires auth. If exposed to a network in future, add `http.MaxBytesReader(w, r.Body, 1<<20)` (1MB cap).
