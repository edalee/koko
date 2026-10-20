# Plan 035: Job days, lunch, reviewer runs, a clearer Worker tab and a team summary

## Problem

- Every job runs Monday to Friday. A job cannot run on fewer days, or once a week.
- Focus time books every free gap, lunch included, unless lunch is on the calendar.
- The review worker has one command and one schedule. A second run with other arguments, such as a weekly one over repos, is not possible.
- The Worker tab is one long page. Shared settings sit apart from the jobs that use them, and the review card mixes how the reviewer runs with which PRs it reviews.
- Nothing tells you what the team did the day before.

## Goal

Four small PRs, in this order. Each one ships on its own.

1. Job days and a lunch blocker.
2. A clearer Worker tab.
3. Several runs of the reviewer command, for PRs or for repos.
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

## 3. Several runs of the reviewer command

`reviewer.runs` is a list. Each run has:

| Field | Meaning |
|---|---|
| `name` | Shown in the UI and in status, for example "daily". Up to 32 lowercase letters, digits and dashes, unique |
| `args` | Words added after the base command |
| `enabled`, `days`, `times` | Its schedule, as in part 1 |
| `target` | `"prs"` (the default) or `"repos"` |
| `mine`, `team` | PRs runs: its PR scopes and age limits |
| `repos` | Repos runs: the `"owner/repo"` names to work on |
| `timeoutMinutes` | The limit for one PR or one repo. Default 45 for PRs, 60 for repos |

- The source, logs folder, base command, marker, orgs and excluded repos stay shared.
- Each run is a scheduled job named `review:<name>`, with its own lock, slots and status. The name is part of the lock file's name, so it is short.
- `koko-worker run review:<name>` runs one run. A bare `run review` runs the first PRs run that is on, else the first PRs run. It never starts a repos run: with no PRs run, it fails and says so.
- Runs take one shared lock, so only one run works at a time. They share the managed reviewer clone and the claude wrapper.
- An older `worker.json` becomes one PRs run named "daily", with the old `jobs.review` times, days and switch, and the old scopes. Nothing changes until you add a run. The worker then ignores `jobs.review`, and the UI writes only `reviewer.runs`.
- The stand-up uses the team, repos and age limit of the first PRs run that is on, else the first PRs run. It never uses a repos run. With no PRs run, it lists no team.

**Decisions** (from the reviews of this part):

1. **Clones for repos runs.** Repos runs work in clones under `~/.cache/koko-worker/repos-runs`, one per repo, which every repos run shares, each with its own lock. PR reviews never use them. Before each repo, the worker unsets `core.hooksPath` and deletes the clone's hooks, all but git's samples, so no hook a tool left runs on the next checkout. If any git step of the checkout fails, the worker deletes that clone, so the next run clones it fresh.
2. **No repo runs twice in a slot.** A scheduled run saves each repo once its tool starts, and saves how it ended. A retry of the slot runs only the repos whose tool never started. A repo the run was stopped in counts as failed and does not run again, because the tool may have pushed part of its work.
3. **A failed clone or fetch is retried.** If the worker cannot get a repo, for example with the network down, it saves nothing for that repo, runs the others, and returns an error the slot retries. The retry runs that repo. The DM waits until every repo has run. Only a repo whose tool started can be final.
4. **A failed DM is retried.** A DM that fails, online or not, makes the slot retry. The retry finds every repo saved, so it only sends the DM. The DM goes out once per slot.
5. **Run now** never reads or writes the slot's repo records, and never marks the DM sent. It runs every repo and sends its own DM. A Run now on a held lock fails at once with "another run is working, try again when it ends". Scheduled runs wait. Its time limit in the app is the run's repos times its limit per repo, plus 10 minutes, and never less than 4 hours. On that limit, koko-worker gets SIGTERM and stops the tool's process group too.
6. **`{claude}` in a repos run** is the review wrapper. It denies some commands, such as `git push`, but it is no guard. A repos run gets the real gh and git on purpose, because its tool opens PRs. ADR-034 says so.
7. **A bare `run review`** starts only a PRs run, as above.
8. **Long repos runs.** The scheduler starts a due repos run in its own process (`koko-worker run review:<name> --scheduled`), so the stand-up and focus time run on time. It still takes the run's lock. In a tick, the other due jobs run first. A PRs run that comes due while a run holds the shared lock waits for a later tick.
9. **Stopping a repos run.** Before each repo, a scheduled run reads `worker.json` again. It stops if you switched the worker off, switched the run off or removed the run, sends a DM with the repos done so far, and the slot ends there. `koko-worker uninstall`, which switching the worker off in Koko calls, also stops the run's process. The run writes its pid and start time to a file next to its job lock, and uninstall sends SIGTERM to that process group. A restart of the agent lets the run go on.
10. **A bad or repeated run name, or a bad target,** fails only that run and `check reviewer`, as plan 030 says for bad reviewer values. A run with a bad name gets no job at all, because the name would make a bad lock file name. A run named like an earlier one gets no job either, because it would share that job. The UI does not save either.

