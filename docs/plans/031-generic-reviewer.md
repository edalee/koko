# Plan 031: a reviewer that is not tied to one CLI

## Problem

Plan 030 added a reviewer contract, but the review worker still assumed one reviewer CLI:

- The defaults named that reviewer: its repo, its script, its marker and its arguments.
- The default log format read that reviewer's logs from a fixed folder.
- Every run set that reviewer's own Claude variable.
- The team scope defaulted to one org's team, and a bare repo name meant a repo in that org.
- The code, the state file and the run logs used that reviewer's name.

## Goal

You set up any reviewer with one command and an optional log folder. Nothing outside the migration names one reviewer CLI or org.

## Settings

`reviewer.command` is the whole command, for example `{reviewer} {pr}`.

- The worker splits it on spaces. Single or double quotes keep a word with spaces together. No shell runs it.
- Leading `NAME=value` words set environment variables, as in a shell.
- The first other word is the program. `~` at its start means your home folder. A bare name is looked up on the worker's `PATH`.
- Placeholders:

| Placeholder | Value |
|---|---|
| `{reviewer}` | The reviewer CLI from the source setting: the script in the managed clone, or the local path |
| `{claude}` | A `claude` wrapper with posting turned off, the same as `$REVIEW_CLAUDE` |
| `{pr}`, `{repo}`, `{url}`, `{sha}` | The PR number, `owner/repo`, URL and head commit |

The worker prepares the managed clone only when the command uses `{reviewer}`.

`reviewer.logs` is the folder the reviewer writes its logs to.

- Empty, the default, means off. The reviewer writes its result to `$REVIEW_RESULT` (plan 030).
- Set, the worker reads each pass's draft and verdict from the logs instead. This is the log format below.

`reviewer.markerPrefix` has no default. The review job reports it as missing until you set it.

`reviewer.format` and `reviewer.args` are gone. `command` and `logs` replace them.

## The log format

Log mode reads one layout, the old reviewer's. It is not reviewer-agnostic:

- three passes, named review, docs and comments
- one file per pass, `*-<key>-<pass>-merged.log`, where `<key>` is `owner/repo|n` with every character that is not a letter or digit turned into `_`
- the draft comment as a fenced block that starts with the marker
- the verdict wording, and `> **LGTM!**` for a clean review

A new reviewer should use the result file.

## Defaults for a new install

- No reviewer, so the review job is off until you set one.
- Team scope off, with no team.
- A bare repo name in the repo list means the team's org. With no team set, the list needs `owner/repo`.

## Migration

0.5.8 to 0.5.13 migrated a `reviewer` section without a `command` key, and the old state keys. 0.5.14 removed these migrations: an old config has no reviewer until you set one in Settings.

## Renames

| Before | After |
|---|---|
| Run, outcome and result names | `runReview`, `runReviewer`, `reviewOutcome`, `ReviewResult` |
| Log reading names | `readLogs`, `logKey`, `logPasses` |
| Comment and scope names | `hasReviewComment`, `reviewScope`, `reviewTargets` |
| Run log, wrapper and binary names | `review-*.log`, `reviewer-claude.sh`, `reviewer-bin` |

## Not in scope

The stand-up's Jira site and Koko's own PR panel. Both lost their org defaults in 0.5.14.
