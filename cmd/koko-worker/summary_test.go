package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSummaryPeriod(t *testing.T) {
	weekdays := JobConfig{Enabled: true, Times: []string{"08:30"}}
	utc := time.UTC
	cases := []struct {
		name       string
		job        JobConfig
		now        time.Time
		start, end string
		label      string
	}{
		{"a normal weekday", weekdays, time.Date(2026, 10, 20, 8, 30, 0, 0, cet),
			"2026-10-19 00:00 CEST", "2026-10-20 00:00 CEST", "Monday 19 October"},
		{"Monday after a weekend", weekdays, time.Date(2026, 10, 19, 8, 30, 0, 0, cet),
			"2026-10-16 00:00 CEST", "2026-10-19 00:00 CEST", "Friday 16 to Sunday 18 October"},
		{"a weekly job", JobConfig{Days: []string{"mon"}}, time.Date(2026, 10, 19, 8, 30, 0, 0, cet),
			"2026-10-12 00:00 CEST", "2026-10-19 00:00 CEST", "Monday 12 to Sunday 18 October"},
		{"a run on a day off", weekdays, time.Date(2026, 10, 24, 10, 0, 0, 0, cet),
			"2026-10-23 00:00 CEST", "2026-10-24 00:00 CEST", "Friday 23 October"},
		{"late in UTC is the next day in Paris", weekdays, time.Date(2026, 10, 19, 23, 30, 0, 0, utc),
			"2026-10-19 00:00 CEST", "2026-10-20 00:00 CEST", "Monday 19 October"},
		{"across a month", weekdays, time.Date(2026, 11, 2, 8, 30, 0, 0, cet),
			"2026-10-30 00:00 CET", "2026-11-02 00:00 CET", "Friday 30 October to Sunday 1 November"},
	}
	for _, c := range cases {
		start, end := summaryPeriod(c.job, c.now, cet)
		const f = "2006-01-02 15:04 MST"
		if start.Format(f) != c.start || end.Format(f) != c.end {
			t.Errorf("%s: got %s to %s, want %s to %s", c.name, start.Format(f), end.Format(f), c.start, c.end)
		}
		if got := periodLabel(start, end); got != c.label {
			t.Errorf("%s: label %q, want %q", c.name, got, c.label)
		}
	}

	// Summer time ends on Sunday 25 October, so that weekend is an hour longer.
	start, end := summaryPeriod(weekdays, time.Date(2026, 10, 26, 8, 30, 0, 0, cet), cet)
	if start.Format("Mon 15:04 MST") != "Fri 00:00 CEST" || end.Format("Mon 15:04 MST") != "Mon 00:00 CET" || end.Sub(start) != 73*time.Hour {
		t.Errorf("over the clock change: %s to %s (%s)", start, end, end.Sub(start))
	}
}

// fakeSummaryGH stands in for gh: it answers each GraphQL call with a reply
// built from the call's searches, and records the calls.
type fakeSummaryGH struct {
	calls [][]string
	reply func(queries map[string]string) (string, error)
}

func (f *fakeSummaryGH) install(t *testing.T) {
	saved := summaryGH
	summaryGH = func(_ context.Context, args ...string) ([]byte, error) {
		f.calls = append(f.calls, args)
		queries := map[string]string{}
		for i, a := range args {
			if i > 0 && args[i-1] == "-f" && strings.HasPrefix(a, "q") {
				k, v, _ := strings.Cut(a, "=")
				queries[k] = v
			}
		}
		out, err := f.reply(queries)
		return []byte(out), err
	}
	t.Cleanup(func() { summaryGH = saved })
}

// summaryNode is PR n in acme/api by author, as GraphQL JSON. Each review is
// "login@time" or "login@time@STATE", and an empty time is a pending review.
func summaryNode(n int, author, created, merged string, reviews ...string) map[string]any {
	var rs []map[string]any
	for _, r := range reviews {
		parts := strings.Split(r, "@")
		state := "APPROVED"
		if len(parts) > 2 {
			state = parts[2]
		}
		typ := "User"
		if strings.HasSuffix(parts[0], "[bot]") {
			typ = "Bot"
		}
		var at any = parts[1]
		if parts[1] == "" {
			at = nil
		}
		rs = append(rs, map[string]any{"author": map[string]any{"login": parts[0], "__typename": typ}, "submittedAt": at, "state": state})
	}
	var m any = merged
	if merged == "" {
		m = nil
	}
	return map[string]any{
		"url": "https://github.com/acme/api/pull/" + strconv.Itoa(n), "number": n, "title": "Change " + strconv.Itoa(n),
		"createdAt": created, "mergedAt": m, "headRefName": "feature/PROJ-" + strconv.Itoa(n),
		"repository": map[string]any{"nameWithOwner": "acme/api"},
		"author":     map[string]any{"login": author, "__typename": "User"},
		"reviews":    map[string]any{"nodes": rs},
	}
}

