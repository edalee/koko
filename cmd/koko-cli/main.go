package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client := newClient(cfg)
	cmd := os.Args[1]

	switch cmd {
	case "sessions":
		cmdSessions(client)
	case "status":
		id := resolveArg(2)
		cmdStatus(client, id)
	case "send":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: koko-cli send <session-id> <text>")
			os.Exit(1)
		}
		cmdSend(client, os.Args[2], strings.Join(os.Args[3:], " "))
	case "output":
		id := resolveArg(2)
		cmdOutput(client, id)
	case "tail":
		id := resolveArg(2)
		cmdTail(client, id)
	case "files":
		dir := resolveArg(2)
		cmdFiles(client, dir)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func resolveArg(idx int) string {
	if len(os.Args) > idx {
		return os.Args[idx]
	}
	return ""
}

func printUsage() {
	fmt.Println(`koko-cli — Koko CLI companion

Commands:
  sessions              List active sessions
  status [slug]         Session state + output snippet
  send <slug> <text>    Send input to a session
  output [slug]         Recent terminal output
  tail [slug]           Stream output (WebSocket, Ctrl+C to stop)
  files [dir]           Git file changes in a directory
  help                  Show this help

Slugs look like "koko-1", "drumstick-2". Use "sessions" to see them.`)
}

func cmdSessions(client *apiClient) {
	body, err := client.get("/api/sessions")
	if err != nil {
		fatal(err)
	}

	var sessions []struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
		Name string `json:"name"`
		Dir  string `json:"dir"`
	}
	if err := json.Unmarshal(body, &sessions); err != nil {
		fatal(err)
	}

	if len(sessions) == 0 {
		fmt.Println("No active sessions.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tNAME\tDIRECTORY")
	for _, s := range sessions {
		slug := s.Slug
		if slug == "" {
			slug = s.ID
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", slug, s.Name, s.Dir)
	}
	_ = w.Flush()
}

func cmdStatus(client *apiClient, id string) {
	if id == "" {
		id = defaultSession(client)
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "Usage: koko-cli status [slug]")
		os.Exit(1)
	}

	body, err := client.get("/api/sessions/" + id)
	if err != nil {
		fatal(err)
	}

	var detail struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
		Name string `json:"name"`
		Dir  string `json:"dir"`
		State string `json:"state"`
	}
	_ = json.Unmarshal(body, &detail)

	slug := detail.Slug
	if slug == "" {
		slug = detail.ID
	}
	fmt.Printf("Session: %s (%s)\n", detail.Name, slug)
	fmt.Printf("Dir:     %s\n", detail.Dir)
	fmt.Printf("State:   %s\n", detail.State)

	// Also show last output
	outBody, err := client.get("/api/sessions/" + id + "/output")
	if err == nil {
		var out struct {
			Output string `json:"output"`
		}
		_ = json.Unmarshal(outBody, &out)
		if out.Output != "" {
			lines := strings.Split(out.Output, "\n")
			if len(lines) > 10 {
				lines = lines[len(lines)-10:]
			}
			fmt.Println("\nRecent output:")
			for _, l := range lines {
				fmt.Println("  " + l)
			}
		}
	}
}

func cmdSend(client *apiClient, id, text string) {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	_, err := client.post("/api/sessions/"+id+"/write", map[string]string{"text": text})
	if err != nil {
		fatal(err)
	}
	fmt.Println("Sent.")
}

func cmdOutput(client *apiClient, id string) {
	if id == "" {
		id = defaultSession(client)
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "Usage: koko-cli output [slug]")
		os.Exit(1)
	}

	body, err := client.get("/api/sessions/" + id + "/output")
	if err != nil {
		fatal(err)
	}

	var out struct {
		Output string `json:"output"`
	}
	_ = json.Unmarshal(body, &out)
	if out.Output == "" {
		fmt.Println("(no output)")
		return
	}
	fmt.Print(out.Output)
}

func cmdTail(client *apiClient, id string) {
	if id == "" {
		id = defaultSession(client)
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "Usage: koko-cli tail [slug]")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Streaming %s (Ctrl+C to stop)...\n", id)
	if err := client.streamWebSocket("/api/sessions/" + id + "/stream"); err != nil {
		fatal(err)
	}
}

func cmdFiles(client *apiClient, dir string) {
	if dir == "" {
		dir, _ = os.Getwd()
	}

	body, err := client.get("/api/files?dir=" + dir)
	if err != nil {
		fatal(err)
	}

	var changes []struct {
		Path   string `json:"path"`
		Status string `json:"status"`
		Staged bool   `json:"staged"`
	}
	if err := json.Unmarshal(body, &changes); err != nil {
		fatal(err)
	}

	if len(changes) == 0 {
		fmt.Println("No file changes.")
		return
	}

	for _, c := range changes {
		icon := "M"
		switch c.Status {
		case "added":
			icon = "A"
		case "deleted":
			icon = "D"
		case "renamed":
			icon = "R"
		}
		staged := ""
		if c.Staged {
			staged = " (staged)"
		}
		fmt.Printf("%s %s%s\n", icon, c.Path, staged)
	}
}

// defaultSession returns the slug (or ID) of the first session if there's exactly one.
func defaultSession(client *apiClient) string {
	body, err := client.get("/api/sessions")
	if err != nil {
		return ""
	}
	var sessions []struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	_ = json.Unmarshal(body, &sessions)
	if len(sessions) == 1 {
		if sessions[0].Slug != "" {
			return sessions[0].Slug
		}
		return sessions[0].ID
	}
	return ""
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
