package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type session struct {
	id   string
	name string
	dir  string
	ptmx *os.File
	cmd  *exec.Cmd
	mu   sync.Mutex
	done chan struct{}
}

type TerminalManager struct {
	ctx      context.Context
	sessions map[string]*session
	mu       sync.Mutex
	nextID   int
}

func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		sessions: make(map[string]*session),
	}
}

func (tm *TerminalManager) setContext(ctx context.Context) {
	tm.ctx = ctx
}

func (tm *TerminalManager) CreateSession(name, dir string, cols, rows int) (string, error) {
	tm.mu.Lock()
	tm.nextID++
	id := fmt.Sprintf("session-%d", tm.nextID)
	tm.mu.Unlock()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}

	// Filter out CLAUDECODE to avoid nested session detection.
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor")

	// Login shell resolves PATH (GUI apps have minimal env),
	// exec replaces shell with claude (no wrapper process).
	cmd := exec.Command(shell, "-l", "-c", "exec claude")
	cmd.Dir = dir
	cmd.Env = env

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start PTY: %w", err)
	}

	s := &session{
		id:   id,
		name: name,
		dir:  dir,
		ptmx: ptmx,
		cmd:  cmd,
		done: make(chan struct{}),
	}

	tm.mu.Lock()
	tm.sessions[id] = s
	tm.mu.Unlock()

	go tm.readLoop(s)

	return id, nil
}

func (tm *TerminalManager) Write(sessionID, data string) error {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return err
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("failed to decode input: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.ptmx.Write(decoded)
	return err
}

func (tm *TerminalManager) Resize(sessionID string, cols, rows int) error {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return err
	}

	return pty.Setsize(s.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (tm *TerminalManager) CloseSession(sessionID string) error {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return err
	}

	tm.mu.Lock()
	delete(tm.sessions, sessionID)
	tm.mu.Unlock()

	_ = s.ptmx.Close()
	_ = s.cmd.Process.Kill()
	_ = s.cmd.Wait()
	return nil
}

func (tm *TerminalManager) readLoop(s *session) {
	defer func() {
		close(s.done)
		runtime.EventsEmit(tm.ctx, "pty:exit:"+s.id)
	}()

	buf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			runtime.EventsEmit(tm.ctx, "pty:data:"+s.id, encoded)
		}
		if err != nil {
			return
		}
	}
}

func (tm *TerminalManager) GetSessions() []SessionInfo {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	sessions := make([]SessionInfo, 0, len(tm.sessions))
	for _, s := range tm.sessions {
		sessions = append(sessions, SessionInfo{
			ID:   s.id,
			Name: s.name,
			Dir:  s.dir,
		})
	}
	return sessions
}

func (tm *TerminalManager) getSession(id string) (*session, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	s, ok := tm.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return s, nil
}
