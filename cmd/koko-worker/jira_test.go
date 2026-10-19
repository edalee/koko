package main

import (
	"strings"
	"testing"
)

// The shape of a real getJiraIssue result, cut down.
const proj249 = `{"issues":{"nodes":[{"id":"269597","key":"PROJ-249","fields":{"summary":"Uploads can still reach the database unvalidated","description":"### Ask\n\nClose both paths so a stored upload is always validated.","assignee":{"accountId":"557058:me","displayName":"Sam Doe"},"status":{"name":"In Progress","statusCategory":{"key":"indeterminate"}}},"webUrl":"https://acme.atlassian.net/browse/PROJ-249"}]},"context":{"atlassianAccountId":"557058:me","cloudId":"e8bb"}}`

func TestParseJiraIssue(t *testing.T) {
	tk, ok := parseJiraIssue(proj249)
	if !ok || tk.Key != "PROJ-249" || !tk.Mine || tk.Status != "In Progress" || tk.StatusCategory != "indeterminate" ||
		tk.URL != "https://acme.atlassian.net/browse/PROJ-249" || !strings.Contains(tk.Description, "Close both paths") {
		t.Errorf("got %+v", tk)
	}
	// Someone else's ticket, with no description: not mine, empty text.
	other := strings.Replace(strings.Replace(proj249, `"557058:me","displayName"`, `"557058:other","displayName"`, 1), `"description":"### Ask\n\nClose both paths so a stored upload is always validated."`, `"description":null`, 1)
	if tk, ok := parseJiraIssue(other); !ok || tk.Mine || tk.Description != "" {
		t.Errorf("other: %+v", tk)
	}
	if _, ok := parseJiraIssue("Issue does not exist or you do not have permission to see it."); ok {
		t.Error("an error text must not parse")
	}
}

func TestTicketKey(t *testing.T) {
	cases := []struct {
		d    PRDetail
		want string
	}{
		{PRDetail{Title: "fix(PROJ-249): validate uploads", HeadRefName: "proj-202-x", Body: "See OLD-1"}, "PROJ-249"},
		{PRDetail{Title: "chore: tidy", HeadRefName: "feat/proj-119-alerts", Body: "OLD-1"}, "PROJ-119"},
		{PRDetail{Title: "chore: tidy", HeadRefName: "main", Body: "Fixes OLD-1, see PROJ-2"}, "OLD-1"},
		{PRDetail{Title: "chore: tidy"}, ""},
	}
	for _, c := range cases {
		if got := ticketKey(c.d); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.d, got, c.want)
		}
	}
}

func TestCheckVerdicts(t *testing.T) {
	tk, _ := parseJiraIssue(proj249)
	pr := judgedPR{pr: SearchPR{URL: "u1"}, detail: PRDetail{Title: "fix(PROJ-249)", Body: "Deploy, then run the backfill."}, ready: true}
	fresh := func() map[string]ticketVerdict {
		return map[string]ticketVerdict{"u1": {URL: "u1", Ticket: tk, Read: true, Covers: "unknown"}}
	}

	// Real quotes: the verdict stands, and a grounded follow-up stays.
	out := fresh()
	reply := `[{"url":"u1","covers":"yes","summary":"Both paths closed.","evidence":["a stored upload is always validated"],
	  "followUps":[{"step":"Run the backfill after deploying","quote":"run the backfill"},{"step":"Write dashboard alert rules","quote":"dashboard alert rules"}]}]`
	if err := checkVerdicts(reply, out, []judgedPR{pr}); err != nil {
		t.Fatal(err)
	}
	v := out["u1"]
	if v.Covers != "yes" || !v.CloseTicket || len(v.FollowUps) != 1 || v.FollowUps[0] != "Run the backfill after deploying" {
		t.Errorf("grounded: %+v", v)
	}

	// An invented quote: the verdict becomes unknown.
	out = fresh()
	reply = `[{"url":"u1","covers":"partly","summary":"Covers all services.","evidence":["the ticket covers all services"]}]`
	if err := checkVerdicts(reply, out, []judgedPR{pr}); err != nil {
		t.Fatal(err)
	}
	if v := out["u1"]; v.Covers != "unknown" || v.CloseTicket {
		t.Errorf("invented: %+v", v)
	}

	// A claim with no evidence at all is not trusted either.
	out = fresh()
	if err := checkVerdicts(`[{"url":"u1","covers":"yes","summary":"Done."}]`, out, []judgedPR{pr}); err != nil {
		t.Fatal(err)
	}
	if v := out["u1"]; v.Covers != "unknown" {
		t.Errorf("no evidence: %+v", v)
	}
}

