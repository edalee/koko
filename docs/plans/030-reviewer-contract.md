# Plan 030: a reviewer contract for the review worker

Status: done in 0.5.7. Plan 031 replaces `reviewer.format` and `reviewer.args` with `reviewer.command` and `reviewer.logs`.

## Problem

The review worker ran one reviewer CLI and nothing else. It was tied to that reviewer in five places:

- the arguments, `<n> --all -l high -R <repo>`
- the reviewer's own Claude wrapper variable
- the reviewer's private logs, `~/.cache/reviewer/logs/*-<pass>-merged.log`, read for drafts and verdicts
- exactly three passes, named review, docs and comments
- `<!-- reviewer:` markers and the reviewer's verdict wording

Two reviewer changes then broke the worker:

1. A clean review now writes an LGTM draft, `> **LGTM!**` with emojis. The worker read no verdict from it, and a clean PR would have got three LGTM comments, one per pass.
2. The reviewer skips the code review when its LGTM is already on the head commit. With `--all`, the docs and comments passes still run. The worker saw 2 of 3 passes and marked the review failed.

## Goal

Any reviewer can do the reviews, if it follows a small contract. The current reviewer keeps working as it is today.

## The contract

The worker runs the reviewer CLI in a clone of the PR's repo, checked out at the PR head.

**Arguments.** `reviewer.args`, default `{pr} --all -l high -R {repo}`. The worker fills in `{pr}`, `{repo}`, `{url}` and `{sha}`.

**Environment:**

| Variable | Value |
|---|---|
| `REVIEW_RESULT` | Where the reviewer writes its result |
| `REVIEW_PR`, `REVIEW_REPO`, `REVIEW_PR_URL`, `REVIEW_SHA` | The PR number, `owner/repo`, URL and head commit |
| `REVIEW_CLAUDE` | A `claude` wrapper with posting turned off. The current reviewer reads it under its own variable name. |
| `PATH` | Starts with read-only `gh` and `git`, which refuse anything that changes GitHub |

**Exit code.** 0 means done. 2 means nothing to review: the PR is closed or a draft, it was already reviewed, or another run holds it. Anything else means the review failed.

**Result.** With `reviewer.format` set to `result-json`, the reviewer writes this to `REVIEW_RESULT`:

```json
{
  "status": "reviewed",
  "reason": "",
  "comments": [
    {"pass": "review", "verdict": "lgtm", "body": "<!-- acme:review sha=abc1234 -->\n## Code Review #1\n..."}
  ]
}
```

- `status` is `reviewed`, `skipped` or `failed`. `skipped` works like exit code 2. `reason` says why.
- A reviewed PR has at least one comment. A clean review is an LGTM comment, so every reviewed PR gets one.
- `verdict` is `lgtm`, `mergeable`, `follow-ups` or `not-mergeable`. `lgtm` counts as ready to approve.
- Each `body` is posted as written. Its first line is the marker, `<!-- <reviewer.markerPrefix>:...`. A PR with any comment behind the marker counts as reviewed, so later runs skip it.
- A pass the reviewer skipped has no comment. It is not a failure.

The reviewer never posts. The worker posts each comment, records the verdicts for the stand-up, and sends the one Slack line.

## The current reviewer: the log adapter

`reviewer.format` defaults to the log adapter until the reviewer writes a result file. The adapter reads the reviewer's verified logs as before, with three fixes:

- `> **LGTM!**` reads as ready to approve.
- When every pass is an LGTM, only the code review's comment is posted, so a clean PR gets one LGTM, not three.
- A missing code review pass counts as skipped, not broken, when the PR already has a `<!-- reviewer:review` LGTM for the head commit. That's the case where the reviewer skips it.

The worker's own LGTM ("LGTM 😃⭐😸") stays only as a fallback, for a reviewer that writes no LGTM draft.

## Settings

Settings > Worker > Review worker > "Advanced: use another reviewer" sets the format, the arguments and the marker prefix. Bad values fail only the review job.

## Next steps

1. The reviewer writes `REVIEW_RESULT` when it is set. That's a change in the reviewer's repo.
2. The default `reviewer.format` switches to `result-json`, and the log adapter can then go.

## Files

`cmd/koko-worker/review_contract.go` (contract types, `readResult`, `expandArgs`, LGTM detection), `cmd/koko-worker/jobs.go` (`commentsFor` and the review and log reading functions), `cmd/koko-worker/config.go` (`reviewer.format`, `args`, `markerPrefix`), `frontend/src/components/WorkerSettings.tsx`.
