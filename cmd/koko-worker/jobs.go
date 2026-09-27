package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Env is what a job needs. notify sends the DM, or prints it in test mode.
type Env struct {
	cfg    Config
	paths  Paths
	claude claudeRunner
	notify func(ctx context.Context, text string) error
	test   bool   // print instead of DM, book nothing
	state  *State // read-only snapshot taken when the run started
	now    time.Time
	// persist changes the state on disk under a lock. It does nothing in test mode.
	persist func(change func(*State)) error
}

// ---- Stand-up ----

var jiraKey = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)

// ticketVerdict is Claude's judgement on one approved PR.
type ticketVerdict struct {
	URL         string   `json:"url"`
	Ticket      string   `json:"ticket"`  // "PDDI-123", or "" if none found
	Covers      string   `json:"covers"`  // yes, partly, no, unknown
	Summary     string   `json:"summary"` // one line on coverage
	CloseTicket bool     `json:"closeTicket"`
	FollowUps   []string `json:"followUps"`
}

type approvedPR struct {
	pr       SearchPR
	detail   PRDetail
	blockers []string
}

func runStandup(ctx context.Context, env Env) error {
	loc := env.cfg.location()
	now := env.now.In(loc)

	events, calErr := listEvents(ctx, env.claude, env.cfg, now)

	// Each section degrades on its own, so a stand-up is never lost to one
	// failing source. A network error still fails the run, so it is retried.
	var prs []approvedPR
	approved, prErr := myApprovedPRs(ctx)
	for _, p := range approved {
		d, err := prDetail(ctx, p.URL)
		if err != nil {
			prErr = err
			break
		}
		prs = append(prs, approvedPR{pr: p, detail: d, blockers: d.blockers()})
	}

	var queue []SearchPR
	requests, queueErr := reviewRequests(ctx)
	if queueErr == nil {
		me, err := myLogin(ctx)
		queue, queueErr = reviewQueue(requests, me), err
	}

	for _, err := range []error{calErr, prErr, queueErr} {
		if err != nil && isNetworkError(err) {
			return err
		}
	}

	verdicts := map[string]ticketVerdict{}
	var jiraErr error
	if len(prs) > 0 {
		verdicts, jiraErr = judgeTickets(ctx, env, prs)
		if jiraErr != nil && isNetworkError(jiraErr) {
			return jiraErr
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "*Stand-up, %s*\n", now.Format("Monday 2 January"))

	b.WriteString("\n*Today*\n")
	switch {
	case calErr != nil:
		fmt.Fprintf(&b, "• Could not read the calendar: %s\n", truncate(calErr.Error(), 200))
	default:
		lines := meetings(events, loc)
		if len(lines) == 0 {
			b.WriteString("• No meetings\n")
		}
		for _, l := range lines {
			b.WriteString("• " + l + "\n")
		}
	}

	var ready, blocked []approvedPR
	for _, p := range prs {
		if len(p.blockers) == 0 {
			ready = append(ready, p)
		} else {
			blocked = append(blocked, p)
		}
	}

	b.WriteString("\n*Ready to merge*\n")
	switch {
	case prErr != nil:
		fmt.Fprintf(&b, "• Could not load your PRs: %s\n", truncate(prErr.Error(), 200))
	case len(ready) == 0:
		b.WriteString("• None\n")
	case jiraErr != nil:
		fmt.Fprintf(&b, "_Could not check Jira: %s_\n", truncate(jiraErr.Error(), 200))
	}
	for _, p := range ready {
		v := verdicts[p.pr.URL]
		fmt.Fprintf(&b, "• %s %s\n", slackLink(p.pr.URL, p.pr.short()), p.pr.Title)
		if line := ticketLine(v); line != "" {
			b.WriteString("    " + line + "\n")
		}
	}

	if len(blocked) > 0 {
		b.WriteString("\n*Approved, not ready*\n")
		for _, p := range blocked {
			fmt.Fprintf(&b, "• %s %s: %s\n", slackLink(p.pr.URL, p.pr.short()), p.pr.Title, strings.Join(p.blockers, ", "))
		}
	}

	b.WriteString("\n*Follow-up steps*\n")
	wrote := false
	for _, p := range ready {
		for _, f := range verdicts[p.pr.URL].FollowUps {
			fmt.Fprintf(&b, "• %s: %s\n", p.pr.short(), f)
			wrote = true
		}
	}
	if !wrote {
		b.WriteString("• None\n")
	}

	b.WriteString("\n*Needs your review*\n")
	switch {
	case queueErr != nil:
		fmt.Fprintf(&b, "• Could not load review requests: %s\n", truncate(queueErr.Error(), 200))
	case len(queue) == 0:
		b.WriteString("• None\n")
	}
	for _, p := range queue {
		fmt.Fprintf(&b, "• %s %s (%s)\n", slackLink(p.URL, p.short()), p.Title, p.Author.Login)
	}

	return env.notify(ctx, b.String())
}

func ticketLine(v ticketVerdict) string {
	switch {
	case v.URL == "":
		return "" // Jira was not checked
	case v.Ticket == "":
		return "No Jira ticket found."
	case v.CloseTicket:
		return fmt.Sprintf("%s: covered. %s Close the ticket after the merge.", v.Ticket, v.Summary)
	default:
		return fmt.Sprintf("%s: %s. %s", v.Ticket, v.Covers, v.Summary)
	}
}

// judgeTickets asks Claude whether each approved PR covers its Jira ticket.
// Go supplies the PR data, so Claude reads only Jira.
func judgeTickets(ctx context.Context, env Env, prs []approvedPR) (map[string]ticketVerdict, error) {
	type in struct {
		URL      string   `json:"url"`
		Title    string   `json:"title"`
		Branch   string   `json:"branch"`
		Body     string   `json:"body"`
		Keys     []string `json:"jiraKeysFound"`
		Ready    bool     `json:"readyToMerge"`
		Blockers []string `json:"blockers,omitempty"`
	}
	var input []in
	for _, p := range prs {
		text := p.detail.Title + " " + p.detail.HeadRefName + " " + p.detail.Body
		input = append(input, in{
			URL: p.pr.URL, Title: p.detail.Title, Branch: p.detail.HeadRefName,
			Body: truncate(p.detail.Body, 3000), Keys: uniq(jiraKey.FindAllString(text, -1)),
			Ready: len(p.blockers) == 0, Blockers: p.blockers,
		})
	}
	data, _ := json.MarshalIndent(input, "", "  ")
	prompt := `These are my approved pull requests. For each one, find its Jira ticket (use jiraKeysFound, or a key in the title, branch or body), and read it with getJiraIssue.
Decide whether the PR covers what the ticket asks for. Do not edit, comment on or move any ticket.

Reply with only a JSON array, one object per PR, in this shape:
[{"url": "<the PR url, copied exactly>", "ticket": "PDDI-123 or empty", "covers": "yes|partly|no|unknown",
  "summary": "one short sentence on coverage", "closeTicket": true if the PR fully covers the ticket and is ready to merge,
  "followUps": ["concrete next step", ...]}]
Follow-ups are for PRs that are ready to merge: for example check the deploy, close the ticket, update docs, or raise a follow-up ticket. At most three each. Use British spelling.

Pull requests:
` + string(data)

	res, err := env.claude.run(ctx, prompt, jiraRead, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	var verdicts []ticketVerdict
	if err := json.Unmarshal([]byte(extractJSON(res.Text)), &verdicts); err != nil {
		return nil, fmt.Errorf("stand-up: unreadable ticket verdicts: %s", truncate(res.Text, 200))
	}
	out := map[string]ticketVerdict{}
	for _, v := range verdicts {
		out[v.URL] = v
	}
	return out, nil
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ---- Focus ----

func runFocus(ctx context.Context, env Env) error {
	now := env.now.In(env.cfg.location())
	events, err := listEvents(ctx, env.claude, env.cfg, now)
	if err != nil {
		return err
	}
	gaps := findFocusGaps(events, env.cfg.Focus, now)
	if len(gaps) == 0 {
		if env.test {
			return env.notify(ctx, "Focus: no gaps to book today.")
		}
		return nil
	}
	if env.test {
		return env.notify(ctx, "Focus (test, nothing booked): would book "+formatGaps(gaps))
	}
	booked, err := createFocusBlocks(ctx, env.claude, env.cfg, gaps)
	if len(booked) > 0 {
		if nerr := env.notify(ctx, ":headphones: Focus time booked today: "+formatGaps(booked)); nerr != nil && err == nil {
			err = nerr
		}
	}
	return err
}

func formatGaps(gaps []Gap) string {
	var parts []string
	for _, g := range gaps {
		parts = append(parts, g.Start.Format("15:04")+"–"+g.End.Format("15:04"))
	}
	return strings.Join(parts, ", ")
}

// ---- Tono ----

// tonoWrapper is TONO_CLAUDE for tono's own Claude runs. It keeps the runs out
// of your conversation list and denies posting, as a guard on top of never
// passing tono's -c flag.
const tonoWrapper = `#!/bin/bash
exec claude "$@" --no-session-persistence \
  "--disallowedTools=Bash(gh pr comment:*),Bash(gh pr review:*),Bash(gh pr merge:*),Bash(gh pr edit:*),Bash(gh pr close:*),Bash(git push:*)"
`

func runTono(ctx context.Context, env Env, onlyURL string) error {
	if _, err := os.Stat(env.cfg.TonoPath); err != nil {
		return fmt.Errorf("tono: not found at %s", env.cfg.TonoPath)
	}
	if err := os.WriteFile(env.paths.TonoWrap, []byte(tonoWrapper), 0o700); err != nil {
		return err
	}
	prs, err := myOpenPRs(ctx)
	if err != nil {
		return err
	}
	if onlyURL == "" && env.state.TonoBaseline.IsZero() && !env.test {
		return tonoBaseline(ctx, env, prs)
	}
	var failures []string
	for _, p := range prs {
		if onlyURL != "" && p.URL != onlyURL {
			continue
		}
		d, err := prDetail(ctx, p.URL)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("%s#%d@%s", p.repo(), p.Number, d.HeadRefOid)
		if _, done := env.state.TonoReviewed[key]; done && onlyURL == "" {
			continue
		}
		report, logPath, err := tonoReview(ctx, env, p, d.HeadRefOid)
		link := slackLink(p.URL, p.short())
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", p.short(), err))
			if nerr := env.notify(ctx, fmt.Sprintf(":warning: Tono could not review %s: %s", link, truncate(err.Error(), 400))); nerr != nil {
				return nerr
			}
		} else if nerr := env.notify(ctx, fmt.Sprintf("*Tono: %s* %s\n\n%s", link, p.Title, cutForSlack(report, logPath))); nerr != nil {
			return nerr
		}
		// A failed review is marked too, so it is not re-reported three times a day.
		if err := env.persist(func(st *State) { st.TonoReviewed[key] = time.Now() }); err != nil {
			return err
		}
	}
	if len(failures) > 0 {
		return reportedError{fmt.Errorf("tono: %s", strings.Join(failures, "; "))}
	}
	return nil
}

// tonoBaseline runs the first time tono is switched on. It marks the open
// PRs' current commits as seen, so only new PRs and new commits get reviewed.
func tonoBaseline(ctx context.Context, env Env, prs []SearchPR) error {
	keys := map[string]bool{}
	for _, p := range prs {
		d, err := prDetail(ctx, p.URL)
		if err != nil {
			return err
		}
		keys[fmt.Sprintf("%s#%d@%s", p.repo(), p.Number, d.HeadRefOid)] = true
	}
	if err := env.persist(func(st *State) {
		now := time.Now()
		for k := range keys {
			st.TonoReviewed[k] = now
		}
		st.TonoBaseline = now
	}); err != nil {
		return err
	}
	return env.notify(ctx, fmt.Sprintf("Tono is on. I marked your %d open PRs as seen. From now on, new PRs and new commits get a review.", len(keys)))
}

// reportedError is a failure the job has already sent you, so the scheduler
// logs it without a second DM.
type reportedError struct{ error }

// tonoReview runs the tono CLI at the PR head in a cache clone. It never
// touches your working clones.
func tonoReview(ctx context.Context, env Env, p SearchPR, headSHA string) (string, string, error) {
	dir := filepath.Join(env.paths.Cache, strings.ReplaceAll(p.repo(), "/", "__"))
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(env.paths.Cache, 0o700); err != nil {
			return "", "", err
		}
		if _, err := gh(ctx, "repo", "clone", p.repo(), dir, "--", "--filter=blob:none", "--quiet"); err != nil {
			return "", "", err
		}
	}
	for _, args := range [][]string{
		{"-C", dir, "fetch", "--quiet", "origin", fmt.Sprintf("pull/%d/head", p.Number)},
		{"-C", dir, "checkout", "--quiet", "--force", "--detach", headSHA},
	} {
		if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
			return "", "", fmt.Errorf("git %s: %s", args[2], truncate(strings.TrimSpace(string(out)), 300))
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, env.cfg.TonoPath, fmt.Sprint(p.Number), "--all", "-l", "high", "-R", p.repo())
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TONO_CLAUDE="+env.paths.TonoWrap)
	// stdout is the report. stderr has tono's progress lines, which go only to the log.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	err := cmd.Run()

	logPath := filepath.Join(env.paths.Logs, fmt.Sprintf("tono-%s-%d-%s.log", shortRepo(p.repo()), p.Number, headSHA[:min(8, len(headSHA))]))
	_ = os.MkdirAll(env.paths.Logs, 0o700)
	_ = os.WriteFile(logPath, append(stderr.Bytes(), stdout.Bytes()...), 0o600)
	if ctx.Err() == context.DeadlineExceeded {
		return "", logPath, fmt.Errorf("timed out after 45 minutes")
	}
	if err != nil {
		return "", logPath, fmt.Errorf("%v: %s", err, truncate(lastLines(stderr.String()+stdout.String(), 5), 400))
	}
	if final := tonoVerified(started, p.repo(), p.Number); final != "" {
		return final, logPath, nil
	}
	return formatTonoReport(stdout.String()), logPath, nil
}

var tonoPasses = []struct{ name, title string }{
	{"review", "Code review"},
	{"docs", "Docs and ticket"},
	{"comments", "Code comments"},
}

// tonoVerified builds the report from each pass's verified output. tono's
// stdout also holds the raw rounds, whose findings the verify step may drop.
func tonoVerified(since time.Time, repo string, number int) string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	var b strings.Builder
	for _, pass := range tonoPasses {
		matches, _ := filepath.Glob(filepath.Join(dir, "tono", "logs", "*-"+tonoLockKey(repo, number)+"-"+pass.name+"-merged.log"))
		var newest string
		var newestAt time.Time
		for _, m := range matches {
			if info, err := os.Stat(m); err == nil && !info.ModTime().Before(since) && info.ModTime().After(newestAt) {
				newest, newestAt = m, info.ModTime()
			}
		}
		if newest == "" {
			continue
		}
		data, err := os.ReadFile(newest)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "*%s*\n%s\n\n", pass.title, truncate(formatTonoReport(string(data)), tonoPassMax))
	}
	return strings.TrimSpace(b.String())
}

