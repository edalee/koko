package main

import (
	"errors"
	"fmt"
)

// ErrConversationHeld is returned when a delete names a conversation that a
// session or a saved tab holds.
var ErrConversationHeld = errors.New("that conversation is open in a session: close the session first")

// Deletion lives on App rather than ClaudeService, because the ownership
// check needs the TerminalManager and the saved tabs (plan 028 step 7).
// ClaudeService is bound to the frontend, so it exposes no delete at all.

// DeleteConversation permanently deletes one of the conversations Claude has
// stored for dir. It refuses one that a session or a saved tab holds.
func (a *App) DeleteConversation(dir, uuid string) error {
	if dir == "" {
		return fmt.Errorf("no directory given")
	}
	if _, err := conversationPath(dir, uuid); err != nil {
		return err
	}
	removed, _, err := a.tm.removeUnheld([]string{uuid}, a.heldByTabs(), func(id string) error {
		return removeConversation(dir, id)
	})
	if err != nil {
		return err
	}
	if len(removed) == 0 {
		return ErrConversationHeld
	}
	return nil
}

// DeleteConversations permanently deletes every conversation Claude has
// stored for dir. It skips any that a session or a saved tab holds, and any
// file with no recorded directory, and counts both as skipped.
func (a *App) DeleteConversations(dir string) (DeleteResult, error) {
	if dir == "" {
		return DeleteResult{}, fmt.Errorf("no directory given")
	}
	ids, unknown := conversationsIn(dir)
	removed, skipped, err := a.tm.removeUnheld(ids, a.heldByTabs(), func(id string) error {
		return removeConversation(dir, id)
	})
	if removed == nil {
		removed = []string{}
	}
	return DeleteResult{Deleted: removed, Skipped: skipped + unknown}, err
}

// heldByTabs returns the conversations of saved tabs, connected or not.
//
// After a restart a disconnected tab has no session in the TerminalManager,
// yet reconnecting it resumes its conversation, so it still holds it.
func (a *App) heldByTabs() map[string]bool {
	held := make(map[string]bool)
	for _, r := range a.cfg.GetSessions().Sessions {
		if r.ClaudeSessionID != "" && (r.Status == "active" || r.Status == "disconnected") {
			held[r.ClaudeSessionID] = true
		}
	}
	return held
}
