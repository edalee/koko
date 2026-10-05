# Plan 029: koko-worker, scheduled work jobs built into Koko

Status: merged to `main` in #20 (commit `e9d32b4`, 30 Sep). The wake test and the other items under "Still to do" are open.

**Where the code differs from this plan.** The code is the source of truth. The sections below keep the plan as written.

- **Config file:** the architecture diagram shows a `worker` section in `config.json`. The code uses its own `worker.json`, as "Keeping clear of other Koko work" says.
- **Slack:** the backend table says `slack.go` DMs "through Koko's Slack token". The worker uses its own bot token and user ID from `worker.json`.
- **Tono scope:** the Tono section describes only the team scope. The code has two switches, `tonoMine` and `tonoTeamPRs`, plus `tonoRepos`, as "Done on 30 Sep" describes.
- **Run now with a PR URL:** only the CLI has it (`koko-worker run tono --pr URL`). The Settings UI's "Run now" takes no URL.
- **Connection checks:** there are seven, not five. `claude` (the CLI on the worker's PATH) and `tono` (the CLI path) join Slack, GitHub, Jira, calendar and wake.
- **Sudoers rule:** it allows `pmset schedule cancel wake` as well as `pmset schedule wake`. The worker cancels an old wake when the times change, and on uninstall.
- **Focus window:** the default window starts at 09:00, not 09:15. The job still runs at 09:15 and books from then onwards.
- **Deny list:** it is wider than the plan's list. It also refuses `Write`, `Edit`, `NotebookEdit`, Jira issue creation, and calendar delete, update and respond.

See ADR-029 to ADR-035 in `docs/architecture/decisions.md` for the decisions behind the build.
Replaces `baldrick-work/docs/plans/001-scheduled-jobs.md`.

## Problem

The work assistant lives in a separate Python repo, `baldrick-work`. It has its own Slack app and its own settings in `.env`. It reads the calendar through EventKit (the macOS calendar framework), which a launchd job can use only through a signed helper binary.

Koko already has a Slack connection, a settings panel and a Claude session model. Two tools do half the job each.

## Goal

- The baldrick-work code moves into Koko and becomes **koko-worker**.
- koko-worker is built into Koko. One switch in the UI turns it on or off.
- Each job has its own switch and times in the UI.
- Connections (Slack, Google Calendar, GitHub, Jira) are set up and checked in the UI.
- The calendar uses the Google Calendar claude.ai connector (the Google Calendar tool Claude can use), not EventKit.

## Non-goals

- No new jobs beyond the three below.
- No cloud hosting. Everything runs on the Mac.
- No changes to Jira. On GitHub, the only write is tono's review comments on the PRs it reviews, posted as you. Otherwise jobs only read, except for booking focus blocks.

## Jobs

| Job | Time (Mon to Fri) | What it does |
|---|---|---|
| Stand-up | 07:00 | DMs today's meetings, your approved PRs ready to merge with a Jira check, follow-up steps, PRs waiting for your review, and team PRs. |
| Focus | 09:15 | Books "Focus" blocks in free gaps of 30 minutes or more, 09:15 to 17:00. |
| Tono | 09:30, 12:00, 14:00 | Reviews each PR once: your own, and the team's (see "Tono"). It posts the review on the PR, or "LGTM 😃⭐😸" when there is nothing to report. |

The times are defaults. The UI can change them.

### Stand-up

1. **Today's meetings.** This replaces the 08:30 calendar DM from baldrick-work.
2. **Ready to merge:** your open PRs that are approved, not drafts, mergeable, with every check green. Claude reads each Jira ticket and says whether the PR covers it. If it does, the ticket can be closed after the merge. For approved PRs that are not ready, it says what blocks them.
3. **Follow-up steps** for each ready PR.
4. **Needs your review:** open, non-draft PRs that request your review, oldest first. Your own PRs and bot PRs (Dependabot, Renovate) are hidden.
   - It opens with a summary: how many PRs wait, how many tono has reviewed at their current commit (code, docs and comments), and of those how many look ready to approve, have follow-ups, or are not mergeable.
   - Each PR shows its tono verdict, "not reviewed yet", or "outside tono's scope" with the reason. "Not reviewed yet" counts only PRs in scope.
   - A reviewed PR links to tono's comment on the PR.

**Layout:** the DM uses Slack blocks, with a divider between sections. Long sections are split at line breaks under Slack's 3,000-character limit per block, and a message that needs more than 50 blocks continues in a second message.

Go fetches the PR lists with `gh`, so the links are exact. Claude only does the judgement: ticket coverage and follow-up steps. (In the baldrick-work test, Claude built the list and gave one PR the wrong link.)

### Focus

- Weekdays only. Gaps of 30 minutes or more, from 09:15 to 17:00.
- Declined events and events marked "free" do not block. Tentative and unanswered invites block.
- An all-day event marked busy, or titled like time off (OOO, holiday, leave, sick, VAB), blocks the day.
- A late run fills from now onwards, starting on the next 5 minutes.
- Blocks are plain busy events titled "Focus" and tagged `[koko-worker:focus]` in the description. They count as busy, so a second run books nothing. The stand-up leaves them out of the meeting list.
- They are not Google's "Focus time" type. That type can decline new invites by itself, and the connector can neither show nor change that setting.
- Google's own `OUT_OF_OFFICE` events block their time, like any busy event.
- One DM lists the booked blocks.

### Tono

- **Scope:** tono reviews only PRs opened by a member of `epidemicsound/content-protection` in the last 4 days (`tonoTeam`, `tonoMaxAgeDays`). The members come from GitHub at every run.
- **One review per PR:** a PR tono has reviewed, at any commit, is not reviewed again. Neither is a PR that already carries a tono comment, for example from a teammate who ran tono with `-c`.
- **Sources:** your own open PRs, then the PRs waiting for your review (people, not bots), oldest first. The stand-up summarises the reviews and links each one.
- **"Run now" with a PR URL** reviews that PR whatever the scope says.
- The verdict comes from the draft PR comment in each verified pass: mergeable, mergeable with follow-ups, or not mergeable. "Ready to approve" means every pass says mergeable or mergeable with follow-ups, and the code review says mergeable.
- Results are keyed `repo#number@sha`, so the stand-up can say when a PR has new commits since its review.
- The review runs in a cache clone at the PR head, never in your working clones.
- It calls the tono CLI directly: `tono <number> --all -l high`. The CLI posts nothing to GitHub without `-c`.
- The CLI grants its own Claude run `Bash(gh:*)` and `Write`. The worker never passes `-c`, and its `TONO_CLAUDE` wrapper denies the posting commands as a second guard.
- **Posting:** the worker's Go code posts the review, not tono's Claude. It takes the draft PR comment from each pass's verified output (tono's `*-merged.log` files), found by its `<!-- tono:` marker, and posts each draft as its own comment with `gh pr comment`.
  - If all three passes ran and none wrote a draft, it posts "LGTM 😃⭐😸" behind `<!-- tono:lgtm sha=… -->`.
  - A pass counts as finished only if its output reads as a review: three or more lines, with a heading or a list. A broken run leaves one error line, such as "API Error: Can't reach the API server". It posts nothing, not even the drafts, if a pass did not finish. It also posts nothing if a draft has no footer, if a marker line has no draft it can read, or if the verdict is bad and no draft exists. Each of these counts as a failed review. A pass that broke, offline or not, marks nothing: the run reports it, and a later run reviews the PR again.
  - Comments are saved as unposted before the first post, and each post is saved as it lands, so a kill between posts loses nothing. The next tono run posts what is left. Before every post, first or retry, it checks that the PR is still open at the reviewed commit. If not, it drops the comments and lets the PR be reviewed again. It then checks the PR for each comment's marker line, so a post that timed out after it went through, or a `--pr` run at the same commit, is not posted twice. If part of a review was posted before the PR changed, a DM says to use "Run now" with its URL. A failed post counts one try, unless the internet is down. After 3 tries the review counts as failed, and a DM says so.
  - The comments post as you. Each one says in its footer that tono and Claude wrote it.
