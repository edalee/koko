# Plan 035: Job days, lunch, reviewer runs, a clearer Worker tab and a team summary

## Problem

- Every job runs Monday to Friday. A job cannot run on fewer days, or once a week.
- Focus time books every free gap, lunch included, unless lunch is on the calendar.
- The review worker has one command and one schedule. A second review with other arguments, such as a weekly one, is not possible.
- The Worker tab is one long page. Shared settings sit apart from the jobs that use them, and the review card mixes how the reviewer runs with which PRs it reviews.
- Nothing tells you what the team did the day before.

## Goal

Four small PRs, in this order. Each one ships on its own.

1. Job days and a lunch blocker.
2. A clearer Worker tab.
3. Several reviewer runs.
4. A team summary job.

Koko stays generic. No code, default, example or help text names a reviewer tool or a company. Examples use `acme`.

## 1. Job days and lunch

**Days:** `JobConfig` gains `days`, a list of `"mon"` to `"sun"`. Empty or missing means Monday to Friday, so an older `worker.json` keeps its schedule.

- `dueRuns`, `nextSlot` and the Mac wake booking use the job's days in place of `isWeekday`.
- Focus time keeps its own weekday rule: it books blocks only on the focus job's days.
- `validate` rejects an unknown day name.
- The Worker tab shows seven day toggles on each job card in place of the fixed "Mon–Fri" label.

**Lunch:** a new top-level `lunch` setting:

| Field | Default | Meaning |
|---|---|---|
| `enabled` | true | Lunch blocks focus time |
| `start` | `"12:00"` | "HH:MM" |
| `end` | `"13:00"` | "HH:MM", after `start` |

`findFocusGaps` treats lunch as a busy interval, the same as a meeting. Lunch is not written to the calendar. The Worker tab shows lunch in the shared settings.

**Tests:** a job on Saturday only, a job once a week, the Mon–Fri default for a job without days, a bad day name, focus gaps either side of lunch, and lunch off.

## 2. A clearer Worker tab

- **General:** Slack, time zone, calendar, Jira site, lunch and Connections.
- **One card per job,** always in this order: what it does, days and times, its settings, then Test and Run now with the card's own output.
- **Review card:** two groups. "Reviewer" is how it runs: command, source, logs, marker. "Which PRs" is Orgs, Excluded repos, My PRs and Team PRs.
- The stand-up card says its Team PRs list uses the review worker's team.
- `WorkerSettings.tsx` splits into a shell and one component per job. Behaviour does not change.

## 3. Several reviewer runs

`reviewer.runs` is a list. Each run has:

| Field | Meaning |
|---|---|
| `name` | Shown in the UI and in status, for example "daily" |
| `args` | Words added after the base command |
| `days`, `times` | Its schedule, as in part 1 |
| `mine`, `team` | Its PR scopes and age limits |
| `markerPrefix` | Its own marker, so its comments count apart from other runs |

- The source, logs folder and base command stay shared.
- Each run keeps its own record of reviewed PRs, so a weekly run reviews a PR the daily run already reviewed.
- Each run is a scheduled job named `review:<name>`, with its own lock, slots and status.
- An older `worker.json` becomes one run named "daily" with the old times and scopes. Nothing changes until you add a run.

## 4. Team summary

A new `summary` job. Default: off, 08:30, Monday to Friday.

- It covers the previous working day of the job's days. On a Monday it covers Friday to Sunday.
- For each member of its team, in its repos: PRs merged, PRs opened, and reviews given.
- If a Jira site is set, it also lists tickets the team moved.
- `claude -p` writes a short paragraph per person from those facts. If that call fails, the DM falls back to plain lists.
- It posts to your Slack DM, as the stand-up does.
- Its card has its own team, repos, days and times.

## Not in scope

- A cron syntax. Days and times cover the daily and weekly cases.
- Posting the summary to a channel.
- Reviewer runs that sweep repos rather than PRs.