func TestClosableSection(t *testing.T) {
	tk, _ := parseJiraIssue(proj249)
	notMine := tk
	notMine.Key, notMine.Mine = "PROJ-119", false
	done := tk
	done.Key, done.StatusCategory = "PROJ-1", "done"
	prs := []judgedPR{
		{pr: SearchPR{URL: "a", Number: 1}, ready: true},
		{pr: SearchPR{URL: "b", Number: 2}, merged: true},
		{pr: SearchPR{URL: "c", Number: 3}, ready: true},
		{pr: SearchPR{URL: "d", Number: 4}, merged: true},
	}
	for i := range prs {
		prs[i].pr.Repository.NameWithOwner = "acme/api"
	}
	merged := tk
	merged.Key = "PROJ-250"
	verdicts := map[string]ticketVerdict{
		"a": {Ticket: tk, Read: true, CloseTicket: true},
		"b": {Ticket: merged, Read: true, CloseTicket: true},
		"c": {Ticket: notMine, Read: true, CloseTicket: true},
		"d": {Ticket: done, Read: true, CloseTicket: true},
	}
	got := closableSection(prs, verdicts)
	now, after, _ := strings.Cut(got, "After the merge:")
	if !strings.Contains(now, "Now:") || !strings.Contains(now, "|PROJ-250>") || !strings.Contains(now, "(merged)") || !strings.Contains(after, "|PROJ-249>") {
		t.Errorf("merged story should be under Now, the ready one after the merge:\n%s", got)
	}
	if !strings.Contains(got, "After the merge:") || !strings.Contains(got, "|PROJ-249>") || strings.Contains(got, "PROJ-119") || strings.Contains(got, "PROJ-1>") {
		t.Errorf("section:\n%s", got)
	}
	if closableSection(prs[2:3], verdicts) != "" {
		t.Error("someone else's ticket must not be listed")
	}
}

func TestTicketLine(t *testing.T) {
	tk, _ := parseJiraIssue(proj249)
	got := ticketLine(ticketVerdict{Ticket: tk, Read: true, Covers: "yes", CloseTicket: true, Summary: "Both paths closed."})
	want := "<https://acme.atlassian.net/browse/PROJ-249|PROJ-249> (In Progress, Sam Doe): covered. Both paths closed. Close the ticket after the merge."
	if got != want {
		t.Errorf("got %q", got)
	}
	if got := ticketLine(ticketVerdict{Ticket: jiraTicket{Key: "PROJ-119"}, Summary: "Could not read the ticket."}); got != "PROJ-119: Could not read the ticket." {
		t.Errorf("unread: %q", got)
	}
	moved := ticketLine(ticketVerdict{Asked: "OLD-778", Ticket: tk, Read: true, Covers: "partly", Summary: "Half done."})
	if !strings.HasPrefix(moved, "OLD-778, now <https://acme.atlassian.net/browse/PROJ-249|PROJ-249> (In Progress") {
		t.Errorf("moved: %q", moved)
	}
}

func TestJiraOffNote(t *testing.T) {
	// A config from before 0.5.14 has no Jira site, so the stand-up says so.
	if got := jiraOffNote("", 2); !strings.Contains(got, "Jira checks are off") || !strings.Contains(got, "Settings > Worker") {
		t.Errorf("no site: %q", got)
	}
	if got := jiraOffNote("acme.atlassian.net", 2); got != "" {
		t.Errorf("site set: %q", got)
	}
	if got := jiraOffNote("", 0); got != "" {
		t.Errorf("no PRs to check: %q", got)
	}
}
