package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The team summary: what each member of a team did on the previous working
// day. GitHub gives the facts, Jira adds the tickets they moved, and Claude
// may turn the facts into a paragraph per person. Go builds every link.

// summaryGH runs gh for the summary. Tests replace it.
var summaryGH = gh

const (
	// summaryFirst is how many PRs one search reads. A search that finds more
	// says so in the DM.
	summaryFirst = 50
	// summaryBatch is how many members one GraphQL call covers, with two
	// searches each.
	summaryBatch = 5
	// summaryMaxMembers caps the members read, so a large team stays cheap.
	summaryMaxMembers = 30
)

const toolSearchJira = "mcp__claude_ai_Atlassian__searchJiraIssuesUsingJql"

var loginRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)

// summaryPeriod is the time the summary on now's day covers: from the start
// of the job's previous run day to the start of today, in loc. After days
// off, such as a weekend, it covers every day since the last run day.
func summaryPeriod(job JobConfig, now time.Time, loc *time.Location) (start, end time.Time) {
	now = now.In(loc)
	end = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	for i := 1; i <= 7; i++ {
		if day := end.AddDate(0, 0, -i); job.runsOn(day) {
			return day, end
		}
	}
	return end.AddDate(0, 0, -1), end // not reached: a job runs on at least one day
}

// periodLabel names the days a period covers, such as "Friday 17 October"
// or "Friday 17 to Sunday 19 October".
func periodLabel(start, end time.Time) string {
	last := end.AddDate(0, 0, -1)
	switch {
	case !last.After(start):
		return start.Format("Monday 2 January")
	case start.Month() == last.Month():
		return start.Format("Monday 2") + " to " + last.Format("Monday 2 January")
	default:
		return start.Format("Monday 2 January") + " to " + last.Format("Monday 2 January")
	}
}

// summaryPR is one PR as the summary's GraphQL search returns it.
type summaryPR struct {
	URL         string    `json:"url"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"createdAt"`
	MergedAt    time.Time `json:"mergedAt"` // zero if not merged
	HeadRefName string    `json:"headRefName"`
	Repository  struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Author  ghActor `json:"author"`
	Reviews struct {
		Nodes []struct {
			Author      ghActor   `json:"author"`
			SubmittedAt time.Time `json:"submittedAt"` // zero for a pending review
			State       string    `json:"state"`
		} `json:"nodes"`
	} `json:"reviews"`
}

type ghActor struct {
	Login    string `json:"login"`
	TypeName string `json:"__typename"` // "User" or "Bot"
}

func (a ghActor) bot() bool {
	return a.TypeName == "Bot" || strings.HasSuffix(strings.ToLower(a.Login), "[bot]")
}

func (p summaryPR) short() string {
	return fmt.Sprintf("%s#%d", shortRepo(p.Repository.NameWithOwner), p.Number)
}

// prFact is one PR in a person's facts. Claude sees ref, title and author.
type prFact struct {
	Ref    string `json:"ref"`
	Title  string `json:"title"`
	Author string `json:"author,omitempty"` // reviews only: whose PR it was
	URL    string `json:"-"`
	key    string // the Jira key the PR names, "" if none
}

// ticketFact is one Jira ticket a person's PR names, moved in the period.
type ticketFact struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
	URL     string `json:"-"`
}

// personFacts is what one team member did in the period.
type personFacts struct {
	Login    string       `json:"login"`
	Merged   []prFact     `json:"merged,omitempty"`
	Opened   []prFact     `json:"opened,omitempty"`
	Reviewed []prFact     `json:"reviewed,omitempty"`
	Tickets  []ticketFact `json:"ticketsMoved,omitempty"`
	// More is true if a search found more PRs than it read.
	More bool `json:"-"`
}

func (p personFacts) active() bool {
	return len(p.Merged)+len(p.Opened)+len(p.Reviewed) > 0
}

// summaryScope is the search qualifiers for the summary's repos, or for the
// team's org without repos.
func summaryScope(s SummaryConfig) string {
	if len(s.Repos) == 0 {
		org, _, _ := strings.Cut(s.Team, "/")
		return "org:" + org
	}
	var parts []string
	for _, r := range s.Repos {
		parts = append(parts, "repo:"+r)
	}
	return strings.Join(parts, " ")
}

