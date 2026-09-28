package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Env is what a job needs. send posts the DM, or prints it in test mode.
type Env struct {
	cfg    Config
	paths  Paths
	claude claudeRunner
	send   func(ctx context.Context, msg Message) error
	test   bool   // print instead of DM, book nothing
	state  *State // read-only snapshot taken when the run started
	now    time.Time
	// persist changes the state on disk under a lock. It does nothing in test mode.
	persist func(change func(*State)) error
}

// notify sends a DM of one section.
func (e Env) notify(ctx context.Context, text string) error {
	return e.send(ctx, Message{Sections: []string{text}})
}

// printMessage is how test mode shows a DM: a line where Slack draws a divider.
func printMessage(msg Message) string {
	const divider = "\n────────────────────\n"
	out := strings.Join(msg.Sections, divider)
	for _, reply := range msg.Thread {
		out += "\n\n    ↳ in the thread:\n" + strings.Join(reply, divider)
	}
	return out
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
	// failing source. Without internet the run fails instead, so the
	// scheduler retries it once the internet is back.
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
		if err != nil && !online(ctx) {
			return err
		}
	}

	verdicts := map[string]ticketVerdict{}
	var jiraErr error
	if len(prs) > 0 {
		verdicts, jiraErr = judgeTickets(ctx, env, prs)
		if jiraErr != nil && !online(ctx) {
			return jiraErr
		}
	}

	sections := []string{fmt.Sprintf("*Stand-up, %s*", now.Format("Monday 2 January"))}

	var b strings.Builder
	b.WriteString("*Today*\n")
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
	sections = append(sections, b.String())

	var ready, blocked []approvedPR
	for _, p := range prs {
		if len(p.blockers) == 0 {
			ready = append(ready, p)
		} else {
			blocked = append(blocked, p)
		}
	}

	b.Reset()
	b.WriteString("*Ready to merge*\n")
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
	sections = append(sections, b.String())

	if len(blocked) > 0 {
		b.Reset()
		b.WriteString("*Approved, not ready*\n")
		for _, p := range blocked {
			fmt.Fprintf(&b, "• %s %s: %s\n", slackLink(p.pr.URL, p.pr.short()), p.pr.Title, strings.Join(p.blockers, ", "))
		}
		sections = append(sections, b.String())
	}

	b.Reset()
	b.WriteString("*Follow-up steps*\n")
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
	sections = append(sections, b.String())

	review, thread, posted := reviewSection(ctx, env, queue, queueErr)
	sections = append(sections, review)

	if err := env.send(ctx, Message{Sections: sections, Thread: thread}); err != nil {
		return err
	}
	if len(posted) > 0 {
		// Each review goes in one stand-up thread only, once per commit.
		return env.persist(func(st *State) {
			for _, k := range posted {
				if r, ok := st.TonoResults[k]; ok {
					r.Posted = true
					st.TonoResults[k] = r
				}
			}
		})
	}
	return nil
}

// reviewSection is the stand-up's "Needs your review" section: a summary of
// tono's reviews, one line per PR, and the reviews not yet posted, for the
// thread. It returns the keys of the reviews it puts in the thread.
func reviewSection(ctx context.Context, env Env, queue []SearchPR, queueErr error) (string, [][]string, []string) {
	var b strings.Builder
	b.WriteString("*Needs your review*\n")
	switch {
	case queueErr != nil:
		fmt.Fprintf(&b, "• Could not load review requests: %s\n", truncate(queueErr.Error(), 200))
		return b.String(), nil, nil
	case len(queue) == 0:
		b.WriteString("• None\n")
		return b.String(), nil, nil
	}

	counts := map[string]int{}
	var lines []string
	var thread [][]string
	var posted []string
	for _, p := range queue {
		line := fmt.Sprintf("• %s %s (%s)", slackLink(p.URL, p.short()), p.Title, p.Author.Login)
		status := "not reviewed yet"
		if d, err := prDetail(ctx, p.URL); err == nil {
			key := tonoKey(p.repo(), p.Number, d.HeadRefOid)
			if r, ok := env.state.TonoResults[key]; ok {
				switch {
				case r.Failed != "":
					status = "tono could not review it"
					counts["failed"]++
				default:
					v := overallVerdict(r.Verdicts)
					status = strings.ToLower(verdictLabel(v))
					counts[v]++
					counts["reviewed"]++
					if !r.Posted && r.Report != "" {
						if report := loadReport(r.Report); len(report) > 0 {
							thread = append(thread, report)
							posted = append(posted, key)
						}
					}
				}
			} else if olderReview(env.state, p) {
				status = "reviewed at an older commit, new review pending"
			}
		}
		lines = append(lines, line+": "+status)
	}
	notReviewed := len(queue) - counts["reviewed"] - counts["failed"]

	fmt.Fprintf(&b, "%d PRs wait for your review. Tono has reviewed %d at their latest commit (code, docs and comments)", len(queue), counts["reviewed"])
	if counts["reviewed"] > 0 {
		fmt.Fprintf(&b, ": %d ready to approve, %d with follow-ups, %d not mergeable", counts[verdictReady], counts[verdictFollowUps], counts[verdictNotMergeable])
		if n := counts[""]; n > 0 {
			fmt.Fprintf(&b, ", %d without a verdict", n)
		}
	}
	b.WriteString(".")
	if notReviewed > 0 {
		fmt.Fprintf(&b, " %d not reviewed yet.", notReviewed)
	}
	if counts["failed"] > 0 {
		fmt.Fprintf(&b, " %d could not be reviewed.", counts["failed"])
	}
	if len(thread) > 0 {
		b.WriteString(" New reviews are in the thread.")
	}
	b.WriteString("\n\n" + strings.Join(lines, "\n"))
	return b.String(), thread, posted
}

