# Plan 016: Safe Working Mode

## Context

Two wellbeing features to help users maintain healthy work habits:

1. **Quiet Hours** — Block app access during configured time windows (e.g., evenings, weekends)
2. **Break Reminders** — Scheduled breaks at intervals (e.g., 15min break every 1h30)

## User Flow

### Quiet Hours
1. User opens Settings → Safe Working section
2. Toggles "Quiet Hours" on
3. Configures start time (e.g., 18:00) and end time (e.g., 08:00)
4. When current time is within the quiet window, the app shows a full-screen overlay blocking all interaction
5. Overlay shows a calming message + countdown to when access resumes
6. No way to dismiss (the point is to enforce boundaries)

### Break Reminders
1. User opens Settings → Safe Working section
2. Toggles "Break Reminders" on
3. Configures work duration (e.g., 1h30) and break duration (e.g., 15min)
4. Timer starts when a session is active
5. When work duration elapses, a break overlay appears covering the app
6. Break overlay shows countdown timer for the break duration
7. When break timer completes, overlay auto-dismisses and work timer resets
8. Optional: "Skip this break" button (with a gentle nudge message)

## Changes

### Go Backend

#### `config_service.go` — Extend AppConfig
```go
type SafeWorkingConfig struct {
    QuietHoursEnabled bool   `json:"quietHoursEnabled"`
    QuietHoursStart   string `json:"quietHoursStart"` // "HH:MM" format
    QuietHoursEnd     string `json:"quietHoursEnd"`   // "HH:MM" format
    BreakEnabled      bool   `json:"breakEnabled"`
    WorkMinutes       int    `json:"workMinutes"`  // e.g., 90
    BreakMinutes      int    `json:"breakMinutes"` // e.g., 15
}

type AppConfig struct {
    SlackToken   string            `json:"slackToken"`
    GitHubRepos  []string          `json:"githubRepos"`
    SlackEnabled bool              `json:"slackEnabled"`
    SafeWorking  SafeWorkingConfig `json:"safeWorking"`
}
```

No new Go methods needed — config is read/saved via existing `GetConfig`/`SaveConfig`.

### Frontend

#### `frontend/src/hooks/useSafeWorking.ts` (new)
- Reads `SafeWorkingConfig` from backend config on mount
- **Quiet Hours**: checks current time against start/end every minute, returns `isQuietHours: boolean`
- **Break Timer**: tracks work elapsed time, returns `isBreakTime: boolean`, `breakSecondsLeft: number`, `workSecondsLeft: number`
- `skipBreak()` — dismisses current break, resets work timer
- Pauses work timer when no active session or during breaks
- Persists timer state to avoid reset on re-render

#### `frontend/src/components/SafeWorkingOverlay.tsx` (new)
Two overlay modes:

**Quiet Hours overlay:**
- Full-screen dark overlay with glassmorphism
- "Time to rest" message with moon/stars icon
- Shows when access resumes (countdown)
- Blocks all interaction (high z-index, no dismiss)

**Break overlay:**
- Full-screen overlay (slightly lighter than quiet hours)
- "Take a break" message with coffee/stretch icon
- Circular countdown timer (visual + numeric)
- Progress ring showing break time remaining
- "Skip this break" button (small, de-emphasized)
- Auto-dismisses when break timer completes

#### `frontend/src/components/SettingsPanel.tsx` — Add Safe Working section
- New section below Slack Integration
- **Quiet Hours** toggle + time pickers (start/end)
- **Break Reminders** toggle + duration inputs (work mins / break mins)
- Preset buttons: "90/15", "60/10", "45/5" for quick selection
- Save triggers `SaveConfig` with updated SafeWorkingConfig

#### `frontend/src/App.tsx` — Wire overlay
- Render `SafeWorkingOverlay` at highest z-index
- Pass quiet hours and break state from `useSafeWorking` hook

## Files Summary

| Action | File | What |
|--------|------|------|
| Modify | `config_service.go` | Add SafeWorkingConfig to AppConfig |
| Create | `frontend/src/hooks/useSafeWorking.ts` | Quiet hours check + break timer logic |
| Create | `frontend/src/components/SafeWorkingOverlay.tsx` | Full-screen overlays for quiet hours + breaks |
| Modify | `frontend/src/components/SettingsPanel.tsx` | Safe working settings UI |
| Modify | `frontend/src/App.tsx` | Wire SafeWorkingOverlay |

## Implementation Order

1. Go: Extend AppConfig with SafeWorkingConfig
2. Frontend: Create useSafeWorking hook
3. Frontend: Create SafeWorkingOverlay component
4. Frontend: Update SettingsPanel with safe working controls
5. Frontend: Wire overlay in App.tsx

## Verification

1. Settings: toggle quiet hours on, set start/end times → config persists
2. Settings: toggle break reminders on, set 1min work / 30s break → verify cycle works
3. Quiet hours: set window around current time → overlay blocks app
4. Quiet hours: set window in the past → no overlay
5. Break: work timer counts down, break overlay appears, countdown completes, auto-dismiss
6. Break: "Skip this break" works, resets work timer
7. Both disabled by default — no impact on existing users
