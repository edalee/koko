# Plan 014: Subagent Process Monitor

## Context

When Claude Code spawns subagents (via the Agent tool), they appear as child processes of the main `claude` process. The right sidebar has a placeholder "Subagents" module — this plan replaces it with a live process tree monitor.

## Research Findings

### Process Tree Structure

Each Koko session runs `claude` via PTY. `cmd.Process.Pid` gives us the claude PID (because `exec` replaces the login shell). Child processes observed:

| Process | Type | Notes |
|---------|------|-------|
| `claude --session-id ...` | **Subagent** | Spawned by Agent tool, has own session ID |
| `nolo serve` | MCP server | Long-lived, ignore in display |
| `caffeinate -i -t 300` | Sleep prevention | Short-lived timer, ignore |
| `docker mcp gateway run` | MCP server | Long-lived, ignore |
| `node ...` | Tool execution | Short-lived bash/edit commands |

**Key insight:** Subagents are identifiable as child `claude` processes. Infrastructure processes (`nolo`, `caffeinate`, `docker`) should be filtered out.

### What We Can Observe (Process Monitoring)

- Process exists (running) or has exited (completed)
- PID, elapsed time, memory usage
- Command line (contains `--session-id` for subagents)
- Child count of the subagent (shows if it's actively running tools)

### What We Cannot Observe (No API Yet)

- Agent description/purpose (e.g., "searching for files")
- Agent type (Explore, Plan, general-purpose)
- Progress percentage
- Token usage
- Output/result summary

### Future: Claude Code Structured Events

Claude Code may eventually expose agent lifecycle events. When available, this would enable:
- `agent:start` — agent ID, type, description, parent session
- `agent:progress` — tool calls, token count, status updates
- `agent:complete` — result summary, duration, tokens used
- `agent:error` — failure reason

**Integration approach:** Listen for events via stdout JSON lines (`--output-format stream-json`) or a local socket/file. The process monitor built now serves as the fallback when structured events aren't available.

## Design

### Right Sidebar — Subagents Module

```
┌─────────────────────────┐
│ Subagents            ↻  │  ← header + refresh
│ 2 running                │
├─────────────────────────┤
│ ● claude [agent]   12s  │  ← green dot = running
│   └ 3 child processes   │  ← tool activity indicator
│                         │
│ ● claude [agent]    8s  │
│   └ 1 child process    │
│                         │
│ ○ nolo serve    3d 19h  │  ← gray dot = infrastructure
│ ○ docker mcp    3d 19h  │  ← collapsible "infrastructure"
└─────────────────────────┘
```

**States:**
- **No session active:** "No active session"
- **Session active, no subagents:** "No active subagents"
- **Subagents running:** List with green dots, elapsed time, child count
- **Infrastructure processes:** Collapsed section, gray dots, de-emphasized

### Process Classification

```
Subagent:       command starts with "claude" and has --session-id
Infrastructure: command matches "nolo", "caffeinate", "docker mcp"
Tool:           everything else (short-lived, usually node/bash)
```

## Changes

### Go Backend

#### `process_monitor.go` (new)

```go
type ProcessInfo struct {
    PID       int    `json:"pid"`
    Command   string `json:"command"`    // short display name
    FullCmd   string `json:"fullCmd"`    // full command line
    Type      string `json:"type"`       // "subagent", "infrastructure", "tool"
    Elapsed   string `json:"elapsed"`    // human-readable duration
    ElapsedMs int64  `json:"elapsedMs"`  // for sorting
    Children  int    `json:"children"`   // number of child processes
    RSS       int64  `json:"rss"`        // memory in KB
}

type ProcessMonitor struct{}

func NewProcessMonitor() *ProcessMonitor

// GetChildProcesses returns classified child processes for a given PID.
func (pm *ProcessMonitor) GetChildProcesses(pid int) ([]ProcessInfo, error)
```

Implementation:
- `exec.Command("ps", "-o", "pid,ppid,etime,rss,command", "-p", <children>)`
- Parse output, classify each process
- Count grandchildren for each child (`pgrep -P <child_pid>`)
- Return sorted: subagents first, then tools, then infrastructure

#### `terminal_manager.go` — Expose session PID

Add `GetSessionPID(sessionID string) (int, error)` method that returns `s.cmd.Process.Pid`.

### Frontend

#### `frontend/src/hooks/useSubagents.ts` (new)

```ts
interface SubagentInfo {
  pid: number;
  command: string;
  type: "subagent" | "infrastructure" | "tool";
  elapsed: string;
  elapsedMs: number;
  children: number;
  rss: number;
}

function useSubagents(sessionId: string | null) {
  // Poll GetSessionPID + GetChildProcesses every 3 seconds
  // Return { processes, loading, agentCount }
}
```

#### `frontend/src/components/RightSidebar.tsx` — Replace placeholder

Replace the "No active subagents" placeholder with live process list:
- Accept `processes` and `agentCount` as props
- Show subagents with green dot, elapsed time, child count
- Collapsible "Infrastructure" section for nolo/docker/etc
- Poll indicator in header

### Wiring

#### `App.tsx`

- Add `useSubagents(activeTab?.id)` hook
- Pass results to RightSidebar

## Files Summary

| Action | File | What |
|--------|------|------|
| Create | `process_monitor.go` | Process tree scanning + classification |
| Modify | `terminal_manager.go` | Add `GetSessionPID()` method |
| Modify | `types.go` | Add `ProcessInfo` struct |
| Modify | `main.go` | Bind `ProcessMonitor` |
| Create | `frontend/src/hooks/useSubagents.ts` | Polling hook |
| Modify | `frontend/src/components/RightSidebar.tsx` | Live process list |
| Modify | `frontend/src/App.tsx` | Wire hook to sidebar |

## Implementation Order

1. Go: Add `ProcessInfo` to `types.go`
2. Go: Create `process_monitor.go` with `GetChildProcesses()`
3. Go: Add `GetSessionPID()` to `terminal_manager.go`
4. Go: Bind `ProcessMonitor` in `main.go`
5. Frontend: Create `useSubagents.ts` hook
6. Frontend: Update `RightSidebar.tsx` agents module
7. Frontend: Wire in `App.tsx`

## Verification

1. Start a Claude session in Koko
2. Open Subagents module in right sidebar
3. Ask Claude to launch a subagent (e.g., "search the codebase for X")
4. Verify subagent appears with green dot and elapsed time
5. Verify it disappears (or shows completed) when done
6. Verify infrastructure processes show in collapsed section
7. Verify "No active subagents" when nothing is running

## Future: Structured Events API

When Claude Code adds agent lifecycle events, upgrade path:

1. Switch from process polling to event listening
2. Add description, type, progress fields to `SubagentInfo`
3. Show rich status: "Explore agent: searching for API endpoints"
4. Track completion results and token usage
5. Keep process monitor as fallback for non-event sessions

Potential event format (speculative):
```json
{"event": "agent:start", "id": "abc123", "type": "Explore", "description": "Find API endpoints"}
{"event": "agent:tool_use", "id": "abc123", "tool": "Grep", "file": "src/api.ts"}
{"event": "agent:complete", "id": "abc123", "duration_ms": 12400, "tokens": 8500}
```

Integration: watch a file/socket at `~/.claude/events/<session-id>.jsonl` or parse `--output-format stream-json` stdout.