// summaryQuery is a GraphQL query with n searches, s0 to s(n-1), each taking
// its search text from the variable $q0 to $q(n-1).
func summaryQuery(n int) string {
	const fields = `url number title createdAt mergedAt headRefName repository { nameWithOwner } author { login __typename } ` +
		`reviews(last: 50) { nodes { author { login __typename } submittedAt state } }`
	var vars, searches []string
	for i := 0; i < n; i++ {
		vars = append(vars, fmt.Sprintf("$q%d: String!", i))
		searches = append(searches, fmt.Sprintf("s%d: search(query: $q%d, type: ISSUE, first: %d) { issueCount nodes { ... on PullRequest { %s } } }",
			i, i, summaryFirst, fields))
	}
	return "query(" + strings.Join(vars, ", ") + ") { " + strings.Join(searches, " ") + " }"
}

// searchActivity reads the PRs each member wrote or reviewed that changed
// since start: two searches per member, five members per gh call. It returns
// the PRs by URL, the members whose searches found more than they read, and
// one note per batch that failed. A failed batch does not stop the others.
func searchActivity(ctx context.Context, s SummaryConfig, logins []string, start time.Time) (map[string]summaryPR, map[string]bool, []string, error) {
	prs := map[string]summaryPR{}
	more := map[string]bool{}
	var notes []string
	var firstErr error
	scope := summaryScope(s)
	since := start.UTC().Format("2006-01-02T15:04:05Z")
	for i := 0; i < len(logins); i += summaryBatch {
		batch := logins[i:min(i+summaryBatch, len(logins))]
		args := []string{"api", "graphql", "-f", "query=" + summaryQuery(2 * len(batch))}
		for j, login := range batch {
			args = append(args,
				"-f", fmt.Sprintf("q%d=is:pr author:%s updated:>=%s %s", 2*j, login, since, scope),
				"-f", fmt.Sprintf("q%d=is:pr reviewed-by:%s updated:>=%s %s", 2*j+1, login, since, scope))
		}
		var resp struct {
			Data map[string]struct {
				IssueCount int         `json:"issueCount"`
				Nodes      []summaryPR `json:"nodes"`
			} `json:"data"`
		}
		out, err := summaryGH(ctx, args...)
		if err == nil {
			err = json.Unmarshal(out, &resp)
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			notes = append(notes, fmt.Sprintf("Could not read GitHub for %s: %s", strings.Join(batch, ", "), truncate(err.Error(), 200)))
			continue
		}
		for j, login := range batch {
			for _, k := range []int{2 * j, 2*j + 1} {
				res := resp.Data[fmt.Sprintf("s%d", k)]
				if res.IssueCount > len(res.Nodes) {
					more[login] = true
				}
				for _, p := range res.Nodes {
					if p.URL != "" {
						prs[p.URL] = p
					}
				}
			}
		}
	}
	return prs, more, notes, firstErr
}

// buildFacts sorts the PRs into each member's merged, opened and reviewed
// lists for the period [start, end). Bots, reviews of your own PR, pending
// reviews and repos outside s.Repos are left out. People come back sorted by login.
func buildFacts(s SummaryConfig, logins []string, prs map[string]summaryPR, more map[string]bool, start, end time.Time) []personFacts {
	in := func(t time.Time) bool { return !t.IsZero() && !t.Before(start) && t.Before(end) }
	inRepos := func(repo string) bool {
		if len(s.Repos) == 0 {
			return true
		}
		for _, r := range s.Repos {
			if strings.EqualFold(r, repo) {
				return true
			}
		}
		return false
	}
	people := map[string]*personFacts{}
	for _, l := range logins {
		people[strings.ToLower(l)] = &personFacts{Login: l, More: more[l]}
	}
	var urls []string
	for u := range prs {
		urls = append(urls, u)
	}
	sort.Strings(urls)
	for _, u := range urls {
		p := prs[u]
		if !inRepos(p.Repository.NameWithOwner) {
			continue
		}
		author := strings.ToLower(p.Author.Login)
		fact := prFact{Ref: p.short(), Title: p.Title, URL: p.URL, key: ticketKey(PRDetail{Title: p.Title, HeadRefName: p.HeadRefName})}
		if who, ok := people[author]; ok && !p.Author.bot() {
			if in(p.MergedAt) {
				who.Merged = append(who.Merged, fact)
			}
			if in(p.CreatedAt) {
				who.Opened = append(who.Opened, fact)
			}
		}
		seen := map[string]bool{}
		for _, r := range p.Reviews.Nodes {
			reviewer := strings.ToLower(r.Author.Login)
			who, ok := people[reviewer]
			if !ok || r.Author.bot() || reviewer == author || r.State == "PENDING" || !in(r.SubmittedAt) || seen[reviewer] {
				continue
			}
			seen[reviewer] = true
			rf := fact
			rf.Author, rf.key = p.Author.Login, ""
			who.Reviewed = append(who.Reviewed, rf)
		}
	}
	var out []personFacts
	for _, l := range logins {
		out = append(out, *people[strings.ToLower(l)])
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Login) < strings.ToLower(out[j].Login) })
	return out
}

