package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ProcessMonitor scans child process trees for Claude sessions.
type ProcessMonitor struct{}

// NewProcessMonitor creates a ProcessMonitor.
func NewProcessMonitor() *ProcessMonitor {
	return &ProcessMonitor{}
}

// GetChildProcesses returns classified child processes for a given PID.
func (pm *ProcessMonitor) GetChildProcesses(pid int) ([]ProcessInfo, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid PID: %d", pid)
	}

	// Get direct children
	childPIDs, err := getChildPIDs(pid)
	if err != nil || len(childPIDs) == 0 {
		return nil, nil
	}

	// Get process details for all children
	processes, err := getProcessDetails(childPIDs)
	if err != nil {
		return nil, err
	}

	// Classify and count grandchildren
	var result []ProcessInfo
	for i := range processes {
		processes[i].Type = classifyProcess(processes[i].Command, processes[i].FullCmd)
		// Use a friendly name for MCP servers
		if processes[i].Type == "mcp" {
			processes[i].Command = mcpDisplayName(processes[i].FullCmd)
		}
		grandchildren, _ := getChildPIDs(processes[i].PID)
		processes[i].Children = len(grandchildren)
		result = append(result, processes[i])
	}

	return result, nil
}

func getChildPIDs(pid int) ([]int, error) {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		// pgrep returns exit 1 when no matches — not an error
		return nil, nil
	}

	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		p, err := strconv.Atoi(line)
		if err == nil {
			pids = append(pids, p)
		}
	}
	return pids, nil
}

func getProcessDetails(pids []int) ([]ProcessInfo, error) {
	if len(pids) == 0 {
		return nil, nil
	}

	// Build pid list for ps
	pidStrs := make([]string, len(pids))
	for i, p := range pids {
		pidStrs[i] = strconv.Itoa(p)
	}

	// ps -o pid=,etime=,command= -p <pids>
	args := []string{"-o", "pid=,etime=,command=", "-p", strings.Join(pidStrs, ",")}
	out, err := exec.Command("ps", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w", err)
	}

	var processes []ProcessInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		info, err := parsePSLine(line)
		if err != nil {
			continue
		}
		processes = append(processes, info)
	}

	return processes, nil
}

// parsePSLine parses a line from `ps -o pid=,etime=,command=`
// Format: "  1234 01:23:45 /usr/bin/claude --flag"
func parsePSLine(line string) (ProcessInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return ProcessInfo{}, fmt.Errorf("too few fields")
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return ProcessInfo{}, err
	}

	elapsed := fields[1]
	fullCmd := strings.Join(fields[2:], " ")
	shortCmd := shortCommandName(fullCmd)

	return ProcessInfo{
		PID:       pid,
		Command:   shortCmd,
		FullCmd:   fullCmd,
		Elapsed:   formatElapsed(elapsed),
		ElapsedMs: parseElapsedMs(elapsed),
	}, nil
}

// classifyProcess determines the type based on command name and full command.
func classifyProcess(cmd, fullCmd string) string {
	lower := strings.ToLower(cmd)
	lowerFull := strings.ToLower(fullCmd)

	// Subagents are claude processes
	if strings.HasPrefix(lower, "claude") {
		return "subagent"
	}

	// MCP servers
	mcpPatterns := []string{"nolo", "docker mcp", "mcp-server", "mcp_server", "npx -y @"}
	for _, p := range mcpPatterns {
		if strings.Contains(lowerFull, p) {
			return "mcp"
		}
	}

	// Infrastructure processes (non-MCP)
	infraPatterns := []string{"caffeinate", "docker-credential"}
	for _, p := range infraPatterns {
		if strings.Contains(lower, p) {
			return "infrastructure"
		}
	}

	return "tool"
}

// mcpDisplayName extracts a friendly name for an MCP server from its command line.
func mcpDisplayName(fullCmd string) string {
	lower := strings.ToLower(fullCmd)

	// "nolo serve" → "nolo"
	if strings.Contains(lower, "nolo") {
		return "nolo"
	}

	// "docker mcp gateway run" → "docker mcp"
	if strings.Contains(lower, "docker mcp") {
		return "docker mcp"
	}

	// "npx -y @modelcontextprotocol/server-filesystem" → "filesystem"
	// "npx -y @some-org/mcp-server-foo" → "foo"
	if idx := strings.Index(fullCmd, "npx"); idx >= 0 {
		parts := strings.Fields(fullCmd[idx:])
		for _, p := range parts {
			if strings.Contains(p, "@") || strings.Contains(p, "mcp") {
				// Extract last segment after / and strip "mcp-server-" or "server-" prefix
				if slash := strings.LastIndex(p, "/"); slash >= 0 {
					name := p[slash+1:]
					name = strings.TrimPrefix(name, "mcp-server-")
					name = strings.TrimPrefix(name, "server-")
					if name != "" {
						return name
					}
				}
			}
		}
	}

	// Fallback: binary name
	parts := strings.Fields(fullCmd)
	if len(parts) > 0 {
		bin := parts[0]
		if idx := strings.LastIndex(bin, "/"); idx >= 0 {
			bin = bin[idx+1:]
		}
		return bin
	}

	return "mcp server"
}

// shortCommandName extracts a readable name from a full command path.
func shortCommandName(fullCmd string) string {
	// Get the binary name (last path segment of first arg)
	parts := strings.Fields(fullCmd)
	if len(parts) == 0 {
		return fullCmd
	}

	bin := parts[0]
	// Strip path
	if idx := strings.LastIndex(bin, "/"); idx >= 0 {
		bin = bin[idx+1:]
	}

	return bin
}

// formatElapsed converts ps etime format to human-readable.
// ps etime formats: "MM:SS", "HH:MM:SS", "D-HH:MM:SS"
func formatElapsed(etime string) string {
	ms := parseElapsedMs(etime)
	d := time.Duration(ms) * time.Millisecond

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// parseElapsedMs converts ps etime format to milliseconds.
func parseElapsedMs(etime string) int64 {
	etime = strings.TrimSpace(etime)
	var days, hours, minutes, seconds int

	// Check for days: "D-HH:MM:SS"
	if idx := strings.Index(etime, "-"); idx >= 0 {
		d, _ := strconv.Atoi(etime[:idx])
		days = d
		etime = etime[idx+1:]
	}

	parts := strings.Split(etime, ":")
	switch len(parts) {
	case 3: // HH:MM:SS
		hours, _ = strconv.Atoi(parts[0])
		minutes, _ = strconv.Atoi(parts[1])
		seconds, _ = strconv.Atoi(parts[2])
	case 2: // MM:SS
		minutes, _ = strconv.Atoi(parts[0])
		seconds, _ = strconv.Atoi(parts[1])
	case 1: // SS
		seconds, _ = strconv.Atoi(parts[0])
	}

	total := time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second

	return total.Milliseconds()
}
