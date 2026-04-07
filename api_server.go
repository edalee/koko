package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// APIServer exposes Koko's functionality over HTTP and WebSocket.
type APIServer struct {
	tm     *TerminalManager
	git    *GitService
	cfg    *ConfigService
	server *http.Server
	mu     sync.Mutex
}

// NewAPIServer creates an APIServer wired to the given services.
func NewAPIServer(tm *TerminalManager, git *GitService, cfg *ConfigService) *APIServer {
	return &APIServer{tm: tm, git: git, cfg: cfg}
}

// Start begins listening on the configured port. Non-blocking.
func (api *APIServer) Start() error {
	config := api.cfg.GetConfig()
	if !config.APIEnabled {
		log.Println("[api] API server disabled")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", api.handleHealth)
	mux.HandleFunc("/api/hooks/permission-request", api.handlePermissionRequest) // no auth — called by Claude hook
	mux.HandleFunc("/api/sessions", api.authMiddleware(api.handleSessions))
	mux.HandleFunc("/api/sessions/", api.authMiddleware(api.handleSessionRoute))
	mux.HandleFunc("/api/files", api.authMiddleware(api.handleFiles))

	addr := fmt.Sprintf("127.0.0.1:%d", config.APIPort)
	api.mu.Lock()
	api.server = &http.Server{Addr: addr, Handler: mux}
	api.mu.Unlock()

	go func() {
		log.Printf("[api] listening on %s", addr)
		if err := api.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[api] server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the API server.
func (api *APIServer) Stop() {
	api.mu.Lock()
	srv := api.server
	api.mu.Unlock()
	if srv != nil {
		_ = srv.Shutdown(context.Background())
	}
}

// authMiddleware checks the Authorization: Bearer <key> header.
func (api *APIServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		config := api.cfg.GetConfig()
		key := config.APIKey

		// Check header
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == key {
			next(w, r)
			return
		}

		// Check query param (for WebSocket)
		if r.URL.Query().Get("token") == key {
			next(w, r)
			return
		}

		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}
}

func (api *APIServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *APIServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessions := api.tm.GetSessions()
		writeJSON(w, http.StatusOK, sessions)
	case http.MethodPost:
		var req struct {
			Name    string `json:"name"`
			Dir     string `json:"dir"`
			Cols    int    `json:"cols"`
			Rows    int    `json:"rows"`
			Resume  bool   `json:"resume"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if req.Cols == 0 {
			req.Cols = 120
		}
		if req.Rows == 0 {
			req.Rows = 40
		}
		id, err := api.tm.CreateSession(req.Name, req.Dir, req.Cols, req.Rows, req.Resume)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (api *APIServer) handleSessionRoute(w http.ResponseWriter, r *http.Request) {
	// Parse: /api/sessions/{id} or /api/sessions/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.SplitN(path, "/", 2)
	sessionID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	// Resolve slug or PTY ID upfront — all sub-handlers use the resolved PTY ID
	resolved := api.resolveSessionID(sessionID)
	if resolved == "" && action != "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	if resolved != "" {
		sessionID = resolved
	}

	switch action {
	case "":
		switch r.Method {
		case http.MethodGet:
			api.handleSessionDetail(w, sessionID)
		case http.MethodDelete:
			api.handleSessionClose(w, sessionID)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	case "write":
		api.handleSessionWrite(w, r, sessionID)
	case "output":
		api.handleSessionOutput(w, sessionID)
	case "state":
		api.handleSessionState(w, sessionID)
	case "stream":
		api.handleSessionStream(w, r, sessionID)
	case "interact":
		api.handleSessionInteract(w, r, sessionID)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

// resolveSessionID resolves a session ID or slug to a PTY session ID.
func (api *APIServer) resolveSessionID(idOrSlug string) string {
	sessions := api.tm.GetSessions()
	for _, s := range sessions {
		if s.ID == idOrSlug {
			return s.ID
		}
	}
	if info := api.tm.GetSessionBySlug(idOrSlug); info != nil {
		return info.ID
	}
	return ""
}

func (api *APIServer) handleSessionDetail(w http.ResponseWriter, sessionID string) {
	resolved := api.resolveSessionID(sessionID)
	if resolved == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	sessions := api.tm.GetSessions()
	for _, s := range sessions {
		if s.ID == resolved {
			state := api.tm.GetSessionState(resolved)
			claudeID, _ := api.tm.GetClaudeSessionID(resolved)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"id":              s.ID,
				"slug":            s.Slug,
				"name":            s.Name,
				"dir":             s.Dir,
				"state":           state,
				"claudeSessionId": claudeID,
			})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
}

func (api *APIServer) handleSessionClose(w http.ResponseWriter, sessionID string) {
	if err := api.tm.CloseSession(sessionID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}

func (api *APIServer) handleSessionWrite(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Normalize newline: PTY requires \r\n to execute input, not just \n.
	// Normalise here so all callers (CLI, MCP, scripts) get correct behaviour.
	text := strings.TrimRight(req.Text, "\r\n") + "\r\n"
	// Encode text as base64 (TerminalManager.Write expects base64)
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	if err := api.tm.Write(sessionID, encoded); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *APIServer) handleSessionOutput(w http.ResponseWriter, sessionID string) {
	output, err := api.tm.ReadOutput(sessionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": output})
}

func (api *APIServer) handleSessionState(w http.ResponseWriter, sessionID string) {
	state := api.tm.GetSessionState(sessionID)
	writeJSON(w, http.StatusOK, map[string]string{"state": state})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (api *APIServer) handleSessionStream(w http.ResponseWriter, r *http.Request, sessionID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[api] websocket upgrade failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	ch := api.tm.Subscribe(sessionID)
	if ch == nil {
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session not found"))
		return
	}
	defer api.tm.Unsubscribe(sessionID, ch)

	// Read goroutine to detect client disconnect
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func (api *APIServer) handleFiles(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dir parameter required"})
		return
	}
	changes, err := api.git.GetFileChanges(dir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, changes)
}

// handleSessionInteract sends text to a session and waits for the response.
// Writes the prompt to the PTY with a delay before Enter (so Claude Code's
// Ink TUI finishes processing the text before the submit keystroke), then
// collects output until quiet.
func (api *APIServer) handleSessionInteract(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text      string `json:"text"`
		TimeoutMs int    `json:"timeout_ms"` // max wait time (default 120000)
		QuietMs   int    `json:"quiet_ms"`   // output settle time (default 8000)
		StartMs   int    `json:"start_ms"`   // max time to wait for first output (default 30000)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.TimeoutMs == 0 {
		req.TimeoutMs = 120000
	}
	if req.QuietMs == 0 {
		req.QuietMs = 8000
	}
	if req.StartMs == 0 {
		req.StartMs = 30000
	}

	// Subscribe to output before sending input
	ch := api.tm.Subscribe(sessionID)
	if ch == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	defer api.tm.Unsubscribe(sessionID, ch)

	// Reject if session is waiting for tool approval — sending a new prompt
	// would conflict with the pending approval flow.
	if state := api.tm.GetSessionState(sessionID); state == "approval" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "session is waiting for approval"})
		return
	}

	promptText := strings.TrimRight(req.Text, "\r\n")
	log.Printf("[interact] session=%s timeout=%dms quiet=%dms start=%dms prompt=%q", sessionID, req.TimeoutMs, req.QuietMs, req.StartMs, truncateLog(promptText, 80))

	// Write each character individually to the PTY, simulating real keyboard
	// input. Claude Code's Ink TUI uses Node's readline.emitKeypressEvents()
	// which processes bulk writes as a single data event — the trailing \r
	// doesn't generate a separate "return" keypress event. Writing char-by-char
	// matches what xterm.js does and ensures \r triggers submission.
	if err := api.tm.WriteKeystrokes(sessionID, promptText+"\r"); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Phase 1: wait for Claude to start responding (first output chunk).
	timeout := time.After(time.Duration(req.TimeoutMs) * time.Millisecond)
	startDeadline := time.After(time.Duration(req.StartMs) * time.Millisecond)
	quiet := time.Duration(req.QuietMs) * time.Millisecond

	var collected []byte

waitForStart:
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				log.Printf("[interact] session=%s channel closed waiting for first output", sessionID)
				goto done
			}
			collected = append(collected, data...)
			log.Printf("[interact] session=%s first output chunk (%d bytes), starting quiet timer", sessionID, len(data))
			break waitForStart
		case <-startDeadline:
			log.Printf("[interact] session=%s start timeout — no output within start_ms", sessionID)
			goto done
		case <-timeout:
			log.Printf("[interact] session=%s overall timeout hit waiting for first output", sessionID)
			goto done
		}
	}

	// Phase 2: collect output, resetting quiet timer on each chunk.
	{
		quietTimer := time.NewTimer(quiet)
		defer quietTimer.Stop()

		for {
			select {
			case data, ok := <-ch:
				if !ok {
					goto done
				}
				collected = append(collected, data...)
				if !quietTimer.Stop() {
					select {
					case <-quietTimer.C:
					default:
					}
				}
				quietTimer.Reset(quiet)
			case <-quietTimer.C:
				// Drain any buffered data
				for len(ch) > 0 {
					data := <-ch
					collected = append(collected, data...)
				}
				if len(ch) > 0 {
					quietTimer.Reset(quiet)
					continue
				}
				log.Printf("[interact] session=%s quiet settle after %d bytes", sessionID, len(collected))
				goto done
			case <-timeout:
				log.Printf("[interact] session=%s overall timeout hit collecting output (%d bytes so far)", sessionID, len(collected))
				goto done
			}
		}
	}

done:
	stripped := stripANSI(collected)
	state := api.tm.GetSessionState(sessionID)
	log.Printf("[interact] session=%s state=%s raw=%d stripped=%d", sessionID, state, len(collected), len(stripped))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"output": stripped,
		"state":  state,
	})
}

// stripANSI removes ANSI escape codes from raw PTY output, converting
// cursor-forward sequences (\x1b[nC) to spaces so column-aligned text
// (like git diff --stat) retains its spacing. Other positioning codes
// (cursor-up/down/absolute) are stripped — they're used by Claude Code's
// Ink TUI to draw decorative elements and would add noise.
func stripANSI(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	// First pass: replace cursor-forward \x1b[nC with n spaces.
	result := cursorForwardRegex.ReplaceAllFunc(raw, func(match []byte) []byte {
		// Parse the count from \x1b[nC — default to 1 if no number.
		s := string(match)
		numStr := s[2 : len(s)-1] // between \x1b[ and C
		n := 1
		if numStr != "" {
			if parsed, err := strconv.Atoi(numStr); err == nil && parsed > 0 {
				n = parsed
			}
		}
		return []byte(strings.Repeat(" ", n))
	})
	// Second pass: strip all remaining escape sequences.
	result = ansiStripRegex.ReplaceAll(result, nil)
	return string(result)
}

var (
	// Matches cursor-forward: \x1b[C or \x1b[nC (CUF).
	cursorForwardRegex = regexp.MustCompile(`\x1b\[\d*C`)
	// Matches all other ANSI escape sequences.
	ansiStripRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07|\x1b[()][0-9A-B]|\x1b\[[\?]?[0-9;]*[hlmsu]`)
)

// truncateLog trims a string to max chars for log lines.
func truncateLog(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// handlePermissionRequest is called by Claude Code's PermissionRequest hook.
// It marks the matching session as waiting for approval, then returns an empty
// response so Claude Code still shows the permission prompt to the user.
// No auth required — called by the Claude process running locally.
func (api *APIServer) handlePermissionRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ToolName  string `json:"tool_name"`
		SessionID string `json:"session_id"`
		CWD       string `json:"cwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	log.Printf("[hooks] PermissionRequest: tool=%s session=%s cwd=%s", req.ToolName, req.SessionID, req.CWD)

	// Match session by Claude UUID first, then by working directory
	sessions := api.tm.GetSessions()
	matched := false

	if req.SessionID != "" {
		for _, s := range sessions {
			claudeID, _ := api.tm.GetClaudeSessionID(s.ID)
			if claudeID == req.SessionID {
				api.tm.SetApprovalState(s.ID, req.ToolName)
				matched = true
				break
			}
		}
	}

	if !matched && req.CWD != "" {
		for _, s := range sessions {
			if s.Dir == req.CWD {
				api.tm.SetApprovalState(s.ID, req.ToolName)
				break
			}
		}
	}

	// Return empty JSON object — tells Claude Code "no decision, show the prompt"
	// Returning a decision would auto-approve/deny without showing the user.
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