// olderReview is true if tono reviewed an earlier commit of the PR.
func olderReview(st *State, p SearchPR) bool {
	for _, r := range st.TonoResults {
		if r.Repo == p.repo() && r.Number == p.Number {
			return true
		}
	}
	return false
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
// of your conversation list and denies the posting commands. It is a guard on
// top of never passing tono's -c flag (post the findings to the PR). The
// read-only gh and git in readOnlyShims are the stronger guard.
const tonoWrapper = `#!/bin/bash
exec claude "$@" --no-session-persistence \
  "--disallowedTools=Bash(gh pr comment:*),Bash(gh pr review:*),Bash(gh pr merge:*),Bash(gh pr edit:*),Bash(gh pr close:*),Bash(git push:*)"
`

// readOnlyGH stands in for gh on tono's PATH. It refuses any command that
// changes GitHub, including gh api calls that send data, and passes reads to
// the real gh. The deny list alone matches only command prefixes, so it
// cannot catch flags such as gh api -X PATCH.
const readOnlyGH = `#!/bin/bash
# koko-worker: read-only gh for tono runs.
refuse() { echo "koko-worker: gh $* is refused in tono runs (read-only)" >&2; exit 1; }
case "$2" in
  create|delete|edit|close|merge|comment|review|ready|reopen|rerun|cancel|lock|unlock|transfer|rename|archive|unarchive|fork|upload|set|remove|add|pin|unpin|develop|enable|disable|run|sync)
    refuse "$@" ;;
esac
case "$1" in
  auth) [ "$2" = status ] || refuse "$@" ;;
  secret|variable|ssh-key|gpg-key|config|extension|alias|codespace|gist|project) refuse "$@" ;;
  api)
    for a in "$@"; do
      case "$a" in -X|--method|-X*|--method=*|-f|-F|-f*|-F*|--field|--field=*|--raw-field|--raw-field=*|--input|--input=*) refuse "$@" ;; esac
    done ;;
esac
exec %q "$@"
`

// readOnlyGit stands in for git on tono's PATH. It refuses push, wherever the
// subcommand sits (git -C . push), and passes everything else to the real git.
const readOnlyGit = `#!/bin/bash
# koko-worker: git without push for tono runs.
args=("$@"); i=0
while [ $i -lt ${#args[@]} ]; do
  case "${args[$i]}" in
    -C|-c|--git-dir|--work-tree|--namespace) i=$((i+2)) ;;
    -*) i=$((i+1)) ;;
    *) break ;;
  esac
done
case "${args[$i]}" in push|send-pack) echo "koko-worker: git push is refused in tono runs" >&2; exit 1 ;; esac
exec %q "$@"
`

// readOnlyShims writes the gh and git stand-ins into dir and returns dir,
// for the front of tono's PATH.
func readOnlyShims(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	for name, body := range map[string]string{"gh": readOnlyGH, "git": readOnlyGit} {
		real, err := exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("%s not found on PATH", name)
		}
		if filepath.Dir(real) == dir {
			return "", fmt.Errorf("%s resolves to the shim itself", name)
		}
		script := strings.Replace(body, "%q", shellQuote(real), 1)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// tonoTarget is one PR for tono to review.
type tonoTarget struct {
	pr   SearchPR
	mine bool
}

// tonoTargets is your open PRs, then the PRs waiting for your review (people,
// not bots) unless TonoOwnPRsOnly is set.
func tonoTargets(ctx context.Context, env Env) ([]tonoTarget, error) {
	mine, err := myOpenPRs(ctx)
	if err != nil {
		return nil, err
	}
	var out []tonoTarget
	seen := map[string]bool{}
	for _, p := range mine {
		out = append(out, tonoTarget{pr: p, mine: true})
		seen[p.URL] = true
	}
	if env.cfg.TonoOwnPRsOnly {
		return out, nil
	}
	requests, err := reviewRequests(ctx)
	if err != nil {
		return nil, err
	}
	me, err := myLogin(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range reviewQueue(requests, me) {
		if !seen[p.URL] {
			out = append(out, tonoTarget{pr: p})
			seen[p.URL] = true
		}
	}
	return out, nil
}

func tonoKey(repo string, number int, sha string) string {
	return fmt.Sprintf("%s#%d@%s", repo, number, sha)
}

func runTono(ctx context.Context, env Env, onlyURL string) error {
	if _, err := os.Stat(env.cfg.TonoPath); err != nil {
		return fmt.Errorf("tono: not found at %s", env.cfg.TonoPath)
	}
	if err := os.WriteFile(env.paths.TonoWrap, []byte(tonoWrapper), 0o700); err != nil {
		return err
	}
	targets, err := tonoTargets(ctx, env)
	if err != nil {
		return err
	}
	if onlyURL == "" && env.state.TonoBaseline.IsZero() && !env.test {
		// Your own backlog is marked as seen. The review queue is still reviewed.
		if err := tonoBaseline(ctx, env, targets); err != nil {
			return err
		}
		env.state = ptr(loadState(env.paths.State))
	}
	var failures []string
	var netErr error
	reviewed := 0
	for _, t := range targets {
		p := t.pr
		if onlyURL != "" && p.URL != onlyURL {
			continue
		}
		// A test run reviews one PR, so the Test button stays quick.
		if env.test && onlyURL == "" && reviewed > 0 {
			break
		}
		d, err := prDetail(ctx, p.URL)
		if err != nil {
			return err
		}
		key := tonoKey(p.repo(), p.Number, d.HeadRefOid)
		if _, done := env.state.TonoReviewed[key]; done && onlyURL == "" {
			continue
		}
		sections, verdicts, err := tonoReview(ctx, env, p, d.HeadRefOid)
		link := slackLink(p.URL, p.short())
		var skipped tonoSkippedError
		switch {
		case errors.As(err, &skipped):
			// tono had nothing to review: another run holds the PR, or it just
			// became closed or a draft. Leave it unmarked. A closed or draft PR
			// drops out of the next search anyway.
			log.Printf("tono: %s skipped: %v", p.short(), err)
			continue
		case err != nil && !online(ctx):
			// Not marked, so the scheduler's retry reviews it once the internet is back.
			netErr = err
			continue
		}
		reviewed++

		result := TonoResult{
			URL: p.URL, Title: p.Title, Repo: p.repo(), Number: p.Number, SHA: d.HeadRefOid,
			Mine: t.mine, At: time.Now(), Verdicts: verdicts,
		}
		header := fmt.Sprintf("*Tono: %s* %s", link, p.Title)
		if v := overallVerdict(verdicts); v != "" {
			header += "\n" + verdictLabel(v)
		}
		switch {
		case err != nil:
			result.Failed = truncate(err.Error(), 400)
			log.Printf("tono: %s failed: %v", p.short(), err)
			// Your own PRs get a warning now. A failed review of someone
			// else's PR shows in the stand-up instead.
			if t.mine {
				failures = append(failures, fmt.Sprintf("%s: %v", p.short(), err))
				if nerr := env.notify(ctx, fmt.Sprintf(":warning: Tono could not review %s: %s", link, result.Failed)); nerr != nil {
					return nerr
				}
			}
		default:
			report := append([]string{header}, sections...)
			if path, serr := saveReport(env.paths.Reviews, key, report); serr == nil {
				result.Report = path
			} else {
				log.Printf("tono: saving the report: %v", serr)
			}
			// Your own PRs get their review now. Reviews of others' PRs go in
			// the stand-up's thread, except in a test run, which prints them.
			if t.mine || env.test {
				if nerr := env.send(ctx, Message{Sections: report}); nerr != nil {
					return nerr
				}
			}
		}
		// A review that failed for a reason other than the network is marked
		// too, so it is not re-reported at every tono slot.
		if err := env.persist(func(st *State) {
			st.TonoReviewed[key] = time.Now()
			st.TonoResults[key] = result
		}); err != nil {
			return err
		}
	}
	if netErr != nil {
		return netErr
	}
	if len(failures) > 0 {
		return reportedError{fmt.Errorf("tono: %s", strings.Join(failures, "; "))}
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

// saveReport writes a review's sections to dir, for the stand-up thread.
func saveReport(dir, key string, sections []string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := strings.Map(func(r rune) rune {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return r
		}
		return '_'
	}, key) + ".json"
	path := filepath.Join(dir, name)
	data, _ := json.Marshal(sections)
	return path, os.WriteFile(path, data, 0o600)
}

func loadReport(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var sections []string
	_ = json.Unmarshal(data, &sections)
	return sections
}

// Tono verdicts, from the verdict line of each pass's draft PR comment.
const (
	verdictReady        = "mergeable"
	verdictFollowUps    = "follow-ups"
	verdictNotMergeable = "not mergeable"
)

var (
	verdictQuote = regexp.MustCompile(`(?im)^>\s*\*\*\s*(not mergeable|mergeable with follow-ups|mergeable)\b`)
	verdictBold  = regexp.MustCompile(`(?i)\*\*\s*(not mergeable|mergeable with follow-ups|mergeable)\b`)
)

// parseVerdict reads the verdict from a pass's verified output. It prefers
// the quoted verdict line of the draft PR comment, then any bold verdict.
func parseVerdict(text string) string {
	m := verdictQuote.FindStringSubmatch(text)
	if m == nil {
		m = verdictBold.FindStringSubmatch(text)
	}
	if m == nil {
		return ""
	}
	switch strings.ToLower(m[1]) {
	case "not mergeable":
		return verdictNotMergeable
	case "mergeable with follow-ups":
		return verdictFollowUps
	default:
		return verdictReady
	}
}

// overallVerdict is the worst pass. "Ready" needs a clean code review, and
// no pass that says not mergeable.
func overallVerdict(verdicts map[string]string) string {
	if len(verdicts) == 0 {
		return ""
	}
	for _, v := range verdicts {
		if v == verdictNotMergeable {
			return verdictNotMergeable
		}
	}
	if verdicts["review"] == verdictReady {
		return verdictReady
	}
	return verdictFollowUps
}

func verdictLabel(v string) string {
	switch v {
	case verdictReady:
		return "Ready to approve"
	case verdictFollowUps:
		return "Mergeable with follow-ups"
	case verdictNotMergeable:
		return "Not mergeable"
	}
	return "Reviewed, no verdict found"
}

// groupCommand starts name in its own process group. When ctx ends, the whole
// group gets SIGTERM, so tono's trap runs and Claude's children stop too.
// Anything in the group still running a minute later gets SIGKILL.
func groupCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		time.AfterFunc(time.Minute, func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) })
		return syscall.Kill(-pgid, syscall.SIGTERM)
	}
	cmd.WaitDelay = time.Minute + 10*time.Second
	return cmd
}

