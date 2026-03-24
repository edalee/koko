package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// SlackMessage represents a DM or @mention for display.
type SlackMessage struct {
	Type      string  `json:"type"`      // "dm" or "mention"
	Channel   string  `json:"channel"`   // channel name or DM user name
	ChannelID string  `json:"channelId"` // for deep linking
	User      string  `json:"user"`      // sender display name
	Text      string  `json:"text"`      // message preview
	Timestamp string  `json:"timestamp"` // Slack ts
	TeamID    string  `json:"teamId"`    // for deep linking
	Unread    bool    `json:"unread"`
	Time      float64 `json:"time"` // unix timestamp for sorting
}

// SlackService fetches DMs and @mentions from Slack.
type SlackService struct {
	config *ConfigService
}

// NewSlackService creates a SlackService.
func NewSlackService(config *ConfigService) *SlackService {
	return &SlackService{config: config}
}

// TestConnection verifies the Slack token works and returns the authenticated user.
func (ss *SlackService) TestConnection() (string, error) {
	cfg := ss.config.GetConfig()
	if cfg.SlackToken == "" {
		return "", fmt.Errorf("no Slack token configured")
	}

	resp, err := ss.apiGet(cfg.SlackToken, "auth.test", nil)
	if err != nil {
		return "", err
	}

	user, _ := resp["user"].(string)
	team, _ := resp["team"].(string)
	return fmt.Sprintf("%s @ %s", user, team), nil
}

// DebugFetch returns raw diagnostic info about what Slack API returns.
func (ss *SlackService) DebugFetch() (string, error) {
	cfg := ss.config.GetConfig()
	if cfg.SlackToken == "" {
		return "", fmt.Errorf("no Slack token configured")
	}
	token := cfg.SlackToken

	var out string

	// Test conversations.list
	convResp, err := ss.apiGet(token, "conversations.list", map[string]string{
		"types":            "im",
		"limit":            "5",
		"exclude_archived": "true",
	})
	if err != nil {
		out += fmt.Sprintf("conversations.list error: %v\n", err)
	} else {
		channels, _ := convResp["channels"].([]interface{})
		out += fmt.Sprintf("conversations.list: %d IM channels\n", len(channels))
		for i, ch := range channels {
			if i >= 3 {
				break
			}
			conv, _ := ch.(map[string]interface{})
			id, _ := conv["id"].(string)
			user, _ := conv["user"].(string)
			out += fmt.Sprintf("  - channel=%s user=%s\n", id, user)

			// Try history for first channel
			if i == 0 {
				histResp, err := ss.apiGet(token, "conversations.history", map[string]string{
					"channel": id,
					"limit":   "2",
				})
				if err != nil {
					out += fmt.Sprintf("  conversations.history error: %v\n", err)
				} else {
					msgs, _ := histResp["messages"].([]interface{})
					out += fmt.Sprintf("  conversations.history: %d messages\n", len(msgs))
					for _, m := range msgs {
						msg, _ := m.(map[string]interface{})
						text, _ := msg["text"].(string)
						if len(text) > 60 {
							text = text[:60] + "..."
						}
						out += fmt.Sprintf("    text=%q\n", text)
					}
				}
			}
		}
	}

	return out, nil
}

// GetMessages returns recent DMs and @mentions.
func (ss *SlackService) GetMessages() ([]SlackMessage, error) {
	cfg := ss.config.GetConfig()
	if cfg.SlackToken == "" {
		return nil, fmt.Errorf("no Slack token configured")
	}

	token := cfg.SlackToken

	// Get authenticated user ID
	authResp, err := ss.apiGet(token, "auth.test", nil)
	if err != nil {
		return nil, err
	}
	selfID, _ := authResp["user_id"].(string)
	teamID, _ := authResp["team_id"].(string)

	// Fetch DMs and mentions in parallel
	type result struct {
		msgs []SlackMessage
		err  error
	}
	dmCh := make(chan result, 1)
	mentionCh := make(chan result, 1)

	go func() {
		msgs, err := ss.fetchDMs(token, selfID, teamID)
		dmCh <- result{msgs, err}
	}()
	go func() {
		msgs, err := ss.fetchMentionsAndThreads(token, selfID, teamID)
		mentionCh <- result{msgs, err}
	}()

	dmResult := <-dmCh
	mentionResult := <-mentionCh

	var messages []SlackMessage
	if dmResult.err == nil {
		messages = append(messages, dmResult.msgs...)
	}
	if mentionResult.err == nil {
		messages = append(messages, mentionResult.msgs...)
	}

	fmt.Printf("[Slack] DMs: %d, Mentions/Threads: %d\n", len(dmResult.msgs), len(mentionResult.msgs))

	// Sort by time, newest first
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Time > messages[j].Time
	})

	// Limit to 20 most recent
	if len(messages) > 20 {
		messages = messages[:20]
	}

	return messages, nil
}

// GetTeamID returns the Slack team ID for deep linking.
func (ss *SlackService) GetTeamID() (string, error) {
	cfg := ss.config.GetConfig()
	if cfg.SlackToken == "" {
		return "", fmt.Errorf("no Slack token configured")
	}

	resp, err := ss.apiGet(cfg.SlackToken, "auth.test", nil)
	if err != nil {
		return "", err
	}

	teamID, _ := resp["team_id"].(string)
	return teamID, nil
}

