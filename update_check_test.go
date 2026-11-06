package main

import "testing"

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		latest, current string
		want            bool
	}{
		{"0.5.10", "0.5.9", true},
		{"0.5.16", "0.5.15", true},
		{"0.10.0", "0.9.3", true},
		{"1.0", "0.9.9", true},
		{"0.5.9", "0.5.10", false},
		{"0.5.15", "0.5.15", false},
		{"0.5", "0.5.0", false},
	} {
		if got := newerVersion(c.latest, c.current); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}
