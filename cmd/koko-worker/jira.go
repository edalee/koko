package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// The stand-up's Jira facts come from Go, never from Claude's prose:
//   - Go picks each PR's ticket key (ticketKey).
//   - One Claude run only calls getJiraIssue, and Go reads each ticket from
//     the tool's raw result (fetchTickets).
//   - A second run, with no tools, judges coverage from the ticket text Go
//     hands it, and must quote that text. Go drops a verdict or follow-up
//     whose quote is not in the ticket or the PR (judgeTickets).

const toolGetJiraIssue = "mcp__claude_ai_Atlassian__getJiraIssue"

// jiraTicket is one Jira issue as getJiraIssue returned it.
type jiraTicket struct {
	Key            string
	Summary        string
	Description    string
	Status         string // "In Progress"
	StatusCategory string // "new", "indeterminate" or "done"
	Assignee       string // display name, "" if unassigned
	URL            string
	Mine           bool // assigned to the account the connector runs as
}

func (t jiraTicket) done() bool { return t.StatusCategory == "done" }

// link is the ticket key, linked to the ticket.
func (t jiraTicket) link() string {
	if t.URL == "" {
		return t.Key
	}
	return slackLink(t.URL, t.Key)
}

// parseJiraIssue reads a getJiraIssue result. The connector wraps the issue
// in {"issues": {"nodes": [...]}} and adds the caller's account under
// "context". A bare issue object is accepted too.
func parseJiraIssue(raw string) (jiraTicket, bool) {
	type issue struct {
		Key    string `json:"key"`
		WebURL string `json:"webUrl"`
		Fields struct {
			Summary     string          `json:"summary"`
			Description json.RawMessage `json:"description"`
			Status      struct {
				Name     string `json:"name"`
				Category struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"status"`
			Assignee *struct {
				AccountID   string `json:"accountId"`
				DisplayName string `json:"displayName"`
			} `json:"assignee"`
		} `json:"fields"`
	}
	var wrapped struct {
		Issues struct {
			Nodes []issue `json:"nodes"`
		} `json:"issues"`
		Context struct {
			AccountID string `json:"atlassianAccountId"`
		} `json:"context"`
	}
	if json.Unmarshal([]byte(raw), &wrapped) != nil {
		return jiraTicket{}, false
	}
	var is issue
	if len(wrapped.Issues.Nodes) > 0 {
		is = wrapped.Issues.Nodes[0]
	} else if json.Unmarshal([]byte(raw), &is) != nil || is.Key == "" {
		return jiraTicket{}, false
	}
	t := jiraTicket{
		Key: is.Key, Summary: is.Fields.Summary, URL: is.WebURL,
		Status: is.Fields.Status.Name, StatusCategory: is.Fields.Status.Category.Key,
	}
	// With markdown output the description is a string. Anything else, such
	// as ADF JSON, is kept as text, so a quote check still has words to match.
	var desc string
	if json.Unmarshal(is.Fields.Description, &desc) != nil && len(is.Fields.Description) > 0 && string(is.Fields.Description) != "null" {
		desc = string(is.Fields.Description)
	}
	t.Description = desc
	if a := is.Fields.Assignee; a != nil {
		t.Assignee = a.DisplayName
		t.Mine = a.AccountID != "" && a.AccountID == wrapped.Context.AccountID
	}
	return t, t.Key != ""
}

// ticketKey picks the PR's Jira key: the first in the title, else in the
// branch name, else in the body. The title is the PR's own claim, while a
// body often names other tickets too.
func ticketKey(d PRDetail) string {
	for _, s := range []string{d.Title, d.HeadRefName, d.Body} {
		if k := jiraKey.FindString(strings.ToUpper(s)); k != "" {
			return k
		}
	}
	return ""
}

// fetchTickets reads each key with getJiraIssue, through Claude, and parses
// the tool's raw results. A key with no successful call is missing from the
// map, and the stand-up says it could not be read.
func fetchTickets(ctx context.Context, cr claudeRunner, site string, keys []string) (map[string]jiraTicket, error) {
	out := map[string]jiraTicket{}
	if len(keys) == 0 {
		return out, nil
	}
	sort.Strings(keys)
	prompt := fmt.Sprintf(`Call %s once for each of these Jira keys: %s.
Use cloudId %q, fields ["summary", "description", "status", "assignee"] and responseContentFormat "markdown".
If the tool is not loaded yet, load it with ToolSearch first. Call no other Atlassian or Jira tool.
Then reply with the single word DONE. Do not summarise the tickets.`,
		toolGetJiraIssue, strings.Join(keys, ", "), site)
	res, err := cr.run(ctx, prompt, []string{toolGetJiraIssue}, 5*time.Minute)
	for _, c := range res.callsTo(toolGetJiraIssue) {
		if c.IsError {
			continue
		}
		t, ok := parseJiraIssue(c.Result)
		if !ok {
			continue
		}
		out[t.Key] = t
		// A ticket moved to another project answers under its new key, so
		// it is stored under the key asked for as well.
		var in struct {
			Key string `json:"issueIdOrKey"`
		}
		if json.Unmarshal(c.Input, &in) == nil && in.Key != "" {
			out[strings.ToUpper(in.Key)] = t
		}
	}
	if len(out) == 0 {
		// Nothing read at all is a Jira problem, not a ticket one, such as
		// the connector being unavailable. Say so once, with Claude's reason.
		if err == nil {
			err = fmt.Errorf("no ticket could be read: %s", truncate(res.Text, 200))
		}
		return out, err
	}
	return out, nil
}

// ticketVerdict is the judgement on one PR's ticket, checked by Go.
type ticketVerdict struct {
	URL         string // the PR
	Ticket      jiraTicket
	Asked       string // the key the PR names, which differs from Ticket.Key if the ticket moved
	Read        bool   // Go has the ticket's own text
	Covers      string // yes, partly, no or unknown
	Summary     string // one line on coverage
	CloseTicket bool   // the PR fully covers the ticket
	FollowUps   []string
}

// judgedPR is one PR whose ticket the stand-up judges.
type judgedPR struct {
	pr     SearchPR
	detail PRDetail
	ready  bool // approved and ready to merge
	merged bool // merged in the last week
}

// judgeTickets decides, for each PR, whether it covers its ticket. Claude
// gets the ticket text from Go and no tools, so it cannot read or invent
// anything else. Each claim must quote the ticket, and Go keeps a verdict
// only if its quotes are in the ticket, and a follow-up only if its quote is
// in the ticket or the PR body.
func judgeTickets(ctx context.Context, env Env, prs []judgedPR, tickets map[string]jiraTicket) (map[string]ticketVerdict, error) {
	out := map[string]ticketVerdict{}
	type in struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		Ready       bool   `json:"readyToMerge"`
		Merged      bool   `json:"merged"`
		Ticket      string `json:"ticket"`
		Summary     string `json:"ticketSummary"`
		Description string `json:"ticketDescription"`
	}
	var input []in
	for _, p := range prs {
		key := ticketKey(p.detail)
		v := ticketVerdict{URL: p.pr.URL, Asked: key, Covers: "unknown"}
		t, ok := tickets[key]
		switch {
		case key == "":
			v.Summary = "No Jira ticket found."
		case !ok:
			v.Ticket = jiraTicket{Key: key}
			v.Summary = "Could not read the ticket."
		case strings.TrimSpace(t.Description) == "":
			// Nothing to judge against. A title alone invites guesses.
			v.Ticket, v.Read = t, true
			v.Summary = "The ticket has no description, so coverage cannot be judged."
		default:
			v.Ticket, v.Read = t, true
			input = append(input, in{
				URL: p.pr.URL, Title: p.detail.Title, Body: truncate(p.detail.Body, 4000),
				Ready: p.ready, Merged: p.merged, Ticket: key,
				Summary: t.Summary, Description: truncate(t.Description, 8000),
			})
		}
		out[p.pr.URL] = v
	}
	if len(input) == 0 {
		return out, nil
	}
	data, _ := json.MarshalIndent(input, "", "  ")
	prompt := `Each item is one of my pull requests with the text of its Jira ticket. You have no tools: use only the text below.
For each PR, decide whether the PR covers what the ticket asks for.

Rules:
- Use only facts in the ticket text and the PR title and body. Do not add services, repos, PRs, tickets or tasks they do not name.
- For every claim about what the ticket asks, put the exact words from ticketSummary or ticketDescription in "evidence". Copy them character for character, 3 to 25 words each.
- If the text does not let you decide, say "unknown".
- Follow-ups are next steps for PRs that are ready or merged, at most three. Each one needs a "quote": exact words from the ticket or the PR body that the step comes from.
- Use British spelling. Do not edit, comment on or move anything.

Reply with only a JSON array, one object per PR:
[{"url": "<the PR url, copied exactly>", "covers": "yes|partly|no|unknown", "summary": "one short sentence on coverage",
  "evidence": ["exact ticket words", ...], "followUps": [{"step": "concrete next step", "quote": "exact words"}]}]

Pull requests:
` + string(data)

	res, err := env.claude.run(ctx, prompt, nil, 10*time.Minute)
	if err != nil {
		return out, err
	}
	return out, checkVerdicts(res.Text, out, prs)
}

// checkVerdicts reads Claude's verdicts into out, for the PRs Go handed it.
// A verdict whose evidence is not in the ticket becomes "unknown", and a
// follow-up whose quote is in neither the ticket nor the PR is dropped.
func checkVerdicts(text string, out map[string]ticketVerdict, prs []judgedPR) error {
	var verdicts []struct {
		URL       string   `json:"url"`
		Covers    string   `json:"covers"`
		Summary   string   `json:"summary"`
		Evidence  []string `json:"evidence"`
		FollowUps []struct {
			Step  string `json:"step"`
			Quote string `json:"quote"`
		} `json:"followUps"`
	}
	if err := json.Unmarshal([]byte(extractJSON(text)), &verdicts); err != nil {
		return fmt.Errorf("stand-up: unreadable ticket verdicts: %s", truncate(text, 200))
	}
	bodies := map[string]string{}
	for _, p := range prs {
		bodies[p.pr.URL] = p.detail.Title + "\n" + p.detail.Body
	}
	for _, j := range verdicts {
		v, ok := out[j.URL]
		if !ok || !v.Read {
			continue // not a PR we asked about, or one Go already settled
		}
		ticketText := v.Ticket.Summary + "\n" + v.Ticket.Description
		if !allQuoted(j.Evidence, ticketText) || (j.Covers != "unknown" && len(j.Evidence) == 0) {
			v.Covers, v.Summary = "unknown", "Could not check the claims against the ticket text."
			out[j.URL] = v
			continue
		}
		v.Covers, v.Summary = j.Covers, j.Summary
		v.CloseTicket = j.Covers == "yes"
		for _, f := range j.FollowUps {
			if f.Step != "" && quoted(f.Quote, ticketText+"\n"+bodies[j.URL]) {
				v.FollowUps = append(v.FollowUps, f.Step)
			}
		}
		out[j.URL] = v
	}
	return nil
}

// quoted is true when q, with spacing and case ignored, is in text.
func quoted(q, text string) bool {
	q = normalSpace(q)
	return len(q) >= 8 && strings.Contains(normalSpace(text), q)
}

func allQuoted(quotes []string, text string) bool {
	for _, q := range quotes {
		if !quoted(q, text) {
			return false
		}
	}
	return true
}

func normalSpace(s string) string {
	s = strings.NewReplacer("*", "", "_", "", "`", "").Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

// ticketLine is the line under a PR in "Ready to merge": the linked ticket,
// its status and owner, and the verdict.
func ticketLine(v ticketVerdict) string {
	t := v.Ticket
	if t.Key == "" {
		return v.Summary
	}
	head := t.link()
	if v.Asked != "" && v.Asked != t.Key {
		head = v.Asked + ", now " + head
	}
	if v.Read {
		who := t.Assignee
		if who == "" {
			who = "unassigned"
		}
		head += fmt.Sprintf(" (%s, %s)", t.Status, who)
	}
	switch {
	case !v.Read:
		return head + ": " + v.Summary
	case v.CloseTicket && !t.done():
		return fmt.Sprintf("%s: covered. %s Close the ticket after the merge.", head, v.Summary)
	default:
		return fmt.Sprintf("%s: %s. %s", head, v.Covers, v.Summary)
	}
}

// closableSection is the stand-up's "Stories you can close": tickets
// assigned to you, not done, that a merged PR covers (close now) or a PR
// ready to merge covers (close after the merge). "" when there are none.
func closableSection(prs []judgedPR, verdicts map[string]ticketVerdict) string {
	var now, after []string
	seen := map[string]bool{}
	for _, p := range prs {
		v := verdicts[p.pr.URL]
		t := v.Ticket
		if !v.Read || !v.CloseTicket || !t.Mine || t.done() || seen[t.Key] || (!p.merged && !p.ready) {
			continue
		}
		seen[t.Key] = true
		line := fmt.Sprintf("• %s %s, covered by %s", t.link(), t.Summary, slackLink(p.pr.URL, p.pr.short()))
		if p.merged {
			now = append(now, line+" (merged)")
		} else {
			after = append(after, line)
		}
	}
	if len(now) == 0 && len(after) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("*Stories you can close*\n")
	if len(now) > 0 {
		b.WriteString("Now:\n" + strings.Join(now, "\n") + "\n")
	}
	if len(after) > 0 {
		b.WriteString("After the merge:\n" + strings.Join(after, "\n") + "\n")
	}
	return b.String()
}
