//go:build !darwin

package main

// idleSeconds returns -1, because Koko reads the idle time only on macOS.
// The break timer then counts all awake time as work.
func idleSeconds() float64 {
	return -1
}