// movedTickets returns the tickets, out of keys, whose status changed in
// the period, by the key asked for. Two Claude runs read them: fetchTickets
// checks the keys exist, because one unknown key fails a whole JQL search.
// Then one JQL search finds those that moved. Jira reads the DURING dates in
// the Jira account's own time zone.
func movedTickets(ctx context.Context, cr claudeRunner, site string, keys []string, start, end time.Time) (map[string]jiraTicket, error) {
	out := map[string]jiraTicket{}
	if len(keys) == 0 {
		return out, nil
	}
	tickets, err := fetchTickets(ctx, cr, site, keys)
	if len(tickets) == 0 {
		return out, err
	}
	var found []string
	seen := map[string]bool{}
	for _, t := range tickets {
		if !seen[t.Key] {
			seen[t.Key] = true
			found = append(found, t.Key)
		}
	}
	sort.Strings(found)
	jql := fmt.Sprintf(`key in (%s) AND status CHANGED DURING ("%s", "%s")`,
		strings.Join(found, ", "), start.Format("2006/01/02"), end.Format("2006/01/02"))
	prompt := fmt.Sprintf(`Call %s once, with cloudId %q, jql %q, fields ["summary", "status", "assignee"] and maxResults 100.
If the tool is not loaded yet, load it with ToolSearch first. Call no other Atlassian or Jira tool.
Then reply with the single word DONE. Do not summarise the tickets.`, toolSearchJira, site, jql)
	res, err := cr.run(ctx, prompt, []string{toolSearchJira}, 5*time.Minute)
	for _, c := range res.callsTo(toolSearchJira) {
		if c.IsError {
			continue
		}
		moved := map[string]bool{}
		for _, t := range parseJiraSearch(c.Result) {
			moved[t.Key] = true
		}
		for asked, t := range tickets {
			if moved[t.Key] {
				out[asked] = t
			}
		}
		return out, nil
	}
	if err == nil {
		err = fmt.Errorf("the Jira search gave no result: %s", truncate(res.Text, 200))
	}
	return out, err
}

// parseJiraSearch reads a searchJiraIssuesUsingJql result. The issues are a
// list, under "issues" or "issues.nodes".
func parseJiraSearch(raw string) []jiraTicket {
	var v struct {
		Issues json.RawMessage `json:"issues"`
	}
	if json.Unmarshal([]byte(raw), &v) != nil {
		return nil
	}
	var items []json.RawMessage
	if json.Unmarshal(v.Issues, &items) != nil {
		var wrapped struct {
			Nodes []json.RawMessage `json:"nodes"`
		}
		_ = json.Unmarshal(v.Issues, &wrapped)
		items = wrapped.Nodes
	}
	var out []jiraTicket
	for _, it := range items {
		if t, ok := parseJiraIssue(string(it)); ok {
			out = append(out, t)
		}
	}
	return out
}

// addTickets puts each moved ticket under the person whose PR names it.
func addTickets(people []personFacts, moved map[string]jiraTicket) {
	for i := range people {
		seen := map[string]bool{}
		for _, f := range append(append([]prFact{}, people[i].Merged...), people[i].Opened...) {
			t, ok := moved[f.key]
			if !ok || seen[t.Key] {
				continue
			}
			seen[t.Key] = true
			people[i].Tickets = append(people[i].Tickets, ticketFact{Key: t.Key, Summary: t.Summary, Status: t.Status, URL: t.URL})
		}
	}
}