// tonoLockKey is tono's name for one review target: "owner/repo|n" with every
// character that is not a letter or digit turned into "_". It matches the
// key tono builds when it is given -R.
func tonoLockKey(repo string, number int) string {
	raw := fmt.Sprintf("%s|%d", repo, number)
	return strings.Map(func(r rune) rune {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, raw)
}

// tonoPassMax keeps each pass short enough that the whole DM stays readable.
const tonoPassMax = 3000

var jsonFence = regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```")

// formatTonoReport turns each fenced JSON list of findings into Slack bullets
// and keeps the prose around them.
func formatTonoReport(report string) string {
	out := jsonFence.ReplaceAllStringFunc(report, func(block string) string {
		m := jsonFence.FindStringSubmatch(block)
		var findings []struct {
			File    string `json:"file"`
			Line    int    `json:"line"`
			Summary string `json:"summary"`
			Verdict string `json:"verdict"`
		}
		if json.Unmarshal([]byte(m[1]), &findings) != nil || len(findings) == 0 {
			return block
		}
		var b strings.Builder
		for _, f := range findings {
			if f.Summary == "" {
				return block
			}
			where := f.File
			if f.Line > 0 {
				where = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			b.WriteString("• ")
			if where != "" {
				b.WriteString("`" + where + "` ")
			}
			if f.Verdict != "" {
				b.WriteString("_" + f.Verdict + "_ ")
			}
			b.WriteString(f.Summary + "\n")
		}
		return strings.TrimRight(b.String(), "\n")
	})
	return strings.TrimSpace(toSlackMarkdown(out))
}

var (
	mdHeading = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	mdBold    = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
)

// toSlackMarkdown turns Markdown headings and **bold** into Slack's *bold*.
func toSlackMarkdown(s string) string {
	s = mdBold.ReplaceAllString(s, "*$1*")
	return mdHeading.ReplaceAllString(s, "*$1*")
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
