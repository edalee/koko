//go:build !darwin

package main

// idleSeconds returns -1: only macOS reports the time since the last input.
// The break timer then counts all awake time as work.
func idleSeconds() float64 {
	return -1
}
