package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
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

// handleSessionInteract sends a prompt to a session's Claude instance using
// `claude -p --resume <uuid>` and returns the structured response.
// This bypasses the PTY entirely — no bracketed paste, no quiet timers, no ANSI.
func (api *APIServer) handleSessionInteract(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text      string `json:"text"`
		TimeoutMs int    `json:"timeout_ms"` // max wait time (default 120000)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.TimeoutMs == 0 {
		req.TimeoutMs = 120000
	}

	// Must be idle — don't spawn a second claude process during approval or active work
	state := api.tm.GetSessionState(sessionID)
	if state == "approval" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "session is waiting for approval",
			"state": "approval",
		})
		return
	}

	// Need the Claude session UUID to resume
	claudeID, err := api.tm.GetClaudeSessionID(sessionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	if claudeID == "" {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "session not ready — Claude session ID not yet captured",
		})
		return
	}

	// Get session directory
	sessions := api.tm.GetSessions()
	var dir string
	for _, s := range sessions {
		if s.ID == sessionID {
			dir = s.Dir
			break
		}
	}

	log.Printf("[interact] session=%s claude=%s timeout=%dms prompt=%q", sessionID, claudeID, req.TimeoutMs, truncateLog(req.Text, 80))

	// Spawn claude -p --resume with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(req.TimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, resolveClaudePath(),
		"-p", req.Text,
		"--resume", claudeID,
		"--output-format", "json",
		"--bare",
	)
	cmd.Dir = dir
	cmd.Env = api.tm.buildEnv()

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[interact] session=%s timeout after %dms", sessionID, req.TimeoutMs)
			writeJSON(w, http.StatusGatewayTimeout, map[string]string{"error": "timeout waiting for Claude response"})
			return
		}
		log.Printf("[interact] session=%s error: %v", sessionID, err)
		// Try to extract stderr for context
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": string(exitErr.Stderr)})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Parse the JSON response from claude -p
	var result struct {
		Result    string `json:"result"`
		SessionID string `json:"session_id"`
		IsError   bool   `json:"is_error"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		log.Printf("[interact] session=%s failed to parse claude output: %v", sessionID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse Claude response"})
		return
	}

	log.Printf("[interact] session=%s done result_len=%d is_error=%v", sessionID, len(result.Result), result.IsError)

	// Check session state after the call (approval may have been triggered during execution)
	postState := api.tm.GetSessionState(sessionID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"output": result.Result,
		"state":  postState,
	})
}

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
