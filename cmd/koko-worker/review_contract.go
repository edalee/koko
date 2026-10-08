package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// The reviewer contract (plans 030 and 031) lets the review worker run any
// reviewer:
//
//   - The worker runs Reviewer.Command in a clone of the PR's repo, checked
//     out at the PR head. reviewerCommand fills in its placeholders.
//   - The environment carries REVIEW_RESULT (where to write the result),
//     REVIEW_PR, REVIEW_REPO, REVIEW_PR_URL and REVIEW_SHA. REVIEW_CLAUDE is
//     a claude wrapper with posting turned off.
//   - Exit code 0 means done, 2 means nothing to review (the PR is closed,
//     a draft, already reviewed, or held by another run), anything else failed.
//   - With no Reviewer.Logs, the reviewer writes resultFile to REVIEW_RESULT.
//
// The reviewer never posts. The worker posts each comment as written.

// reviewComment is one PR comment for the worker to post.
type reviewComment struct {
	Pass    string `json:"pass"`    // "review", "docs", "comments" or the reviewer's own name
	Verdict string `json:"verdict"` // lgtm, mergeable, follow-ups or not-mergeable
	Body    string `json:"body"`    // Markdown, starting with the marker line
}

// resultFile is the file a FormatResultJSON reviewer writes. A reviewed
// PR gets at least one comment: a clean review is an LGTM comment.
type resultFile struct {
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

// splitCommand splits a command into words on spaces and tabs. Single or
// double quotes keep a word with spaces together, and are dropped. Nothing
// else is special: no shell runs the command.
func splitCommand(command string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	inWord := false
	for _, c := range command {
		switch {
		case quote != 0 && c == quote:
			quote = 0
		case quote != 0:
			word.WriteRune(c)
		case c == '\'' || c == '"':
			quote, inWord = c, true
		case c == ' ' || c == '\t' || c == '\n':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(c)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("the reviewer command has an unclosed %c quote", quote)
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

var envWord = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// splitEnv splits a command into its leading NAME=value words, which set
// environment variables as in a shell, and the program with its arguments.
func splitEnv(command string) (env, words []string, err error) {
	words, err = splitCommand(command)
	if err != nil {
		return nil, nil, err
	}
	for len(words) > 0 && envWord.MatchString(words[0]) {
		env, words = append(env, words[0]), words[1:]
	}
	if len(words) == 0 {
		return nil, nil, fmt.Errorf("the reviewer command %q names no program", command)
	}
	return env, words, nil
}

// reviewerCommand is the program to run for one PR, its arguments and the
// environment the command sets. It fills in {reviewer} (the CLI from the
// source setting), {claude} (the wrapper), {pr}, {repo}, {url} and {sha}. A
// "~" at the start of the program means your home folder.
func reviewerCommand(command, cli, claude string, p SearchPR, sha string) (env []string, prog string, args []string, err error) {
	env, words, err := splitEnv(command)
	if err != nil {
		return nil, "", nil, err
	}
	r := strings.NewReplacer("{reviewer}", cli, "{claude}", claude,
		"{pr}", fmt.Sprint(p.Number), "{repo}", p.repo(), "{url}", p.URL, "{sha}", sha)
	for i := range env {
		env[i] = r.Replace(env[i])
	}
	for i := range words {
		words[i] = r.Replace(words[i])
	}
	return env, expandHome(words[0]), words[1:], nil
}

// commandProgram is the program a command runs, before any placeholder is
// filled in, so a check can look for it.
func commandProgram(command string) string {
	_, words, err := splitEnv(command)
	if err != nil {
		return ""
	}
	return expandHome(words[0])
}

// readResult reads the reviewer's result file into an outcome. A
// "skipped" status is reviewSkippedError, like exit code 2. A "failed" status,
// a missing file or one the worker cannot read is an error.
func readResult(path string) (reviewOutcome, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return reviewOutcome{}, fmt.Errorf("the reviewer exited 0 but wrote no result file")
	}
	if err != nil {
		return reviewOutcome{}, err
	}
	var res resultFile
	if err := json.Unmarshal(data, &res); err != nil {
		return reviewOutcome{}, fmt.Errorf("the reviewer's result file is not valid JSON: %v", err)
	}
	switch res.Status {
	case "skipped":
		return reviewOutcome{}, reviewSkippedError{fmt.Errorf("%s", strings.TrimSpace(res.Reason))}
	case "failed":
		return reviewOutcome{}, fmt.Errorf("the reviewer failed: %s", truncate(strings.TrimSpace(res.Reason), 300))
	case "reviewed":
	default:
		return reviewOutcome{}, fmt.Errorf("the reviewer's result has status %q, want reviewed, skipped or failed", res.Status)
	}
	out := reviewOutcome{verdicts: map[string]string{}}
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

// lgtmLine finds the log format's clean-review verdict, "> **LGTM!**", in a draft.
var lgtmLine = regexp.MustCompile(`(?im)^>\s*\*\*\s*LGTM!?\s*\*\*`)

// isLGTMDraft is true for a clean-review comment in the log format.
func isLGTMDraft(body string) bool { return lgtmLine.MatchString(body) }

// lgtmOnHead is true when the PR has a code review comment behind prefix that
// is an LGTM for the head commit. A reviewer in the log format then skips the
// code review, as it was already clean at this commit.
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
