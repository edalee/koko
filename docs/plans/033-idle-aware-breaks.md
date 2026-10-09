# Plan 033: Break reminders that follow you

## Problem

The break timer (plan 016) does not track your real screen time:

- It adds one second per timer tick. WKWebView slows or stops the timers when Koko's window is hidden, and they stop while the Mac sleeps. Ninety minutes of work can count as far less, so the break comes late or never.
- Work means "a session tab is open". Time away from the desk counts as work.
- Time away does not count as a break. After 20 minutes away, the next break still comes on the old schedule.

## Goal

A break is a rest for your eyes, away from the keyboard. So:

- Work is time you are active at the Mac, in any app.
- Time away as long as a break counts as the break, and the work cycle starts again.
- A break only counts down while Koko has focus, or while you are away.

## Signals

`App.Activity()` returns three numbers to the frontend:

| Field | Meaning |
|---|---|
| `idleSeconds` | Seconds since the last key press, click or mouse move, in any app. -1 where unknown (Linux) |
| `peakIdleSeconds` | The longest idle time since the last call. The backend reads the idle time every second, because WKWebView can delay the frontend's timer while Koko is hidden. A time away that ended between two samples still counts |
| `sleptSeconds` | Seconds the Mac has slept since Koko started: wall-clock time minus monotonic time. Go's monotonic clock on macOS stops during sleep |

The idle time comes from IOKit's `HIDIdleTime`, which needs no permission.

## Rules

The frontend samples every second. For each sample, `elapsed` is the wall-clock time since the last one, and `slept` is the growth in `sleptSeconds`. `awake` is `elapsed - slept`.

You are **active** if your last input was under 60 seconds ago. A short pause to read still counts as work.

**Away run:** the time since your last input, including sleep. Fresh input ends it. When the away run reaches the break length, it was a break: the work cycle starts again, and any break due or under way ends.

**Work phase:**
- If idle is unknown, all awake time counts as work.
- If you are active, all awake time counts as work. This covers a throttled gap with recent input.
- If the peak idle time shows a time away of 60 seconds or more that ended in this sample, that time does not count. A peak as long as a break starts a new cycle.
- If you are not active, only the awake time before your last input counts.
- When work reaches the work length, the break is due.

**Break phase:**
- The break counts down while Koko has focus, or while you are away (idle 60 seconds or more, or asleep).
- If another app has focus and you are active, the break waits until you come back to Koko.
- When the countdown ends, the work cycle starts again. "Skip this break" also starts it again.

**Quiet hours** pause the timer, as before.

**No open session needed.** The timer runs whenever break reminders are on, because they are about your screen time, not Koko's sessions.

## Files

- `idle_darwin.go` (new): `HIDIdleTime` from IOKit, via cgo.
- `idle_other.go` (new): returns -1.
- `activity.go` (new): `App.Activity()` and the sleep maths.
- `frontend/src/lib/breaks.ts` (new): the rules above, as a pure function.
- `frontend/src/hooks/useSafeWorking.ts`: samples `Activity()` and calls the function.
- `frontend/src/App.tsx`: no session gate.

## Not tested

Sleep needs a real Mac to sleep, so it is checked by hand: close the lid for longer than the break.

## Later

A break that comes due while Koko is hidden waits for you to look at Koko. A macOS notification (Wails 2.16 has them) could tell you sooner.