func jsonString(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestSearchActivity(t *testing.T) {
	start := time.Date(2026, 10, 16, 0, 0, 0, 0, cet)
	s := SummaryConfig{Team: "acme/platform", Repos: []string{"acme/api", "acme/platform"}}
	logins := []string{"a1", "a2", "a3", "a4", "a5", "bob"}
	f := &fakeSummaryGH{reply: func(q map[string]string) (string, error) {
		if strings.Contains(q["q0"], "author:bob ") {
			return "", errors.New("gh api: HTTP 502")
		}
		data := map[string]any{}
		for k := range q {
			alias := "s" + strings.TrimPrefix(k, "q")
			data[alias] = map[string]any{"issueCount": 0, "nodes": []any{}}
		}
		data["s0"] = map[string]any{"issueCount": 60, "nodes": []any{summaryNode(1, "a1", "2026-10-16T09:00:00Z", "")}}
		return jsonString(map[string]any{"data": data}), nil
	}}
	f.install(t)

	prs, more, notes, err := searchActivity(context.Background(), s, logins, start)
	if len(f.calls) != 2 {
		t.Fatalf("gh calls = %d, want 2 (5 members, then 1)", len(f.calls))
	}
	joined := strings.Join(f.calls[0], " ")
	for _, want := range []string{
		"q0=is:pr author:a1 updated:>=2026-10-15T22:00:00Z repo:acme/api repo:acme/platform",
		"q1=is:pr reviewed-by:a1 updated:>=2026-10-15T22:00:00Z",
		"q9=is:pr reviewed-by:a5 ",
		"$q9: String!",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("first call lacks %q:\n%s", want, joined)
		}
	}
	if err == nil || len(notes) != 1 || !strings.Contains(notes[0], "Could not read GitHub for bob") {
		t.Errorf("failed batch: err %v, notes %q", err, notes)
	}
	if len(prs) != 1 || !more["a1"] || more["a2"] {
		t.Errorf("prs %d, more %v", len(prs), more)
	}

	// Without repos, the search covers the team's org.
	if got := summaryScope(SummaryConfig{Team: "acme/platform"}); got != "org:acme" {
		t.Errorf("scope = %q", got)
	}
}

func decodePRs(t *testing.T, nodes ...map[string]any) map[string]summaryPR {
	t.Helper()
	out := map[string]summaryPR{}
	for _, n := range nodes {
		var p summaryPR
		if err := json.Unmarshal([]byte(jsonString(n)), &p); err != nil {
			t.Fatal(err)
		}
		out[p.URL] = p
	}
	return out
}

