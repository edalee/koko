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

	// Test search.messages
	searchResp, err := ss.apiGet(token, "search.messages", map[string]string{
		"query": "to:me",
		"count": "3",
		"sort":  "timestamp",
	})
	if err != nil {
		out += fmt.Sprintf("search.messages error: %v\n", err)
	} else {
		msgsObj, _ := searchResp["messages"].(map[string]interface{})
		matches, _ := msgsObj["matches"].([]interface{})
		total, _ := msgsObj["total"].(float64)
		out += fmt.Sprintf("search.messages: %d matches (total: %.0f)\n", len(matches), total)
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
	var messages []SlackMessage
	var errs []string

	// Fetch DMs
	dms, err := ss.fetchDMs(token)
	if err != nil {
		errs = append(errs, "DMs: "+err.Error())
	} else {
		messages = append(messages, dms...)
	}

	// Fetch @mentions via search
	mentions, err := ss.fetchMentions(token)
	if err != nil {
		errs = append(errs, "mentions: "+err.Error())
	} else {
		messages = append(messages, mentions...)
	}

	fmt.Printf("[Slack] DMs: %d, Mentions: %d, Errors: %v\n", len(dms), len(mentions), errs)

	// Sort by time, newest first
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Time > messages[j].Time
	})

	// Limit to 20 most recent
	if len(messages) > 20 {
		messages = messages[:20]
	}

	// If we got no messages and had errors, surface the errors
	if len(messages) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("failed to fetch: %s", fmt.Sprintf("%v", errs))
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

func (ss *SlackService) fetchDMs(token string) ([]SlackMessage, error) {
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

	// Get team ID for deep links
	teamID, _ := ss.getTeamID(token)

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

		// Get recent messages in this DM
		histResp, err := ss.apiGet(token, "conversations.history", map[string]string{
			"channel": channelID,
			"limit":   "3",
		})
		if err != nil {
			continue
		}

		msgs, ok := histResp["messages"].([]interface{})
		if !ok || len(msgs) == 0 {
			continue
		}

		// Get display name for this user
		userName := ss.resolveUser(token, userID, userCache)

		for _, m := range msgs {
			msg, ok := m.(map[string]interface{})
			if !ok {
				continue
			}

			text, _ := msg["text"].(string)
			ts, _ := msg["ts"].(string)
			senderID, _ := msg["user"].(string)
			senderName := ss.resolveUser(token, senderID, userCache)

			// Truncate text for preview
			if len(text) > 120 {
				text = text[:120] + "..."
			}

			unixTime := tsToUnix(ts)

			messages = append(messages, SlackMessage{
				Type:      "dm",
				Channel:   userName,
				ChannelID: channelID,
				User:      senderName,
				Text:      text,
				Timestamp: ts,
				TeamID:    teamID,
				Unread:    true,
				Time:      unixTime,
			})
		}
	}

	return messages, nil
}

func (ss *SlackService) fetchMentions(token string) ([]SlackMessage, error) {
	resp, err := ss.apiGet(token, "search.messages", map[string]string{
		"query": "to:me",
		"count": "10",
		"sort":  "timestamp",
	})
	if err != nil {
		return nil, err
	}

	msgsObj, ok := resp["messages"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected search response")
	}

	matches, ok := msgsObj["matches"].([]interface{})
	if !ok {
		return nil, nil
	}

	teamID, _ := ss.getTeamID(token)
	userCache := make(map[string]string)

	var messages []SlackMessage
	for _, m := range matches {
		match, ok := m.(map[string]interface{})
		if !ok {
			continue
		}

		text, _ := match["text"].(string)
		ts, _ := match["ts"].(string)
		userID, _ := match["user"].(string)

		channelObj, _ := match["channel"].(map[string]interface{})
		channelID, _ := channelObj["id"].(string)
		channelName, _ := channelObj["name"].(string)

		userName := ss.resolveUser(token, userID, userCache)

		if len(text) > 120 {
			text = text[:120] + "..."
		}

		unixTime := tsToUnix(ts)

		messages = append(messages, SlackMessage{
			Type:      "mention",
			Channel:   "#" + channelName,
			ChannelID: channelID,
			User:      userName,
			Text:      text,
			Timestamp: ts,
			TeamID:    teamID,
			Unread:    true,
			Time:      unixTime,
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

func (ss *SlackService) getTeamID(token string) (string, error) {
	resp, err := ss.apiGet(token, "auth.test", nil)
	if err != nil {
		return "", err
	}
	teamID, _ := resp["team_id"].(string)
	return teamID, nil
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
