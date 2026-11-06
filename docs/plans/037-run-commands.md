# Plan 037: each run is a command, judged by its exit code

## Problem

- The worker reads the reviewer's output: a result file, or the reviewer's logs in one fixed layout (plans 030 and 031). A reviewer that changes its output breaks the worker. The new review comment of the old reviewer did exactly that: every review failed with "a pass wrote a PR comment the worker could not read".
- The worker posts the comments, finds earlier ones by a marker, works out verdicts and an LGTM, and retries failed posts. A reviewer that posts its own comment needs none of it.
- Every run shares one base command, and adds its own arguments. A run that calls a different program cannot.
- The worker runs commands with launchd's short PATH. A tool that needs `node`, `uv` or `gcloud` cannot find them.

## Goal

The worker runs a command on a schedule and reads only its exit code. The tool does its own work: it reviews, posts and decides when there is nothing to do.

Koko stays generic. No code, default, example or help text names a reviewer tool or a company.

## Runs

Each run has its own `command`, in place of the shared command and the run's `args`.

| Field | Meaning |
|---|---|
| `command` | The whole command, split and filled in as plan 031 says |
| `name`, `enabled`, `days`, `times`, `target`, `mine`, `team`, `repos`, `timeoutMinutes` | As in plan 035 |

Placeholders:

| Placeholder | Value |
|---|---|
| `{reviewer}` | The CLI from the source setting, as before |
| `{pr}`, `{repo}`, `{url}`, `{sha}` | PRs runs: the PR number, `owner/repo`, URL and head commit. A repos run gets `{repo}` only |

`{claude}` is gone. A command that uses it fails its run, and the message says to remove it.

The source, orgs and excluded repos stay shared.

## PRs runs

For each PR in the run's scopes:

1. Skip it if the orgs or excluded repos say so, or if the worker has a done record for its head commit. A failed record does not skip, so a failure from before 037 gets a new run.
2. Check out the head commit in the cache clone, as before.
3. Run the command there.
4. Judge the exit code:

| Exit | Meaning | Record |
|---|---|---|
| 0 | Done | Done, at this commit |
| 2 | Nothing to do: closed, a draft, already reviewed, or another run holds it | None, so a later slot asks again |
| Other, or a timeout | Failed | Failed, at this commit. Your own PR also gets a DM, once per commit |

A failure while the Mac is offline gets no record, so the scheduler's retry runs it again.

The record is keyed by the PR and its head commit. A new commit runs again. The tool decides if the new commit needs a review. A failed PR runs again at each slot, and the tool decides again.

A test run prints the command for one PR and runs nothing, as a repos run does. The command may post, and a test must not. Run now runs it for real.

The environment carries `REVIEW_PR`, `REVIEW_REPO`, `REVIEW_PR_URL`, `REVIEW_SHA` and `RUN_KIND=prs`. gh and git are the real ones, so the tool can post.

## Repos runs

As in plan 035, with the run's own command.

## PATH for commands

A new General setting, `commandPath`. Empty means the worker's own PATH.

- Commands run with it, so a tool finds `node`, `uv` or `gcloud`.
- The check of the command's program looks it up on the same PATH.
- A button fills it from your login shell's PATH. Koko runs `$SHELL -lic` with a clean environment (only `HOME`, `USER` and `SHELL`), so nothing from the shell that opened Koko leaks in. It drops repeated folders and folders that do not exist.

## Stop

Each job's card shows Stop while the job runs, whether the scheduler started it or Run now did.

- `koko-worker stop <job>` writes a stop file next to the job's lock. It fails if nobody holds the lock, because then nothing runs.
- Every run, in any process, checks for its stop file every 2 seconds. It removes a stale one when it starts.
- On a stop, the run's context ends, as on a timeout, so the tool's process group gets SIGTERM.
- A stopped run is a stopError: the scheduler records it as failed and does not try the slot again. A PR stopped halfway gets no record, so a later slot runs it.
- `koko-worker status` says which jobs run, and the app reads it every 30 seconds.

## What goes

- The result file (`$REVIEW_RESULT`) and the log format (`reviewer.logs`).
- Posting, finding and editing comments, the marker (`reviewer.markerPrefix`), verdicts, LGTM and the queue of unposted comments.
- The read-only gh and git, and the claude wrapper.
- The shared `reviewer.command` and each run's `args`.

## Migration

- The old shared command does not carry over. It ran with a read-only gh, so it never posted. Run with the real gh, it might post, or exit 0 having posted nothing, and the worker would record the PR as done.
- A run with no `command` fails, and says to set a command that does its own work, posting included.
- Old done records stay. A PR reviewed at its head commit is not run again.
- Old fields in `worker.json` and the state file are ignored.

## Stand-up

A PR shows as reviewed, failed or not yet. Verdicts go.

## Decision

ADR-043 records this. It replaces the guards of ADR-034, ADR-039 and ADR-040: the read-only gh and git, the claude wrapper and the result file.

## Example

| Run | Target | Command | Schedule |
|---|---|---|---|
| reviews | PRs | `{reviewer} {url} -c` | Weekdays at 09:30, 12:00 and 14:00 |
| updates | Repos | `{reviewer} --update-deps` | Mondays at 08:00, 60 minutes per repo |

## Not in scope

- Reading a tool's output for a summary. The DM names the log of a failure.
