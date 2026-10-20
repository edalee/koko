# Plan 036: A clean summary for each session

## Problem

It is hard to see what each session was last working on.

- `GetLastMessage(dir)` reads the newest transcript in the folder, not the tab's own. Two tabs in one repo, or a headless `claude -p` run there, take over the preview.
- It shows Claude's last text reply. That is often an answer to a `!` command, a sub-agent report or a notification, not what you asked.
- `lastMsg` updates only when a tab closes or its process exits.
- The session list does not show it at all.
- The conversation picker lists headless runs (`entrypoint: "sdk-cli"`) as conversations you can resume.

## Goal

Each tab shows what its own conversation is about, built only from what you asked and from the summaries Claude Code writes. Headless runs never appear as a tab, in the history or in the picker.

## Sources

Claude Code writes these records to `~/.claude/projects/<dir>/<uuid>.jsonl`. Koko reads only these.

| Field | Record | Rule |
|---|---|---|
| Title | `ai-title` | The last one. Claude rewrites it many times |
| Recap | `system` with `subtype: "away_summary"` | The last one's `content` |
| Recent prompts | `user` with `origin.kind == "human"` | The last 10, not `isMeta`. Tool results, `!` commands, `/` commands, notifications and messages from other sessions have no human origin, so they drop out |
| Last prompt | `last-prompt` | Its `lastPrompt`. Falls back to the newest recent prompt |
| Branch | `gitBranch` on the newest record | |
| PR | `pr-link` | The last one: number, repo and URL |
| Ticket | none | The first Jira key, `[A-Z][A-Z0-9]+-\d+`, found in the branch, then the PR, then the title, then the recent prompts, newest first |

## Design

### Backend

- `ClaudeService.GetSessionSummary(dir, uuid)` returns the summary of that one transcript. With no UUID it returns an empty summary. It never falls back to the newest file.
- It reads the file backwards in chunks, as `lastAssistantText` does, until it has every field or reaches the 2MB budget. It reads the head only if the title is still missing.
- A cache keyed on path, size and modification time returns the last result while the file is unchanged.
- A transcript whose first records say `entrypoint: "sdk-cli"` is a headless run. `ListConversations` leaves it out, and `GetSessionSummary` returns empty for it.
- `GetLastMessage` stays for older callers, but reads only the tab's UUID. Nothing new calls it.
- The Jira key rule moves to a small shared helper in the app. The worker keeps its own copy, because it is a separate Go module.

### Frontend

- **Session list:** each row gets a second line: the ticket as a small chip, then the title. With no title, the last prompt.
- **Hover card:** a small card on hover or focus, after a short delay. It shows the recap, branch, PR and ticket, then the last 10 prompts with their age. It replaces the native `title` tooltip on rows.
- **Reconnect card, history and picker:** they show the title and recap in place of Claude's last reply.
- **Live:** the list fetches summaries for open tabs every 15 seconds while Koko is visible, and when a tab goes idle. The cache makes an unchanged file cheap.
- **Saved history:** `SessionRecord` keeps the title, recap and ticket at close, so a closed session still shows them.

### Headless runs

- They never become a tab. Koko starts tabs only for sessions you open, so this rule covers the picker and history.
- The picker and the history leave out `sdk-cli` transcripts.

## Not in scope

- A list of worker runs.
- Writing our own summary with Claude. The `away_summary` record is enough.

## Tests

- A transcript with tool results, `!` commands, notifications and messages from other sessions gives only the human prompts.
- The last `ai-title` and `away_summary` win.
- The ticket order: branch, PR, title, prompts.
- Two transcripts in one folder: each tab gets its own.
- An `sdk-cli` transcript is left out of the picker and gives an empty summary.
- The cache returns the same result for an unchanged file and re-reads a changed one.
- The frontend row shows the ticket and title, and the hover card shows the prompts.