func TestBuildFacts(t *testing.T) {
	start := time.Date(2026, 10, 16, 0, 0, 0, 0, cet)
	end := time.Date(2026, 10, 19, 0, 0, 0, 0, cet)
	other := summaryNode(9, "alice", "2026-10-16T09:00:00Z", "2026-10-16T10:00:00Z")
	other["repository"] = map[string]any{"nameWithOwner": "acme/website"}
	prs := decodePRs(t,
		// alice merged #1, opened the week before.
		summaryNode(1, "alice", "2026-10-12T09:00:00Z", "2026-10-16T15:00:00Z", "bob@2026-10-13T09:00:00Z"),
		// bob opened #2. alice reviewed it twice, bob replied to himself, a bot
		// reviewed, and carol's review is pending.
		summaryNode(2, "bob", "2026-10-17T09:00:00Z", "",
			"alice@2026-10-17T10:00:00Z", "alice@2026-10-17T11:00:00Z@COMMENTED", "bob@2026-10-17T12:00:00Z@COMMENTED",
			"review-app[bot]@2026-10-17T10:00:00Z@COMMENTED", "carol@@PENDING"),
		// Merged after the period ended, in Paris time.
		summaryNode(3, "bob", "2026-10-12T09:00:00Z", "2026-10-18T22:30:00Z"),
		// In a repo outside the summary's repos.
		other,
	)
	s := SummaryConfig{Team: "acme/platform", Repos: []string{"acme/api"}}
	people := buildFacts(s, []string{"carol", "bob", "alice"}, prs, map[string]bool{"bob": true}, start, end)
	if len(people) != 3 || people[0].Login != "alice" || people[2].Login != "carol" {
		t.Fatalf("people = %+v", people)
	}
	alice, bob, carol := people[0], people[1], people[2]
	if len(alice.Merged) != 1 || alice.Merged[0].Ref != "api#1" || len(alice.Opened) != 0 {
		t.Errorf("alice merged/opened = %+v / %+v", alice.Merged, alice.Opened)
	}
	if len(alice.Reviewed) != 1 || alice.Reviewed[0].Ref != "api#2" || alice.Reviewed[0].Author != "bob" {
		t.Errorf("alice reviewed = %+v", alice.Reviewed)
	}
	if len(bob.Opened) != 1 || len(bob.Merged) != 0 || len(bob.Reviewed) != 0 || !bob.More {
		t.Errorf("bob = %+v", bob)
	}
	if carol.active() {
		t.Errorf("carol = %+v, want no activity", carol)
	}
	if keys := strings.Join(ticketKeys(people), ","); keys != "PROJ-1,PROJ-2" {
		t.Errorf("ticket keys = %q", keys)
	}
	addTickets(people, map[string]jiraTicket{"PROJ-2": {Key: "PROJ-2", Summary: "Add caching", Status: "In Review"}})
	if len(people[1].Tickets) != 1 || len(people[0].Tickets) != 0 {
		t.Errorf("tickets: alice %+v, bob %+v", people[0].Tickets, people[1].Tickets)
	}
}

func TestSummaryLogins(t *testing.T) {
	got := summaryLogins(map[string]bool{"bob": true, "alice": true, "ci[bot]": true, "x y": true})
	if strings.Join(got, ",") != "alice,bob" {
		t.Errorf("logins = %v", got)
	}
}

func summaryPeople() []personFacts {
	return []personFacts{
		{Login: "alice",
			Merged:   []prFact{{Ref: "api#1", Title: "Fix <login>", URL: "https://github.com/acme/api/pull/1"}},
			Reviewed: []prFact{{Ref: "api#2", Title: "Add caching", Author: "bob", URL: "https://github.com/acme/api/pull/2"}},
			Tickets:  []ticketFact{{Key: "PROJ-1", Summary: "Login fails", Status: "Done", URL: "https://acme.atlassian.net/browse/PROJ-1"}}},
		{Login: "bob", Opened: []prFact{{Ref: "api#2", Title: "Add caching", URL: "https://github.com/acme/api/pull/2"}}},
		{Login: "carol"},
		{Login: "dave"},
	}
}

func TestSummaryMessage(t *testing.T) {
	people := summaryPeople()
	msg := summaryMessage("Friday 16 to Sunday 18 October", people,
		map[string]string{"alice": "Alice merged api#1 & reviewed api#2 <b>."}, []string{"Could not check Jira: no site"})
	got := printMessage(msg)
	for _, want := range []string{
		"*Team summary, Friday 16 to Sunday 18 October*\n_Could not check Jira: no site_",
		"*alice*\nAlice merged api#1 &amp; reviewed api#2 &lt;b&gt;.\nPRs: <https://github.com/acme/api/pull/1|api#1>, <https://github.com/acme/api/pull/2|api#2>\nTickets: <https://acme.atlassian.net/browse/PROJ-1|PROJ-1>",
		"*bob*\n• Opened <https://github.com/acme/api/pull/2|api#2> Add caching",
		"No activity: carol, dave.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("message lacks %q:\n%s", want, got)
		}
	}
	if len(msg.Sections) != 4 {
		t.Errorf("sections = %d, want header, alice, bob and the no-activity line", len(msg.Sections))
	}
	// Without a paragraph, alice gets lists, with titles escaped.
	got = personSection(people[0], "")
	for _, want := range []string{
		"• Merged <https://github.com/acme/api/pull/1|api#1> Fix &lt;login&gt;",
		"• Reviewed <https://github.com/acme/api/pull/2|api#2> Add caching (bob)",
		"• Moved <https://acme.atlassian.net/browse/PROJ-1|PROJ-1> Login fails (Done)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lists lack %q:\n%s", want, got)
		}
	}
}