- A review of your own PR that fails for a reason other than the network sends one DM warning. It is not retried for the same commit.
- A network failure marks nothing, so the scheduler's retry reviews that commit once the internet is back.
- If another tono run holds the PR's lock, the PR is left for the next slot.
- One review per cache clone at a time, so a "Run now" never checks out another PR under a review in progress.
- A review is capped at 45 minutes. On timeout, tono's whole process group gets SIGTERM, so tono's trap removes its lock. Anything still running a minute later gets SIGKILL.
- **Read-only tools:** tono's runs get their own `gh` and `git` first on the PATH. They refuse any command that changes GitHub, including `gh api` calls that send data, and any `git push`. The deny list alone matches only command prefixes.
- A first clone gets 15 minutes. A failed clone is removed, so it cannot break later reviews.
- The Test button reviews one PR only.

## Architecture

```
Koko app (UI)                      koko-worker (background)
  Settings > Worker                  launchd agent, KeepAlive
   - on/off, per-job on/off           - reads worker config each minute
   - times                            - runs due jobs, catches up missed ones
   - connection checks                - books the next Mac wake (pmset)
   - run now, last runs + logs        - writes run history
        |                                   |
        +---- ~/Library/Application Support/koko/config.json (worker section)
        +---- ~/Library/Application Support/koko/worker/ (state, history, logs)
```

