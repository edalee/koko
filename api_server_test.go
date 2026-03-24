package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
