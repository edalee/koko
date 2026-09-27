package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// slackMaxText is where long reports are cut, well under Slack's limit of
// 40,000 characters for one message, so a DM stays readable.
const slackMaxText = 11000

type slackClient struct {
	token  string
	userID string
	http   *http.Client
}

func newSlack(cfg SlackConfig) slackClient {
	return slackClient{token: cfg.BotToken, userID: cfg.UserID, http: &http.Client{Timeout: 30 * time.Second}}
}

func (s slackClient) call(ctx context.Context, method string, form url.Values) (map[string]any, error) {
	if s.token == "" {
		return nil, fmt.Errorf("slack: no bot token set")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/"+method, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("slack %s: unreadable reply", method)
	}
	if ok, _ := body["ok"].(bool); !ok {
		return body, fmt.Errorf("slack %s: %v", method, body["error"])
	}
	return body, nil
}

// dm sends text to the configured user. A bot token and a user ID open a DM
// with the bot, so it notifies like any message.
func (s slackClient) dm(ctx context.Context, text string) error {
	if s.userID == "" {
		return fmt.Errorf("slack: no user ID set")
	}
	_, err := s.call(ctx, "chat.postMessage", url.Values{
		"channel":      {s.userID},
		"text":         {text},
		"unfurl_links": {"false"},
	})
	return err
}

// check confirms the token works and is a bot token. It never returns the token.
func (s slackClient) check(ctx context.Context) (string, error) {
	body, err := s.call(ctx, "auth.test", url.Values{})
	if err != nil {
		return "", err
	}
	if _, isBot := body["bot_id"]; !isBot {
		return "", fmt.Errorf("slack: this is a user token, use a bot token (xoxb-)")
	}
	if s.userID == "" {
		return "", fmt.Errorf("slack: token works, but no user ID is set")
	}
	return fmt.Sprintf("connected as %v in %v", body["user"], body["team"]), nil
}

// cutForSlack keeps a report under slackMaxText and points to the full log.
func cutForSlack(text, logPath string) string {
	if len(text) <= slackMaxText {
		return text
	}
	cut := text[:slackMaxText]
	if i := strings.LastIndexByte(cut, '\n'); i > slackMaxText/2 {
		cut = cut[:i]
	}
	return cut + "\n…\n_Full report: `" + logPath + "`_"
}
