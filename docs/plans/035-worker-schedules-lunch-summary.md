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

**Settings:** `summary` holds `team` ("org/team-slug"), `repos`, `jira` (default on) and `prose` (default on). An empty team stops only this job and `check summary`, not the other jobs.

**Period:**

- It always ends at the start of today, in the worker's time zone.
- The state keeps `summaryCovered`, the end of the last period a real run covered. The next run starts there, at most 7 days back.
- `dueRuns` drops a missed slot. Because the next run starts at the stored end, a missed day is never lost and never counted twice.
- With no stored end, or with today already covered, it covers the job's previous run day. On a Monday that is Friday to Sunday.
- A test run never moves the stored end.

**GitHub:**

- The members come from the team, without bots, at most 30.
- One `gh api graphql` call covers 5 members. Each member gets two searches: `author:` and `reviewed-by:`, with `updated:>=` the period start, in the repos or the team's org.
- Each search reads at most 50 PRs, with each PR's last 50 reviews. A search that finds more says so in the DM.
- Go keeps PRs merged and PRs opened inside the period, by members.
- A review counts once per PR per person, if it was submitted inside the period. Pending reviews, reviews of your own PR and bots do not count.
- A failed batch adds a note, and the other batches go on.

**Jira:**

- It does not map GitHub logins to Jira accounts.
- It takes one ticket key from each merged or opened PR: from the title, else the branch, else the body.
- One Claude run reads each ticket and its current status. Only keys that exist go on, because one unknown key fails a JQL search.
- A second run makes one search: `key in (...) AND status CHANGED DURING (start, end)`. Moved means a status change inside the period. Jira reads those dates in the account's time zone.
- If the search fails, the DM still shows each status, claims no move, and says why.

**Prose:**

- `claude -p` gets the facts as JSON and no tools, with the same settings as the other runs.
- It writes a short paragraph per person. A paragraph that names a PR or ticket outside that person's facts is dropped.
- If the call fails or times out, the DM lists the facts instead. Go adds every PR link either way.

**Message:** it goes to your Slack DM, as the stand-up does. A header names the days covered, then one section per active person, then one line naming everyone with no activity.

**Card:** the summary's settings sit inside the shared job card, which gives it the switch, days, times, Test and Run now.

## Not in scope

- A cron syntax. Days and times cover the daily and weekly cases.
- Posting the summary to a channel.
- Reviewer runs that sweep repos rather than PRs.
