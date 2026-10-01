package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
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

		// Plugin skills: ~/.claude/plugins/marketplaces/*/plugins/*/skills/*/SKILL.md
		pluginGlob := filepath.Join(home, ".claude", "plugins", "marketplaces", "*", "plugins", "*", "skills", "*", "SKILL.md")
		if matches, err := filepath.Glob(pluginGlob); err == nil {
			for _, match := range matches {
				skillDir := filepath.Dir(match)
				pluginDir := filepath.Dir(filepath.Dir(skillDir))
				pluginName := filepath.Base(pluginDir)
				skillName := filepath.Base(skillDir)
				qualifiedName := pluginName + ":" + skillName

				if seen[qualifiedName] {
					continue
				}
				desc := skillFrontmatterField(match, "description")
				commands = append(commands, CommandInfo{
					Name:        qualifiedName,
					Source:      "plugin",
					Type:        "command",
					Description: desc,
				})
				seen[qualifiedName] = true
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
	// Claude stores sessions at ~/.claude/projects/-<path-with-dashes>/
	sessionDir := claudeProjectDir(dir)

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

// maxConversations caps how many sessions the picker offers. A single
// directory can hold hundreds, and each one costs two file reads.
const maxConversations = 20

// headReadBudget bounds how much of a file the head read scans for a title and
// cwd. In an interactive session Claude writes ai-title after the first user
// record, and that record can be megabytes when it holds a pasted image, so
// the read has to be able to step past it.
const headReadBudget = 4 * 1024 * 1024

// headMaxRecords stops the head read on a file with many small records and no
// title, rather than walking the whole budget.
const headMaxRecords = 64

// headLineCap is the longest record the head read parses. A longer one is
// skipped, and only its cwd is salvaged.
const headLineCap = 256 * 1024

// cwdField finds a cwd key in raw JSON. Used only on records too long to parse.
var cwdField = regexp.MustCompile(`"cwd":"((?:[^"\\]|\\.)*)"`)

// tailChunk is how much is read from the end of a session file at a time.
const tailChunk = 64 * 1024

// tailBudget caps the backwards search. Beyond this a session has no recent
// assistant message worth showing.
const tailBudget = 2 * 1024 * 1024

// ListConversations returns the Claude conversations stored for a directory,
// newest first.
//
// Sessions for other directories can share the same folder, because the
// project key folds every non-alphanumeric character to "-", so /x/foo_bar
// and /x/foo-bar land together. Each file records its own cwd, which is what
// separates them.
//
// Conversations Koko did not start are included on purpose. A plain `claude`
// run in the directory is exactly the history a user wants back.
func (cs *ClaudeService) ListConversations(dir string) ([]Conversation, error) {
	sessionDir := claudeProjectDir(dir)
	if sessionDir == "" {
		return []Conversation{}, nil
	}

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return []Conversation{}, nil // no sessions for this dir
	}

	type candidate struct {
		path     string
		uuid     string
		modified time.Time
		size     int64
	}

	var candidates []candidate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path:     filepath.Join(sessionDir, e.Name()),
			uuid:     strings.TrimSuffix(e.Name(), ".jsonl"),
			modified: info.ModTime(),
			size:     info.Size(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modified.After(candidates[j].modified)
	})

	// Filter after sorting, so the newest matching sessions win the cap.
	out := make([]Conversation, 0, maxConversations)
	for _, c := range candidates {
		if len(out) == maxConversations {
			break
		}
		head := readConversationHead(c.path)
		if !head.belongsTo(dir) {
			continue // another directory sharing this project folder
		}
		out = append(out, Conversation{
			UUID:       c.uuid,
			Title:      head.title,
			Preview:    lastAssistantText(c.path),
			ModifiedAt: c.modified.UnixMilli(),
			SizeBytes:  c.size,
		})
	}
	return out, nil
}

// conversationHead is what a head read of a session file yields.
type conversationHead struct {
	title string
	cwd   string
}

