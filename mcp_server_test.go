package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMCPInitialize(t *testing.T) {
	resp := handleMCPRequest(nil, &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})

	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.ID != 1 {
		t.Fatalf("expected ID 1, got %v", resp.ID)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Fatalf("expected protocol 2024-11-05, got %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("expected serverInfo map")
	}
	if serverInfo["name"] != "koko" {
		t.Fatalf("expected server name 'koko', got %v", serverInfo["name"])
	}
}

func TestMCPToolsList(t *testing.T) {
	resp := handleMCPRequest(nil, &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	tools, ok := result["tools"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected tools array")
	}

	if len(tools) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(tools))
	}

	// Verify expected tool names
	expectedNames := map[string]bool{
		"list_sessions":     true,
		"get_session_state": true,
		"read_output":       true,
		"send_input":        true,
		"interact":          true,
		"create_session":    true,
		"close_session":     true,
		"list_file_changes": true,
	}
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if !expectedNames[name] {
			t.Fatalf("unexpected tool: %s", name)
		}
		delete(expectedNames, name)
	}
	if len(expectedNames) > 0 {
		t.Fatalf("missing tools: %v", expectedNames)
	}
}

func TestMCPNotificationReturnsNil(t *testing.T) {
	resp := handleMCPRequest(nil, &jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	})

	if resp != nil {
		t.Fatal("expected nil response for notification")
	}
}

func TestMCPUnknownMethodWithID(t *testing.T) {
	resp := handleMCPRequest(nil, &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "unknown/method",
	})

	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.Error == nil {
		t.Fatal("expected error field")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected -32601, got %d", resp.Error.Code)
	}
}

func TestMCPUnknownMethodNoID(t *testing.T) {
	resp := handleMCPRequest(nil, &jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "unknown/notification",
	})

	if resp != nil {
		t.Fatal("expected nil for notification-style unknown method")
	}
}

func TestMCPToolsCallInvalidParams(t *testing.T) {
	resp := handleMCPRequest(nil, &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  json.RawMessage(`not json`),
	})

	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.Error == nil {
		t.Fatal("expected error field")
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("expected -32602, got %d", resp.Error.Code)
	}
}

func TestMCPToolDefinitions_HaveRequiredFields(t *testing.T) {
	tools := mcpToolDefinitions()
	for _, tool := range tools {
		name, ok := tool["name"].(string)
		if !ok || name == "" {
			t.Fatal("tool missing name")
		}
		desc, ok := tool["description"].(string)
		if !ok || desc == "" {
			t.Fatalf("tool %s missing description", name)
		}
		schema, ok := tool["inputSchema"].(map[string]interface{})
		if !ok {
			t.Fatalf("tool %s missing inputSchema", name)
		}
		if schema["type"] != "object" {
			t.Fatalf("tool %s inputSchema type should be object", name)
		}
	}
}

func TestCallMCPTool_UnknownTool(t *testing.T) {
	_, err := callMCPTool(nil, "nonexistent_tool", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

// newCaptureMCPClient starts a test HTTP server that records the last request
// body and returns a 200 with the given responseJSON.
func newCaptureMCPClient(t *testing.T, responseJSON string) (*mcpClient, func() []byte) {
	t.Helper()
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responseJSON))
	}))
	t.Cleanup(srv.Close)
	client := &mcpClient{
		baseURL:    srv.URL,
		apiKey:     "test",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	return client, func() []byte { return captured }
}

// TestCallMCPTool_SendInput_NormalizesNewline proves the full MCP → /write chain
// sends \r\n so commands execute inside koko terminals (not just echo to cursor).
func TestCallMCPTool_SendInput_NormalizesNewline(t *testing.T) {
	client, captured := newCaptureMCPClient(t, `{"status":"ok"}`)

	_, err := callMCPTool(client, "send_input", map[string]interface{}{
		"session_id": "session-1",
		"text":       "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(captured(), &req)

	if !strings.HasSuffix(req.Text, "\r\n") {
		t.Fatalf("MCP send_input must send \\r\\n to execute commands, got %q", req.Text)
	}
	if !strings.HasPrefix(req.Text, "hello") {
		t.Fatalf("text content should be preserved, got %q", req.Text)
	}
}

// TestCallMCPTool_SendInput_IdempotentCRLF confirms that text already ending
// with \r\n is not double-appended.
func TestCallMCPTool_SendInput_IdempotentCRLF(t *testing.T) {
	client, captured := newCaptureMCPClient(t, `{"status":"ok"}`)

	_, err := callMCPTool(client, "send_input", map[string]interface{}{
		"session_id": "session-1",
		"text":       "hello\r\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(captured(), &req)

	if req.Text != "hello\r\n" {
		t.Fatalf("expected exactly one \\r\\n, got %q", req.Text)
	}
}

func TestCallMCPTool_MissingRequired(t *testing.T) {
	// These tools require session_id but we pass empty args
	for _, tool := range []string{"get_session_state", "read_output", "close_session"} {
		_, err := callMCPTool(nil, tool, map[string]interface{}{})
		if err == nil {
			t.Fatalf("expected error for %s with no session_id", tool)
		}
	}

	_, err := callMCPTool(nil, "send_input", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for send_input with no session_id")
	}

	_, err = callMCPTool(nil, "create_session", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for create_session with no dir")
	}

	_, err = callMCPTool(nil, "list_file_changes", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for list_file_changes with no dir")
	}
}
