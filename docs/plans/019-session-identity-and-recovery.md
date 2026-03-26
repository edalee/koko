# Plan 019: Session Identity, History, and Recovery

## Problems

### 1. Session IDs are useless
`session-1`, `session-10`, `session-9` — auto-incrementing, reset on restart, meaningless to humans and bots. MCP/Slack clients can't reliably reference sessions across app restarts.

### 2. Restart = blank session
Reconnecting with `claude --continue` starts a new PTY with a blank terminal. The user has to wait for Claude to respond before knowing what the session was doing. No context card, no last message, nothing.

### 3. `--continue` can't target a specific session
`claude --continue` resumes the most recent session in the directory. If you had multiple sessions in the same directory, you can't pick which one to resume. But `claude --resume <uuid>` can — and we're not using it.

### 4. History is directory-based, not session-based
Closing two different sessions in `~/project` produces one history entry. Earlier sessions are overwritten. "Recent history" is really "recent directories with the last session's snippet."

### 5. Sessions accumulate instead of upserting
Creating a new session in the same directory doesn't reuse or replace the old one — it creates `session-N+1`. Over time the sidebar fills with disconnected duplicates for the same project.

### 6. No backup or recovery
If the sessions file is corrupted or the app crashes during a write, all session state is lost. No backups, no journaling, no recovery.

## Key Discovery: `claude --resume <uuid>`

Claude Code supports `claude --resume <session-uuid>` to reconnect to a **specific** session by ID. Session UUIDs are stored as filenames in `~/.claude/projects/-{dir-path}/{uuid}.jsonl`. Each message entry also contains a `sessionId` field matching the filename.

This means:
- We can capture the Claude session UUID after spawning a session
- Store it in our session record
- Resume any session precisely with `--resume <uuid>` instead of `--continue`
- Multiple sessions in the same directory are independently recoverable

### Detecting the session UUID

After `CreateSession` spawns `claude` in a directory:
1. Watch `~/.claude/projects/-{dir-with-dashes}/` for the newest `.jsonl` file
2. The filename (minus `.jsonl`) is the session UUID
3. Store it in the SessionRecord

For reconnects: `claude --resume <stored-uuid>` instead of `claude --continue`.

## Design

### Session Identity

Replace `session-N` with **directory-scoped slugs**: `{dir-slug}/n`.

```
~/Projects/koko      → koko/1, koko/2
~/Projects/drumstick → drumstick/1
/tmp                 → tmp/1
```

- `dir-slug` is the last path component (or last two if ambiguous)
- `/n` is the session number within that directory, incrementing per-directory
- Displayed as `koko/1` in the UI and API — human-readable, bot-friendly
- Stored with the full directory path for uniqueness

### Session Record

Replace `SavedSessionTab` + `SavedSessionHistory` with a unified model:

```go
type SessionRecord struct {
    Slug           string `json:"slug"`           // e.g. "koko/1"
    Name           string `json:"name"`           // display name
    Directory      string `json:"directory"`      // full path
    ClaudeSessionID string `json:"claudeSessionId"` // UUID from Claude Code
    CreatedAt      int64  `json:"createdAt"`
    ClosedAt       int64  `json:"closedAt,omitempty"`
    Status         string `json:"status"`         // "active", "disconnected", "closed"
    LastMsg        string `json:"lastMsg,omitempty"`
}
```

- **Active**: PTY running, session visible in sidebar
- **Disconnected**: PTY dead, session still in sidebar, can reconnect via `--resume <uuid>`
- **Closed**: User explicitly closed, moves to history list

### Multiple Sessions Per Directory

A directory can have multiple sessions (e.g. `koko/1`, `koko/2`). This supports:
- Running separate sessions for different tasks in the same repo
- Bot creating a session in a dir that already has one
- Each session tracks its own Claude UUID for independent recovery

### Session Recovery with `--resume`

When reconnecting a disconnected session:
1. Look up the stored `ClaudeSessionID`
2. Spawn `claude --resume <uuid>` instead of `claude --continue`
3. Claude restores that exact conversation, not just "most recent in dir"
4. If the UUID is missing (legacy sessions), fall back to `--continue`

### Session Context Card