// belongsTo reports whether the session file belongs to dir. A file with no
// recorded cwd is given the benefit of the doubt, as the picker lists it.
//
// Listing and deleting both go through this one test, so the two can never
// disagree about which conversations a directory has.
func (h conversationHead) belongsTo(dir string) bool {
	return h.cwd == "" || sameDir(h.cwd, dir)
}

// readConversationHead reads the start of a session file for its title and
// working directory.
//
// Claude writes its own "ai-title" record near the top. Files from older
// versions may not have one, so the first real user prompt is the fallback.
// Meta and sidechain records are skipped: they are injected context and
// subagent turns, not what the user typed.
func readConversationHead(path string) conversationHead {
	f, err := os.Open(path)
	if err != nil {
		return conversationHead{}
	}
	defer func() { _ = f.Close() }()

	r := bufio.NewReaderSize(io.LimitReader(f, headReadBudget), headLineCap)

	var head conversationHead
	var fallback string
	for records := 0; records < headMaxRecords; records++ {
		line, isPrefix, err := r.ReadLine()
		if err != nil {
			break
		}
		if isPrefix {
			// Too long to parse, most likely a pasted image. Skip to the end
			// of the record and keep going: stopping here would hide the
			// ai-title written after it. Take the last cwd match, because in
			// a user record cwd follows the message body.
			salvaged := lastCWD(line)
			for isPrefix && err == nil {
				line, isPrefix, err = r.ReadLine()
				if m := lastCWD(line); m != "" {
					salvaged = m
				}
			}
			if head.cwd == "" {
				head.cwd = salvaged
			}
			if err != nil {
				break
			}
			continue
		}

		var entry struct {
			Type        string `json:"type"`
			AiTitle     string `json:"aiTitle"`
			CWD         string `json:"cwd"`
			IsMeta      bool   `json:"isMeta"`
			IsSidechain bool   `json:"isSidechain"`
			Message     struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if head.cwd == "" && entry.CWD != "" {
			head.cwd = entry.CWD
		}
		if head.title == "" && entry.Type == "ai-title" && entry.AiTitle != "" {
			head.title = shorten(entry.AiTitle, 120)
		}
		if fallback == "" && entry.Type == "user" && !entry.IsMeta && !entry.IsSidechain {
			fallback = shorten(userContentText(entry.Message.Content), 120)
		}
		if head.title != "" && head.cwd != "" {
			break
		}
	}
	if head.title == "" {
		head.title = fallback
	}
	return head
}

// lastCWD returns the last cwd value in a fragment of raw JSON, or "".
func lastCWD(fragment []byte) string {
	matches := cwdField.FindAllSubmatch(fragment, -1)
	if len(matches) == 0 {
		return ""
	}
	var cwd string
	quoted := append(append([]byte{'"'}, matches[len(matches)-1][1]...), '"')
	if err := json.Unmarshal(quoted, &cwd); err != nil {
		return ""
	}
	return cwd
}

// userContentText pulls plain text out of a user message. The content is
// either a bare string or an array of typed blocks.
func userContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			return strings.TrimSpace(b.Text)
		}
	}
	return ""
}

// shorten trims s to n characters, counting runes so a cut cannot split a
// multi-byte character.
func shorten(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 4 {
		// No room for an ellipsis, and r[:n-3] would slice with a negative
		// bound and panic.
		return string(r[:max(n, 0)])
	}
	return string(r[:n-3]) + "..."
}