// ticketKeys is every Jira key the people's merged and opened PRs name.
func ticketKeys(people []personFacts) []string {
	var keys []string
	seen := map[string]bool{}
	for _, p := range people {
		for _, f := range append(append([]prFact{}, p.Merged...), p.Opened...) {
			if f.key != "" && !seen[f.key] {
				seen[f.key] = true
				keys = append(keys, f.key)
			}
		}
	}
	return keys
}

// prosePrompt asks for one paragraph per person, from the facts only.
func prosePrompt(label string, people []personFacts) string {
	data, _ := json.MarshalIndent(people, "", "  ")
	return `Below is what each member of a software team did on ` + label + `, as JSON. You have no tools: use only these facts.
For each person, write one short paragraph in plain English: what they merged, opened and reviewed, and the tickets they moved.

Rules:
- Use only the facts given. Do not add reasons, outcomes, opinions or anything else they do not say.
- Name PRs by their ref, such as api#12, and tickets by their key.
- Refer to each person by their login. Do not guess anyone's pronouns.
- British spelling. Active voice. At most three sentences per person.

Reply with only a JSON object from each login to its paragraph, such as {"alice": "..."}.

Facts:
` + string(data)
}

var refRe = regexp.MustCompile(`[A-Za-z0-9._-]+#\d+`)

// parseProse reads Claude's paragraphs. A paragraph is dropped, so its
// person falls back to lists, if it names a PR or ticket not in that
// person's facts.
func parseProse(text string, people []personFacts) (map[string]string, error) {
	var raw map[string]string
	if err := json.Unmarshal([]byte(extractJSON(text)), &raw); err != nil {
		return nil, fmt.Errorf("unreadable paragraphs: %s", truncate(text, 200))
	}
	out := map[string]string{}
	for _, p := range people {
		para := strings.TrimSpace(raw[p.Login])
		if para == "" {
			continue
		}
		known := map[string]bool{}
		for _, f := range append(append(append([]prFact{}, p.Merged...), p.Opened...), p.Reviewed...) {
			known[strings.ToLower(f.Ref)] = true
		}
		for _, t := range p.Tickets {
			known[strings.ToLower(t.Key)] = true
		}
		ok := true
		for _, ref := range refRe.FindAllString(para, -1) {
			ok = ok && known[strings.ToLower(ref)]
		}
		for _, key := range jiraKey.FindAllString(para, -1) {
			ok = ok && known[strings.ToLower(key)]
		}
		if ok {
			out[p.Login] = para
		}
	}
	return out, nil
}

