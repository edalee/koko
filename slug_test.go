package main

import "testing"

func TestSplitSlug(t *testing.T) {
	cases := []struct {
		slug string
		base string
		n    int
		ok   bool
	}{
		{"koko-1", "koko", 1, true},
		{"koko-12", "koko", 12, true},
		{"my-repo-3", "my-repo", 3, true},
		{"koko", "", 0, false},   // no number
		{"koko-", "", 0, false},  // nothing after the dash
		{"-1", "", 0, false},     // no base
		{"koko-0", "", 0, false}, // slugs start at 1
		{"koko-x", "", 0, false},
	}
	for _, c := range cases {
		base, n, ok := splitSlug(c.slug)
		if ok != c.ok || base != c.base || n != c.n {
			t.Errorf("splitSlug(%q) = (%q, %d, %v), want (%q, %d, %v)",
				c.slug, base, n, ok, c.base, c.n, c.ok)
		}
	}
}

// dirSlug is the basename, so two directories can share one base. Counting
// per directory handed "koko-1" to both, and a slug has to name one session.
func TestNextSlug_CountsByBaseNotDirectory(t *testing.T) {
	tm := newTestManager()
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if got := tm.nextSlug("/a/koko"); got != "koko-1" {
		t.Fatalf("got %q", got)
	}
	if got := tm.nextSlug("/b/koko"); got != "koko-2" {
		t.Fatalf("a second directory reused the slug: %q", got)
	}
	if got := tm.nextSlug("/c/other"); got != "other-1" {
		t.Fatalf("got %q", got)
	}
}

func TestSeedSlugs_RaisesTheCounterPastWhatIsSaved(t *testing.T) {
	tm := newTestManager()
	// A saved koko-3 must not be handed out again after a restart.
	tm.SeedSlugs([]string{"koko-1", "koko-3", "other-2", "", "not-a-slug"})

	tm.mu.Lock()
	defer tm.mu.Unlock()
	if got := tm.nextSlug("/a/koko"); got != "koko-4" {
		t.Errorf("got %q, want koko-4", got)
	}
	if got := tm.nextSlug("/a/other"); got != "other-3" {
		t.Errorf("got %q, want other-3", got)
	}
	// "not-a-slug" splits to base "not-a" with n=0, which is rejected, so it
	// must not have reserved anything.
	if got := tm.nextSlug("/a/not-a"); got != "not-a-1" {
		t.Errorf("got %q, want not-a-1", got)
	}
}

func TestSeedSlugs_NeverLowersTheCounter(t *testing.T) {
	tm := newTestManager()
	tm.SeedSlugs([]string{"koko-5"})
	tm.SeedSlugs([]string{"koko-2"})

	tm.mu.Lock()
	defer tm.mu.Unlock()
	if got := tm.nextSlug("/a/koko"); got != "koko-6" {
		t.Errorf("got %q, want koko-6", got)
	}
}

// addSlugSession registers a session holding a slug, alive or dead.
func addSlugSession(tm *TerminalManager, id, slug string, alive bool) {
	s := &session{
		id:          id,
		slug:        slug,
		done:        make(chan struct{}),
		subscribers: make(map[chan []byte]struct{}),
	}
	if !alive {
		close(s.done)
	}
	tm.mu.Lock()
	tm.sessions[id] = s
	tm.mu.Unlock()
}

// Reconnecting koko-1 finds the old koko-1 still in the map, because readLoop
// never removes an exited session. That entry is defunct, so the slug is
// reused and the dead entry evicted, rather than two sessions sharing it.
func TestClaimSlug_ReusesASlugHeldOnlyByADeadSession(t *testing.T) {
	tm := newTestManager()
	addSlugSession(tm, "old", "koko-1", false)

	tm.mu.Lock()
	slug, evicted := tm.claimSlugLocked("koko-1", "/a/koko")
	_, stillThere := tm.sessions["old"]
	tm.mu.Unlock()

	if slug != "koko-1" {
		t.Fatalf("expected the slug back, got %q", slug)
	}
	if len(evicted) != 1 || stillThere {
		t.Fatalf("expected the dead holder evicted, got %d evicted, stillThere=%v", len(evicted), stillThere)
	}
}

// Legacy migration gives every tab in a directory "<base>-1", so two tabs can
// ask for the same slug. The second one must not share it.
func TestClaimSlug_FallsBackWhenALiveSessionHoldsIt(t *testing.T) {
	tm := newTestManager()
	addSlugSession(tm, "live", "koko-1", true)
	tm.SeedSlugs([]string{"koko-1"})

	tm.mu.Lock()
	slug, evicted := tm.claimSlugLocked("koko-1", "/a/koko")
	tm.mu.Unlock()

	if slug == "koko-1" {
		t.Fatal("two live sessions share koko-1")
	}
	if slug != "koko-2" {
		t.Errorf("expected koko-2, got %q", slug)
	}
	if len(evicted) != 0 {
		t.Errorf("nothing should be evicted when a live holder exists, got %d", len(evicted))
	}
}

// A dead holder must not be evicted when a live session also holds the slug,
// or it would be dropped from the map without being closed.
func TestClaimSlug_EvictsNothingWhenAnyHolderIsLive(t *testing.T) {
	tm := newTestManager()
	addSlugSession(tm, "dead", "koko-1", false)
	addSlugSession(tm, "live", "koko-1", true)

	tm.mu.Lock()
	_, evicted := tm.claimSlugLocked("koko-1", "/a/koko")
	_, deadStill := tm.sessions["dead"]
	tm.mu.Unlock()

	if len(evicted) != 0 || !deadStill {
		t.Fatalf("expected nothing evicted, got %d, deadStill=%v", len(evicted), deadStill)
	}
}

func TestClaimSlug_EmptyRequestTakesTheNextFree(t *testing.T) {
	tm := newTestManager()
	tm.mu.Lock()
	slug, _ := tm.claimSlugLocked("", "/a/koko")
	tm.mu.Unlock()
	if slug != "koko-1" {
		t.Errorf("got %q", slug)
	}
}

func TestReserveSlug_BlocksTheSameSlugLater(t *testing.T) {
	tm := newTestManager()
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// A recovered session keeps koko-2 before anything else is created.
	tm.reserveSlug("koko-2")
	if got := tm.nextSlug("/a/koko"); got != "koko-3" {
		t.Errorf("got %q, want koko-3", got)
	}
}
