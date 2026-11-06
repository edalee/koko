package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The command contract (plans 031 and 037) lets a run call any tool:
//
//   - A PRs run runs its command in a clone of the PR's repo, checked out at
//     the PR head. A repos run runs it in a clean checkout of the repo's
//     default branch. reviewerCommand fills in the placeholders.
//   - The environment carries REVIEW_PR, REVIEW_REPO, REVIEW_PR_URL,
//     REVIEW_SHA and RUN_KIND (prs or repos). PATH is CommandPath, if set.
//   - Exit code 0 means done, 2 means nothing to do (the PR is closed, a
//     draft, already reviewed, or held by another run), anything else failed.
//
// The tool does its own work, posting included. The worker reads only the
// exit code.

// splitCommand splits a command into words on spaces, tabs and newlines. Single or
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

// reviewerCommand is the program to run, its arguments and the environment
// the command sets. It fills in {reviewer} (the CLI from the source setting),
// {pr}, {repo}, {url} and {sha}. A "~" at the start of the program means your
// home folder.
func reviewerCommand(command, cli string, p SearchPR, sha string) (env []string, prog string, args []string, err error) {
	env, words, err := splitEnv(command)
	if err != nil {
		return nil, "", nil, err
	}
	r := strings.NewReplacer("{reviewer}", cli,
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

// commandEnv is the environment of a run's command: base, with PATH set to
// path if path is not empty, then the command's own NAME=value words, then
// extra. exec keeps the last of a name, so extra wins.
func commandEnv(base []string, path string, cmdEnv []string, extra ...string) []string {
	out := make([]string, 0, len(base)+len(cmdEnv)+len(extra)+1)
	for _, kv := range base {
		if path == "" || !strings.HasPrefix(kv, "PATH=") {
			out = append(out, kv)
		}
	}
	if path != "" {
		out = append(out, "PATH="+path)
	}
	return append(append(out, cmdEnv...), extra...)
}

// findProgram is the file prog names. A bare name is looked up on path, else
// on the worker's own PATH. A name with a slash is used as it is.
func findProgram(prog, path string) (string, error) {
	if strings.Contains(prog, "/") {
		return prog, nil
	}
	if path == "" {
		path = os.Getenv("PATH")
	}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		file := filepath.Join(dir, prog)
		if executable(file) {
			return file, nil
		}
	}
	return "", fmt.Errorf("%s is not on the PATH for commands. Set it in Settings > Worker > General, or use the full path", prog)
}