// tonoSkippedError means tono exited with tonoSkipExit.
type tonoSkippedError struct{ error }

// tonoSkipExit is tono's "nothing to review" exit code: another run holds the
// lock, or the PR is closed or a draft.
const tonoSkipExit = 2

// tonoTimeout caps one review. On timeout, tono's whole process group gets
// SIGTERM, so tono's trap removes its lock. A plain kill would leave the lock
// behind and block every later review of that PR.
const tonoTimeout = 45 * time.Minute

// tonoBaseline runs the first time tono is switched on. It marks your own open
// PRs' current commits as seen, so only your new PRs and new commits get
// reviewed. The PRs waiting for your review are not marked: they all get a
// review.
func tonoBaseline(ctx context.Context, env Env, targets []tonoTarget) error {
	keys := map[string]bool{}
	for _, t := range targets {
		if !t.mine {
			continue
		}
		d, err := prDetail(ctx, t.pr.URL)
		if err != nil {
			return err
		}
		keys[tonoKey(t.pr.repo(), t.pr.Number, d.HeadRefOid)] = true
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
	return env.notify(ctx, fmt.Sprintf("Tono is on. I marked your %d open PRs as seen, so only your new PRs and new commits get a review. PRs waiting for your review all get one.", len(keys)))
}

// reportedError is a failure the job has already sent you, so the scheduler
// logs it without a second DM.
type reportedError struct{ error }

// tonoReview runs the tono CLI at the PR head in a cache clone. It never
// touches your working clones. One review per cache clone at a time, so a
// "Run now" cannot check out another PR under a review in progress.
func tonoReview(ctx context.Context, env Env, p SearchPR, headSHA string) ([]string, map[string]string, error) {
	dir := filepath.Join(env.paths.Cache, strings.ReplaceAll(p.repo(), "/", "__"))
	if err := os.MkdirAll(env.paths.Cache, 0o700); err != nil {
		return nil, nil, err
	}
	unlock, err := lockFile(dir + ".lock")
	if err != nil {
		return nil, nil, err
	}
	defer unlock()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		cloneCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		out, err := exec.CommandContext(cloneCtx, "gh", "repo", "clone", p.repo(), dir, "--", "--filter=blob:none", "--quiet").CombinedOutput()
		cancel()
		if err != nil {
			_ = os.RemoveAll(dir) // a half-made clone would break every later review
			return nil, nil, fmt.Errorf("clone %s: %v: %s", p.repo(), err, truncate(strings.TrimSpace(string(out)), 300))
		}
	}
	for _, args := range [][]string{
		{"-C", dir, "fetch", "--quiet", "origin", fmt.Sprintf("pull/%d/head", p.Number)},
		{"-C", dir, "checkout", "--quiet", "--force", "--detach", headSHA},
	} {
		if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
			return nil, nil, fmt.Errorf("git %s: %s", args[2], truncate(strings.TrimSpace(string(out)), 300))
		}
	}

	ctx, cancel := context.WithTimeout(ctx, tonoTimeout)
	defer cancel()
	cmd := groupCommand(ctx, env.cfg.TonoPath, fmt.Sprint(p.Number), "--all", "-l", "high", "-R", p.repo())
	cmd.Dir = dir
	shims, err := readOnlyShims(filepath.Join(env.paths.Dir, "tono-bin"))
	if err != nil {
		return nil, nil, err
	}
	cmd.Env = append(os.Environ(), "TONO_CLAUDE="+env.paths.TonoWrap, "PATH="+shims+":"+os.Getenv("PATH"))
	// stdout is the report. stderr has tono's progress lines, which go only to the log.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	err = cmd.Run()

	logPath := filepath.Join(env.paths.Logs, fmt.Sprintf("tono-%s-%d-%s.log", shortRepo(p.repo()), p.Number, headSHA[:min(8, len(headSHA))]))
	_ = os.MkdirAll(env.paths.Logs, 0o700)
	_ = os.WriteFile(logPath, append(stderr.Bytes(), stdout.Bytes()...), 0o600)
	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, fmt.Errorf("timed out after %s", tonoTimeout)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == tonoSkipExit {
		return nil, nil, tonoSkippedError{fmt.Errorf("%s", truncate(lastLines(stderr.String(), 1), 200))}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%v: %s", err, truncate(lastLines(stderr.String()+stdout.String(), 5), 400))
	}
	sections, verdicts := tonoVerified(started, p.repo(), p.Number)
	if len(sections) == 0 {
		sections = []string{formatTonoReport(stdout.String())}
	}
	sections = append(sections, "_Full log: `"+logPath+"`_")
	return sections, verdicts, nil
}

var tonoPasses = []struct{ name, title string }{
	{"review", "Code review"},
	{"docs", "Docs and ticket"},
	{"comments", "Code comments"},
}

// tonoVerified returns one section per pass from each pass's verified
// output, and each pass's verdict. tono's stdout also holds the raw rounds,
// whose findings the verify step may drop.
func tonoVerified(since time.Time, repo string, number int) ([]string, map[string]string) {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	var sections []string
	verdicts := map[string]string{}
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
		if v := parseVerdict(string(data)); v != "" {
			verdicts[pass.name] = v
		}
		sections = append(sections, fmt.Sprintf("*%s*\n%s", pass.title, truncate(formatTonoReport(string(data)), tonoPassMax)))
	}
	return sections, verdicts
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