func (ss *SlackService) fetchDMs(token, selfID, teamID string) ([]SlackMessage, error) {
	// List IM conversations
	convResp, err := ss.apiGet(token, "conversations.list", map[string]string{
		"types": "im",
		"limit": "20",
	})
	if err != nil {
		return nil, err
	}

	channels, ok := convResp["channels"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected conversations response")
	}

	// Cache user names
	userCache := make(map[string]string)

	var messages []SlackMessage
	for _, ch := range channels {
		conv, ok := ch.(map[string]interface{})
		if !ok {
			continue
		}

		channelID, _ := conv["id"].(string)
		userID, _ := conv["user"].(string)

		// Fetch only the most recent message, within the last 6 hours
		oneHourAgo := fmt.Sprintf("%d", time.Now().Unix()-6*3600)
		histResp, err := ss.apiGet(token, "conversations.history", map[string]string{
			"channel": channelID,
			"limit":   "1",
			"oldest":  oneHourAgo,
		})
		if err != nil {
			continue
		}

		msgs, ok := histResp["messages"].([]interface{})
		if !ok || len(msgs) == 0 {
			continue
		}

		// Only show conversations where the most recent message is from
		// someone else — i.e. DMs waiting for your reply.
		latest, ok := msgs[0].(map[string]interface{})
		if !ok {
			continue
		}
		latestSender, _ := latest["user"].(string)
		if latestSender == selfID {
			continue // you replied last, skip
		}

		userName := ss.resolveUser(token, userID, userCache)
		text, _ := latest["text"].(string)
		ts, _ := latest["ts"].(string)
		senderName := ss.resolveUser(token, latestSender, userCache)

		if len(text) > 120 {
			text = text[:120] + "..."
		}

		messages = append(messages, SlackMessage{
			Type:      "dm",
			Channel:   userName,
			ChannelID: channelID,
			User:      senderName,
			Text:      text,
			Timestamp: ts,
			TeamID:    teamID,
			Unread:    true,
			Time:      tsToUnix(ts),
		})
	}

	return messages, nil
}

// fetchMentionsAndThreads uses search.messages to find @mentions and thread replies
// directed at the user within the last 6 hours. Requires search:read scope.
func (ss *SlackService) fetchMentionsAndThreads(token, selfID, teamID string) ([]SlackMessage, error) {
	// Slack search "after:" expects a date string (YYYY-MM-DD), not a unix timestamp.
	// Use today's date to get recent mentions. We filter by timestamp after fetching.
	cutoff := time.Now().Add(-6 * time.Hour)
	afterDate := cutoff.Format("2006-01-02")

	// Search for messages that mention the user, sorted newest first
	searchResp, err := ss.apiGet(token, "search.messages", map[string]string{
		"query": fmt.Sprintf("<@%s> after:%s", selfID, afterDate),
		"sort":  "timestamp",
		"count": "20",
	})
	if err != nil {
		// search:read scope might not be available — fail silently
		fmt.Printf("[Slack] search.messages failed (need search:read scope?): %v\n", err)
		return nil, nil
	}

	messagesObj, _ := searchResp["messages"].(map[string]interface{})
	matches, _ := messagesObj["matches"].([]interface{})

	userCache := make(map[string]string)
	var messages []SlackMessage

	for _, m := range matches {
		match, ok := m.(map[string]interface{})
		if !ok {
			continue
		}

		senderID, _ := match["user"].(string)
		// Skip own messages
		if senderID == selfID {
			continue
		}

		text, _ := match["text"].(string)
		ts, _ := match["ts"].(string)

		// Enforce 6-hour cutoff (search "after:" is date-granularity only)
		if tsToUnix(ts) < float64(cutoff.Unix()) {
			continue
		}

		channelObj, _ := match["channel"].(map[string]interface{})
		channelID, _ := channelObj["id"].(string)
		channelName, _ := channelObj["name"].(string)

		// Determine if this is a thread reply
		threadTS, _ := match["thread_ts"].(string)
		msgType := "mention"
		if threadTS != "" && threadTS != ts {
			msgType = "thread"
		}

		senderName := ss.resolveUser(token, senderID, userCache)

		if len(text) > 120 {
			text = text[:120] + "..."
		}

		messages = append(messages, SlackMessage{
			Type:      msgType,
			Channel:   channelName,
			ChannelID: channelID,
			User:      senderName,
			Text:      text,
			Timestamp: ts,
			TeamID:    teamID,
			Unread:    true,
			Time:      tsToUnix(ts),
		})
	}

	return messages, nil
}

func (ss *SlackService) resolveUser(token, userID string, cache map[string]string) string {
	if name, ok := cache[userID]; ok {
		return name
	}
	if userID == "" {
		return "Unknown"
	}

	resp, err := ss.apiGet(token, "users.info", map[string]string{
		"user": userID,
	})
	if err != nil {
		cache[userID] = userID
		return userID
	}

	userObj, ok := resp["user"].(map[string]interface{})
	if !ok {
		cache[userID] = userID
		return userID
	}

	profile, _ := userObj["profile"].(map[string]interface{})
	displayName, _ := profile["display_name"].(string)
	if displayName == "" {
		displayName, _ = profile["real_name"].(string)
	}
	if displayName == "" {
		displayName, _ = userObj["name"].(string)
	}

	cache[userID] = displayName
	return displayName
}

func (ss *SlackService) apiGet(token, method string, params map[string]string) (map[string]interface{}, error) {
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

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
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
		needed, _ := result["needed"].(string)
		provided, _ := result["provided"].(string)
		if needed != "" {
			return result, fmt.Errorf("slack API error: %s (needed: %s, have: %s)", errMsg, needed, provided)
		}
		return result, fmt.Errorf("slack API error: %s", errMsg)
	}

	return result, nil
}

func tsToUnix(ts string) float64 {
	var f float64
	_, _ = fmt.Sscanf(ts, "%f", &f)
	return f
}
