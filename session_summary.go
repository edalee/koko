package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// headlessEntrypoint marks a transcript written by `claude -p`, such as a
// koko-worker run. Koko never shows these as conversations (plan 036).
const headlessEntrypoint = "sdk-cli"

// maxRecentPrompts is how many of the user's prompts a summary keeps.
const maxRecentPrompts = 10

// promptRunes and recapRunes cap the text a summary carries.
const (
	promptRunes = 160
	recapRunes  = 400
)

// summaryCacheSize bounds the summary cache. It covers the open tabs and a
// picker's worth of conversations.
const summaryCacheSize = 64

// GetSessionSummary returns what one conversation is about: the tab's own,
// named by its UUID. With no UUID it returns an empty summary. It never falls
// back to the newest file in the folder, which may belong to another tab.
func (cs *ClaudeService) GetSessionSummary(dir, uuid string) (SessionSummary, error) {
	path := sessionFilePath(dir, uuid)
	if path == "" {
		return emptySummary(), nil
	}
	return cs.summaryOf(path, nil), nil
}

// sessionFilePath returns the session file for uuid in dir's project folder.
// The id comes from the webview, so anything but a bare file name gives "".
func sessionFilePath(dir, uuid string) string {
	if uuid == "" || uuid == "." || uuid == ".." || filepath.Base(uuid) != uuid {
		return ""
	}
	sessionDir := claudeProjectDir(dir)
	if sessionDir == "" {
		return ""
	}
	return filepath.Join(sessionDir, uuid+".jsonl")
}

// emptySummary has an empty prompt list rather than a nil one, so the
// frontend gets [] and not null.
func emptySummary() SessionSummary {
	return SessionSummary{RecentPrompts: []SessionPrompt{}}
}

// summaryOf returns the summary of a session file, from the cache while the
// file is unchanged. head is the file's head read, if the caller has one.
func (cs *ClaudeService) summaryOf(path string, head *conversationHead) SessionSummary {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return emptySummary()
	}
	if cached, ok := cs.summaries.get(path, info); ok {
		return cached
	}

	entrypoint, _ := transcriptEntrypoint(path, head)
	s := emptySummary()
	if entrypoint != headlessEntrypoint {
		s = readSummary(path)
		if s.Title == "" {
			// Only when the tail had no title: an older file, or one whose
			// records near the end are too large to reach.
			if head == nil {
				h := readConversationHead(path)
				head = &h
			}
			s.Title = head.title
		}
		s.Ticket = sessionTicket(s)
		s.UpdatedAt = info.ModTime().UnixMilli()
	}
	cs.summaries.put(path, info, s)
	return s
}

// entrypointCacheSize bounds the entrypoint cache. Past it the cache starts
// again, which costs only a head read per file.
const entrypointCacheSize = 1024

// entrypoints holds the entrypoint of each session file read so far. The first
// records never change once written, so an entry never goes stale. This keeps
// the head read off the polling path.
var entrypoints = struct {
	mu sync.Mutex
	m  map[string]string
}{m: make(map[string]string)}

// transcriptEntrypoint returns the entrypoint a session file's first records
// name: "sdk-cli" for a headless run, "" in older files. written is false
// while the file has no real record yet, so its kind is not yet known.
// head is the file's head read, if the caller has one.
func transcriptEntrypoint(path string, head *conversationHead) (entrypoint string, written bool) {
	entrypoints.mu.Lock()
	ep, ok := entrypoints.m[path]
	entrypoints.mu.Unlock()
	if ok {
		return ep, true
	}

	if head == nil {
		h := readConversationHead(path)
		head = &h
	}
	if head.entrypoint != "" {
		entrypoints.mu.Lock()
		if len(entrypoints.m) >= entrypointCacheSize {
			entrypoints.m = make(map[string]string)
		}
		entrypoints.m[path] = head.entrypoint
		entrypoints.mu.Unlock()
	}
	return head.entrypoint, head.entrypoint != "" || head.cwd != ""
}

// sessionTicket picks the Jira key a session is about. The branch is the
// strongest sign, then the PR, then the title, then the prompts, newest first.
// A pr-link record has no PR title, so only its repository and URL are searched.
func sessionTicket(s SessionSummary) string {
	texts := []string{s.Branch}
	if s.PR != nil {
		texts = append(texts, s.PR.Repo, s.PR.URL)
	}
	texts = append(texts, s.Title)
	for _, p := range s.RecentPrompts {
		texts = append(texts, p.Text)
	}
	return firstJiraKey(texts...)
}