func TestParseProse(t *testing.T) {
	people := summaryPeople()[:2]
	text := "```json\n" + `{"alice": "Alice merged api#1 and closed PROJ-1.", "bob": "Bob opened api#2 and api#7.", "mallory": "Hi."}` + "\n```"
	got, err := parseProse(text, people)
	if err != nil {
		t.Fatal(err)
	}
	if got["alice"] == "" || got["bob"] != "" || got["mallory"] != "" {
		t.Errorf("paragraphs = %v, want alice only (bob names a PR not in his facts)", got)
	}
	if _, err := parseProse("I cannot help with that.", people); err == nil {
		t.Error("want an error for a reply without JSON")
	}
	if p := prosePrompt("Monday 19 October", people); !strings.Contains(p, `"ref": "api#1"`) || strings.Contains(p, "https://") {
		t.Errorf("prompt should carry refs and no URLs:\n%s", p)
	}
}

func TestSummaryProseFallsBackToLists(t *testing.T) {
	cfg := defaultConfig()
	// A zero claudeRunner cannot write its settings file, so the call fails.
	env := Env{cfg: cfg, claude: claudeRunner{}}
	got := printMessage(summaryProseMessage(context.Background(), env, "Monday 19 October", summaryPeople(), nil))
	if !strings.Contains(got, "_Claude could not write the paragraphs, so this lists the facts:") ||
		!strings.Contains(got, "• Merged <https://github.com/acme/api/pull/1|api#1>") {
		t.Errorf("fallback:\n%s", got)
	}
	// Prose off: lists, and no note.
	env.cfg.Summary.Prose = false
	got = printMessage(summaryProseMessage(context.Background(), env, "Monday 19 October", summaryPeople(), nil))
	if strings.Contains(got, "Claude") || !strings.Contains(got, "• Opened") {
		t.Errorf("prose off:\n%s", got)
	}
}

func TestParseJiraSearch(t *testing.T) {
	issue := `{"key": "PROJ-1", "webUrl": "https://acme.atlassian.net/browse/PROJ-1", "fields": {"summary": "Login fails", "status": {"name": "Done", "statusCategory": {"key": "done"}}}}`
	for _, raw := range []string{`{"issues": [` + issue + `]}`, `{"issues": {"nodes": [` + issue + `]}}`} {
		got := parseJiraSearch(raw)
		if len(got) != 1 || got[0].Key != "PROJ-1" || got[0].Status != "Done" {
			t.Errorf("%s: got %+v", raw, got)
		}
	}
	if got := parseJiraSearch("not json"); got != nil {
		t.Errorf("got %+v", got)
	}
}

func TestSummaryConfig(t *testing.T) {
	def := defaultConfig()
	job := def.Jobs[JobSummary]
	if job.Enabled || strings.Join(job.Times, ",") != "08:30" || len(job.Days) != 0 || !def.Summary.Jira || !def.Summary.Prose {
		t.Errorf("defaults: job %+v, summary %+v", job, def.Summary)
	}
	path := t.TempDir() + "/worker.json"
	body := `{"summary": {"team": " acme/platform ", "repos": [" acme/api ", ""]}, "jobs": {"summary": {"enabled": true}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Summary
	if s.Team != "acme/platform" || strings.Join(s.Repos, ",") != "acme/api" || !s.Jira || !s.Prose {
		t.Errorf("loaded = %+v", s)
	}
	if strings.Join(cfg.Jobs[JobSummary].Times, ",") != "08:30" {
		t.Errorf("times = %v", cfg.Jobs[JobSummary].Times)
	}
	if err := s.problem(); err != nil {
		t.Errorf("problem = %v", err)
	}
	for _, bad := range []SummaryConfig{{}, {Team: "platform"}, {Team: "acme/"}, {Team: "acme/platform", Repos: []string{"api"}}} {
		if bad.problem() == nil {
			t.Errorf("%+v: want a problem", bad)
		}
	}
	// A bad summary team does not stop the other jobs.
	cfg.Summary.Team = "platform"
	if err := cfg.validate(); err != nil {
		t.Errorf("validate = %v", err)
	}
	if got := manualSlots(cfg, JobSummary, time.Date(2026, 10, 19, 7, 0, 0, 0, cet)); len(got) != 1 {
		t.Errorf("a summary run by hand before 08:30 covers %v", got)
	}
}
