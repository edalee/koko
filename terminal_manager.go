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

// ringBuffer stores recent PTY output so late-connecting frontends can replay it.
type ringBuffer struct {
	data []byte
	size int
	pos  int
	full bool
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{data: make([]byte, size), size: size}
}

func (rb *ringBuffer) Write(p []byte) {
	for _, b := range p {
		rb.data[rb.pos] = b
		rb.pos = (rb.pos + 1) % rb.size
		if rb.pos == 0 {
			rb.full = true
		}
	}
}

func (rb *ringBuffer) Bytes() []byte {
	if !rb.full {
		return rb.data[:rb.pos]
	}
	out := make([]byte, rb.size)
	copy(out, rb.data[rb.pos:])
	copy(out[rb.size-rb.pos:], rb.data[:rb.pos])
	return out
}

type session struct {
	id   string
	name string
	dir  string
	ptmx *os.File
	cmd  *exec.Cmd
	mu   sync.Mutex
	done chan struct{}
	buf  *ringBuffer // buffered PTY output for late-connecting frontends
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

func (tm *TerminalManager) CreateSession(name, dir string, cols, rows int, resume bool) (string, error) {
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
	claudeCmd := "exec claude"
	if resume {
		claudeCmd = "exec claude --continue"
	}
	cmd := exec.Command(shell, "-l", "-c", claudeCmd)
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

func (tm *TerminalManager) CreateShellSession(dir string, cols, rows int) (string, error) {
	tm.mu.Lock()
	tm.nextID++
	id := fmt.Sprintf("shell-%d", tm.nextID)
	tm.mu.Unlock()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}

	env := append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")

	cmd := exec.Command(shell, "-l")
	cmd.Dir = dir
	cmd.Env = env

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start shell PTY: %w", err)
	}

	s := &session{
		id:   id,
		name: "Quick Terminal",
		dir:  dir,
		ptmx: ptmx,
		cmd:  cmd,
		done: make(chan struct{}),
		buf:  newRingBuffer(64 * 1024),
	}

	tm.mu.Lock()
	tm.sessions[id] = s
	tm.mu.Unlock()

	go tm.readLoop(s)

	return id, nil
}

func (tm *TerminalManager) ReplayBuffer(sessionID string) (string, error) {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.buf == nil {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(s.buf.Bytes()), nil
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

	readBuf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(readBuf)
		if n > 0 {
			chunk := readBuf[:n]
			if s.buf != nil {
				s.mu.Lock()
				s.buf.Write(chunk)
				s.mu.Unlock()
			}
			encoded := base64.StdEncoding.EncodeToString(chunk)
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

func (tm *TerminalManager) GetSessionPID(sessionID string) (int, error) {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return 0, err
	}
	if s.cmd.Process == nil {
		return 0, fmt.Errorf("session process not started")
	}
	return s.cmd.Process.Pid, nil
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