- **koko-worker** is a Go binary in the Koko repo, `cmd/koko-worker/`, like `koko-cli`.
- **It runs without the Koko window.** launchd keeps it alive, so jobs run with Koko closed.
- **Switching on** in the UI installs and starts the launchd agent. **Switching off** stops and removes it.
- **One agent with its own scheduler**, not one launchd agent per job. Time changes in the UI then need no plist rewrite.
- **Catch-up:** every 30 seconds the worker checks which jobs were due today and have not run. A job missed while the Mac slept runs on wake.
- **Changed times:** a run time counts only on days when it was set before the slot came round. So moving a time to earlier than now, or switching a job on after its time, waits for the next day.
- **One run per job at a time:** the scheduler and "Run now" share a lock per job. A stand-up run by hand at 06:55 therefore replaces the 07:00 one.
- **Staying awake:** while jobs run, `caffeinate` stops the Mac idle-sleeping. It cannot stop the sleep that closing the lid causes on battery.
- **No internet:** a job waits and retries until it connects, then sends. It is never skipped. Only a real loss of internet counts, not an error that mentions the network. The next wake is still booked while offline.
- **Other failures:** a job is tried 3 times, 5 minutes apart, then you get one warning DM. Network retries do not count towards the 3.
- **Stand-up sections fail on their own.** If Jira, the calendar or a PR list fails, that section says so and the rest still arrives.
- **"Run now"** is recorded against today's slots, so the scheduler does not repeat it. A stand-up run by hand at 06:55 replaces the 07:00 one.
- **State** (`state.json`) is changed under a file lock, so the scheduler and a "Run now" never overwrite each other.

### Claude runs

- Jobs that need judgement or connectors run `claude -p` with the prompt on stdin.
- `--permission-mode dontAsk` refuses any tool not on the allowlist, so a run never waits for an answer.
- Runs use `--no-session-persistence`, so they never appear in Koko's conversation list.
- Runs use isolated settings (`--setting-sources ""` plus a worker settings file). Tested: Jira, `gh` and the calendar still work. Otherwise they inherit your user allow rules, hooks and plugins, and the cache clone's project settings.
- Each job has a tight allowlist. Writes are denied explicitly: `gh pr merge|edit|comment|review|close`, `gh api`, `git push|commit`, and the Jira edit, comment and transition tools.
- The focus job may use only the Google Calendar list and create tools.

### Calendar through the Google connector

- **Focus, step 1:** `claude -p` calls `list_events` for today and returns the events as JSON.
- **Step 2:** Go works out the gaps. This part is pure and unit tested.
- **Step 3:** `claude -p` calls `create_event` once per gap, with exact times.
- **Stand-up:** the same `list_events` call gives today's meetings.
- This removes the EventKit helper, the signed binary and the calendar permission problem.
- **Tested under launchd (27 Sep):** list, create and delete all work. Events carry your response status (`attendees[self].responseStatus`) and an event type. `create_event` takes a description, a time zone, `availability` and `eventType: FOCUS_TIME`.
- Go reads the raw `list_events` result from `--output-format stream-json`, so no event data is retyped by Claude.

### Slack

- Koko's own `slackToken` and `slackOwnerId` are empty, so the worker has its own Slack settings.
- You enter a bot token and your Slack user ID in the Worker UI. The baldrick-work Slack app's bot token works for this.
- They are stored in the worker's own config file, mode 0600, like Koko's config.

### Waking the Mac

