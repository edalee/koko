package main

import (
	"errors"
	"testing"
	"time"
)

// addSession registers a session holding a conversation.
func addSession(tm *TerminalManager, id, dir, uuid string) *session {
	s := &session{
		id:              id,
		slug:            id,
		dir:             dir,
		claudeSessionID: uuid,
		startedAt:       time.Now(),
		done:            make(chan struct{}),
		subscribers:     make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions[id] = s
	tm.mu.Unlock()
	return s
}

func TestCreateSessionWithOpts_RefusesAHeldConversation(t *testing.T) {
	tm := newTestManager()
	addSession(tm, "holder", "/repo", "conv-1")

	_, err := tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             "/repo",
		Resume:          true,
		ClaudeSessionID: "conv-1",
	})
	if !errors.Is(err, ErrConversationBusy) {
		t.Fatalf("expected ErrConversationBusy, got %v", err)
	}
}

// A dead session still owns its conversation, because reconnecting that tab
// resumes it. Only the session being replaced is exempt.
func TestCreateSessionWithOpts_RefusesEvenWhenTheHolderIsDead(t *testing.T) {
	tm := newTestManager()
	holder := addSession(tm, "holder", "/repo", "conv-1")
	close(holder.done)

	_, err := tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             "/repo",
		Resume:          true,
		ClaudeSessionID: "conv-1",
	})
	if !errors.Is(err, ErrConversationBusy) {
		t.Fatalf("expected ErrConversationBusy, got %v", err)
	}
}

// The trap the guard has to avoid. readLoop leaves a dead session in the map
// and only closeTab calls CloseSession, so reconnect would otherwise be
// refused its own conversation every time after the first.
func TestCreateSessionWithOpts_ExemptsTheSessionBeingReplaced(t *testing.T) {
	tm := newTestManager()
	old := addSession(tm, "old", "/repo", "conv-1")
	close(old.done)

	tm.mu.Lock()
	busy := tm.uuidClaimedLocked("conv-1", "old")
	exempt := tm.uuidClaimedLocked("conv-1", "")
	tm.mu.Unlock()

	if busy {
		t.Error("the replaced session should be exempt")
	}
	if !exempt {
		t.Error("without the exemption it should read as held")
	}
}

func TestCreateSessionWithOpts_AllowsAFreeConversation(t *testing.T) {
	tm := newTestManager()
	addSession(tm, "other", "/repo", "conv-2")

	tm.mu.Lock()
	held := tm.uuidClaimedLocked("conv-1", "")
	tm.mu.Unlock()
	if held {
		t.Fatal("conv-1 is not held by anything")
	}
}

// Two creates racing on one UUID must not both pass. The new session does not
// reach tm.sessions until much later, so the reservation is what closes it.
func TestCreateSessionWithOpts_ReservationBlocksARace(t *testing.T) {
	tm := newTestManager()

	tm.mu.Lock()
	tm.pendingUUIDs["conv-1"] = true
	tm.mu.Unlock()

	_, err := tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             "/repo",
		Resume:          true,
		ClaudeSessionID: "conv-1",
	})
	if !errors.Is(err, ErrConversationBusy) {
		t.Fatalf("expected ErrConversationBusy, got %v", err)
	}
}

func TestReleaseUUID_ClearsTheReservation(t *testing.T) {
	tm := newTestManager()
	tm.mu.Lock()
	tm.pendingUUIDs["conv-1"] = true
	tm.mu.Unlock()

	tm.releaseUUID("conv-1")

	tm.mu.Lock()
	still := tm.pendingUUIDs["conv-1"]
	tm.mu.Unlock()
	if still {
		t.Fatal("reservation was not released")
	}

	tm.releaseUUID("") // must not panic or reserve an empty key
	tm.mu.Lock()
	n := len(tm.pendingUUIDs)
	tm.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected an empty set, got %d", n)
	}
}

// A refused create must not leave the conversation reserved, or the next
// attempt would be refused too.
func TestCreateSessionWithOpts_RefusalLeavesNoReservation(t *testing.T) {
	tm := newTestManager()
	addSession(tm, "holder", "/repo", "conv-1")

	_, _ = tm.CreateSessionWithOpts(CreateSessionOpts{
		Dir:             "/repo",
		Resume:          true,
		ClaudeSessionID: "conv-1",
	})

	tm.mu.Lock()
	pending := tm.pendingUUIDs["conv-1"]
	tm.mu.Unlock()
	if pending {
		t.Fatal("a refused create left the conversation reserved")
	}
}

// A create with no conversation id is never refused, and reserves nothing.
func TestCreateSessionWithOpts_NoConversationIDReservesNothing(t *testing.T) {
	tm := newTestManager()
	// A session that has not captured a conversation yet holds "", so an
	// empty query must not match it.
	addSession(tm, "fresh", "/repo", "")
	tm.releaseUUID("")
	tm.mu.Lock()
	held := tm.uuidClaimedLocked("", "")
	n := len(tm.pendingUUIDs)
	tm.mu.Unlock()
	if held {
		t.Error("an empty conversation id must not read as held")
	}
	if n != 0 {
		t.Errorf("expected no reservations, got %d", n)
	}
}
