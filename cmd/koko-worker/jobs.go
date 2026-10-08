package main

import (
	"bytes"
	"context"
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
	// post adds a comment to a PR and returns the comment's URL.
	post func(ctx context.Context, prURL, body string) (string, error)
	// findComment looks on a PR for a comment whose first line is line.
	findComment func(ctx context.Context, repo string, number int, line string) (url string, found bool, err error)
	// prHead returns a PR's state (OPEN, MERGED or CLOSED) and head commit.
	prHead func(ctx context.Context, prURL string) (state, sha string, err error)
	// online is true if the internet answers.
	online func(ctx context.Context) bool
	test   bool   // print instead of DM or comment, book nothing
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
	return strings.Join(msg.Sections, divider)
}

// ---- Stand-up ----

var jiraKey = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)

type approvedPR struct {
	pr       SearchPR
	detail   PRDetail
	blockers []string
}

// loadDetails reads each PR's detail, trying a failed one once more. A PR
// that still fails is returned in failed with its reason, and the others go
// on, so one slow PR does not hide the rest.
func loadDetails(ctx context.Context, prs []SearchPR) (ok []SearchPR, details []PRDetail, failed []string) {
	for _, p := range prs {
		d, err := prDetail(ctx, p.URL)
		if err != nil {
			d, err = prDetail(ctx, p.URL)
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: could not load (%s)", slackLink(p.URL, p.short()), truncate(err.Error(), 150)))
			continue
		}
		ok, details = append(ok, p), append(details, d)
	}
	return ok, details, failed
}