Instead of replaying raw terminal buffer (which gets overwritten by Claude's TUI), show a **context card** while Claude reconnects:

- Last assistant message (from `GetLastMessage()` — already exists)
- Directory + branch name
- Time since last activity
- Session name

Displayed as a semi-transparent overlay on the terminal pane while the PTY is loading. Fades out once Claude produces output.

### Session Upsert on New

When the user creates a "new" session in a directory that already has disconnected sessions:
- Show a prompt: "Reconnect existing session `koko/1`?" with option list of disconnected sessions
- "New session" option creates `koko/2`
- Choosing reconnect resumes via `--resume <uuid>`

### Sidebar: Grouped by Directory

The session sidebar groups sessions under their directory name with nested indentation:

```
▾ koko
    Session 1          ● active
    Bug fix            ○ disconnected
▾ drumstick
    Session 1          ● active
▸ baldrick (1 closed)
```

- Directory header: folder icon + last path component, collapsible
- Sessions indented under their directory, showing name + status dot
- Active = green dot, disconnected = amber dot, closed = grey (in history)
- Collapsed directories show count badge
- "New Session" button at top creates in chosen directory
- Right-click directory header → "New session here"
- Drag sessions between directories not supported (would change working dir)

### History

History shows all closed sessions (not just one per directory). Capped at 50 entries. Searchable by name and directory. Each entry shows last message snippet and timestamp.

### Backup and Recovery

**Write-ahead approach:**
1. Before writing `sessions.json`, write to `sessions.json.new`
2. Rename `sessions.json` → `sessions.json.bak`
3. Rename `sessions.json.new` → `sessions.json`

If the app crashes mid-write, recovery on startup:
- If `sessions.json` exists and is valid → use it
- If `sessions.json` is corrupt but `sessions.json.bak` exists → restore from backup
- If both are missing → start fresh

**Periodic backup:**
- On app startup, copy `sessions.json` → `sessions.json.{date}` if the existing backup is >24h old
- Keep last 3 dated backups

## API Changes

### Session list response
```json
[
  {"slug": "koko/1", "name": "Feature work", "dir": "/Users/ed/Projects/koko", "status": "active", "claudeSessionId": "e9088da2-..."},
  {"slug": "koko/2", "name": "Bug fix", "dir": "/Users/ed/Projects/koko", "status": "disconnected", "claudeSessionId": "8deec030-..."}
]
```

### MCP/Slack/CLI commands
- `list_sessions` → returns slugs
- `send koko/1 hello` → uses slug
- `status koko/1` → works
- `resume koko/2` → reconnects via `--resume <uuid>`

### Backwards compatibility
- Old `session-N` IDs still accepted during transition
- API returns both `id` (PTY ID) and `slug` (human-friendly)

## Implementation Order

1. **Claude UUID capture** — `terminal_manager.go`: after spawning claude, detect session UUID from JSONL file
2. **Session slug generation** — `terminal_manager.go`: `{dir-slug}/n` IDs, per-directory counters
3. **SessionRecord type** — `types.go`: unified record with slug, UUID, status
4. **`--resume` support** — `terminal_manager.go`: use stored UUID for reconnects
5. **Atomic writes + backup** — `config_service.go`: write-ahead saves, crash recovery
6. **Session context card** — `TerminalPane.tsx`: overlay with last message while reconnecting
7. **Frontend: upsert dialog** — `NewSessionDialog.tsx`: reconnect prompt for existing sessions
8. **Frontend: grouped sidebar** — `SessionSidebar.tsx`: group sessions by directory, collapsible headers, nested indentation, status dots
9. **Frontend: richer history** — `useSessionTabs.ts`, `SessionSidebar.tsx`: full session history with search
10. **API: slug support** — `api_server.go`, `mcp_tools.go`, `slack_commands.go`, `cmd/koko-cli/`

## Modified Files
- `terminal_manager.go` — UUID capture, slug generation, `--resume` support
- `config_service.go` — atomic writes, backup, SessionRecord storage
- `types.go` — SessionRecord replaces SavedSessionTab + SavedSessionHistory
- `api_server.go` — slug in responses and routing
- `mcp_tools.go` — slug in tool descriptions
- `slack_commands.go` — slug in command parsing
- `cmd/koko-cli/main.go` — slug in commands
- `frontend/src/hooks/useSessionTabs.ts` — unified session state, UUID storage, reconnect upsert
- `frontend/src/components/TerminalPane.tsx` — context card overlay
- `frontend/src/components/NewSessionDialog.tsx` — reconnect prompt
- `frontend/src/components/SessionSidebar.tsx` — richer session display
