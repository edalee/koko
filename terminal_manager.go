package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ansiRegex strips ANSI escape sequences from terminal output.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07|\x1b[()][0-9A-B]|\x1b\[[\?]?[0-9;]*[hlmsu]`)

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
	id              string
	slug            string
	name            string
	dir             string
	claudeSessionID string // Claude Code session UUID for --resume
	ptmx            *os.File
	cmd             *exec.Cmd
	mu              sync.Mutex
	done            chan struct{}
	buf             *ringBuffer // buffered PTY output for late-connecting frontends
	tailText        *ringBuffer // last 32KB of ANSI-stripped text for API ReadOutput
	lastOutputAt    time.Time   // when PTY last produced output
	subscribers     map[chan []byte]struct{}
	waitingApproval bool   // set by PermissionRequest hook, cleared on next PTY output
	approvalTool    string // tool name waiting for approval (e.g. "Bash", "Edit")
}

type TerminalManager struct {
	ctx       context.Context
	sessions  map[string]*session
	mu        sync.Mutex
	nextID    int
	slugCount map[string]int // per-directory slug counter: dir -> next number
	loginPath string         // full PATH from login shell, resolved once at startup
}

func NewTerminalManager() *TerminalManager {
	tm := &TerminalManager{
		sessions:  make(map[string]*session),
		slugCount: make(map[string]int),
	}
	tm.loginPath = resolveLoginPath()
	return tm
}

// dirSlug returns a human-friendly slug for a directory path.
// e.g. "/Users/ed/Projects/koko" → "koko"
func dirSlug(dir string) string {
	dir = strings.TrimRight(dir, "/")
	parts := strings.Split(dir, "/")
	if len(parts) == 0 {
		return "session"
	}
	slug := parts[len(parts)-1]
	if slug == "" {
		return "session"
	}
	return slug
}

// nextSlug generates the next slug for a directory, e.g. "koko/1", "koko/2".
func (tm *TerminalManager) nextSlug(dir string) string {
	base := dirSlug(dir)
	tm.slugCount[dir]++
	return fmt.Sprintf("%s/%d", base, tm.slugCount[dir])
}

// resolveLoginPath gets the full PATH from an interactive login shell.
// GUI apps launched from Finder have a minimal PATH that doesn't include
// user additions from .zshrc/.bashrc. This runs once at startup.
func resolveLoginPath() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	out, err := exec.Command(shell, "-l", "-i", "-c", "echo $PATH").Output()
	if err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			log.Printf("[pty] resolved login PATH: %s", p)
			return p
		}
	}
	log.Printf("[pty] failed to resolve login PATH: %v", err)
	return ""
}

func (tm *TerminalManager) setContext(ctx context.Context) {
	tm.ctx = ctx
}

// buildEnv returns the process environment with login PATH injected and CLAUDECODE filtered.
func (tm *TerminalManager) buildEnv() []string {
	var env []string
	hasPath := false
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			continue
		}
		if strings.HasPrefix(e, "PATH=") && tm.loginPath != "" {
			env = append(env, "PATH="+tm.loginPath)
			hasPath = true
			continue
		}
		env = append(env, e)
	}
	if !hasPath && tm.loginPath != "" {
		env = append(env, "PATH="+tm.loginPath)
	}
	env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
	return env
}

// resolveClaudePath finds the absolute path to claude.
// GUI apps launched from Finder have a minimal PATH that won't include ~/.local/bin.
func resolveClaudePath() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			log.Printf("[pty] found claude at: %s", p)
			return p
		}
	}
	log.Printf("[pty] could not find claude binary, falling back to bare 'claude'")
	return "claude"
}

// CreateSessionOpts holds options for creating a session.
type CreateSessionOpts struct {
	Name            string `json:"name"`
	Dir             string `json:"dir"`
	Cols            int    `json:"cols"`
	Rows            int    `json:"rows"`
	Resume          bool   `json:"resume"`
	ClaudeSessionID string `json:"claudeSessionId"` // UUID for --resume (empty = --continue)
}

func (tm *TerminalManager) CreateSession(name, dir string, cols, rows int, resume bool) (string, error) {
	return tm.CreateSessionWithOpts(CreateSessionOpts{
		Name:   name,
		Dir:    dir,
		Cols:   cols,
		Rows:   rows,
		Resume: resume,
	})
}

// CreateSessionWithOpts creates a session with full options including Claude session ID for --resume.
func (tm *TerminalManager) CreateSessionWithOpts(opts CreateSessionOpts) (string, error) {
	tm.mu.Lock()
	tm.nextID++
	id := fmt.Sprintf("session-%d", tm.nextID)
	slug := tm.nextSlug(opts.Dir)
	tm.mu.Unlock()

	if opts.Cols == 0 {
		opts.Cols = 120
	}
	if opts.Rows == 0 {
		opts.Rows = 40
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}

	env := tm.buildEnv()

	claudePath := resolveClaudePath()
	claudeCmd := fmt.Sprintf("exec %s", claudePath)
	if opts.Resume {
		if opts.ClaudeSessionID != "" {
			// Resume specific session by UUID
			claudeCmd = fmt.Sprintf("exec %s --resume %s", claudePath, opts.ClaudeSessionID)
		} else {
			// Fall back to most recent session in directory
			claudeCmd = fmt.Sprintf("exec %s --continue", claudePath)
		}
	}
	cmd := exec.Command(shell, "-l", "-c", claudeCmd)
	cmd.Dir = opts.Dir
	cmd.Env = env

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(opts.Rows),
		Cols: uint16(opts.Cols),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start PTY: %w", err)
	}

	s := &session{
		id:              id,
		slug:            slug,
		name:            opts.Name,
		dir:             opts.Dir,
		claudeSessionID: opts.ClaudeSessionID,
		ptmx:            ptmx,
		cmd:             cmd,
		done:            make(chan struct{}),
		tailText:        newRingBuffer(32 * 1024),
		subscribers:     make(map[chan []byte]struct{}),
	}

	tm.mu.Lock()
	tm.sessions[id] = s
	tm.mu.Unlock()

	// Capture Claude session UUID in background
	if !opts.Resume || opts.ClaudeSessionID == "" {
		go tm.detectClaudeSessionID(s)
	}

	go tm.readLoop(s)

	return id, nil
}

// detectClaudeSessionID watches the Claude projects dir for a new .jsonl file
// and stores the UUID (filename) on the session.
func (tm *TerminalManager) detectClaudeSessionID(s *session) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	projectKey := strings.ReplaceAll(s.dir, "/", "-")
	sessionDir := filepath.Join(home, ".claude", "projects", projectKey)

	// Record existing files before Claude starts
	existing := make(map[string]bool)
	if entries, err := os.ReadDir(sessionDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".jsonl") {
				existing[e.Name()] = true
			}
		}
	}

	// Poll for a new .jsonl file (Claude creates it on first message)
	for i := 0; i < 60; i++ { // up to 30 seconds
		time.Sleep(500 * time.Millisecond)

		select {
		case <-s.done:
			return // session ended
		default:
		}

		entries, err := os.ReadDir(sessionDir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			if existing[e.Name()] {
				continue
			}
			// New file found — extract UUID from filename
			uuid := strings.TrimSuffix(e.Name(), ".jsonl")
			s.mu.Lock()
			s.claudeSessionID = uuid
			s.mu.Unlock()
			log.Printf("[pty] captured Claude session UUID: %s for %s", uuid, s.slug)
			return
		}
	}
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

	env := tm.buildEnv()

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
		id:          id,
		name:        "Quick Terminal",
		dir:         dir,
		ptmx:        ptmx,
		cmd:         cmd,
		done:        make(chan struct{}),
		buf:         newRingBuffer(64 * 1024),
		subscribers: make(map[chan []byte]struct{}),
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
		_ = s.cmd.Wait()
		close(s.done)
		runtime.EventsEmit(tm.ctx, "pty:exit:"+s.id)
	}()

	readBuf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(readBuf)
		if n > 0 {
			chunk := readBuf[:n]
			s.mu.Lock()
			if s.buf != nil {
				s.buf.Write(chunk)
			}
			if s.tailText != nil {
				stripped := ansiRegex.ReplaceAll(chunk, nil)
				s.tailText.Write(stripped)
			}
			s.lastOutputAt = time.Now()
			// Clear approval state — output means the user responded
			s.waitingApproval = false
			s.approvalTool = ""
			// Fan out to API subscribers (non-blocking)
			for ch := range s.subscribers {
				select {
				case ch <- append([]byte(nil), chunk...):
				default:
					// Slow subscriber, drop
				}
			}
			s.mu.Unlock()
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
			Slug: s.slug,
			Name: s.name,
			Dir:  s.dir,
		})
	}
	return sessions
}

// GetClaudeSessionID returns the Claude Code session UUID for a session.
func (tm *TerminalManager) GetClaudeSessionID(sessionID string) (string, error) {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.claudeSessionID, nil
}

// GetSessionBySlug finds a session by its slug (e.g. "koko/1").
func (tm *TerminalManager) GetSessionBySlug(slug string) *SessionInfo {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for _, s := range tm.sessions {
		if s.slug == slug {
			return &SessionInfo{ID: s.id, Slug: s.slug, Name: s.name, Dir: s.dir}
		}
	}
	return nil
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

// GetSessionState returns "approval" if the session is waiting for tool
// approval (set by PermissionRequest hook), or "idle" otherwise.
func (tm *TerminalManager) GetSessionState(sessionID string) string {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return "idle"
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.waitingApproval {
		return "approval"
	}
	return "idle"
}

// SetApprovalState is called by the PermissionRequest hook endpoint
// to mark a session as waiting for tool approval.
func (tm *TerminalManager) SetApprovalState(sessionID, toolName string) {
	s, err := tm.getSession(sessionID)
	if err != nil {
		// Try matching by directory (hook doesn't know PTY session ID)
		tm.mu.Lock()
		for _, sess := range tm.sessions {
			if sess.dir == sessionID {
				s = sess
				break
			}
		}
		tm.mu.Unlock()
		if s == nil {
			return
		}
	}
	s.mu.Lock()
	s.waitingApproval = true
	s.approvalTool = toolName
	s.mu.Unlock()
}

// Subscribe returns a channel that receives raw PTY output for the given session.
// Returns nil if the session doesn't exist.
func (tm *TerminalManager) Subscribe(sessionID string) chan []byte {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return nil
	}
	ch := make(chan []byte, 256)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel from a session.
func (tm *TerminalManager) Unsubscribe(sessionID string, ch chan []byte) {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return
	}
	s.mu.Lock()
	delete(s.subscribers, ch)
	s.mu.Unlock()
}

// ReadOutput returns the recent ANSI-stripped text from a session's tail buffer.
func (tm *TerminalManager) ReadOutput(sessionID string) (string, error) {
	s, err := tm.getSession(sessionID)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tailText == nil {
		return "", nil
	}
	return string(s.tailText.Bytes()), nil
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