func runStandup(ctx context.Context, env Env) error {
	loc := env.cfg.location()
	now := env.now.In(loc)

	events, calErr := listEvents(ctx, env.claude, env.cfg, now)

	// Each section degrades on its own, so a stand-up is never lost to one
	// failing source. Without internet the run fails instead, so the
	// scheduler retries it once the internet is back.
	var prs []approvedPR
	var judged []judgedPR
	approved, prErr := myApprovedPRs(ctx)
	okPRs, details, prFailed := loadDetails(ctx, approved)
	for i, p := range okPRs {
		a := approvedPR{pr: p, detail: details[i], blockers: details[i].blockers()}
		prs = append(prs, a)
		judged = append(judged, judgedPR{pr: p, detail: a.detail, ready: len(a.blockers) == 0})
	}
	// Your PRs merged in the last week, for stories that can be closed now.
	merged, mergedErr := myMergedPRs(ctx, now.AddDate(0, 0, -7))
	okMerged, mergedDetails, _ := loadDetails(ctx, merged)
	for i, p := range okMerged {
		judged = append(judged, judgedPR{pr: p, detail: mergedDetails[i], merged: true})
	}

	var queue, team []SearchPR
	me, meErr := myLogin(ctx)
	requests, queueErr := reviewRequests(ctx)
	if queueErr == nil {
		queue, queueErr = reviewQueue(requests, me), meErr
	}
	var teamErr error
	if env.cfg.Reviewer.Team.Enabled && len(env.cfg.Reviewer.Team.Repos) > 0 {
		var inRepos []SearchPR
		inRepos, teamErr = repoPRs(ctx, env.cfg.Reviewer.Team.Repos)
		if teamErr == nil {
			team, teamErr = reviewQueue(inRepos, me), meErr
		}
	}

	for _, err := range []error{calErr, prErr, queueErr, teamErr} {
		if err != nil && !online(ctx) {
			return err
		}
	}

	verdicts := map[string]ticketVerdict{}
	var jiraErr error
	if len(judged) > 0 {
		var keys []string
		seen := map[string]bool{}
		for _, j := range judged {
			if k := ticketKey(j.detail); k != "" && !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
		var tickets map[string]jiraTicket
		tickets, jiraErr = fetchTickets(ctx, env.claude, env.cfg.JiraSite, keys)
		if jiraErr == nil {
			verdicts, jiraErr = judgeTickets(ctx, env, judged, tickets)
		}
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
	case len(ready) == 0 && len(prFailed) == 0:
		b.WriteString("• None\n")
	case jiraErr != nil:
		fmt.Fprintf(&b, "_Could not check Jira: %s_\n", truncate(jiraErr.Error(), 200))
	}
	for _, p := range ready {
		fmt.Fprintf(&b, "• %s %s\n", slackLink(p.pr.URL, p.pr.short()), p.pr.Title)
		if v, ok := verdicts[p.pr.URL]; ok {
			b.WriteString("    " + ticketLine(v) + "\n")
		}
	}
	for _, f := range prFailed {
		b.WriteString("• " + f + "\n")
	}
	sections = append(sections, b.String())

	if s := closableSection(judged, verdicts); s != "" {
		sections = append(sections, s)
	} else if mergedErr != nil {
		sections = append(sections, fmt.Sprintf("*Stories you can close*\n• Could not load your merged PRs: %s\n", truncate(mergedErr.Error(), 200)))
	}

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

	tally := &reviewTally{}
	sections = append(sections, reviewSection(ctx, env, tally, queue, queueErr))
	if s := teamSection(ctx, env, tally, team, teamErr, queue); s != "" {
		sections = append(sections, s)
	}

	return env.send(ctx, Message{Sections: sections})
}

// reviewTally reads tono's results for the stand-up's PR lines. A PR keeps
// its review after new commits, because tono reviews each PR once.
type reviewTally struct {
	scope    tonoScope
	scopeErr error
	loaded   bool
}

func (t *reviewTally) loadScope(ctx context.Context, env Env) {
	if !t.loaded {
		t.scope, t.scopeErr = loadScope(ctx, env.cfg, env.now)
		t.loaded = true
	}
}

// status is tono's status for one PR, counted in counts. A posted review
// links to its first PR comment.
func (t *reviewTally) status(ctx context.Context, env Env, p SearchPR, counts map[string]int) string {
	t.loadScope(ctx, env)
	r, reviewed := latestResult(env.state, p.repo(), p.Number)
	switch {
	case reviewed && r.Failed != "" && len(r.Comments) > 0:
		// Part of the review reached the PR before posting failed.
		counts["failed"]++
		return "the review worker posted part of its review (" + slackLink(r.Comments[0], "review") + ")"
	case reviewed && r.Failed != "":
		counts["failed"]++
		return "the review worker could not review it"
	case reviewed && len(r.Unposted) > 0:
		// Not on GitHub yet, so no verdict to show.
		counts["waiting"]++
		return "reviewed, not posted yet"
	case reviewed:
		v := overallVerdict(r.Verdicts)
		status := strings.ToLower(verdictLabel(v))
		if r.LGTM {
			// An LGTM review can have no verdict line at all, so it counts as ready.
			v, status = verdictReady, "LGTM, ready to approve"
		}
		if d, err := prDetail(ctx, p.URL); err == nil && d.HeadRefOid != r.SHA {
			status += " (reviewed at an earlier commit)"
		}
		counts[v]++
		counts["reviewed"]++
		if len(r.Comments) > 0 {
			status += " (" + slackLink(r.Comments[0], "review") + ")"
		}
		return status
	case t.scopeErr == nil && t.scope.outside(p) != "":
		counts["outside"]++
		return "outside the review scope: " + t.scope.outside(p)
	default:
		counts["waiting"]++
		return "not reviewed yet"
	}
}

// summary is the sentence after a section's count: what the review worker made of the PRs.
func (t *reviewTally) summary(env Env, counts map[string]int) string {
	var b strings.Builder
	fmt.Fprintf(&b, " The review worker has reviewed %d (code, docs and comments)", counts["reviewed"])
	if counts["reviewed"] > 0 {
		fmt.Fprintf(&b, ": %d ready to approve, %d with follow-ups, %d not mergeable", counts[verdictReady], counts[verdictFollowUps], counts[verdictNotMergeable])
		if n := counts[""]; n > 0 {
			fmt.Fprintf(&b, ", %d without a verdict", n)
		}
	}
	b.WriteString(".")
	if counts["waiting"] > 0 {
		fmt.Fprintf(&b, " %d not reviewed yet.", counts["waiting"])
	}
	if counts["failed"] > 0 {
		fmt.Fprintf(&b, " %d could not be reviewed.", counts["failed"])
	}
	if counts["outside"] > 0 {
		// Only team PRs are counted here, so outside the scope means too old.
		if counts["outside"] == 1 {
			fmt.Fprintf(&b, " 1 was opened more than %d days ago, so it is skipped.", env.cfg.Reviewer.Team.MaxAgeDays)
		} else {
			fmt.Fprintf(&b, " %d were opened more than %d days ago, so they are skipped.", counts["outside"], env.cfg.Reviewer.Team.MaxAgeDays)
		}
	}
	if t.scopeErr != nil {
		fmt.Fprintf(&b, " _Could not load the review scope: %s_", truncate(t.scopeErr.Error(), 150))
	}
	return b.String()
}

func prLine(p SearchPR, status string) string {
	return fmt.Sprintf("• %s %s (%s): %s", slackLink(p.URL, p.short()), p.Title, p.Author.Login, status)
}

// reviewSection is the stand-up's "Needs your review" section: a summary of
// tono's reviews, then one line per PR.
func reviewSection(ctx context.Context, env Env, tally *reviewTally, queue []SearchPR, queueErr error) string {
	var b strings.Builder
	b.WriteString("*Needs your review*\n")
	switch {
	case queueErr != nil:
		fmt.Fprintf(&b, "• Could not load review requests: %s\n", truncate(queueErr.Error(), 200))
		return b.String()
	case len(queue) == 0:
		b.WriteString("• None\n")
		return b.String()
	}

	// Only the team's PRs get a line. The rest are counted, so the section
	// stays short. Without the team's members, every PR is listed.
	tally.loadScope(ctx, env)
	team := queue
	if tally.scopeErr == nil {
		team = nil
		for _, p := range queue {
			if tally.scope.members[strings.ToLower(p.Author.Login)] {
				team = append(team, p)
			}
		}
	}
	counts := map[string]int{}
	var lines []string
	for _, p := range team {
		lines = append(lines, prLine(p, tally.status(ctx, env, p, counts)))
	}
	others := len(queue) - len(team)
	if others == 0 {
		fmt.Fprintf(&b, "%d PRs wait for your review.", len(queue))
	} else {
		fmt.Fprintf(&b, "%d PRs wait for your review, %d of them from %s.", len(queue), len(team), env.cfg.Reviewer.Team.Team)
	}
	if len(team) > 0 {
		b.WriteString(tally.summary(env, counts))
	}
	if others > 0 {
		fmt.Fprintf(&b, " %d from outside the team are not listed.", others)
	}
	if len(lines) > 0 {
		b.WriteString("\n\n" + strings.Join(lines, "\n"))
	}
	return b.String()
}

// teamSection is the stand-up's "Team PRs" section: the team's open PRs in
// Reviewer.Team.Repos. It skips PRs outside the review scope, and PRs that ask for your
// review, because "Needs your review" lists those. It is empty if the
// section is switched off or has no repos.
func teamSection(ctx context.Context, env Env, tally *reviewTally, prs []SearchPR, prsErr error, queue []SearchPR) string {
	if !env.cfg.Reviewer.Team.Enabled || len(env.cfg.Reviewer.Team.Repos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("*Team PRs*\n")
	if prsErr != nil {
		fmt.Fprintf(&b, "• Could not load the team's PRs: %s\n", truncate(prsErr.Error(), 200))
		return b.String()
	}
	inQueue := map[string]bool{}
	for _, p := range queue {
		inQueue[p.URL] = true
	}
	tally.loadScope(ctx, env)
	counts := map[string]int{}
	var lines []string
	for _, p := range prs {
		if inQueue[p.URL] || (tally.scopeErr == nil && tally.scope.outside(p) != "") {
			continue
		}
		lines = append(lines, prLine(p, tally.status(ctx, env, p, counts)))
	}
	if len(lines) == 0 {
		fmt.Fprintf(&b, "• None in %s\n", strings.Join(env.cfg.Reviewer.Team.Repos, ", "))
		return b.String()
	}
	fmt.Fprintf(&b, "%d open team PRs in %s.", len(lines), strings.Join(env.cfg.Reviewer.Team.Repos, ", "))
	b.WriteString(tally.summary(env, counts))
	b.WriteString("\n\n" + strings.Join(lines, "\n"))
	return b.String()
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
# koko-worker: read-only gh for review runs.
refuse() { echo "koko-worker: gh $* is refused in review runs (read-only)" >&2; exit 1; }
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
# koko-worker: git without push for review runs.
args=("$@"); i=0
while [ $i -lt ${#args[@]} ]; do
  case "${args[$i]}" in
    -C|-c|--git-dir|--work-tree|--namespace) i=$((i+2)) ;;
    -*) i=$((i+1)) ;;
    *) break ;;
  esac
done
case "${args[$i]}" in push|send-pack) echo "koko-worker: git push is refused in review runs" >&2; exit 1 ;; esac
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

// tonoScope is which team PRs the review worker may review: opened by a member of the team, in
// the last Reviewer.Team.MaxAgeDays days.
type tonoScope struct {
	team    string
	days    int
	members map[string]bool
	since   time.Time
}

func loadScope(ctx context.Context, cfg Config, now time.Time) (tonoScope, error) {
	members, err := teamMembers(ctx, cfg.Reviewer.Team.Team)
	if err != nil {
		return tonoScope{}, err
	}
	return tonoScope{
		team: cfg.Reviewer.Team.Team, days: cfg.Reviewer.Team.MaxAgeDays, members: members,
		since: now.AddDate(0, 0, -cfg.Reviewer.Team.MaxAgeDays),
	}, nil
}

// outside says why a PR is outside the scope, or "" if it is inside.
func (s tonoScope) outside(p SearchPR) string {
	if !s.members[strings.ToLower(p.Author.Login)] {
		return "not opened by " + s.team
	}
	created, err := time.Parse(time.RFC3339, p.CreatedAt)
	if err != nil || created.Before(s.since) {
		return fmt.Sprintf("opened more than %d days ago", s.days)
	}
	return ""
}

// latestResult is tono's most recent result for the PR, at any commit.
func latestResult(st *State, repo string, number int) (TonoResult, bool) {
	var best TonoResult
	found := false
	for _, r := range st.TonoResults {
		if r.Repo == repo && r.Number == number && (!found || r.At.After(best.At)) {
			best, found = r, true
		}
	}
	return best, found
}

// reviewedBefore is true if tono has looked at the PR at any commit, even if
// the review failed. Each PR gets one review.
func reviewedBefore(st *State, repo string, number int) bool {
	prefix := fmt.Sprintf("%s#%d@", repo, number)
	for k := range st.TonoReviewed {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

// tonoTarget is one PR for tono to review.
type tonoTarget struct {
	pr     SearchPR
	mine   bool
	tooOld bool // your PR, opened more than Reviewer.Mine.MaxAgeDays ago
}

// tonoTargets is your open PRs if Reviewer.Mine is on. If Reviewer.Team is on,
// the PRs waiting for your review follow, then the open PRs in Reviewer.Team.Repos.
// Others' PRs are people's, not bots', and runTono checks them against the team scope.
// Your PRs opened more than Reviewer.Mine.MaxAgeDays ago are marked tooOld.
func tonoTargets(ctx context.Context, env Env) ([]tonoTarget, error) {
	var out []tonoTarget
	seen := map[string]bool{}
	if env.cfg.Reviewer.Mine.Enabled {
		mine, err := myOpenPRs(ctx)
		if err != nil {
			return nil, err
		}
		since := env.now.AddDate(0, 0, -env.cfg.Reviewer.Mine.MaxAgeDays)
		for _, p := range mine {
			created, err := time.Parse(time.RFC3339, p.CreatedAt)
			old := err != nil || created.Before(since)
			out = append(out, tonoTarget{pr: p, mine: true, tooOld: old})
			seen[p.URL] = true
		}
	}
	if !env.cfg.Reviewer.Team.Enabled {
		return out, nil
	}
	requests, err := reviewRequests(ctx)
	if err != nil {
		return nil, err
	}
	inRepos, err := repoPRs(ctx, env.cfg.Reviewer.Team.Repos)
	if err != nil {
		return nil, err
	}
	me, err := myLogin(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range append(reviewQueue(requests, me), reviewQueue(inRepos, me)...) {
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
	// Before the PR loop, so a reviewer that cannot run marks nothing reviewed.
	cli, err := prepareReviewer(ctx, env)
	if err != nil {
		return err
	}
	if err := os.WriteFile(env.paths.TonoWrap, []byte(tonoWrapper), 0o700); err != nil {
		return err
	}
	var failures []string
	if !env.test {
		gaveUp, err := postUnposted(ctx, env)
		if err != nil {
			return err
		}
		failures = append(failures, gaveUp...)
	}
	targets, err := tonoTargets(ctx, env)
	if err != nil {
		return err
	}
	var scope tonoScope
	if env.cfg.Reviewer.Team.Enabled {
		if scope, err = loadScope(ctx, env.cfg, env.now); err != nil {
			return err
		}
	}
	var netErr error
	var broken []string // reviews where a tono pass broke, left unmarked
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
		// A PR named with --pr is reviewed whatever the scope or its age says.
		// Your own PRs are outside the team scope, and have their own age limit.
		if onlyURL == "" {
			if (!t.mine && scope.outside(p) != "") || t.tooOld || reviewedBefore(env.state, p.repo(), p.Number) {
				continue
			}
			onGitHub, err := hasTonoComment(ctx, p.repo(), p.Number, env.cfg.Reviewer.MarkerPrefix)
			if err != nil {
				return err
			}
			if onGitHub {
				continue // someone already posted a tono review on the PR
			}
		}
		d, err := prDetail(ctx, p.URL)
		if err != nil {
			return err
		}
		key := tonoKey(p.repo(), p.Number, d.HeadRefOid)
		out, err := tonoReview(ctx, env, cli, p, d.HeadRefOid)
		link := slackLink(p.URL, p.short())
		var skipped tonoSkippedError
		switch {
		case errors.As(err, &skipped):
			// tono had nothing to review: another run holds the PR, or it just
			// became closed or a draft. Leave it unmarked. A closed or draft PR
			// drops out of the next search anyway.
			log.Printf("review: %s skipped: %v", p.short(), err)
			continue
		case err != nil && !env.online(ctx):
			// Not marked, so the scheduler's retry reviews it once the
			// internet is back.
			netErr = err
			continue
		case err == nil && len(out.broken) > 0:
			// A pass that broke, offline or not, says nothing about the PR.
			// It leaves an error line, such as "Not logged in" or "API
			// Error: Can't reach the API server". The PR stays unmarked, so a
			// later run reviews it.
			bad := fmt.Errorf("%s: a review pass broke: %s", p.short(), strings.Join(out.broken, "; "))
			log.Printf("review: %v", bad)
			if !env.online(ctx) {
				netErr = bad
			} else {
				broken = append(broken, bad.Error())
			}
			continue
		}
		reviewed++

		result := TonoResult{
			URL: p.URL, Title: p.Title, Repo: p.repo(), Number: p.Number, SHA: d.HeadRefOid,
			Mine: t.mine, At: time.Now(), Verdicts: out.verdicts,
		}
		var comments []string
		if err != nil {
			result.Failed = truncate(err.Error(), 400)
		} else {
			comments, result.LGTM, result.Failed = commentsFor(out, d.HeadRefOid, env.cfg.Reviewer.MarkerPrefix)
		}
		if result.Failed != "" {
			log.Printf("review: %s failed: %s", p.short(), result.Failed)
			// Your own PRs get a warning now. A failed review of someone
			// else's PR shows in the stand-up instead.
			if t.mine {
				failures = append(failures, fmt.Sprintf("%s: %s", p.short(), result.Failed))
				if nerr := env.notify(ctx, fmt.Sprintf(":warning: The review worker could not review %s: %s", link, result.Failed)); nerr != nil {
					return nerr
				}
			}
		}
		// Each PR gets one review. A review that failed for a reason other
		// than the network is marked too, so it is not re-reported at every
		// tono slot. The comments are saved as unposted first, so a kill
		// during posting leaves them for the next run.
		result.Unposted = comments
		if err := env.persist(func(st *State) {
			st.TonoReviewed[key] = time.Now()
			st.TonoResults[key] = result
		}); err != nil {
			return err
		}
		gaveUp, err := postPending(ctx, env, key, result)
		var offline offlineError
		switch {
		case errors.As(err, &offline):
			netErr = err // the comments stay unposted for the retry
		case err != nil:
			return err
		case gaveUp != "":
			failures = append(failures, fmt.Sprintf("%s: %s", p.short(), gaveUp))
		}
	}
	if netErr != nil {
		return netErr
	}
	if len(broken) > 0 {
		// Not a reportedError: no DM has said so yet, so the scheduler does.
		return fmt.Errorf("review: %s", strings.Join(append(broken, failures...), "; "))
	}
	if len(failures) > 0 {
		return reportedError{fmt.Errorf("review: %s", strings.Join(failures, "; "))}
	}
	return nil
}

// commentsFor picks what to post for a review, and whether it is an LGTM.
// A posted comment makes later runs skip the PR, so a review that may be
// incomplete posts nothing and fails instead.
//
// A FormatResultJSON reviewer's comments are posted as written. Each must
// start with the marker line, and a reviewed PR needs at least one.
//
// For FormatTonoLogs, each pass's draft is posted. When every pass is an
// LGTM, only the first is posted, so a clean PR gets one LGTM, not three. With
// no draft at all, from a tono older than its LGTM drafts, the worker posts
// its own LGTM. It fails when a pass did not finish (a skipped code review
// does not count), when a marker line has no draft it can read, when a draft
// has no footer, or when the verdict is bad but no pass wrote it up.
func commentsFor(out tonoOutcome, sha, prefix string) (comments []string, lgtm bool, failed string) {
	if out.format == FormatResultJSON {
		if len(out.comments) == 0 {
			return nil, false, "the reviewer reviewed the PR but wrote no comment, see " + out.log
		}
		for _, c := range out.comments {
			if !strings.HasPrefix(firstLine(c.Body), "<!-- "+prefix+":") {
				return nil, false, fmt.Sprintf("a %s comment does not start with the marker <!-- %s:, see %s", c.Pass, prefix, out.log)
			}
			comments = append(comments, c.Body)
		}
		return comments, out.lgtmPasses == len(out.comments), ""
	}

	if out.passes < out.expected {
		why := ""
		if len(out.broken) > 0 {
			why = " (" + strings.Join(out.broken, "; ") + ")"
		}
		return nil, false, fmt.Sprintf("only %d of %d passes finished%s, see %s", out.passes, out.expected, why, out.log)
	}
	if out.unreadable > 0 {
		return nil, false, "a pass wrote a PR comment the worker could not read, see " + out.log
	}
	for _, c := range out.comments {
		if !strings.HasSuffix(c.Body, "</sub>") {
			return nil, false, "a draft PR comment has no footer, so it may be cut short, see " + out.log
		}
	}
	if len(out.comments) > 0 {
		if out.lgtmPasses == len(out.comments) && out.passes == len(out.comments) {
			return []string{out.comments[0].Body}, true, ""
		}
		for _, c := range out.comments {
			comments = append(comments, c.Body)
		}
		return comments, false, ""
	}
	// The fallback LGTM needs no verdicts at all, or a ready code review. A
	// docs "mergeable" alone, with no code verdict, is not enough.
	if len(out.verdicts) == 0 || (out.verdicts["review"] == verdictReady && overallVerdict(out.verdicts) == verdictReady) {
		return []string{lgtmComment(prefix, sha)}, true, ""
	}
	return nil, false, fmt.Sprintf("the reviewer wrote no PR comment (verdict %q), see %s", overallVerdict(out.verdicts), out.log)
}

// maxPostTries is the failed try that fails the review: the third failed try
// to post its comments.
const maxPostTries = 3

// offlineError is a post that failed with the internet down. It does not
// count as a try, and the comments wait for the scheduler's retry.
type offlineError struct{ error }

// postPending posts r's unposted comments on the PR, in order, and saves the
// state after each one, so a kill between posts loses nothing. A failed post
// stops the loop.
//
// It first checks that the PR is still open at the reviewed commit. A review
// can take 45 minutes, and a retry can come days later. If the PR moved on,
// the comments are dropped, a DM says so, and the PR may be reviewed again.
// It then looks on the PR for each comment's marker line, so a post that
// timed out after it went through, or a --pr run at the same commit, does
// not post the same review twice.
//
// A failed post counts one try, unless the internet is down. The try that
// reaches maxPostTries fails the review, drops the rest, and a DM says so.
// gaveUp then holds the reason.
func postPending(ctx context.Context, env Env, key string, r TonoResult) (gaveUp string, err error) {
	save := func(change func(*State)) error {
		return env.persist(func(st *State) {
			st.TonoResults[key] = r
			if change != nil {
				change(st)
			}
		})
	}
	link := slackLink(r.URL, fmt.Sprintf("%s#%d", shortRepo(r.Repo), r.Number))
	if len(r.Unposted) > 0 {
		state, head, err := env.prHead(ctx, r.URL)
		if err == nil && (state != "OPEN" || (head != "" && head != r.SHA)) {
			r.Failed = fmt.Sprintf("the PR changed before the review was posted (%s at %.7s)", strings.ToLower(state), head)
			r.Unposted = nil
			log.Printf("review: %s#%d: %s", r.Repo, r.Number, r.Failed)
			// The review is of an old commit or a closed PR. Forget it, so
			// an open PR gets a fresh review at its new head.
			prefix := fmt.Sprintf("%s#%d@", r.Repo, r.Number)
			if err := save(func(st *State) {
				for k := range st.TonoReviewed {
					if strings.HasPrefix(k, prefix) {
						delete(st.TonoReviewed, k)
					}
				}
			}); err != nil {
				return "", err
			}
			if len(r.Comments) > 0 {
				// Part of the review is already on the PR, and its marker
				// blocks a fresh review. Only a "Run now" with the URL gets one.
				return "", env.notify(ctx, fmt.Sprintf(":warning: The review worker posted part of its review of %s, then the PR changed (%s). To review it again, use Run now with its URL.", link, r.Failed))
			}
			return "", nil
		}
	}
	for len(r.Unposted) > 0 {
		body := r.Unposted[0]
		url, found, err := env.findComment(ctx, r.Repo, r.Number, firstLine(body))
		if err == nil && !found {
			url, err = env.post(ctx, r.URL, body)
		}
		if err != nil {
			if !env.online(ctx) {
				return "", offlineError{err}
			}
			r.PostTries++
			log.Printf("review: posting on %s#%d (try %d): %v", r.Repo, r.Number, r.PostTries, err)
			permanent := refusedForGood(err)
			if r.PostTries < maxPostTries && !permanent {
				return "", save(nil)
			}
			r.Failed = truncate("could not post the review: "+err.Error(), 400)
			r.Unposted = nil
			if err := save(nil); err != nil {
				return "", err
			}
			why := fmt.Sprintf("after %d tries", r.PostTries)
			if permanent {
				why = "because GitHub refuses comments on it"
			}
			if nerr := env.notify(ctx, fmt.Sprintf(":warning: The review worker could not post its review of %s %s: %s", link, why, r.Failed)); nerr != nil {
				return "", nerr
			}
			return r.Failed, nil
		}
		if url != "" {
			r.Comments = append(r.Comments, url)
		}
		r.Unposted = r.Unposted[1:]
		if err := save(nil); err != nil {
			return "", err
		}
	}
	// Once the whole review is on the PR, one Slack line says so. The review
	// itself never goes to Slack.
	if len(r.Comments) > 0 && r.Failed == "" && !r.Pinged {
		if err := env.notify(ctx, reviewPing(r)); err != nil {
			log.Printf("review: Slack line for %s#%d: %v", r.Repo, r.Number, err)
			return "", nil // the review is posted, so this is not worth a retry
		}
		r.Pinged = true
		return "", save(nil)
	}
	return "", nil
}

// refusedForGood is true for a post GitHub will never accept, such as on an
// archived repo or a locked PR. Retrying those only wastes a day of tries.
func refusedForGood(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"was archived", "is locked", "read-only"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// reviewPing is the one Slack line for a posted review: the PR, the verdict
// and a link to the review.
func reviewPing(r TonoResult) string {
	verdict := strings.ToLower(verdictLabel(overallVerdict(r.Verdicts)))
	if r.LGTM {
		verdict = "LGTM"
	}
	pr := slackLink(r.URL, fmt.Sprintf("%s#%d", shortRepo(r.Repo), r.Number))
	return fmt.Sprintf("Review worker reviewed %s %s: %s (%s)", pr, r.Title, verdict, slackLink(r.Comments[0], "review"))
}

// postUnposted posts the comments earlier runs left unposted. It returns the
// reviews it gave up on.
func postUnposted(ctx context.Context, env Env) ([]string, error) {
	var failures []string
	for key, r := range env.state.TonoResults {
		if len(r.Unposted) == 0 {
			continue
		}
		gaveUp, err := postPending(ctx, env, key, r)
		if err != nil {
			return failures, err
		}
		if gaveUp != "" {
			failures = append(failures, fmt.Sprintf("%s#%d: %s", shortRepo(r.Repo), r.Number, gaveUp))
		}
	}
	return failures, nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
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
// the quoted verdict line of the draft PR comment, then any bold verdict. A
// clean review's "> **LGTM!**" counts as ready to approve.
func parseVerdict(text string) string {
	if lgtmLine.MatchString(text) {
		return verdictReady
	}
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
// no pass that says not mergeable. A reviewer whose passes have other names
// is ready when every pass is.
func overallVerdict(verdicts map[string]string) string {
	if len(verdicts) == 0 {
		return ""
	}
	allReady := true
	for _, v := range verdicts {
		if v == verdictNotMergeable {
			return verdictNotMergeable
		}
		allReady = allReady && v == verdictReady
	}
	if review, ok := verdicts["review"]; ok {
		if review == verdictReady {
			return verdictReady
		}
		return verdictFollowUps
	}
	if allReady {
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

// reportedError is a failure the job has already sent you, so the scheduler
// logs it without a second DM.
type reportedError struct{ error }

// tonoReview runs the reviewer CLI, cli, at the PR head in a cache clone. It never
// touches your working clones. One review per cache clone at a time, so a
// "Run now" cannot check out another PR under a review in progress.
func tonoReview(ctx context.Context, env Env, cli string, p SearchPR, headSHA string) (tonoOutcome, error) {
	dir := filepath.Join(env.paths.Cache, strings.ReplaceAll(p.repo(), "/", "__"))
	if err := os.MkdirAll(env.paths.Cache, 0o700); err != nil {
		return tonoOutcome{}, err
	}
	unlock, err := lockFile(dir + ".lock")
	if err != nil {
		return tonoOutcome{}, err
	}
	defer unlock()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		cloneCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		out, err := exec.CommandContext(cloneCtx, "gh", "repo", "clone", p.repo(), dir, "--", "--filter=blob:none", "--quiet").CombinedOutput()
		cancel()
		if err != nil {
			_ = os.RemoveAll(dir) // a half-made clone would break every later review
			return tonoOutcome{}, fmt.Errorf("clone %s: %v: %s", p.repo(), err, truncate(strings.TrimSpace(string(out)), 300))
		}
	}
	for _, args := range [][]string{
		{"-C", dir, "fetch", "--quiet", "origin", fmt.Sprintf("pull/%d/head", p.Number)},
		{"-C", dir, "checkout", "--quiet", "--force", "--detach", headSHA},
	} {
		if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
			return tonoOutcome{}, fmt.Errorf("git %s: %s", args[2], truncate(strings.TrimSpace(string(out)), 300))
		}
	}

	rv := env.cfg.Reviewer
	logPath := filepath.Join(env.paths.Logs, fmt.Sprintf("tono-%s-%d-%s.log", shortRepo(p.repo()), p.Number, headSHA[:min(8, len(headSHA))]))
	resultPath := strings.TrimSuffix(logPath, ".log") + ".result.json"
	_ = os.MkdirAll(env.paths.Logs, 0o700)
	_ = os.Remove(resultPath) // a result from an earlier run must not count

	ctx, cancel := context.WithTimeout(ctx, tonoTimeout)
	defer cancel()
	cmd := groupCommand(ctx, cli, expandArgs(rv.Args, p, headSHA)...)
	cmd.Dir = dir
	shims, err := readOnlyShims(filepath.Join(env.paths.Dir, "tono-bin"))
	if err != nil {
		return tonoOutcome{}, err
	}
	// The reviewer contract's environment (plan 030). TONO_CLAUDE is tono's
	// name for REVIEW_CLAUDE.
	cmd.Env = append(os.Environ(),
		"REVIEW_RESULT="+resultPath, "REVIEW_PR="+fmt.Sprint(p.Number), "REVIEW_REPO="+p.repo(),
		"REVIEW_PR_URL="+p.URL, "REVIEW_SHA="+headSHA,
		"REVIEW_CLAUDE="+env.paths.TonoWrap, "TONO_CLAUDE="+env.paths.TonoWrap,
		"PATH="+shims+":"+os.Getenv("PATH"))
	// stdout and stderr go only to the log. The review comes from the result
	// file, or from tono's verified logs, read by tonoVerified.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	err = cmd.Run()

	_ = os.WriteFile(logPath, append(stderr.Bytes(), stdout.Bytes()...), 0o600)
	if ctx.Err() == context.DeadlineExceeded {
		return tonoOutcome{}, fmt.Errorf("timed out after %s", tonoTimeout)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == tonoSkipExit {
		return tonoOutcome{}, tonoSkippedError{fmt.Errorf("%s", truncate(lastLines(stderr.String(), 1), 200))}
	}
	if err != nil {
		return tonoOutcome{}, fmt.Errorf("%v: %s", err, truncate(lastLines(stderr.String()+stdout.String(), 5), 400))
	}

	if rv.Format == FormatResultJSON {
		out, err := readResult(resultPath)
		out.log = logPath
		return out, err
	}
	out := tonoVerified(started, p.repo(), p.Number, rv.MarkerPrefix)
	out.log = logPath
	if !out.ran["review"] && out.passes < out.expected {
		// tono skips the code review when its LGTM is already on the head
		// commit. Then two passes are the whole review, not a broken one.
		if lgtm, err := lgtmOnHead(ctx, p.repo(), p.Number, rv.MarkerPrefix, headSHA); err == nil && lgtm {
			out.expected--
		}
	}
	return out, nil
}

// tonoOutcome is what a review left: the comments to post, and each pass's
// verdict. A FormatResultJSON reviewer fills in comments and verdicts. For
// FormatTonoLogs they come from tono's verified logs, one draft per pass that
// wrote one, and the other fields say how complete the review is.
type tonoOutcome struct {
	format     string
	comments   []reviewComment
	verdicts   map[string]string
	lgtmPasses int // comments that are an LGTM
	// expected is how many passes should have run: three, less a code review
	// tono skipped because its LGTM is already on the head commit.
	expected int
	passes   int             // passes whose verified output reads as a review
	ran      map[string]bool // those passes, by name
	// unreadable counts passes with a tono marker line but no draft that
	// draftComment could read.
	unreadable int
	// broken holds the first line of each pass whose verified output is not
	// a review, such as "API Error: Can't reach the API server".
	broken []string
	log    string // the worker's log of the run
}

var tonoPasses = []struct{ name string }{{"review"}, {"docs"}, {"comments"}}

// looksLikeReview is true when a pass's verified output reads as a review:
// at least minReviewLines lines with text, and a Markdown heading or list. A
// clean pass can write no verdict and no draft, so this is what tells it apart
// from a run that broke, which leaves one error line or nothing.
func looksLikeReview(verified string) bool {
	lines, structured := 0, false
	for _, line := range strings.Split(verified, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		lines++
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			structured = true
		}
	}
	return lines >= minReviewLines && structured
}

const minReviewLines = 3

// tonoVerified reads each pass's verified output from this run: its draft PR
// comment, if it wrote one, and its verdict. tono's stdout also holds the raw
// rounds, whose findings the verify step may drop.
func tonoVerified(since time.Time, repo string, number int, prefix string) tonoOutcome {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	out := tonoOutcome{format: FormatTonoLogs, verdicts: map[string]string{}, expected: len(tonoPasses), ran: map[string]bool{}}
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
		if !looksLikeReview(string(data)) {
			out.broken = append(out.broken, pass.name+": "+truncate(firstLine(strings.TrimSpace(string(data))), 120))
			continue
		}
		out.passes++
		out.ran[pass.name] = true
		if v := parseVerdict(string(data)); v != "" {
			out.verdicts[pass.name] = v
		}
		if d := draftComment(string(data), prefix); d != "" {
			c := reviewComment{Pass: pass.name, Body: d}
			if isLGTMDraft(d) {
				c.Verdict = contractLGTM
				out.lgtmPasses++
			}
			out.comments = append(out.comments, c)
		} else if hasMarkerLine(string(data), prefix) {
			out.unreadable++
		}
	}
	return out
}

// codeFence reads a Markdown fence line: ``` or ~~~, three or more, with any
// indent. info is true when a language or other text follows the fence.
func codeFence(line string) (indent int, fence string, info, ok bool) {
	trimmed := strings.TrimLeft(line, " \t")
	indent = len(line) - len(trimmed)
	if len(trimmed) < 3 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, "", false, false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == trimmed[0] {
		n++
	}
	if n < 3 {
		return 0, "", false, false
	}
	return indent, trimmed[:n], strings.TrimSpace(trimmed[n:]) != "", true
}

// draftComment is the pass's draft PR comment: the fenced block that starts
// with the marker "<!-- prefix:". The heading above the block varies from run
// to run, so the marker is what finds it. "" means the pass wrote no comment.
//
// A fence inside the draft that names a language (```go) opens an inner
// block, and the next bare fence closes it. A bare inner fence at least as
// long as the outer one closes the draft early, and the missing footer then
// fails the review in commentsFor.
func draftComment(verified, prefix string) string {
	lines := strings.Split(verified, "\n")
	for i := 0; i < len(lines); i++ {
		indent, fence, _, ok := codeFence(lines[i])
		if !ok {
			continue
		}
		var body []string
		depth, closed := 0, false
		j := i + 1
		for ; j < len(lines); j++ {
			if _, f, info, ok := codeFence(lines[j]); ok && f[0] == fence[0] {
				switch {
				case info:
					depth++
				case depth > 0:
					depth--
				case len(f) >= len(fence):
					closed = true
				}
				if closed {
					break
				}
			}
			line := lines[j]
			if cut := min(indent, len(line)-len(strings.TrimLeft(line, " \t"))); cut > 0 {
				line = line[cut:]
			}
			body = append(body, line)
		}
		if !closed {
			return ""
		}
		if text := strings.TrimSpace(strings.Join(body, "\n")); strings.HasPrefix(text, "<!-- "+prefix+":") {
			return text
		}
		i = j
	}
	return ""
}

// hasMarkerLine is true when a line starts with the marker "<!-- prefix:". A
// pass with one but no draft that draftComment can read must not count as clean.
func hasMarkerLine(verified, prefix string) bool {
	for _, line := range strings.Split(verified, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "<!-- "+prefix+":") {
			return true
		}
	}
	return false
}

// lgtmComment is the worker's own comment for a clean review, for a tono that
// writes no LGTM draft of its own. Its marker makes hasTonoComment true, so
// later runs skip the PR.
func lgtmComment(prefix, sha string) string {
	short := sha[:min(7, len(sha))]
	return fmt.Sprintf("<!-- %s:lgtm sha=%s -->\nLGTM 😃⭐😸\n\n<sub>Koko review worker: code, docs and code comments at `%s`, run with Claude</sub>", prefix, short, short)
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

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
