package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SlackCommandHandler listens for DMs to the bot and responds to commands.
type SlackCommandHandler struct {
	cfg           *ConfigService
	tm            *TerminalManager
	git           *GitService
	lastTS        string   // track last processed message timestamp
	botUserID     string
	imChannelIDs  []string // cached IM channel IDs (resolved once, refreshed periodically)
	imRefreshedAt time.Time
	httpClient    *http.Client
}

// NewSlackCommandHandler creates a new handler.
func NewSlackCommandHandler(cfg *ConfigService, tm *TerminalManager, git *GitService) *SlackCommandHandler {
	return &SlackCommandHandler{
		cfg:        cfg,
		tm:         tm,
		git:        git,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Start begins polling the bot's DM channel for commands.
func (h *SlackCommandHandler) Start() {
	for {
		config := h.cfg.GetConfig()
		if config.SlackToken == "" {
			time.Sleep(30 * time.Second)
			continue
		}

		if h.botUserID == "" {
			h.initBotIdentity(config.SlackToken)
		}

		if h.botUserID != "" {
			h.pollMessages(config.SlackToken)
		}

		time.Sleep(5 * time.Second)
	}
}

func (h *SlackCommandHandler) initBotIdentity(token string) {
	resp, err := h.slackAPI(token, "auth.test", nil)
	if err != nil {
		return
	}
	h.botUserID, _ = resp["user_id"].(string)
	if h.botUserID == "" {
		return
	}
	log.Printf("[slack-cmd] bot identity: %s", h.botUserID)
	h.refreshIMChannels(token)
}

// refreshIMChannels fetches IM channel IDs once and caches them.
// Refreshes every 5 minutes to pick up new DM conversations.
func (h *SlackCommandHandler) refreshIMChannels(token string) {
	convResp, err := h.slackAPI(token, "conversations.list", map[string]string{
		"types": "im",
		"limit": "50",
	})
	if err != nil {
		log.Printf("[slack-cmd] failed to list IMs: %v", err)
		return
	}

	channels, _ := convResp["channels"].([]interface{})
	h.imChannelIDs = nil
	for _, ch := range channels {
		conv, ok := ch.(map[string]interface{})
		if !ok {
			continue
		}
		channelID, _ := conv["id"].(string)
		if channelID != "" {
			h.imChannelIDs = append(h.imChannelIDs, channelID)
		}
	}
	h.imRefreshedAt = time.Now()
	log.Printf("[slack-cmd] cached %d IM channels", len(h.imChannelIDs))
}

// pollMessages checks cached IM channels for new commands.
// Calls conversations.history once per channel (no conversations.list per poll).
// Refreshes the channel list every 5 minutes.
func (h *SlackCommandHandler) pollMessages(token string) {
	// Refresh channel list periodically (picks up new DM conversations)
	if time.Since(h.imRefreshedAt) > 5*time.Minute {
		h.refreshIMChannels(token)
	}

	for _, channelID := range h.imChannelIDs {
		h.pollChannel(token, channelID)
	}
}

// pollChannel fetches new messages from a single channel and handles commands.
func (h *SlackCommandHandler) pollChannel(token, channelID string) {
	params := map[string]string{
		"channel": channelID,
		"limit":   "5",
	}
	if h.lastTS != "" {
		params["oldest"] = h.lastTS
	} else {
		params["oldest"] = fmt.Sprintf("%d", time.Now().Unix()-30)
	}

	histResp, err := h.slackAPI(token, "conversations.history", params)
	if err != nil {
		return
	}

	msgs, ok := histResp["messages"].([]interface{})
	if !ok {
		return
	}

	// Only respond to the configured owner
	ownerID := h.cfg.GetConfig().SlackOwnerID

	for i := len(msgs) - 1; i >= 0; i-- {
		msg, ok := msgs[i].(map[string]interface{})
		if !ok {
			continue
		}

		sender, _ := msg["user"].(string)
		if sender == h.botUserID {
			continue
		}

		// Ignore messages from non-owners
		if ownerID != "" && sender != ownerID {
			continue
		}

		if _, hasSubtype := msg["subtype"]; hasSubtype {
			continue
		}

		text, _ := msg["text"].(string)
		ts, _ := msg["ts"].(string)

		if ts > h.lastTS {
			h.lastTS = ts
		}

		reply := h.handleCommand(text)
		if reply != "" {
			h.sendMessage(token, channelID, reply)
		}
	}
}

func (h *SlackCommandHandler) handleCommand(text string) string {
	text = strings.TrimSpace(text)
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return ""
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "sessions":
		sessions := h.tm.GetSessions()
		if len(sessions) == 0 {
			return "No active sessions."
		}
		var lines []string
		for _, s := range sessions {
			state := h.tm.GetSessionState(s.ID)
			lines = append(lines, fmt.Sprintf("*%s* (`%s`) — %s\n  `%s`", s.Name, s.ID, state, s.Dir))
		}
		return strings.Join(lines, "\n")

	case "status":
		id := h.resolveSessionID(parts)
		if id == "" {
			return "Usage: `status <session-id>`"
		}
		state := h.tm.GetSessionState(id)
		output, err := h.tm.ReadOutput(id)
		if err != nil {
			return fmt.Sprintf("Session `%s` not found.", id)
		}
		// Trim to last ~500 chars
		if len(output) > 500 {
			output = output[len(output)-500:]
		}
		return fmt.Sprintf("*State:* %s\n```\n%s\n```", state, output)

	case "send":
		if len(parts) < 3 {
			return "Usage: `send <session-id> <text>`"
		}
		id := parts[1]
		text := strings.Join(parts[2:], " ") + "\n"
		encoded := slackEncodeBase64([]byte(text))
		if err := h.tm.Write(id, encoded); err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		return fmt.Sprintf("Sent to `%s`.", id)

	case "files":
		if len(parts) < 2 {
			return "Usage: `files <session-id>`"
		}
		// Get directory from session
		sessions := h.tm.GetSessions()
		var dir string
		for _, s := range sessions {
			if s.ID == parts[1] {
				dir = s.Dir
				break
			}
		}
		if dir == "" {
			return fmt.Sprintf("Session `%s` not found.", parts[1])
		}
		changes, err := h.git.GetFileChanges(dir)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		if len(changes) == 0 {
			return "No file changes."
		}
		var lines []string
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
			lines = append(lines, fmt.Sprintf("`%s` %s%s", icon, c.Path, staged))
		}
		return strings.Join(lines, "\n")

	case "output":
		id := h.resolveSessionID(parts)
		if id == "" {
			return "Usage: `output <session-id>`"
		}
		output, err := h.tm.ReadOutput(id)
		if err != nil {
			return fmt.Sprintf("Session `%s` not found.", id)
		}
		if output == "" {
			return "(no output)"
		}
		// Last ~50 lines
		lines := strings.Split(output, "\n")
		if len(lines) > 50 {
			lines = lines[len(lines)-50:]
		}
		return fmt.Sprintf("```\n%s\n```", strings.Join(lines, "\n"))

	case "help":
		return "*Koko Commands:*\n" +
			"`sessions` — List active sessions\n" +
			"`status <id>` — Session state + output snippet\n" +
			"`send <id> <text>` — Send text to session\n" +
			"`files <id>` — Git file changes\n" +
			"`output <id>` — Last ~50 lines of output\n" +
			"`help` — This message"

	default:
		return fmt.Sprintf("Unknown command: `%s`. Type `help` for available commands.", cmd)
	}
}

func (h *SlackCommandHandler) resolveSessionID(parts []string) string {
	if len(parts) < 2 {
		// Default to first session if only one exists
		sessions := h.tm.GetSessions()
		if len(sessions) == 1 {
			return sessions[0].ID
		}
		return ""
	}
	return parts[1]
}

func (h *SlackCommandHandler) sendMessage(token, channel, text string) {
	payload := map[string]string{
		"channel": channel,
		"text":    text,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", strings.NewReader(string(data)))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Printf("[slack-cmd] send failed: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.ReadAll(resp.Body)
}

func (h *SlackCommandHandler) slackAPI(token, method string, params map[string]string) (map[string]interface{}, error) {
	u, _ := url.Parse("https://slack.com/api/" + method)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if ok, _ := result["ok"].(bool); !ok {
		errMsg, _ := result["error"].(string)
		return nil, fmt.Errorf("slack API error: %s", errMsg)
	}

	return result, nil
}

func slackEncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
