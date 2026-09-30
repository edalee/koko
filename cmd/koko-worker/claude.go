package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// claudeSettings is the only settings source for worker runs. Your user
// settings are left out, so their allow rules, hooks and plugins do not apply.
const claudeSettings = `{"permissions": {"allow": [], "deny": []}, "disableAllHooks": true}`

// denyWrites is refused in every run, whatever the allowlist says.
var denyWrites = []string{
	"Write", "Edit", "NotebookEdit",
	"Bash(gh pr merge:*)", "Bash(gh pr edit:*)", "Bash(gh pr comment:*)", "Bash(gh pr review:*)",
	"Bash(gh pr close:*)", "Bash(gh api:*)", "Bash(git push:*)", "Bash(git commit:*)",
	"mcp__claude_ai_Atlassian__addCommentToJiraIssue", "mcp__claude_ai_Atlassian__editJiraIssue",
	"mcp__claude_ai_Atlassian__transitionJiraIssue", "mcp__claude_ai_Atlassian__createJiraIssue",
	"mcp__claude_ai_Google_Calendar__delete_event", "mcp__claude_ai_Google_Calendar__update_event",
	"mcp__claude_ai_Google_Calendar__respond_to_event",
}

const (
	toolListEvents  = "mcp__claude_ai_Google_Calendar__list_events"
	toolCreateEvent = "mcp__claude_ai_Google_Calendar__create_event"
)

var jiraRead = []string{
	"mcp__claude_ai_Atlassian__getAccessibleAtlassianResources",
	"mcp__claude_ai_Atlassian__getJiraIssue",
	"mcp__claude_ai_Atlassian__searchJiraIssuesUsingJql",
}

// ToolCall is one tool use in a run and its raw result.
type ToolCall struct {
	Name    string
	Input   json.RawMessage
	Result  string
	IsError bool
}

// ClaudeResult is the final text and every tool call the run made.
type ClaudeResult struct {
	Text  string
	Calls []ToolCall
}

// callsTo returns the calls to one tool, in order.
func (r ClaudeResult) callsTo(name string) []ToolCall {
	var out []ToolCall
	for _, c := range r.Calls {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

type claudeRunner struct {
	settingsPath string
}

// run starts `claude -p` with the prompt on stdin. The tool flags take several
// values, so a prompt passed as an argument after them would be swallowed.
func (c claudeRunner) run(ctx context.Context, prompt string, allowed []string, timeout time.Duration) (ClaudeResult, error) {
	if err := os.WriteFile(c.settingsPath, []byte(claudeSettings), 0o600); err != nil {
		return ClaudeResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-p",
		"--permission-mode", "dontAsk",
		"--no-session-persistence",
		"--setting-sources", "",
		"--settings", c.settingsPath,
		"--output-format", "stream-json", "--verbose",
		"--allowedTools=" + strings.Join(allowed, ","),
		"--disallowedTools=" + strings.Join(denyWrites, ","),
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return ClaudeResult{}, fmt.Errorf("claude timed out after %s", timeout)
	}
	res, perr := parseStream(out)
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = res.Text
		}
		return res, fmt.Errorf("claude failed: %v: %s", err, truncate(msg, 400))
	}
	return res, perr
}

// parseStream reads stream-json output: assistant tool uses, their raw
// results, and the final result text.
func parseStream(out []byte) (ClaudeResult, error) {
	var res ClaudeResult
	index := map[string]int{} // tool_use id -> position in res.Calls
	gotResult := false

	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var msg struct {
			Type    string `json:"type"`
			Result  string `json:"result"`
			IsError bool   `json:"is_error"`
			Message struct {
				Content []struct {
					Type      string          `json:"type"`
					ID        string          `json:"id"`
					Name      string          `json:"name"`
					Input     json.RawMessage `json:"input"`
					ToolUseID string          `json:"tool_use_id"`
					Content   json.RawMessage `json:"content"`
					IsError   bool            `json:"is_error"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &msg) != nil {
			continue
		}
		switch msg.Type {
		case "assistant":
			for _, c := range msg.Message.Content {
				if c.Type == "tool_use" {
					index[c.ID] = len(res.Calls)
					res.Calls = append(res.Calls, ToolCall{Name: c.Name, Input: c.Input})
				}
			}
		case "user":
			for _, c := range msg.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				if i, ok := index[c.ToolUseID]; ok {
					res.Calls[i].Result = toolResultText(c.Content)
					res.Calls[i].IsError = c.IsError
				}
			}
		case "result":
			res.Text = strings.TrimSpace(msg.Result)
			gotResult = true
			if msg.IsError {
				return res, fmt.Errorf("claude run ended in error: %s", truncate(res.Text, 400))
			}
		}
	}
	if !gotResult {
		return res, fmt.Errorf("claude gave no result")
	}
	return res, nil
}

// toolResultText flattens a tool_result content, which is a string or a list of text blocks.
func toolResultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var b strings.Builder
		for _, bl := range blocks {
			if bl.Type == "text" {
				b.WriteString(bl.Text)
			}
		}
		return b.String()
	}
	return string(raw)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// extractJSON returns the first JSON array or object in Claude's text,
// ignoring code fences and any words around it.
func extractJSON(text string) string {
	start := strings.IndexAny(text, "[{")
	if start < 0 {
		return ""
	}
	open, close := text[start], byte(']')
	if open == '{' {
		close = '}'
	}
	end := strings.LastIndexByte(text, close)
	if end < start {
		return ""
	}
	return text[start : end+1]
}
