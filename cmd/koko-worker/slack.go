package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

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

// Message is one DM: sections with a divider between each, and replies to
// post in its thread. Each reply is a list of sections too.
type Message struct {
	Sections []string
	Thread   [][]string
}

const (
	slackBlockText = 2900 // Slack's limit for one section block is 3,000 characters
	slackMaxBlocks = 50   // Slack's limit for blocks in one message
)

// post sends msg to the configured user. A bot token and a user ID open a DM
// with the bot, so it notifies like any message. Thread replies go under the
// first message.
func (s slackClient) post(ctx context.Context, msg Message) error {
	if s.userID == "" {
		return fmt.Errorf("slack: no user ID set")
	}
	ts, err := s.postSections(ctx, msg.Sections, "")
	if err != nil {
		return err
	}
	for _, reply := range msg.Thread {
		if _, err := s.postSections(ctx, reply, ts); err != nil {
			return err
		}
	}
	return nil
}

// postSections posts sections as block messages and returns the first
// message's timestamp, which identifies it for thread replies.
func (s slackClient) postSections(ctx context.Context, sections []string, threadTS string) (string, error) {
	var first string
	for _, blocks := range buildBlocks(sections) {
		data, _ := json.Marshal(blocks)
		form := url.Values{
			"channel":      {s.userID},
			"blocks":       {string(data)},
			"text":         {fallbackText(sections)},
			"unfurl_links": {"false"},
		}
		if threadTS != "" {
			form.Set("thread_ts", threadTS)
		}
		body, err := s.call(ctx, "chat.postMessage", form)
		if err != nil {
			return first, err
		}
		if first == "" {
			first, _ = body["ts"].(string)
		}
	}
	return first, nil
}

type block map[string]any

// buildBlocks turns sections into Slack blocks, with a divider between
// sections. A long section is split at line breaks. Blocks that do not fit
// in one message continue in the next, which never starts with a divider.
func buildBlocks(sections []string) [][]block {
	var messages [][]block
	var cur []block
	for i, sec := range sections {
		for j, chunk := range splitText(sec, slackBlockText) {
			var add []block
			if i > 0 && j == 0 && len(cur) > 0 {
				add = append(add, block{"type": "divider"})
			}
			add = append(add, block{"type": "section", "text": block{"type": "mrkdwn", "text": chunk}})
			if len(cur)+len(add) > slackMaxBlocks {
				messages = append(messages, cur)
				cur = nil
				add = add[len(add)-1:] // drop the divider at the top of a new message
			}
			cur = append(cur, add...)
		}
	}
	if len(cur) > 0 {
		messages = append(messages, cur)
	}
	return messages
}

// splitText cuts text into pieces of at most max bytes, at line breaks where it can.
func splitText(text string, max int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []string
	for len(text) > max {
		cut := strings.LastIndexByte(text[:max], '\n')
		if cut < max/2 {
			cut = max
			for cut > 0 && !utf8.RuneStart(text[cut]) {
				cut--
			}
		}
		out = append(out, strings.TrimSpace(text[:cut]))
		text = strings.TrimSpace(text[cut:])
	}
	return append(out, text)
}

// fallbackText is what notifications and screen readers show.
func fallbackText(sections []string) string {
	if len(sections) == 0 {
		return ""
	}
	first := strings.SplitN(strings.TrimSpace(sections[0]), "\n", 2)[0]
	return truncate(first, 200)
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