**PRs runs** work as the single review job did: once per PR, with the shared marker, the record of reviewed PRs and the comments. Runs share that record, so a PR is reviewed once, whichever run reaches it first.

**Repos runs** are for a tool that changes repos, such as one that updates dependencies:

- The command runs once per repo, one after another. Its working folder is the repos runs' clone of the repo, reset to the newest commit of the default branch, with no stray files and no hooks.
- `{reviewer}`, `{claude}` and `{repo}` are filled in. `{pr}`, `{url}` and `{sha}` name a PR, so the worker refuses a repos run whose command uses them.
- The environment carries `REVIEW_REPO` and `RUN_KIND=repos`. gh and git are not read-only, so the tool can push and open PRs.
- Exit 0 means done, 2 means nothing to do, anything else failed. The output goes to a log per repo as it comes. The worker keeps only its last line in memory, for the DM.
- After the run, one Slack DM lists each repo as done, nothing to do or failed, with up to 5 links from its output, the last ones, and the log of a failure.
- No markers, review records or comments.
- A test run prints the command for each repo and runs nothing.

Example: a "weekly" repos run on Mondays at 08:00 with the arguments `--update-deps`, over `acme/api` and `acme/web`.

## 4. Team summary

A new `summary` job. Default: off, 08:30, Monday to Friday.

**Settings:** `summary` holds `team` ("org/team-slug"), `repos`, `jira` (default on) and `prose` (default on). An empty team stops only this job and `check summary`, not the other jobs.

**Period:**

- It always ends at the start of today, in the worker's time zone.
- The state keeps `summaryCovered`, the end of the period of the last summary sent as a DM. A test run never moves it.
- The next run starts at the stored end, at most 7 days back. `dueRuns` drops a missed slot, but a gap of up to 7 days still loses no day and counts none twice.
- After a longer gap, the summary covers the last 7 days. Its DM names the days it leaves out.
- With no stored end, it covers the job's previous run day. On a Monday that is Friday to Sunday.
- A second summary on the same day, such as a second Run now, covers the same previous run day again.
- If any GitHub batch fails, no DM goes out and the stored end stays. The scheduler tries again, and a later run covers the period.

**GitHub:**

- The members come from the team, without bots, at most 30.
- One `gh api graphql` call covers 5 members. Each member gets two searches: `author:` and `reviewed-by:`, with `updated:>=` the period start, in the repos or the team's org.
- Each search reads at most 50 PRs, with each PR's last 50 reviews. A search that finds more says so in the DM.
- Go keeps PRs merged and PRs opened inside the period, by members.
- A review counts once per PR per person, if it was submitted inside the period. Pending reviews, reviews of your own PR and bots do not count.
- A failed batch fails the whole run, as the period rules say.

**Jira:**

- It does not map GitHub logins to Jira accounts.
- It takes one ticket key from each merged or opened PR: from the title, else the branch, else the body.
- One Claude run reads each ticket and its current status. Only keys that exist go on, because one unknown key fails a JQL search.
- A second run makes one search: `key in (...) AND status CHANGED DURING (start, end)`. Moved means a status change inside the period. Jira reads those dates in the account's time zone.
- If the search fails, the DM still shows each status, claims no move, and says why.
- The DM says "Could not read some tickets" only if a ticket read returns an error. A key that comes back without a result and without an error gets no note.

**Prose:**

- `claude -p` gets the facts as JSON and no tools, with the same settings as the other runs.
- It writes a short paragraph per person.
- A paragraph may name the person's PR refs, any Jira-like key in their PRs' titles and branches, the keys their PRs name, and their tickets.
- A paragraph that names anything else is dropped. That person gets plain lists, and the DM says so.
- If the call fails or times out, the DM lists the facts instead. Go adds every PR link either way.

**Message:** it goes to your Slack DM, as the stand-up does. A header names the days covered, then one section per active person, then one line naming everyone with no activity.

**Card:** the summary's settings sit inside the shared job card, which gives it the switch, days, times, Test and Run now.

## Not in scope

- A cron syntax. Days and times cover the daily and weekly cases.
- Posting the summary to a channel.
