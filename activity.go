package main

import "time"

// Activity is what the break timer needs to know about the user (plan 033).
type Activity struct {
	// IdleSeconds is the time since the last key press, click or mouse move,
	// in any app. It is -1 where the OS does not say.
	IdleSeconds float64 `json:"idleSeconds"`
	// SleptSeconds is the time the computer has slept since Koko started.
	SleptSeconds float64 `json:"sleptSeconds"`
}

// Activity returns the user's idle time and the time slept since Koko started.
func (a *App) Activity() Activity {
	return Activity{
		IdleSeconds:  idleSeconds(),
		SleptSeconds: sleptSince(a.started, time.Now()),
	}
}

// sleptSince returns the seconds slept between start and now.
//
// Go's monotonic clock on macOS is mach_absolute_time, which stops while the
// Mac sleeps. The wall clock does not stop. So wall time minus monotonic time
// is the time asleep. Both times need their monotonic reading, as time.Now
// gives them.
func sleptSince(start, now time.Time) float64 {
	return slept(now.Round(0).Sub(start.Round(0)), now.Sub(start))
}

// slept returns wall minus monotonic time in seconds. A small wall-clock
// correction can make it negative, so it is clamped at 0.
func slept(wall, mono time.Duration) float64 {
	if wall <= mono {
		return 0
	}
	return (wall - mono).Seconds()
}
