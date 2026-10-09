package main

import (
	"context"
	"sync"
	"time"
)

// Activity is what the break timer needs to know about the user (plan 033).
type Activity struct {
	// IdleSeconds is the time since the last key press, click or mouse move,
	// in any app. It is -1 where the OS does not say.
	IdleSeconds float64 `json:"idleSeconds"`
	// PeakIdleSeconds is the longest idle time seen since the last call. A
	// late frontend timer can miss a time away that ended between its checks,
	// so the backend watches for it.
	PeakIdleSeconds float64 `json:"peakIdleSeconds"`
	// SleptSeconds is the time the computer has slept since Koko started.
	SleptSeconds float64 `json:"sleptSeconds"`
}

// Activity returns the user's idle time, the longest idle time since the
// last call, and the time slept since Koko started.
func (a *App) Activity() Activity {
	idle := idleSeconds()
	return Activity{
		IdleSeconds:     idle,
		PeakIdleSeconds: a.idle.take(idle),
		SleptSeconds:    sleptSince(a.started, time.Now()),
	}
}

// idleWatch keeps the longest idle time seen between two Activity calls.
type idleWatch struct {
	mu   sync.Mutex
	peak float64
}

// observe records one idle reading.
func (w *idleWatch) observe(idle float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if idle > w.peak {
		w.peak = idle
	}
}

// take returns the longest idle time since the last take, this reading
// included. The next period starts from this reading, because a time away
// still under way carries on into it.
func (w *idleWatch) take(idle float64) float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	peak := max(w.peak, idle)
	w.peak = idle
	return peak
}

// watchIdle reads the idle time every second until ctx ends. Go timers keep
// running while WKWebView slows the frontend's timers in a hidden window.
func (a *App) watchIdle(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.idle.observe(idleSeconds())
		}
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
