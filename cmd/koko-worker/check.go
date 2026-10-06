package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CheckResult is one connection row in the Koko UI.
type CheckResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

var checkNames = []string{"slack", "github", "claude", "jira", "calendar", "reviewer", "wake"}

// cmdCheck prints the checks as JSON. It exits 0 even when a check fails,
// because a failed check is a result, not an error.
func cmdCheck(ctx context.Context, paths Paths, args []string) error {
	cfg, err := loadConfig(paths.Config)
	if err != nil {
		return err
	}
	names := checkNames
	if len(args) > 0 {
		names = args
	}
	var results []CheckResult
	for _, name := range names {
		results = append(results, runCheck(ctx, cfg, paths, name))
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func runCheck(ctx context.Context, cfg Config, paths Paths, name string) CheckResult {
	r := CheckResult{Name: name}
	cr := claudeRunner{settingsPath: paths.Settings}
	switch name {
	case "slack":
		detail, err := newSlack(cfg.Slack).check(ctx)
		r.OK, r.Detail = err == nil, detail
		if err != nil {
			r.Detail = err.Error()
			r.Fix = "Enter a Slack bot token (xoxb-) and your Slack user ID. The bot needs the chat:write scope."
		}

	case "github":
		login, err := myLogin(ctx)
		r.OK, r.Detail = err == nil, "signed in as "+login
		if err != nil {
			r.Detail = err.Error()
			r.Fix = "Run `gh auth login` in a terminal."
		}

	case "claude":
		out, err := exec.CommandContext(ctx, "claude", "--version").Output()
		r.OK, r.Detail = err == nil, strings.TrimSpace(string(out))
		if err != nil {
			r.Detail = "claude not found on the worker's PATH: " + os.Getenv("PATH")
			r.Fix = "Install Claude Code, or link it into ~/.local/bin."
		}

	case "jira":
		res, err := cr.run(ctx, "Call getAccessibleAtlassianResources once, then reply with the single word DONE.",
			[]string{"mcp__claude_ai_Atlassian__getAccessibleAtlassianResources"}, 3*time.Minute)
		calls := res.callsTo("mcp__claude_ai_Atlassian__getAccessibleAtlassianResources")
		switch {
		case err != nil:
			r.Detail = err.Error()
		case len(calls) == 0 || calls[0].IsError:
			r.Detail = "the Atlassian connector did not answer"
		default:
			r.OK, r.Detail = true, "connected"
		}
		if !r.OK {
			r.Fix = "Connect Atlassian in claude.ai under Settings, Connectors."
		}

	case "calendar":
		events, err := listEvents(ctx, cr, cfg, time.Now())
		r.OK, r.Detail = err == nil, fmt.Sprintf("connected, %d events today", len(events))
		if err != nil {
			r.Detail = err.Error()
			r.Fix = "Reconnect Google Calendar in claude.ai under Settings, Connectors, and allow calendar access on Google's consent screen."
		}

	case "reviewer", legacyJobTono:
		rv := cfg.Reviewer
		cli := cfg.reviewerCLI(workerPaths())
		_, cloneErr := os.Stat(filepath.Join(workerPaths().Reviewer, ".git"))
		switch {
		case rv.problem() != nil:
			r.Detail = rv.problem().Error()
			r.Fix = "Fix the reviewer settings in Settings > Worker > Review worker."
		case rv.Source == SourceLocal:
			r.OK, r.Detail = rv.Path != "" && executable(cli), "local reviewer at "+cli
			if !r.OK {
				r.Detail = "no reviewer CLI at " + cli
				if rv.Path == "" {
					r.Detail = "the reviewer source is a local path, but no path is set"
				}
				r.Fix = "Set the reviewer path to an executable, or switch the source to managed."
			}
		case os.IsNotExist(cloneErr):
			// Not a failure: the first review run clones it.
			r.OK = true
			r.Detail = fmt.Sprintf("managed %s at %s, not cloned yet. The next review run clones it", rv.Repo, rv.Branch)
		case !executable(cli):
			r.Detail = fmt.Sprintf("the %s clone has no reviewer CLI at %s", rv.Repo, rv.Script)
			r.Fix = "Check the reviewer repo and branch in Settings. The script must be at that path in the repo."
		default:
			st := loadState(workerPaths().State)
			r.OK = true
			r.Detail = fmt.Sprintf("managed %s at %s, commit %s", rv.Repo, rv.Branch, st.ReviewerCommit)
		}

	case "wake":
		err := exec.CommandContext(ctx, "sudo", "-n", "-l", "/usr/bin/pmset", "schedule", "wake", "01/01/30 00:00:00").Run()
		r.OK = err == nil
		r.Detail = "the worker can book Mac wakes"
		if !r.OK {
			r.Detail = "the worker cannot book Mac wakes yet"
			r.Fix = "Run `sudo visudo -f /etc/sudoers.d/koko-worker` and add this line:\n" + sudoersRule()
		}

	default:
		r.Detail = "unknown check"
	}
	return r
}