// lastAssistantText returns the last assistant message in a JSONL file.
//
// It walks backwards in chunks rather than reading one fixed window. A single
// attachment or tool result can be hundreds of kilobytes, so a fixed 64KB tail
// often began inside one record and missed the assistant message just before
// it. That left most previews empty.
func lastAssistantText(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return ""
	}
	size := info.Size()

	for window := int64(tailChunk); ; window *= 2 {
		if window > tailBudget {
			window = tailBudget
		}
		offset := size - window
		atStart := offset <= 0
		if atStart {
			offset = 0
		}

		buf := make([]byte, size-offset)
		if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
			return ""
		}

		lines := bytes.Split(buf, []byte("\n"))
		// Unless the window reached the start of the file, the first line is
		// a fragment of a record that began earlier.
		if !atStart && len(lines) > 0 {
			lines = lines[1:]
		}
		for i := len(lines) - 1; i >= 0; i-- {
			if text := extractAssistantText(lines[i]); text != "" {
				return shorten(text, 120)
			}
		}

		if atStart || window >= tailBudget {
			return ""
		}
	}
}

// extractAssistantText extracts text content from a JSONL entry.
// Handles both formats:
//   - type:"assistant" with message.content[] (current Claude Code format)
//   - type:"progress" with data.message.message.content[] (legacy format)
func extractAssistantText(line []byte) string {
	if !strings.Contains(string(line), `"assistant"`) {
		return ""
	}

	var entry struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
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

	// Current format: type:"assistant", message.role:"assistant"
	if entry.Type == "assistant" && entry.Message.Role == "assistant" {
		for _, block := range entry.Message.Content {
			if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
				return strings.TrimSpace(block.Text)
			}
		}
	}

	// Legacy format: type:"progress", data.message.message.role:"assistant"
	if entry.Type == "progress" && entry.Data.Message.Message.Role == "assistant" {
		for _, block := range entry.Data.Message.Message.Content {
			if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
				return strings.TrimSpace(block.Text)
			}
		}
	}

	return ""
}

// skillFrontmatterField reads a specific field from YAML frontmatter in a SKILL.md file.
func skillFrontmatterField(path, field string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	prefix := field + ": "
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inFrontmatter {
				return "" // end of frontmatter, field not found
			}
			inFrontmatter = true
			continue
		}
		if inFrontmatter && strings.HasPrefix(trimmed, prefix) {
			val := strings.TrimPrefix(trimmed, prefix)
			if len(val) > 80 {
				val = val[:77] + "..."
			}
			return val
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

// conversationPath returns the session file for a conversation the picker
// lists for dir.
//
// The id comes from the webview and names a file to delete, so it must be a
// bare file name, the file must exist in dir's project folder, and it must
// pass the same directory test as the listing.
func conversationPath(dir, uuid string) (string, error) {
	if uuid == "" || uuid == "." || uuid == ".." || filepath.Base(uuid) != uuid {
		return "", fmt.Errorf("invalid conversation id %q", uuid)
	}
	sessionDir := claudeProjectDir(dir)
	if sessionDir == "" {
		return "", fmt.Errorf("no Claude project folder for %s", dir)
	}
	path := filepath.Join(sessionDir, uuid+".jsonl")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("conversation %s not found for %s", uuid, dir)
	}
	if !readConversationHead(path).belongsTo(dir) {
		return "", fmt.Errorf("conversation %s belongs to another directory", uuid)
	}
	return path, nil
}

// conversationsIn returns every conversation stored for dir, with no cap.
//
// Files with no recorded cwd are left out and counted as unknown. In a
// project folder that several directories share, such a file may belong to
// a neighbour, and a bulk delete must not guess.
func conversationsIn(dir string) (ids []string, unknown int) {
	sessionDir := claudeProjectDir(dir)
	if sessionDir == "" {
		return nil, 0
	}
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, 0
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		head := readConversationHead(filepath.Join(sessionDir, e.Name()))
		switch {
		case head.cwd == "":
			unknown++
		case sameDir(head.cwd, dir):
			ids = append(ids, strings.TrimSuffix(e.Name(), ".jsonl"))
		}
	}
	return ids, unknown
}

// removeConversation deletes a conversation's session file, and the folder
// Claude keeps beside it for subagent and tool output.
func removeConversation(dir, uuid string) error {
	path, err := conversationPath(dir, uuid)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return os.RemoveAll(strings.TrimSuffix(path, ".jsonl"))
}