- launchd cannot wake a sleeping Mac. `pmset schedule wake` can, but it needs root.
- A sudoers rule lets koko-worker run only `pmset schedule wake` without a password. You add it once with `sudo visudo -f /etc/sudoers.d/koko-worker`. The UI shows the exact line and checks that it works.
- After each run, the worker books the next wake, two minutes before the next job. With the default times: 06:58, 09:13, 09:28, 11:58, 13:58.
- **Doubt:** a MacBook with the lid closed may wake only briefly, or only on power. This needs a test on your Mac. If the wake fails, the job runs when the lid opens.

## Keeping clear of other Koko work

Another session is changing Koko's sessions and UI. The worker must not affect that work.

- **Separate checkout:** all worker work happens in the worktree `~/Projects/personal/koko-worker` on `feat/koko-worker`.
- **Own config file:** `~/Library/Application Support/koko/worker.json`, not `config.json`. Koko's `SaveConfig` rewrites the whole `config.json` from its struct, so a `worker` key there could be wiped. This also avoids edits to `config_service.go` and `types.go`.
- **Own runtime:** the worker never calls Koko's API and never touches Koko's sessions. It has its own launchd label, `com.koko.worker`, and its own state directory.
- **Stages:** the worker module is new files only (`cmd/koko-worker/`). The Koko app side uses new files (`worker_service.go`, `WorkerSettings.tsx`). The only edits to shared files are a few lines in `main.go`, `SettingsPanel.tsx` and `Makefile`. They come last, after the other branch merges.
- **The tono CLI** starts its own Claude runs. The worker sets `TONO_CLAUDE` to a wrapper that adds `--no-session-persistence` and denies the posting commands (`gh pr comment`, `gh pr review`, `gh pr merge`, `gh pr edit`, `git push`).

## Backend

koko-worker is a nested Go module, like `koko-cli`. All worker logic lives there. The Koko app only manages the launchd agent and calls the worker binary.

| File | Change |
|---|---|
| `cmd/koko-worker/main.go` | Subcommands: `serve` (scheduler loop), `run <job> [--test [--date]]`, `check [name]`, `status`, `install`, `uninstall`. |
| `cmd/koko-worker/config.go` | `worker.json`, defaults and paths. |
| `cmd/koko-worker/calendar.go` | Google events, gap finder, list and create through the connector. |
| `cmd/koko-worker/github.go` | PR lists, readiness, bot filter, and posting tono's PR comments: the worker's only GitHub write. |
| `cmd/koko-worker/launchd.go` | Agent install and removal, Mac wake booking. |
| `cmd/koko-worker/check.go` | Connection checks for the UI. |
| `cmd/koko-worker/scheduler.go` | Due and catch-up logic, per-job per-day run record on disk, wake booking. |
| `cmd/koko-worker/jobs.go` | Stand-up, focus and tono jobs. Gap logic as a pure function. |
| `cmd/koko-worker/claude.go` | `claude -p` runner: stdin prompt, isolated settings, allowlist, deny list, timeout. |
| `cmd/koko-worker/slack.go` | DM through Koko's Slack token. |
| `worker_service.go` | Wails-bound: config, on/off, run now, status, checks, log. It calls the worker binary, which copies itself to Application Support and installs or removes the agent. All methods pass JSON strings, so the shared `models.ts` does not change. |
| `frontend/wailsjs/go/main/WorkerService.*` | Bindings, in the generated format. |
| `frontend/src/components/WorkerSettings.tsx` | The Worker section. |
| `Makefile` | `build-worker` and `test-worker`, wired into `make test` and `make check`. |

## Frontend

A new **Worker** section in `SettingsPanel.tsx`:
- **Main switch:** on or off, with the agent's state (running, stopped, error).
- **Jobs:** a switch, times and a "Run now" button per job. Shows the last run's time, result and log.
- **Worker config** is read from and written to `worker.json` through `worker_service.go`.
- **Connections:** one row each for Slack, Google Calendar, GitHub (`gh`), Jira and Mac wake. Each has a "Check" button with a plain result, such as "Google Calendar: connected, 6 events today". Where a connection is missing, the row says how to fix it.

## What moves from baldrick-work

| baldrick-work part | In Koko |
|---|---|
| Scheduled jobs (unbuilt draft) | Rewritten in Go as koko-worker. The Python draft is reference only. |
| 08:30 calendar DM | Replaced by the stand-up. |
| Koko approval DMs | Koko already knows the approval state, so it can send these itself. |
| Slack DM chat bot, `/today`, `/koko`, Claude brain, SQLite memory | Out of scope. A possible follow-up plan. |
| EventKit calendar reader | Dropped, replaced by the Google connector. |

