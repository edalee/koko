package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	// online is true if the internet answers.
	online func(ctx context.Context) bool
	test   bool // print instead of DM or comment, book nothing
	// timeout caps one PR or one repo of a run. Zero means reviewTimeout.
	timeout time.Duration
	// slot is the first slot a scheduled run covers. A scheduled repos run
	// keys its repos' outcomes on it. Zero means none.
	slot time.Time
	// scheduled is true for a run the scheduler started, not a Run now.
	scheduled bool
	// noWait makes a Run now fail at once on a held lock, not wait.
	noWait bool
	state  *State // read-only snapshot. A review or repos run loads it again after it takes the reviewer lock.
	now    time.Time
	// persist changes the state on disk under a lock. It does nothing in test mode.
	persist func(change func(*State)) error
}

// runTimeout caps one PR or one repo of the run at work.
func (e Env) runTimeout() time.Duration {
	if e.timeout > 0 {
		return e.timeout
	}
	return reviewTimeout
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

// jiraOffNote says why the stand-up checked no tickets, so a user who
// updated from a version without a Jira site field notices. It is empty with
// a site set, or with no PRs to check.
func jiraOffNote(site string, prs int) string {
	if site != "" || prs == 0 {
		return ""
	}
	return "_Jira checks are off until you set the Jira site in Settings > Worker._\n"
}

func runStandup(ctx context.Context, env Env) error {
	// The team PRs, repos and age limit are those of the first PRs run that
	// is on, else the first PRs run. With no PRs run, there are none.
	env.cfg.Reviewer = env.cfg.Reviewer.forRun(env.cfg.standupRun())
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
	// With no Jira site set, the stand-up reads no tickets.
	if len(judged) > 0 && env.cfg.JiraSite != "" {
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
	b.WriteString(jiraOffNote(env.cfg.JiraSite, len(judged)))
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

// reviewTally reads the review results for the stand-up's PR lines. A PR
// shows its latest run, which may be at an earlier commit.
type reviewTally struct {
	scope    reviewScope
	scopeErr error
	loaded   bool
}

func (t *reviewTally) loadScope(ctx context.Context, env Env) {
	if t.loaded {
		return
	}
	t.loaded = true
	if env.cfg.Reviewer.Team.Team == "" {
		t.scopeErr = errNoTeam
		return
	}
	t.scope, t.scopeErr = loadScope(ctx, env.cfg, env.now)
}

// errNoTeam means no review team is set, so the stand-up lists every PR and
// has no scope to report on.
var errNoTeam = errors.New("no review team is set")

// status is the review status for one PR, counted in counts: reviewed,
// failed or not yet. The worker reads only the command's exit code, so it has
// no verdict to show (plan 037).
func (t *reviewTally) status(ctx context.Context, env Env, p SearchPR, counts map[string]int) string {
	t.loadScope(ctx, env)
	r, ran := latestResult(env.state, p.repo(), p.Number)
	skip := env.cfg.Reviewer.skipRepo(p.repo())
	switch {
	case ran && r.Failed != "":
		counts["failed"]++
		return "the review worker's run failed on it"
	case skip != "" && !ran:
		counts["skipped"]++
		return "skipped: " + skip
	case ran:
		counts["reviewed"]++
		if d, err := prDetail(ctx, p.URL); err == nil && d.HeadRefOid != r.SHA {
			return "reviewed at an earlier commit"
		}
		return "reviewed"
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
	fmt.Fprintf(&b, " The review worker has reviewed %d.", counts["reviewed"])
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
	if n := counts["skipped"]; n == 1 {
		b.WriteString(" 1 is in a repo the review worker skips.")
	} else if n > 1 {
		fmt.Fprintf(&b, " %d are in repos the review worker skips.", n)
	}
	if t.scopeErr != nil && !errors.Is(t.scopeErr, errNoTeam) {
		fmt.Fprintf(&b, " _Could not load the review scope: %s_", truncate(t.scopeErr.Error(), 150))
	}
	return b.String()
}

func prLine(p SearchPR, status string) string {
	return fmt.Sprintf("• %s %s (%s): %s", slackLink(p.URL, p.short()), p.Title, p.Author.Login, status)
}

// reviewSection is the stand-up's "Needs your review" section: a summary of
// the review worker's reviews, then one line per PR.
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
// Reviewer.Team.Repos. It skips PRs outside the review scope, PRs in repos
// the review worker skips, and PRs that ask for your review, because "Needs
// your review" lists those. It is empty if the
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
		if inQueue[p.URL] || env.cfg.Reviewer.skipRepo(p.repo()) != "" || (tally.scopeErr == nil && tally.scope.outside(p) != "") {
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
	// Only Test and Run now get here on such a day: the scheduler skips it.
	if !env.cfg.Jobs[JobFocus].runsOn(now) {
		return env.notify(ctx, fmt.Sprintf("Focus: the focus job does not run on %s, so it books nothing today.", now.Weekday()))
	}
	events, err := listEvents(ctx, env.claude, env.cfg, now)
	if err != nil {
		return err
	}
	gaps := findFocusGaps(events, env.cfg, now)
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

// ---- Review ----

// reviewScope is which team PRs the review worker may review: opened by a member of the team, in
// the last Reviewer.Team.MaxAgeDays days.
type reviewScope struct {
	team    string
	days    int
	members map[string]bool
	since   time.Time
}

func loadScope(ctx context.Context, cfg Config, now time.Time) (reviewScope, error) {
	members, err := teamMembers(ctx, cfg.Reviewer.Team.Team)
	if err != nil {
		return reviewScope{}, err
	}
	return reviewScope{
		team: cfg.Reviewer.Team.Team, days: cfg.Reviewer.Team.MaxAgeDays, members: members,
		since: now.AddDate(0, 0, -cfg.Reviewer.Team.MaxAgeDays),
	}, nil
}

// outside says why a PR is outside the scope, or "" if it is inside.
func (s reviewScope) outside(p SearchPR) string {
	if !s.members[strings.ToLower(p.Author.Login)] {
		return "not opened by " + s.team
	}
	created, err := time.Parse(time.RFC3339, p.CreatedAt)
	if err != nil || created.Before(s.since) {
		return fmt.Sprintf("opened more than %d days ago", s.days)
	}
	return ""
}

// latestResult is the most recent review result for the PR, at any commit.
func latestResult(st *State, repo string, number int) (ReviewResult, bool) {
	var best ReviewResult
	found := false
	for _, r := range st.Results {
		if r.Repo == repo && r.Number == number && (!found || r.At.After(best.At)) {
			best, found = r, true
		}
	}
	return best, found
}

// doneAt is true if a run of the command ended with exit 0 at this commit
// (key is reviewKey). A failed run does not count, so a later slot runs it
// again. Before plan 037, a failure was in Reviewed too, so Results decides.
func doneAt(st *State, key string) bool {
	_, ok := st.Reviewed[key]
	return ok && st.Results[key].Failed == ""
}

// reviewTarget is one PR to review.
type reviewTarget struct {
	pr     SearchPR
	mine   bool
	tooOld bool // your PR, opened more than Reviewer.Mine.MaxAgeDays ago
}

// reviewTargets is your open PRs if Reviewer.Mine is on. If Reviewer.Team is on,
// the PRs waiting for your review follow, then the open PRs in Reviewer.Team.Repos.
// Others' PRs are people's, not bots', and runReview checks them against the team scope.
// Your PRs opened more than Reviewer.Mine.MaxAgeDays ago are marked tooOld.
func reviewTargets(ctx context.Context, env Env) ([]reviewTarget, error) {
	var out []reviewTarget
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
			out = append(out, reviewTarget{pr: p, mine: true, tooOld: old})
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
			out = append(out, reviewTarget{pr: p})
			seen[p.URL] = true
		}
	}
	return out, nil
}

func reviewKey(repo string, number int, sha string) string {
	return fmt.Sprintf("%s#%d@%s", repo, number, sha)
}

// runReview runs the run's command once per PR in its scopes, at the PR's
// head commit, and judges only its exit code (plan 037). The command does its
// own work, posting included. Exit 0 records the commit as done, so the run
// skips it from then on. Exit 2 records nothing, so a later slot asks again.
// Any other exit records a failure, which a later slot also runs again.
func runReview(ctx context.Context, env Env, onlyURL string) error {
	// Runs share the reviewer clone, so one run works at a time. The state is
	// read again under the lock, to see what the run before recorded.
	if !env.test {
		unlock, err := takeLock(filepath.Join(env.paths.Dir, "reviewer.lock"), env.noWait)
		if err != nil {
			return err
		}
		defer unlock()
		st := loadState(env.paths.State)
		env.state = &st
	}
	// Before the PR loop, so a reviewer that cannot run records nothing.
	cli, err := prepareReviewer(ctx, env)
	if err != nil {
		return err
	}
	targets, err := reviewTargets(ctx, env)
	if err != nil {
		return err
	}
	var scope reviewScope
	if env.cfg.Reviewer.Team.Enabled {
		if scope, err = loadScope(ctx, env.cfg, env.now); err != nil {
			return err
		}
	}
	var netErr error
	var failures []string
	ran := 0
	for _, t := range targets {
		p := t.pr
		if onlyURL != "" && p.URL != onlyURL {
			continue
		}
		// A test run shows one PR, so the Test button stays quick.
		if env.test && ran > 0 {
			break
		}
		// A PR named with --pr runs whatever the scope, the orgs, the excluded
		// repos, its age or its record say. Your own PRs are outside the team
		// scope, and have their own age limit.
		if onlyURL == "" && (env.cfg.Reviewer.skipRepo(p.repo()) != "" || (!t.mine && scope.outside(p) != "") || t.tooOld) {
			continue
		}
		d, err := prDetail(ctx, p.URL)
		if err != nil {
			return err
		}
		key := reviewKey(p.repo(), p.Number, d.HeadRefOid)
		if onlyURL == "" && doneAt(env.state, key) {
			continue
		}
		ran++
		if env.test {
			// The command may post, so a test only shows it.
			_, prog, args, err := reviewerCommand(env.cfg.Reviewer.Command, cli, p, d.HeadRefOid)
			if err != nil {
				return err
			}
			msg := fmt.Sprintf("The run would run, for %s at its head commit:\n%s\nIt may take up to %s. Nothing runs in a test.",
				slackLink(p.URL, p.short()), strings.Join(append([]string{prog}, args...), " "), env.runTimeout())
			if err := env.notify(ctx, msg); err != nil {
				return err
			}
			continue
		}
		err = runReviewer(ctx, env, cli, p, d.HeadRefOid)
		var skipped reviewSkippedError
		switch {
		case errors.As(err, &skipped):
			// Nothing to do: the command says the PR is closed, a draft,
			// already reviewed or held by another run. No record, so a later
			// slot asks again.
			log.Printf("review: %s skipped: %v", p.short(), err)
			continue
		case err != nil && !env.online(ctx):
			// No record, so the scheduler's retry runs it once the internet is back.
			netErr = err
			continue
		}
		result := ReviewResult{
			URL: p.URL, Title: p.Title, Repo: p.repo(), Number: p.Number, SHA: d.HeadRefOid,
			Mine: t.mine, At: time.Now(),
		}
		failedBefore := env.state.Results[key].Failed != ""
		if err != nil {
			result.Failed = truncate(err.Error(), 400)
			log.Printf("review: %s failed: %s", p.short(), result.Failed)
		}
		if err := env.persist(func(st *State) {
			st.Results[key] = result
			if result.Failed == "" {
				st.Reviewed[key] = time.Now()
			}
		}); err != nil {
			return err
		}
		// Your own PRs get a warning, once per commit. A failed run on someone
		// else's PR shows in the stand-up instead.
		if result.Failed != "" && t.mine && !failedBefore {
			failures = append(failures, fmt.Sprintf("%s: %s", p.short(), result.Failed))
			link := slackLink(p.URL, p.short())
			if err := env.notify(ctx, fmt.Sprintf(":warning: The review worker's run failed on %s: %s", link, result.Failed)); err != nil {
				return err
			}
		}
	}
	if env.test && ran == 0 {
		return env.notify(ctx, "The run found no PR to run on now.")
	}
	if netErr != nil {
		return netErr
	}
	if len(failures) > 0 {
		return reportedError{fmt.Errorf("review: %s", strings.Join(failures, "; "))}
	}
	return nil
}

// groupCommand starts name in its own process group. When ctx ends, the whole
// group gets SIGTERM, so the reviewer's trap runs and Claude's children stop too.
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

// reviewSkippedError means the reviewer exited with skipExit.
type reviewSkippedError struct{ error }

// skipExit is the command's "nothing to do" exit code. For a PR: another run
// holds the lock, or the PR is closed or a draft. For a repo: the tool has
// nothing to change.
const skipExit = 2

// reviewTimeout is the default cap on one review. On timeout, the reviewer's whole process
// group gets SIGTERM, so its trap can remove its lock. A plain kill would leave the lock
// behind and block every later review of that PR.
const reviewTimeout = 45 * time.Minute

// reportedError is a failure the job has already sent you, so the scheduler
// logs it without a second DM.
type reportedError struct{ error }

// runReviewer runs the run's command, with cli in place of {reviewer}, at the
// PR head in a cache clone. It never touches your working clones. One run per
// cache clone at a time, so a Run now cannot check out another PR under a run
// in progress. It returns nil for exit 0, reviewSkippedError for exit 2, and
// an error that names the log for anything else.
func runReviewer(ctx context.Context, env Env, cli string, p SearchPR, headSHA string) error {
	dir := filepath.Join(env.paths.Cache, strings.ReplaceAll(p.repo(), "/", "__"))
	if err := os.MkdirAll(env.paths.Cache, 0o700); err != nil {
		return err
	}
	unlock, err := lockFile(dir + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		cloneCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		out, err := exec.CommandContext(cloneCtx, "gh", "repo", "clone", p.repo(), dir, "--", "--filter=blob:none", "--quiet").CombinedOutput()
		cancel()
		if err != nil {
			_ = os.RemoveAll(dir) // a half-made clone would break every later run
			return fmt.Errorf("clone %s: %v: %s", p.repo(), err, truncate(strings.TrimSpace(string(out)), 300))
		}
	}
	for _, args := range [][]string{
		{"-C", dir, "fetch", "--quiet", "origin", fmt.Sprintf("pull/%d/head", p.Number)},
		{"-C", dir, "checkout", "--quiet", "--force", "--detach", headSHA},
	} {
		if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %s", args[2], truncate(strings.TrimSpace(string(out)), 300))
		}
	}

	cmdEnv, prog, args, err := reviewerCommand(env.cfg.Reviewer.Command, cli, p, headSHA)
	if err != nil {
		return err
	}
	if prog, err = findProgram(prog, env.cfg.CommandPath); err != nil {
		return err
	}
	logPath := filepath.Join(env.paths.Logs, fmt.Sprintf("%s%s-%d-%s.log", reviewLogPrefix, shortRepo(p.repo()), p.Number, headSHA[:min(8, len(headSHA))]))
	_ = os.MkdirAll(env.paths.Logs, 0o700)
	logFile := io.Writer(io.Discard)
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
		defer func() { _ = f.Close() }()
		logFile = f
	}
	tail := &tailWriter{}
	out := io.MultiWriter(logFile, tail)

	limit := env.runTimeout()
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := groupCommand(ctx, prog, args...)
	cmd.Dir = dir
	cmd.Env = commandEnv(os.Environ(), env.cfg.CommandPath, cmdEnv,
		"REVIEW_PR="+fmt.Sprint(p.Number), "REVIEW_REPO="+p.repo(), "REVIEW_PR_URL="+p.URL,
		"REVIEW_SHA="+headSHA, "RUN_KIND=prs")
	cmd.Stdout, cmd.Stderr = out, out
	err = cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timed out after %s, see %s", limit, logPath)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == skipExit {
		return reviewSkippedError{fmt.Errorf("%s", truncate(tail.lastLine(), 200))}
	}
	if err != nil {
		return fmt.Errorf("%v: %s, see %s", err, truncate(tail.lastLine(), 300), logPath)
	}
	return nil
}

// reviewLogPrefix starts the worker's log of each run of a command.
const reviewLogPrefix = "review-"
