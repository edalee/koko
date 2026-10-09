package main

import (
	"testing"
	"time"
)

func TestSlept(t *testing.T) {
	cases := []struct {
		name       string
		wall, mono time.Duration
		want       float64
	}{
		{"awake", time.Hour, time.Hour, 0},
		{"slept ten minutes", time.Hour, 50 * time.Minute, 600},
		{"wall clock set back", 50 * time.Minute, time.Hour, 0},
	}
	for _, c := range cases {
		if got := slept(c.wall, c.mono); got != c.want {
			t.Errorf("%s: slept = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSleptSinceAwake(t *testing.T) {
	start := time.Now()
	if got := sleptSince(start, start.Add(time.Hour)); got != 0 {
		t.Errorf("sleptSince = %v, want 0 with no sleep", got)
	}
}

func TestActivity(t *testing.T) {
	a := &App{started: time.Now()}
	got := a.Activity()
	// The wall and monotonic clocks are read a few nanoseconds apart.
	if got.SleptSeconds >= 1 {
		t.Errorf("SleptSeconds = %v, want about 0 just after start", got.SleptSeconds)
	}
	if got.IdleSeconds < -1 {
		t.Errorf("IdleSeconds = %v, want -1 or more", got.IdleSeconds)
	}
}

func TestIdleWatch(t *testing.T) {
	var w idleWatch
	for _, idle := range []float64{1, 2, 900, 901, 0.5} {
		w.observe(idle)
	}
	// A time away of 901 seconds ended between two calls.
	if got := w.take(1); got != 901 {
		t.Errorf("take = %v, want 901", got)
	}
	// The next period starts from the last reading.
	if got := w.take(2); got != 2 {
		t.Errorf("take = %v, want 2", got)
	}
	// A time away still under way carries on.
	w.observe(30)
	if got := w.take(31); got != 31 {
		t.Errorf("take = %v, want 31", got)
	}
}
