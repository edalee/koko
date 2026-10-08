package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// The reviewer contract (plan 030) lets the review worker run any reviewer,
// not only tono:
//
//   - The worker runs the reviewer CLI with Reviewer.Args, in a clone of the
//     PR's repo checked out at the PR head. {pr}, {repo}, {url} and {sha} in
//     the arguments are filled in.
//   - The environment carries REVIEW_RESULT (where to write the result),
//     REVIEW_PR, REVIEW_REPO, REVIEW_PR_URL and REVIEW_SHA. REVIEW_CLAUDE, and
//     TONO_CLAUDE for tono, is a claude wrapper with posting turned off.
//   - Exit code 0 means done, 2 means nothing to review (the PR is closed,
//     a draft, already reviewed, or held by another run), anything else failed.
//   - With FormatResultJSON, the reviewer writes reviewResult to REVIEW_RESULT.
//
// The reviewer never posts. The worker posts each comment as written.

// reviewComment is one PR comment for the worker to post.
type reviewComment struct {
	Pass    string `json:"pass"`    // "review", "docs", "comments" or the reviewer's own name
	Verdict string `json:"verdict"` // lgtm, mergeable, follow-ups or not-mergeable
	Body    string `json:"body"`    // Markdown, starting with the marker line
}

// reviewResult is the file a FormatResultJSON reviewer writes. A reviewed
// PR gets at least one comment: a clean review is an LGTM comment.
type reviewResult struct {
	Status   string          `json:"status"` // reviewed, skipped or failed
	Reason   string          `json:"reason"` // why it skipped or failed
	Comments []reviewComment `json:"comments"`
}

// Verdicts in the contract.
const (
	contractLGTM         = "lgtm"
	contractMergeable    = "mergeable"
	contractFollowUps    = "follow-ups"
	contractNotMergeable = "not-mergeable"
)

// internalVerdict maps a contract verdict to the worker's own, and says
// whether it is an LGTM. An LGTM counts as ready to approve.
func internalVerdict(v string) (verdict string, lgtm bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case contractLGTM:
		return verdictReady, true
	case contractMergeable:
		return verdictReady, false
	case contractFollowUps:
		return verdictFollowUps, false
	case contractNotMergeable:
		return verdictNotMergeable, false
	}
	return "", false
}

// expandArgs fills {pr}, {repo}, {url} and {sha} into the reviewer's arguments.
func expandArgs(args []string, p SearchPR, sha string) []string {
	r := strings.NewReplacer("{pr}", fmt.Sprint(p.Number), "{repo}", p.repo(), "{url}", p.URL, "{sha}", sha)
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = r.Replace(a)
	}
	return out
}

// readResult reads a FormatResultJSON reviewer's file into an outcome. A
// "skipped" status is tonoSkippedError, like exit code 2. A "failed" status,
// a missing file or one the worker cannot read is an error.
func readResult(path string) (tonoOutcome, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return tonoOutcome{}, fmt.Errorf("the reviewer exited 0 but wrote no result file")
	}
	if err != nil {
		return tonoOutcome{}, err
	}
	var res reviewResult
	if err := json.Unmarshal(data, &res); err != nil {
		return tonoOutcome{}, fmt.Errorf("the reviewer's result file is not valid JSON: %v", err)
	}
	switch res.Status {
	case "skipped":
		return tonoOutcome{}, tonoSkippedError{fmt.Errorf("%s", strings.TrimSpace(res.Reason))}
	case "failed":
		return tonoOutcome{}, fmt.Errorf("the reviewer failed: %s", truncate(strings.TrimSpace(res.Reason), 300))
	case "reviewed":
	default:
		return tonoOutcome{}, fmt.Errorf("the reviewer's result has status %q, want reviewed, skipped or failed", res.Status)
	}
	out := tonoOutcome{format: FormatResultJSON, verdicts: map[string]string{}}
	for _, c := range res.Comments {
		v, lgtm := internalVerdict(c.Verdict)
		if c.Pass != "" && v != "" {
			out.verdicts[c.Pass] = v
		}
		out.comments = append(out.comments, reviewComment{Pass: c.Pass, Verdict: c.Verdict, Body: strings.TrimSpace(c.Body)})
		if lgtm {
			out.lgtmPasses++
		}
	}
	return out, nil
}

// lgtmLine finds tono's clean-review verdict, "> **LGTM!**", in a draft.
var lgtmLine = regexp.MustCompile(`(?im)^>\s*\*\*\s*LGTM!?\s*\*\*`)

// isLGTMDraft is true for tono's clean-review comment.
func isLGTMDraft(body string) bool { return lgtmLine.MatchString(body) }

// lgtmOnHead is true when the PR has a code review comment behind prefix that
// is an LGTM for the head commit. tono then skips the code review, as it
// was already clean at this commit.
func lgtmOnHead(ctx context.Context, repo string, number int, prefix, sha string) (bool, error) {
	out, err := gh(ctx, "api", "--paginate", fmt.Sprintf("repos/%s/issues/%d/comments", repo, number),
		"--jq", `.[] | .body | gsub("\n"; "\\n")`)
	if err != nil {
		return false, err
	}
	short := sha[:min(7, len(sha))]
	for _, line := range strings.Split(string(out), "\n") {
		body := strings.ReplaceAll(line, `\n`, "\n")
		first := firstLine(body)
		if strings.HasPrefix(first, "<!-- "+prefix+":review") && strings.Contains(first, "sha="+short) && isLGTMDraft(body) {
			return true, nil
		}
	}
	return false, nil
}
