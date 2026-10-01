package main

import (
	"errors"
	"fmt"
	"os"
)

// ErrConversationHeld is returned when a delete names a conversation that a
// session, a saved tab or a pending create holds, or that a running session
// without an id yet may be writing.
var ErrConversationHeld = errors.New("that conversation is, or may be, open in a session: close the session first")

// Deletion lives on App rather than ClaudeService, because the ownership
// check needs the TerminalManager and the saved tabs (plan 028 step 7a).
// Every exported method on a bound struct is callable from the frontend, so
// ClaudeService exposes no delete at all.

// DeleteConversation permanently deletes one of the conversations Claude has
// stored for dir. It refuses one that is, or may be, in use: see deleteUnheld.
func (a *App) DeleteConversation(dir, uuid string) error {
	if dir == "" {
		return fmt.Errorf("no directory given")
	}
	path, err := conversationPath(dir, uuid)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	file := conversationFile{uuid: uuid, path: path, modified: info.ModTime()}
	deleted, _, failed := a.tm.deleteUnheld(dir, []conversationFile{file}, a.heldByTabs())
	if failed > 0 {
		return fmt.Errorf("could not delete conversation %s", uuid)
	}
	if len(deleted) == 0 {
		return ErrConversationHeld
	}
	return nil
}

// DeleteConversations permanently deletes every conversation Claude has
// stored for dir. It skips any that is, or may be, in use, and any file with
// no recorded directory, and counts both as skipped. A delete that fails is
// counted too, and the rest carry on, so the result always lists what went.
func (a *App) DeleteConversations(dir string) (DeleteResult, error) {
	if dir == "" {
		return DeleteResult{}, fmt.Errorf("no directory given")
	}
	files, unknown := conversationsIn(dir)
	deleted, skipped, failed := a.tm.deleteUnheld(dir, files, a.heldByTabs())
	return DeleteResult{Deleted: deleted, Skipped: skipped + unknown, Failed: failed}, nil
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