Your uncommitted work in baldrick-work (the 08:30 briefing loop) stays there until baldrick-work is retired.

## Testing

- Unit tests for gap finding (the 14 cases from the Python draft), catch-up scheduling, wake times and PR filtering.
- Each job run once under launchd in a test mode that prints instead of DMs.
- One real focus block booked on a weekday, checked in Google Calendar, then deleted.
- One wake tested with the lid closed.

## Progress (27 Sep)

Built on `feat/koko-worker`, not committed:
- **Worker module:** all three jobs, the scheduler, checks, install and wake booking. 45 unit tests pass. `go vet` and `golangci-lint` are clean.
- **Tested under launchd:** checks, the stand-up (exact links, bots hidden, Jira verdicts), focus against Monday's real calendar, and tono on kalimba#28 (nothing posted). One real focus block was booked through the worker's own code on 15 January 2027, then deleted.
- **Tested directly:** install, status and uninstall of the agent.
- **Checked:** worker and tono runs save no conversations under `~/.claude/projects`.
- **Second tono review (fixed in the third commit):** `WorkerService` added to `Bind`, changed times wait for the next day, a job lock shared with "Run now", wakes booked while offline and not lost on shutdown, reinstall only for an idle running agent, read-only `gh` and `git` for tono, `caffeinate` during jobs, a longer first clone, and six more code comments corrected.
- **Tono review of the branch (fixed in the second commit):** empty UI values no longer wipe defaults, a tono timeout no longer leaves its lock, one review per cache clone, the worker sets its own PATH, a stale agent copy is reinstalled, network waits are labelled apart from failures, and six code comments are corrected.
- **Review fixes:** plain busy focus blocks, locked state, retries, a stand-up that degrades per section, tono's first-run baseline, wake status set only on success, and a working directory for the agent.
- **Koko app side:** `worker_service.go`, the bindings and `WorkerSettings.tsx`. `go vet`, `tsc` and `biome` are clean. Not yet visible in the app, because nothing is hooked in.

Done since:
- The Worker section shows in Settings, with fields for tono's team and window, and focus time's minimum gap.
- `make build-worker`, `make test-worker`, and the worker in `make dev`, `make test` and `make lint`.
- `make build` puts `koko-worker` inside `Koko.app`, next to Koko's own binary.
- The Slack bot token and user ID are copied from baldrick-work.
- End-to-end test of the serve loop: one stand-up slot, one DM, one state record.

Done on 30 Sep:
- The Worker section was tested in the app: switch on and off, "Test", "Run now" and "Check all".
- Settings fills the window, with a tab for each section. The panel is nearly opaque, so it is easier to read.
- Every toggle knob now stays inside its track.
- A timed invite from midnight to midnight shows as "All day" in the stand-up.
- The Slack tab is hidden. It held the old Slack DM bot, which lets you control Koko sessions by DM.
- Tono has two separate settings, each reviewed once per PR:
  - **My PRs** (`tonoMine`): every open PR you opened, whatever the team or age.
  - **Team PRs** (`tonoTeamPRs`): PRs opened by `tonoTeam` members in the last `tonoMaxAgeDays` days. It covers PRs that ask for your review, plus every open PR in `tonoRepos`. You add and remove those repos in Settings.
  - The old `tonoOwnPRsOnly: true` reads as Team PRs off.
- The stand-up has a "Team PRs" section for team PRs in `tonoRepos` that do not ask for your review.
- Tono's reviews go on the PR, not to Slack:
  - The worker posts each pass's draft comment on the PR, through your `gh` auth. Tono's own Claude stays read-only.
  - If every pass ran and none wrote a draft, the worker posts "LGTM 😃⭐😸" behind a `<!-- tono:lgtm -->` marker. Every posted comment carries a `<!-- tono:` marker, so later runs skip the PR.
  - Comments that fail to post are retried at the next tono run, up to 3 tries. Then the review counts as failed.
  - The stand-up lists each reviewed PR with its verdict and a link to the review. The stand-up thread and the review DMs are gone. A failed review of your own PR still comes by DM.
  - "Test" prints the comments instead of posting them.