// writeProse has Claude, with no tools, write the paragraphs.
func writeProse(ctx context.Context, cr claudeRunner, label string, people []personFacts) (map[string]string, error) {
	res, err := cr.run(ctx, prosePrompt(label, people), nil, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	return parseProse(res.Text, people)
}

// slackEscape keeps Slack from reading text as a link or a mention.
func slackEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// personSection is one person's part of the DM: Claude's paragraph and the
// links, or plain lists without a paragraph.
func personSection(p personFacts, para string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%s*\n", p.Login)
	if para != "" {
		b.WriteString(slackEscape(para) + "\n")
		var links []string
		seen := map[string]bool{}
		for _, f := range append(append(append([]prFact{}, p.Merged...), p.Opened...), p.Reviewed...) {
			if !seen[f.URL] {
				seen[f.URL] = true
				links = append(links, slackLink(f.URL, f.Ref))
			}
		}
		b.WriteString("PRs: " + strings.Join(links, ", ") + "\n")
		if len(p.Tickets) > 0 {
			var keys []string
			for _, t := range p.Tickets {
				keys = append(keys, ticketLink(t))
			}
			b.WriteString("Tickets: " + strings.Join(keys, ", ") + "\n")
		}
	} else {
		for _, list := range []struct {
			name  string
			facts []prFact
		}{{"Merged", p.Merged}, {"Opened", p.Opened}, {"Reviewed", p.Reviewed}} {
			for _, f := range list.facts {
				line := fmt.Sprintf("• %s %s %s", list.name, slackLink(f.URL, f.Ref), slackEscape(f.Title))
				if f.Author != "" {
					line += " (" + f.Author + ")"
				}
				b.WriteString(line + "\n")
			}
		}
		for _, t := range p.Tickets {
			fmt.Fprintf(&b, "• Moved %s %s (%s)\n", ticketLink(t), slackEscape(t.Summary), t.Status)
		}
	}
	if p.More {
		fmt.Fprintf(&b, "_Some searches found more than %d PRs, so this may miss some._\n", summaryFirst)
	}
	return b.String()
}

func ticketLink(t ticketFact) string {
	if t.URL == "" {
		return t.Key
	}
	return slackLink(t.URL, t.Key)
}

// summaryMessage is the DM: a header with any notes, one section per active
// person, and one line naming everyone with no activity.
func summaryMessage(label string, people []personFacts, prose map[string]string, notes []string) Message {
	head := fmt.Sprintf("*Team summary, %s*", label)
	for _, n := range notes {
		head += "\n_" + slackEscape(n) + "_"
	}
	sections := []string{head}
	var idle []string
	for _, p := range people {
		if !p.active() {
			idle = append(idle, p.Login)
			continue
		}
		sections = append(sections, personSection(p, prose[p.Login]))
	}
	if len(idle) > 0 {
		sections = append(sections, "No activity: "+strings.Join(idle, ", ")+".")
	}
	return Message{Sections: sections}
}

// summaryLogins is the team's members to report on, sorted, without bots
// and odd names, at most summaryMaxMembers.
func summaryLogins(members map[string]bool) []string {
	var out []string
	for m := range members {
		if loginRe.MatchString(m) && !strings.HasSuffix(m, "[bot]") {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out[:min(len(out), summaryMaxMembers)]
}

func runSummary(ctx context.Context, env Env) error {
	s := env.cfg.Summary
	if err := s.problem(); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	start, end := summaryPeriod(env.cfg.Jobs[JobSummary], env.now, env.cfg.location())
	label := periodLabel(start, end)
	members, err := teamMembers(ctx, s.Team)
	if err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	logins := summaryLogins(members)
	var notes []string
	if len(members) > len(logins) && len(logins) == summaryMaxMembers {
		notes = append(notes, fmt.Sprintf("The team has %d members. This covers the first %d.", len(members), summaryMaxMembers))
	}

	prs, more, ghNotes, ghErr := searchActivity(ctx, s, logins, start)
	batches := (len(logins) + summaryBatch - 1) / summaryBatch
	if ghErr != nil && (len(ghNotes) == batches || !online(ctx)) {
		// Nothing read, or no internet: the scheduler tries again.
		return fmt.Errorf("summary: %w", ghErr)
	}
	notes = append(notes, ghNotes...)
	people := buildFacts(s, logins, prs, more, start, end)

	// With no Jira site set, the summary reads no tickets.
	if keys := ticketKeys(people); s.Jira && env.cfg.JiraSite != "" && len(keys) > 0 {
		moved, err := movedTickets(ctx, env.claude, env.cfg.JiraSite, keys, start, end)
		if err != nil && !online(ctx) {
			return err
		}
		if err != nil {
			notes = append(notes, "Could not check Jira: "+truncate(err.Error(), 200))
		}
		addTickets(people, moved)
	}

	return env.send(ctx, summaryProseMessage(ctx, env, label, people, notes))
}

// summaryProseMessage builds the DM, with Claude's paragraphs if Prose is
// on. If Claude fails, the DM lists the facts and says why.
func summaryProseMessage(ctx context.Context, env Env, label string, people []personFacts, notes []string) Message {
	var active []personFacts
	for _, p := range people {
		if p.active() {
			active = append(active, p)
		}
	}
	var prose map[string]string
	if env.cfg.Summary.Prose && len(active) > 0 {
		var err error
		prose, err = writeProse(ctx, env.claude, label, active)
		if err != nil {
			notes = append(notes, "Claude could not write the paragraphs, so this lists the facts: "+truncate(err.Error(), 150))
		}
	}
	return summaryMessage(label, people, prose, notes)
}
