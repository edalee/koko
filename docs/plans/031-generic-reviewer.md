# Plan 031: a reviewer that is not tied to tono

## Problem

Plan 030 added a reviewer contract, but the review worker still assumed tono:

- The defaults named tono: the `epidemicsound/tonometer` repo, the `tono` script, the `tono` marker and tono's arguments.
- The default `tono-logs` format read tono's logs from a fixed folder, `~/.cache/tono/logs`.
- Every run set `TONO_CLAUDE`.
- The team scope defaulted to `epidemicsound/content-protection`, and a bare repo name meant `epidemicsound/<name>`.
- The code, the state file and the run logs used tono's name: `runTono`, `tonoResults`, `tono-*.log`, `tono-claude.sh`, `tono-bin`.

## Goal

You set up any reviewer with one command and an optional log folder. Nothing outside the migration names tono or Epidemic Sound.

## Settings

`reviewer.command` is the whole command, for example `{reviewer} {pr} --all`.

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

Log mode reads one layout, tono's. It is not reviewer-agnostic:

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

A `reviewer` section without a `command` key is from 0.5.7 or older. The worker and the Settings tab migrate it the same way:

- `command` becomes `TONO_CLAUDE={claude} {reviewer} ` plus the old `args`, or tono's arguments if there were none.
- `logs` becomes `~/.cache/tono/logs`, unless `format` was `result-json`.
- `markerPrefix` becomes `tono` if it was not set.

An empty `command` stays empty. Only a missing key counts as old.

The state file's `tonoReviewed` and `tonoResults` become `reviewed` and `results` on load, so no PR is reviewed twice. Old `tono-*.log` run logs are still deleted after 60 days.

These strings name tono only in the migration code and in your own `worker.json`.

## Renames

| Before | After |
|---|---|
| `runTono`, `tonoReview`, `tonoOutcome`, `TonoResult` | `runReview`, `runReviewer`, `reviewOutcome`, `ReviewResult` |
| `tonoVerified`, `tonoLockKey`, `tonoPasses` | `readLogs`, `logKey`, `logPasses` |
| `hasTonoComment`, `tonoScope`, `tonoTargets` | `hasReviewComment`, `reviewScope`, `reviewTargets` |
| `tono-*.log`, `tono-claude.sh`, `tono-bin` | `review-*.log`, `reviewer-claude.sh`, `reviewer-bin` |

## Not in scope

The stand-up's Jira site and Koko's own PR panel still default to Epidemic Sound. Neither is part of the reviewer.
