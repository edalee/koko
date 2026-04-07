package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// newTestAPIServer creates an APIServer backed by a TerminalManager with no real PTY sessions.
func newTestAPIServer(t *testing.T) (*APIServer, *TerminalManager) {
	t.Helper()
	tm := NewTerminalManager()
	git := NewGitService()
	cfg := NewConfigService()
	// Set a known API key for testing
	cfg.config.APIKey = "test-key-123"
	cfg.config.APIEnabled = true
	cfg.config.APIPort = 0
	api := NewAPIServer(tm, git, cfg)
	return api, tm
}

func TestHealthEndpoint(t *testing.T) {
	api, _ := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	api.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", body["status"])
	}
}

func TestAuthMiddleware_RejectsNoAuth(t *testing.T) {
	api, _ := newTestAPIServer(t)

	handler := api.authMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_AcceptsBearerToken(t *testing.T) {
	api, _ := newTestAPIServer(t)

	handler := api.authMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_AcceptsQueryToken(t *testing.T) {
	api, _ := newTestAPIServer(t)

	handler := api.authMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("GET", "/api/sessions?token=test-key-123", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_RejectsWrongToken(t *testing.T) {
	api, _ := newTestAPIServer(t)

	handler := api.authMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListSessions_Empty(t *testing.T) {
	api, _ := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	api.handleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var sessions []SessionInfo
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestSessionDetail_NotFound(t *testing.T) {
	api, _ := newTestAPIServer(t)

	w := httptest.NewRecorder()
	api.handleSessionDetail(w, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSessionOutput_NotFound(t *testing.T) {
	api, _ := newTestAPIServer(t)

	w := httptest.NewRecorder()
	api.handleSessionOutput(w, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSessionWrite_NotFound(t *testing.T) {
	api, _ := newTestAPIServer(t)

	body := strings.NewReader(`{"text":"hello"}`)
	req := httptest.NewRequest("POST", "/api/sessions/nonexistent/write", body)
	w := httptest.NewRecorder()
	api.handleSessionWrite(w, req, "nonexistent")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestSessionWrite_InvalidBody(t *testing.T) {
	api, _ := newTestAPIServer(t)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/api/sessions/test/write", body)
	w := httptest.NewRecorder()
	api.handleSessionWrite(w, req, "test")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSessionWrite_MethodNotAllowed(t *testing.T) {
	api, _ := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/api/sessions/test/write", nil)
	w := httptest.NewRecorder()
	api.handleSessionWrite(w, req, "test")

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleFiles_MissingDir(t *testing.T) {
	api, _ := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/api/files", nil)
	w := httptest.NewRecorder()
	api.handleFiles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSessionRoute_UnknownAction(t *testing.T) {
	api, _ := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/api/sessions/test/unknown", nil)
	w := httptest.NewRecorder()
	api.handleSessionRoute(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSessionsPost_InvalidBody(t *testing.T) {
	api, _ := newTestAPIServer(t)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/api/sessions", body)
	w := httptest.NewRecorder()
	api.handleSessions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// newPipeSession creates a session whose ptmx is backed by an os.Pipe so tests
// can read exactly what bytes were written to the "PTY".  Returns the session
// and the read-end of the pipe.  The caller must close both ends when done.
func newPipeSession(t *testing.T, id string) (*session, *os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	s := &session{
		id:          id,
		slug:        id,
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
		ptmx:        w,
	}
	return s, r, w
}

// TestSessionWrite_NormalizesNewline proves that plain \n sent via /write is
// upgraded to \r\n before reaching the PTY — the key requirement for commands
// to execute inside koko terminals.
func TestSessionWrite_NormalizesNewline(t *testing.T) {
	api, tm := newTestAPIServer(t)

	s, r, w := newPipeSession(t, "write-nl-test")
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	body := strings.NewReader(`{"text":"hello"}`)
	req := httptest.NewRequest("POST", "/api/sessions/write-nl-test/write", body)
	rw := httptest.NewRecorder()
	api.handleSessionWrite(rw, req, s.id)

	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rw.Code, rw.Body.String())
	}

	// Close the write end so io.ReadAll returns; the write already completed.
	_ = w.Close()
	got, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != "hello\r\n" {
		t.Fatalf("expected PTY to receive %q, got %q", "hello\\r\\n", string(got))
	}
}

// TestSessionWrite_IdempotentCRLF confirms that text already ending with \r\n
// is not double-appended.
func TestSessionWrite_IdempotentCRLF(t *testing.T) {
	api, tm := newTestAPIServer(t)

	s, r, w := newPipeSession(t, "write-crlf-test")
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	// JSON encodes \r\n as \\r\\n
	body := strings.NewReader("{\"text\":\"hello\\r\\n\"}")
	req := httptest.NewRequest("POST", "/api/sessions/write-crlf-test/write", body)
	rw := httptest.NewRecorder()
	api.handleSessionWrite(rw, req, s.id)

	if rw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rw.Code)
	}

	_ = w.Close()
	got, _ := io.ReadAll(r)
	_ = r.Close()

	if string(got) != "hello\r\n" {
		t.Fatalf("expected exactly one \\r\\n, got %q", string(got))
	}
}

func TestSessionInteract_NotFound(t *testing.T) {
	api, _ := newTestAPIServer(t)

	body := strings.NewReader(`{"text":"hello"}`)
	req := httptest.NewRequest("POST", "/api/sessions/nonexistent/interact", body)
	w := httptest.NewRecorder()
	api.handleSessionInteract(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSessionInteract_InvalidBody(t *testing.T) {
	api, _ := newTestAPIServer(t)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/api/sessions/test/interact", body)
	w := httptest.NewRecorder()
	api.handleSessionInteract(w, req, "test")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSessionInteract_MethodNotAllowed(t *testing.T) {
	api, _ := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/api/sessions/test/interact", nil)
	w := httptest.NewRecorder()
	api.handleSessionInteract(w, req, "test")

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSessionInteract_ApprovalState(t *testing.T) {
	api, tm := newTestAPIServer(t)

	s := &session{
		id:              "interact-approval",
		name:            "Test",
		dir:             "/tmp",
		done:            make(chan struct{}),
		tailText:        newRingBuffer(2048),
		subscribers:     make(map[chan []byte]struct{}),
		waitingApproval: true,
		approvalTool:    "Bash",
	}
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	body := strings.NewReader(`{"text":"hello"}`)
	req := httptest.NewRequest("POST", "/api/sessions/interact-approval/interact", body)
	w := httptest.NewRecorder()
	api.handleSessionInteract(w, req, s.id)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionInteract_WritesToPTY(t *testing.T) {
	api, tm := newTestAPIServer(t)

	s, r, pw := newPipeSession(t, "interact-write")
	defer func() { _ = r.Close() }()
	defer func() { _ = pw.Close() }()
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	// Use short timeouts so the test completes quickly
	body := strings.NewReader(`{"text":"hello world","timeout_ms":500,"quiet_ms":200,"start_ms":300}`)
	req := httptest.NewRequest("POST", "/api/sessions/"+s.id+"/interact", body)
	w := httptest.NewRecorder()
	api.handleSessionInteract(w, req, s.id)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Read all bytes written to the pipe (the PTY master side).
	// WriteKeystrokes writes char-by-char, so we may need multiple reads.
	var all []byte
	buf := make([]byte, 4096)
	for {
		_ = r.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := r.Read(buf)
		if n > 0 {
			all = append(all, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	got := string(all)
	if !strings.Contains(got, "hello world") {
		t.Errorf("expected pipe to contain 'hello world', got %q", got)
	}
	if !strings.HasSuffix(got, "\r") {
		t.Errorf("expected pipe data to end with \\r (Enter), got %q", got)
	}
}

func TestRenderVT_PreservesSpacing(t *testing.T) {
	// Simulate Claude Code output with cursor positioning escape codes.
	// \x1b[10C = cursor forward 10 (should produce 10 spaces).
	// \x1b[2;1H = move cursor to row 2, col 1.
	raw := []byte("hello\x1b[5Cworld\x1b[2;1H1 file changed")
	got := renderVT(raw, 80, 24)
	if !strings.Contains(got, "hello     world") {
		t.Errorf("expected 'hello     world', got first line %q", strings.SplitN(got, "\n", 2)[0])
	}
	if !strings.Contains(got, "1 file changed") {
		t.Errorf("expected '1 file changed' on second line, got %q", got)
	}
}

func TestRenderVT_EmptyInput(t *testing.T) {
	got := renderVT(nil, 80, 24)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSessionState_ExistingSession(t *testing.T) {
	api, tm := newTestAPIServer(t)

	s := &session{
		id:          "state-test",
		name:        "Test",
		dir:         "/tmp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	w := httptest.NewRecorder()
	api.handleSessionState(w, s.id)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["state"] != "idle" {
		t.Fatalf("expected 'idle', got %q", result["state"])
	}
}

func TestListSessions_WithData(t *testing.T) {
	api, tm := newTestAPIServer(t)

	s := &session{
		id:          "list-test",
		slug:        "project-1",
		name:        "My Project",
		dir:         "/tmp/project",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	w := httptest.NewRecorder()
	api.handleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var sessions []SessionInfo
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Slug != "project-1" {
		t.Fatalf("expected slug 'project-1', got %q", sessions[0].Slug)
	}
	if sessions[0].Name != "My Project" {
		t.Fatalf("expected name 'My Project', got %q", sessions[0].Name)
	}
}

func TestPermissionRequest_MatchesByDir(t *testing.T) {
	api, tm := newTestAPIServer(t)

	s := &session{
		id:          "perm-test",
		name:        "Test",
		dir:         "/projects/myapp",
		done:        make(chan struct{}),
		tailText:    newRingBuffer(2048),
		subscribers: make(map[chan []byte]struct{}),
	}
	tm.mu.Lock()
	tm.sessions[s.id] = s
	tm.mu.Unlock()

	body := strings.NewReader(`{"tool_name":"Bash","cwd":"/projects/myapp"}`)
	req := httptest.NewRequest("POST", "/api/hooks/permission-request", body)
	w := httptest.NewRecorder()
	api.handlePermissionRequest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	s.mu.Lock()
	waiting := s.waitingApproval
	tool := s.approvalTool
	s.mu.Unlock()

	if !waiting {
		t.Fatal("expected session to be in approval state")
	}
	if tool != "Bash" {
		t.Fatalf("expected tool 'Bash', got %q", tool)
	}
}

func TestPermissionRequest_MethodNotAllowed(t *testing.T) {
	api, _ := newTestAPIServer(t)

	req := httptest.NewRequest("GET", "/api/hooks/permission-request", nil)
	w := httptest.NewRecorder()
	api.handlePermissionRequest(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestPermissionRequest_InvalidBody(t *testing.T) {
	api, _ := newTestAPIServer(t)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/api/hooks/permission-request", body)
	w := httptest.NewRecorder()
	api.handlePermissionRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
