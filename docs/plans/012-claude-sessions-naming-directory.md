# Plan 012: Claude Sessions with Naming and Directory Picker

## Status: Implemented

## Context

Currently, Koko spawns `zsh -l` shells in PTY sessions. Users manually run `claude` inside them. The goal is to make Koko a **dedicated Claude Code launcher** — each session directly runs `claude` (interactive mode) in a user-chosen directory, with a custom name. The xterm.js terminal stays (Claude's full interactive TUI renders there), but Koko manages session metadata (name, directory, claude session ID) for sidebar display and potential future `--resume` support.

## User Flow

1. User clicks **"+ New Session"** in the sidebar
2. A **creation overlay** appears (reusing the glassmorphism OverlayPage pattern):
   - **Name input**: text field with auto-generated placeholder (e.g. "morning-fox")
   - **Directory picker**: shows recent directories + "Browse..." button (native macOS dialog)
   - **Create button**: launches the session
3. Session appears in sidebar with custom name + directory path
4. Terminal renders Claude Code's full interactive TUI (slash commands, tool approvals, keyboard shortcuts, etc.)

## Changes

### Go Backend

| File | Change |
|------|--------|
| `app.go` | Added `PickDirectory()` — opens native macOS directory dialog via Wails runtime |
| `terminal_manager.go` | `CreateSession(name, dir, cols, rows)` — accepts name + directory, spawns `claude` instead of `zsh -l`, sets working directory. Added `name`/`dir` to `session` struct. Added `GetSessions()` method. |
| `types.go` | Added `SessionInfo` struct (ID, Name, Dir) |

### Frontend

| File | Change |
|------|--------|
| `frontend/src/types/index.ts` | `SessionTab`: replaced `title` with `name` + `directory` |
| `frontend/src/lib/names.ts` | New — random name generator (adjective-noun) |
| `frontend/src/components/NewSessionDialog.tsx` | New — glassmorphism overlay with name input, directory picker (native dialog + recent dirs from localStorage), Create/Cancel, Enter/Escape keyboard shortcuts |
| `frontend/src/hooks/useSessionTabs.ts` | `createTab(name, directory)` accepts name + dir, updates recent dirs. Added `renameTab()`. |
| `frontend/src/components/SessionSidebar.tsx` | Shows `session.name` + shortened directory path. Double-click name for inline rename. Added `onRenameSession` prop. |
| `frontend/src/App.tsx` | Wired `NewSessionDialog`, removed auto-create on startup. "New Session" opens dialog. |

### Key Details

- **Name generation**: adjective-noun format (e.g. "cosmic-falcon", "amber-grove") — no external dependencies
- **Recent directories**: stored in `localStorage` under `koko:recent-dirs`, max 5, most recent first
- **Directory display**: shortened with `~` for home directory (e.g. `~/Projects/koko`)
- **Inline rename**: double-click session name in sidebar, Enter to confirm, Escape to cancel
- **No auto-session**: app starts with empty session list; user must create explicitly

## Verification

1. `make dev` — app starts with empty session list (no auto-created session)
2. Click "+ New Session" → creation overlay appears with name field + directory picker
3. Browse button opens native macOS directory dialog
4. Recent directories appear after first session
5. Create session → Claude Code starts in the chosen directory (full interactive TUI)
6. Session name + directory path visible in sidebar
7. Double-click name → inline rename works
8. Multiple sessions work independently in different directories
9. Close session (X button) → PTY cleaned up properly
10. Escape / backdrop click dismisses creation dialog without creating
