//go:build live

package main

// Live tests use your real connectors. They are skipped unless you pass
// -tags live. Run: go test -tags live -run TestLive -v

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestLiveFocusBooking books one focus block on Friday 15 January 2027,
// through the same path the focus job uses. Delete the event afterwards:
// the worker itself is not allowed to delete events.
func TestLiveFocusBooking(t *testing.T) {
	cfg := defaultConfig()
	loc := cfg.location()
	gap := Gap{
		Start: time.Date(2027, 1, 15, 10, 0, 0, 0, loc),
		End:   time.Date(2027, 1, 15, 10, 30, 0, 0, loc),
	}
	cr := claudeRunner{settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	booked, err := createFocusBlocks(context.Background(), cr, cfg, []Gap{gap})
	if err != nil {
		t.Fatal(err)
	}
	if len(booked) != 1 || !booked[0].Start.Equal(gap.Start) || !booked[0].End.Equal(gap.End) {
		t.Fatalf("booked %v, want %v", booked, gap)
	}

	events, err := listEvents(context.Background(), cr, cfg, gap.Start)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Description == focusMarker {
			t.Logf("found focus block %s (%s) — delete it by hand", e.ID, e.Summary)
			if got := findFocusGaps(events, cfg.Focus, gap.Start.Add(-3*time.Hour)); len(got) > 0 {
				for _, g := range got {
					if g.Start.Before(gap.End) && g.End.After(gap.Start) {
						t.Errorf("the booked block is still seen as free: %v", g)
					}
				}
			}
			return
		}
	}
	t.Errorf("focus block not found on %s", gap.Start.Format("2006-01-02"))
}
