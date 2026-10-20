# Plan 036: A clean summary for each session

## Problem

It is hard to see what each session was last working on.

- `GetLastMessage(dir)` reads the newest transcript in the folder, not the tab's own. Two tabs in one repo, or a headless `claude -p` run there, take over the preview.
- It shows Claude's last text reply. That is often an answer to a `!` command, a sub-agent report or a notification, not what you asked.
- `lastMsg` updates only when a tab closes or its process exits.
- The session list does not show it at all.
- The conversation picker lists headless runs (an entrypoint such as `"sdk-cli"` or `"sdk-go"`) as conversations you can resume.

## Goal

Each tab shows what its own conversation is about, built only from what you asked and from the summaries Claude Code writes. Headless runs never appear as a tab's conversation, in the history or in the picker. A saved tab that points at one keeps its directory but loses the run.

## Sources

Claude Code writes these records to `~/.claude/projects/<dir>/<uuid>.jsonl`. Koko reads only these.

| Field | Record | Rule |
|---|---|---|
| Title | `ai-title` | The last one in the 2MB tail. Claude rewrites it many times. With none there, the first `ai-title` in the head, then the first prompt you typed (a human-origin user record, as for recent prompts) |
| Recap | `system` with `subtype: "away_summary"` | The last one's `content` |
| Recent prompts | `user` with `origin.kind == "human"` | The last 10, not `isMeta`. Tool results, `!` commands, `/` commands, notifications and messages from other sessions have no human origin, so they drop out |
| Last prompt | `last-prompt` | Its `lastPrompt`. Falls back to the newest recent prompt |
| Branch | `gitBranch` on the newest record | |
| PR | `pr-link` | The last one: number, repo and URL |
| Ticket | none | The first Jira key, `[A-Z]{2,10}-\d+` with no letter or digit either side, found in the branch, then the PR, then the title. The recent prompts, newest first, are searched only after those. `_`, `/` and `-` all count as edges. A project in the denylist is never a ticket |

## Design

### Backend

- `ClaudeService.GetSessionSummary(dir, uuid)` returns the summary of that one transcript. With no UUID it returns an empty summary. It never falls back to the newest file.
- It reads the file backwards in 64KB chunks. It stops once it has the title, recap, branch and 10 prompts, or at the 2MB budget. It then takes a PR link only from the lines already read.
- A cache keyed on path, size and modification time returns the last result while the file is unchanged.
- A transcript whose first records name an entrypoint starting `sdk-` is a headless run. `cli` and any other value are interactive. A file with no entrypoint is treated as interactive, as before.
- Finding the entrypoint reads the first 8KB of the file. Only a file that opens with a longer record gets a full head read. The entrypoint is cached once found, because the first records never change once written. Beyond that, a summary reads the head only if the title is still missing.
- A UUID with no file gives an empty summary marked not found. The frontend then keeps the title, recap and ticket it saved.
- `GetLastMessage`, which read Claude's last reply, is removed. Nothing calls it.
- The app gets its first Jira key helper. The worker's pattern is separate and looser, with no denylist, because the worker is its own Go module.
- A key's project is 2 to 10 letters. These common terms are never a ticket: UTF, SHA, ISO, GPT, CVE, RFC, HTTP, TLS, SSL, MD, AES, RSA, ECMA, UTC, GMT, PEP, IEEE, ES and IE.

### Frontend

- **Session list:** each row gets a second line: the ticket as a small chip, then the title. With no title, the last prompt.
- **Hover card:** a small card on hover or focus, after a short delay. It shows the recap, branch, PR and ticket, then the last 10 prompts with their age. It replaces the native `title` tooltip on rows.
- **Reconnect card, history and picker:** they show the title and recap in place of Claude's last reply. A record with no summary shows its name only. Koko no longer shows the old `lastMsg`.
- **Live:** the list fetches summaries for open tabs every 15 seconds while Koko is visible, and when a tab goes idle. The cache makes an unchanged file cheap.
- **Saved history:** `SessionRecord` keeps the title, recap and ticket at close, so a closed session still shows them.
- **Reconnect:** a reconnect into another conversation, or a fresh one, clears the tab's title, recap and ticket. An empty summary for the tab's conversation clears them too. A summary marked not found does not.

### Headless runs

- **Picker:** `ListConversations` leaves them out, and `GetSessionSummary` returns an empty summary for them.
- **History:** closed sessions whose conversation is a headless run are left out. This covers records saved before this plan. The first save then drops them from `sessions.json`.
- **Saved tabs:** an active or disconnected tab whose conversation is a headless run loses its conversation id on load. It keeps its directory, and never shows or resumes the run.
- **Tab capture:** a tab finds its conversation by watching for a new file in the folder. A headless run's file is never claimed. A file with no record that names a cwd or an entrypoint waits a round, because its kind is not yet known.

## Not in scope

- A list of worker runs.
- Writing our own summary with Claude. The `away_summary` record is enough.

## Tests

- A transcript with tool results, `!` commands, notifications and messages from other sessions gives only the human prompts.
- The last `ai-title` and `away_summary` win.
- The ticket order: branch, PR, title, prompts.
- Two transcripts in one folder: each tab gets its own.
- An `sdk-cli` or `sdk-go` transcript is left out of the picker and gives an empty summary.
- A closed session whose conversation is headless is left out of the history. A saved open tab keeps its directory and loses the id.
- A missing transcript keeps the saved title, recap and ticket.
- "CON-184_retry" and "feature_CON-184" give CON-184.
- Tab capture claims the interactive file beside a headless one, and claims nothing with only a headless file.
- "UTF-8", "SHA-256", "CVE-2024-1234", "GPT-4" and "ISO-8601" give no ticket.
- The walk stops once the main fields are found, and takes a PR link only from the lines already read.
- A reconnect into another conversation clears the tab's summary.
- The cache returns the same result for an unchanged file and re-reads a changed one.
- The frontend row shows the ticket and title, and the hover card shows the prompts.
