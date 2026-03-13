package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ClaudeService provides session context data (MCP servers, agents, commands).
type ClaudeService struct{}

// NewClaudeService creates a ClaudeService.
func NewClaudeService() *ClaudeService {
	return &ClaudeService{}
}

// cleanEnv returns os.Environ() with CLAUDECODE stripped to avoid nested session detection.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	return env
}

// GetMCPServers returns configured MCP servers and their connection status.
func (cs *ClaudeService) GetMCPServers(dir string) ([]MCPServer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15_000_000_000) // 15s
	defer cancel()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}

	cmd := exec.CommandContext(ctx, shell, "-l", "-c", "claude mcp list")
	cmd.Dir = dir
	cmd.Env = cleanEnv()

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("claude mcp list failed: %w", err)
	}

	return parseMCPList(string(out)), nil
}

// GetAgents returns built-in Claude agents.
func (cs *ClaudeService) GetAgents(dir string) ([]AgentInfo, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10_000_000_000) // 10s
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-l", "-c", "claude agents")
	cmd.Dir = dir
	cmd.Env = cleanEnv()

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("claude agents failed: %w", err)
	}

	return parseAgentsList(string(out)), nil
}

// GetCommands scans for slash commands and custom agents from the filesystem.
func (cs *ClaudeService) GetCommands(dir string) ([]CommandInfo, error) {
	seen := make(map[string]bool)
	var commands []CommandInfo

	// Project commands: dir/.claude/commands/*.md
	projectCmds := filepath.Join(dir, ".claude", "commands")
	if entries, err := os.ReadDir(projectCmds); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			desc := firstLine(filepath.Join(projectCmds, e.Name()))
			commands = append(commands, CommandInfo{Name: name, Source: "project", Type: "command", Description: desc})
			seen[name] = true
		}
	}

	// Project agents: dir/.claude/agents/*.md
	projectAgents := filepath.Join(dir, ".claude", "agents")
	if entries, err := os.ReadDir(projectAgents); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			desc := firstLine(filepath.Join(projectAgents, e.Name()))
			commands = append(commands, CommandInfo{Name: name, Source: "project", Type: "agent", Description: desc})
			seen[name] = true
		}
	}

	// Global commands: ~/.claude/commands/ (file or dir)
	home, _ := os.UserHomeDir()
	if home != "" {
		globalCmds := filepath.Join(home, ".claude", "commands")
		if entries, err := os.ReadDir(globalCmds); err == nil {
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() {
					// Directory-based commands: look for index.md or just use dir name
					if seen[name] {
						continue
					}
					desc := firstLine(filepath.Join(globalCmds, name, "index.md"))
					commands = append(commands, CommandInfo{Name: name, Source: "global", Type: "command", Description: desc})
					seen[name] = true
				} else if strings.HasSuffix(name, ".md") {
					trimmed := strings.TrimSuffix(name, ".md")
					if seen[trimmed] {
						continue
					}
					desc := firstLine(filepath.Join(globalCmds, name))
					commands = append(commands, CommandInfo{Name: trimmed, Source: "global", Type: "command", Description: desc})
					seen[trimmed] = true
				}
			}
		}
	}

	return commands, nil
}

// parseMCPList parses output from `claude mcp list`.
func parseMCPList(output string) []MCPServer {
	var servers []MCPServer
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Checking") || strings.HasPrefix(line, "─") {
			continue
		}

		// Format: "name: command - ✓ Connected"
		colonIdx := strings.Index(line, ": ")
		if colonIdx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colonIdx])
		rest := line[colonIdx+2:]

		command := rest
		status := "error"

		if dashIdx := strings.LastIndex(rest, " - "); dashIdx >= 0 {
			command = strings.TrimSpace(rest[:dashIdx])
			statusPart := rest[dashIdx+3:]
			if strings.Contains(statusPart, "✓") || strings.Contains(statusPart, "Connected") {
				status = "connected"
			} else if strings.Contains(statusPart, "!") || strings.Contains(statusPart, "auth") {
				status = "auth_needed"
			}
		}

		servers = append(servers, MCPServer{
			Name:    name,
			Command: command,
			Status:  status,
		})
	}
	return servers
}

// parseAgentsList parses output from `claude agents`.
// Format:
//
//	5 active agents
//
//	Built-in agents:
//	  claude-code-guide · haiku
//	  Explore · haiku
func parseAgentsList(output string) []AgentInfo {
	var agents []AgentInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Only parse lines with middle dot separator: "name · model"
		if parts := strings.SplitN(line, "·", 2); len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			model := strings.TrimSpace(parts[1])
			if name != "" && model != "" {
				agents = append(agents, AgentInfo{Name: name, Model: model})
			}
		}
	}
	return agents
}

// GetLastMessage returns the last assistant text from the most recent Claude session for a directory.
func (cs *ClaudeService) GetLastMessage(dir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Claude stores sessions at ~/.claude/projects/-<path-with-dashes>/
	projectKey := strings.ReplaceAll(dir, "/", "-")
	sessionDir := filepath.Join(home, ".claude", "projects", projectKey)

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return "", nil // no sessions for this dir
	}

	// Find most recent .jsonl by mod time
	var newest string
	var newestTime int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UnixMilli() > newestTime {
			newestTime = info.ModTime().UnixMilli()
			newest = filepath.Join(sessionDir, e.Name())
		}
	}
	if newest == "" {
		return "", nil
	}

	return lastAssistantText(newest), nil
}

// lastAssistantText reads the last N lines of a JSONL file and extracts the last assistant text.
func lastAssistantText(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	// Read last 64KB to find recent messages
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	offset := info.Size() - 64*1024
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return ""
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	var lastText string
	for scanner.Scan() {
		line := scanner.Bytes()
		text := extractAssistantText(line)
		if text != "" {
			lastText = text
		}
	}

	if len(lastText) > 120 {
		lastText = lastText[:117] + "..."
	}
	return lastText
}

// extractAssistantText extracts text content from a JSONL progress entry.
func extractAssistantText(line []byte) string {
	// Quick reject — avoid parsing non-assistant lines
	if !strings.Contains(string(line), `"assistant"`) {
		return ""
	}

	var entry struct {
		Type string `json:"type"`
		Data struct {
			Message struct {
				Message struct {
					Role    string `json:"role"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			} `json:"message"`
		} `json:"data"`
	}

	if err := json.Unmarshal(line, &entry); err != nil {
		return ""
	}

	if entry.Type != "progress" || entry.Data.Message.Message.Role != "assistant" {
		return ""
	}

	for _, block := range entry.Data.Message.Message.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return strings.TrimSpace(block.Text)
		}
	}
	return ""
}

// firstLine reads the first non-empty, non-frontmatter line from a file, capped at 80 chars.
func firstLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter || line == "" {
			continue
		}
		// Strip markdown heading prefix
		line = strings.TrimLeft(line, "# ")
		if len(line) > 80 {
			line = line[:77] + "..."
		}
		return line
	}
	return ""
}