// summaryRecord holds the fields of a transcript record that a summary uses.
type summaryRecord struct {
	Type         string          `json:"type"`
	Subtype      string          `json:"subtype"`
	AiTitle      string          `json:"aiTitle"`
	LastPrompt   string          `json:"lastPrompt"`
	Content      json.RawMessage `json:"content"`
	PRNumber     int             `json:"prNumber"`
	PRRepository string          `json:"prRepository"`
	PRURL        string          `json:"prUrl"`
	GitBranch    string          `json:"gitBranch"`
	IsMeta       bool            `json:"isMeta"`
	IsSidechain  bool            `json:"isSidechain"`
	Timestamp    string          `json:"timestamp"`
	Origin       *struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// readSummary walks a session file backwards, newest record first, until it
// has every field or reaches tailBudget. It leaves the title empty if the walk
// found none, and leaves Ticket and UpdatedAt to the caller.
//
// Only prompts the user typed count. Tool results, `!` and `/` commands, task
// notifications and messages from other sessions have no human origin.
func readSummary(path string) SessionSummary {
	s := emptySummary()
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return s
	}

	eachLineBackwards(f, info.Size(), tailBudget, func(line []byte) bool {
		var r summaryRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return true
		}
		if s.Branch == "" {
			s.Branch = r.GitBranch
		}
		switch r.Type {
		case "ai-title":
			if s.Title == "" {
				s.Title = shorten(r.AiTitle, 120)
			}
		case "last-prompt":
			if s.LastPrompt == "" {
				s.LastPrompt = shorten(oneLine(r.LastPrompt), promptRunes)
			}
		case "pr-link":
			if s.PR == nil && r.PRURL != "" {
				s.PR = &SessionPR{Number: r.PRNumber, Repo: r.PRRepository, URL: r.PRURL}
			}
		case "system":
			if s.Recap == "" && r.Subtype == "away_summary" {
				var text string
				if json.Unmarshal(r.Content, &text) == nil {
					s.Recap = shorten(text, recapRunes)
				}
			}
		case "user":
			if len(s.RecentPrompts) < maxRecentPrompts && isHumanPrompt(r) {
				if text := oneLine(userContentText(r.Message.Content)); text != "" {
					s.RecentPrompts = append(s.RecentPrompts, SessionPrompt{
						Text:      shorten(text, promptRunes),
						Timestamp: unixMillis(r.Timestamp),
					})
				}
			}
		}
		done := s.Title != "" && s.Recap != "" && s.LastPrompt != "" && s.Branch != "" &&
			s.PR != nil && len(s.RecentPrompts) == maxRecentPrompts
		return !done
	})

	if s.LastPrompt == "" && len(s.RecentPrompts) > 0 {
		s.LastPrompt = s.RecentPrompts[0].Text
	}
	return s
}

// isHumanPrompt reports whether a user record is a prompt the user typed.
func isHumanPrompt(r summaryRecord) bool {
	return r.Origin != nil && r.Origin.Kind == "human" && !r.IsMeta && !r.IsSidechain
}

// oneLine folds runs of whitespace, newlines included, into single spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// unixMillis parses a record's RFC 3339 timestamp, or gives 0.
func unixMillis(ts string) int64 {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// eachLineBackwards calls fn on each complete line in the last budget bytes of
// a file, newest first, until fn returns false.
//
// It reads tailChunk bytes at a time and carries a part line over to the next
// read. A line that begins before the budget is never passed on, because only
// part of it was read.
func eachLineBackwards(f *os.File, size, budget int64, fn func(line []byte) bool) {
	var carry []byte // text after pos, up to its first newline
	pos := size
	for pos > 0 && size-pos < budget {
		n := min(int64(tailChunk), pos, budget-(size-pos))
		pos -= n
		buf := make([]byte, int(n)+len(carry))
		if _, err := f.ReadAt(buf[:n], pos); err != nil && err != io.EOF {
			return
		}
		copy(buf[n:], carry)
		for {
			i := bytes.LastIndexByte(buf, '\n')
			if i < 0 {
				break
			}
			if line := buf[i+1:]; len(line) > 0 && !fn(line) {
				return
			}
			buf = buf[:i]
		}
		carry = buf
	}
	if pos == 0 && len(carry) > 0 {
		fn(carry)
	}
}

// summaryCache holds the last summary of each session file, keyed on path and
// checked against size and modification time. The zero value is ready to use.
type summaryCache struct {
	mu      sync.Mutex
	entries map[string]*cachedSummary
	clock   uint64 // orders entries by last use, for eviction
}

type cachedSummary struct {
	size    int64
	modTime time.Time
	summary SessionSummary
	used    uint64
}

// get returns the summary for path, if the entry still matches the file.
func (c *summaryCache) get(path string, info os.FileInfo) (SessionSummary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[path]
	if !ok || e.size != info.Size() || !e.modTime.Equal(info.ModTime()) {
		return SessionSummary{}, false
	}
	c.clock++
	e.used = c.clock
	return e.summary, true
}

// put stores a summary, evicting the least recently used entry when full.
func (c *summaryCache) put(path string, info os.FileInfo, s SessionSummary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]*cachedSummary)
	}
	if _, ok := c.entries[path]; !ok && len(c.entries) >= summaryCacheSize {
		var oldest string
		var oldestUse uint64
		for p, e := range c.entries {
			if oldest == "" || e.used < oldestUse {
				oldest, oldestUse = p, e.used
			}
		}
		delete(c.entries, oldest)
	}
	c.clock++
	c.entries[path] = &cachedSummary{
		size:    info.Size(),
		modTime: info.ModTime(),
		summary: s,
		used:    c.clock,
	}
}