Done on 1 Oct: the stand-up's Jira facts come from Go, not Claude.
- A stand-up claimed that clarinet#14's ticket covered "all DRM services", and it named kalimba alert rules that neither the ticket nor the PR mentions. CON-119 has only a title.
- Go picks each PR's ticket key: the first in the title, then the branch, then the body.
- One Claude run only calls `getJiraIssue`. Go reads the summary, description, status, assignee and link from the tool's raw result. A moved ticket is matched under the key the PR names, and the line says "PDDI-778, now CON-166".
- A second Claude run gets the ticket text and no tools. Every claim must quote the ticket. Go keeps a verdict only if its quotes are in the ticket, and a follow-up only if its quote is in the ticket or the PR. Anything else reads "unknown".
- A ticket with no description reads "unknown", without asking Claude. A ticket Go could not read says so.
- Each ticket links to Jira and shows its status and assignee.
- A new section, "Stories you can close", lists tickets assigned to you that are not done. Each one is covered by a PR merged in the last 7 days ("Now") or a PR ready to merge ("After the merge"). "Yours" means the ticket's assignee matches the account the Jira connector runs as.
- One approved PR that fails to load no longer hides the rest. Each failed PR is tried once more, then gets its own line with the reason. A `gh` call that times out says so, instead of an empty error.

Done on 2 Oct:
- Released as 0.5.0. A release now builds `koko-worker` into `Koko.app`, and `koko-worker version` reports the build.
- Once a review is on the PR, one Slack line says so: the PR, the verdict and a link to the review. It goes once per review (`TonoResult.Pinged`), even if posting finishes on a retry.
- "Needs your review" lists only PRs opened by the team. The rest are counted in one line. Without the team's members, every PR is listed.

Done on 5 Oct:
- Tono reviewed Tools#191, a PR of yours from May 2022, in a repo that has since been archived. "My PRs" had no age limit, and the PR searches did not leave out archived repos. GitHub then refused the comment three times over a day.
- "My PRs" now reviews only PRs opened in the last `tonoMineMaxAgeDays` days, default 14. "Run now" with a PR's URL still reviews any PR.
- Every PR search passes `--archived=false`.
- A post GitHub refuses for good (an archived repo, a locked PR) fails at once, with one DM.
- `install` waits for the old agent to unload before starting the new one (0.5.2).

Still to do:
1. You add the sudoers rule. Then a wake test: Mac asleep, wake, slot, DM.
2. Turn off baldrick-work's launchd agent.
3. Drop or rebuild the old Slack DM bot (`slack_commands.go`). See the last item under "Out of scope". The Slack tab stays hidden until then.
4. Make the GitHub tab in Settings easier to use. It holds the followed repos and the hidden PRs.

## Rollout

1. Done: the Google Calendar and Jira connectors work in headless `claude -p` under launchd.
2. Build the koko-worker module: config, scheduler, Claude runner, Slack, and the stand-up job, with `run --test` and `check`.
3. Add the focus job.
4. Add the tono job.
5. Add wake booking and the sudoers check.
6. Add the Koko app side (`worker_service.go`, `WorkerSettings.tsx`). Hook it into shared files after the other branch merges.
7. Turn off baldrick-work's launchd agent when you say so. Its chat bot stays until the follow-up plan decides.

## Decisions taken

1. Port to Go, as a nested module like `koko-cli`.
2. The chat bot is out of scope for this plan.

## Out of scope (future plans)

- **A strategy for Dependabot PRs.** Approve a bump when it is appropriate, and close it when it is not relevant. Needs rules first, because approving and closing act on GitHub under your account:
  - which bumps may be approved: patch and minor, green checks, no breaking-change notes, repos with tests;
  - which get closed: the dependency is no longer used, or a newer bump supersedes it;
  - whether tono reviews a bump before approval;
  - an approval limit per day, and a DM listing every action taken.

- **The Slack chat bot.** It has not worked well, because its Slack and Koko connections are hard to keep alive. If a follow-up plan solves that, the bot may move into koko-worker or merge into Koko's Slack handler.

- **Koko's old Slack DM bot: drop it or rebuild it.** `slack_commands.go` polls Slack DMs and runs Koko commands. Its settings sit in the hidden Slack tab (`SettingsPanel.tsx`). It stays idle while no token is saved. A follow-up plan decides between two options:
  - **Drop it:** delete `slack_commands.go`, its start call in `main.go`, the Slack tab, and `slackToken` and `slackOwnerId` in the config.
  - **Rebuild it:** one Slack bot for Koko and koko-worker, with a single token, and a connection that survives sleep and restarts.
