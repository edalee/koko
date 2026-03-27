package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// mcpToolDefinitions returns the MCP tool schema definitions.
func mcpToolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "list_sessions",
			"description": "List all active Koko terminal sessions (Claude Code instances)",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "get_session_state",
			"description": "Get the state of a session (idle or waiting for approval)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]string{
						"type":        "string",
						"description": "Session slug (e.g. koko-1) or PTY ID",
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			"name":        "read_output",
			"description": "Read recent terminal output from a session (last ~2KB of text, ANSI stripped)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]string{
						"type":        "string",
						"description": "Session slug (e.g. koko-1) or PTY ID",
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			"name":        "send_input",
			"description": "Send text input to a session's terminal (appends newline by default)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]string{
						"type":        "string",
						"description": "Session slug (e.g. koko-1) or PTY ID",
					},
					"text": map[string]string{
						"type":        "string",
						"description": "Text to send (newline appended unless ends with \\n)",
					},
				},
				"required": []string{"session_id", "text"},
			},
		},
		{
			"name":        "create_session",
			"description": "Create a new Claude Code session in a directory",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]string{
						"type":        "string",
						"description": "Session display name",
					},
					"dir": map[string]string{
						"type":        "string",
						"description": "Working directory for the session",
					},
					"resume": map[string]string{
						"type":        "boolean",
						"description": "Resume previous session (--continue flag)",
					},
				},
				"required": []string{"dir"},
			},
		},
		{
			"name":        "close_session",
			"description": "Close/terminate a session",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]string{
						"type":        "string",
						"description": "Session slug (e.g. koko-1) or PTY ID",
					},
				},
				"required": []string{"session_id"},
			},
		},
		{
			"name":        "interact",
			"description": "Send text to a session and wait for the response. Blocks until output settles (no new data for quiet_ms). Returns ANSI-stripped text. Use this instead of send_input + read_output for conversational interaction.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]string{
						"type":        "string",
						"description": "Session slug (e.g. koko-1) or PTY ID",
					},
					"text": map[string]string{
						"type":        "string",
						"description": "Text to send (newline appended automatically)",
					},
					"timeout_ms": map[string]interface{}{
						"type":        "number",
						"description": "Max wait time in ms (default 120000)",
					},
					"quiet_ms": map[string]interface{}{
						"type":        "number",
						"description": "Output settle time in ms — response is complete when no output for this long (default 3000)",
					},
				},
				"required": []string{"session_id", "text"},
			},
		},
		{
			"name":        "list_file_changes",
			"description": "List git file changes (staged and unstaged) in a directory",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dir": map[string]string{
						"type":        "string",
						"description": "Directory to check for git changes",
					},
				},
				"required": []string{"dir"},
			},
		},
	}
}

// callMCPTool dispatches a tool call to the Koko HTTP API.
func callMCPTool(client *mcpClient, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "list_sessions":
		body, err := client.get("/api/sessions")
		if err != nil {
			return "", err
		}
		return formatJSON(body), nil

	case "get_session_state":
		id, _ := args["session_id"].(string)
		if id == "" {
			return "", fmt.Errorf("session_id is required")
		}
		body, err := client.get("/api/sessions/" + id + "/state")
		if err != nil {
			return "", err
		}
		return formatJSON(body), nil

	case "read_output":
		id, _ := args["session_id"].(string)
		if id == "" {
			return "", fmt.Errorf("session_id is required")
		}
		body, err := client.get("/api/sessions/" + id + "/output")
		if err != nil {
			return "", err
		}
		var result struct {
			Output string `json:"output"`
		}
		_ = json.Unmarshal(body, &result)
		if result.Output == "" {
			return "(no output)", nil
		}
		return result.Output, nil

	case "send_input":
		id, _ := args["session_id"].(string)
		text, _ := args["text"].(string)
		if id == "" {
			return "", fmt.Errorf("session_id is required")
		}
		// Append newline if not already present
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		_, err := client.post("/api/sessions/"+id+"/write", map[string]string{"text": text})
		if err != nil {
			return "", err
		}
		return "Input sent successfully", nil

	case "interact":
		id, _ := args["session_id"].(string)
		text, _ := args["text"].(string)
		if id == "" {
			return "", fmt.Errorf("session_id is required")
		}
		if text == "" {
			return "", fmt.Errorf("text is required")
		}
		payload := map[string]interface{}{
			"text": text,
		}
		if timeoutMs, ok := args["timeout_ms"].(float64); ok && timeoutMs > 0 {
			payload["timeout_ms"] = int(timeoutMs)
		}
		if quietMs, ok := args["quiet_ms"].(float64); ok && quietMs > 0 {
			payload["quiet_ms"] = int(quietMs)
		}
		body, err := client.post("/api/sessions/"+id+"/interact", payload)
		if err != nil {
			return "", err
		}
		var result struct {
			Output string `json:"output"`
		}
		_ = json.Unmarshal(body, &result)
		if result.Output == "" {
			return "(no output)", nil
		}
		return result.Output, nil

	case "create_session":
		dir, _ := args["dir"].(string)
		name, _ := args["name"].(string)
		resume, _ := args["resume"].(bool)
		if dir == "" {
			return "", fmt.Errorf("dir is required")
		}
		if name == "" {
			name = "MCP Session"
		}
		body, err := client.post("/api/sessions", map[string]interface{}{
			"name":   name,
			"dir":    dir,
			"resume": resume,
		})
		if err != nil {
			return "", err
		}
		return formatJSON(body), nil

	case "close_session":
		id, _ := args["session_id"].(string)
		if id == "" {
			return "", fmt.Errorf("session_id is required")
		}
		_, err := client.delete("/api/sessions/" + id)
		if err != nil {
			return "", err
		}
		return "Session closed", nil

	case "list_file_changes":
		dir, _ := args["dir"].(string)
		if dir == "" {
			return "", fmt.Errorf("dir is required")
		}
		body, err := client.get("/api/files?dir=" + dir)
		if err != nil {
			return "", err
		}
		return formatJSON(body), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func formatJSON(data []byte) string {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(data)
	}
	return string(pretty)
}
