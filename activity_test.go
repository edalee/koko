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
