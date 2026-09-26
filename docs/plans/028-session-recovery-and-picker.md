# Plan 028: Session Recovery and Conversation Picker

## Problem

Koko has no way to choose between recovering work and starting fresh.
Every entry point silently creates a new session.

| Entry point | What you click | What happens today |
|---|---|---|
| Disconnected tab in the sidebar | The "Click to resume" card | `switchTab` calls `reconnectTab` at once, with no prompt (`useSessionTabs.ts:307`) |
| "Recent Sessions", inside the new session dialog | A closed session | `onCreate(name, directory)` runs `createTab`, a fresh session (`NewSessionDialog.tsx:380`) |
| Worktrees module | An existing worktree | `onOpenSession` runs `createTab` too, a fresh session (`WorktreesModule.tsx:271`) |

Only the first resumes. It resumes without asking.

The other two throw away `entry.claudeSessionId`, even though history
stores it. `createTab` hardcodes `resume: false` and
`claudeSessionId: ""` (`useSessionTabs.ts:176`). The Claude conversation
is lost, and the worktree gets a second unrelated session.

Three further gaps:

1. A directory can hold many conversations. There is no way to pick one.
   Resume means "the one Koko recorded", or nothing.
2. Slugs are not stable. Two separate faults, see "Slugs" below.
3. Worktree recovery and conversation recovery are treated as one thing.
   They are separate. A worktree is a directory. It pairs with a new
   conversation or an old one.

## Goal

One dialog that answers two questions in order: where, then what.
Reached from every entry point, so recovery is never accidental and
never silent.

## Non-goals

- Editing or merging conversations. We open them, nothing more.
- Cross-directory search. The picker is scoped to one directory.
- Changing how Claude stores its session files.

## Depends on

Plan 028 assumes the UUID capture work merged in `35e7042`. Without a
trustworthy UUID the picker cannot promise that "Resume" reopens the
conversation you chose.

That work leaves three cases open on purpose, and the picker closes all
three by letting the user read previews and choose:

1. Two sessions in one directory, both submitted to before either is
   identified, end with no UUID.
2. A session that runs `/resume` inside Claude never captures, because
   Claude appends to a file that existed before launch.
3. A lone foreign candidate, from a plain `claude` run in the same
   directory, is skipped rather than claimed.

## Decisions taken

Both are settled. Recorded here because they shape the dialog.

**D1. A disconnected tab with no UUID. Decided.** Preselect nothing and
keep Open disabled until a row is chosen. Every tab saved before
`35e7042` is in this state, so this is the common case at first, then
disappears: any tab created from now on captures its UUID on the first
submitted line and gets the one-keystroke path in UX rule 2.

Preselecting the newest conversation was rejected. That is what
`--continue` means, and with several tabs in one directory the newest
conversation is usually a sibling's.

**D2. Choosing a conversation another tab holds. Decided.** The row
reads "reconnect koko-2", and choosing it reconnects that tab rather
than opening a second one. A dead session still owns its conversation,
so the holder is often a disconnected tab, and the obvious action should
work rather than be greyed out.

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Entry points                                                 │
│                                                               │
│  New Session button ─┐                                        │
│  Worktrees module ───┼──▶  SessionDialog                      │
│  Disconnected tab ───┘       │                                │
│                              ▼                                │
│                    ┌──────────────────────┐                   │
│                    │ Step 1: where        │                   │
│                    │  directory, worktree │                   │
│                    │  or recent session   │                   │
│                    └──────────┬───────────┘                   │
│                               ▼                               │
│                    ┌──────────────────────┐                   │
│                    │ Step 2: what         │                   │
│                    │  ○ New conversation  │                   │
│                    │  ○ Conversation 1..N │                   │
│                    └──────────┬───────────┘                   │
│                               ▼                               │
│         CreateSessionWithOpts{dir, slug, resume, uuid}        │
└──────────────────────────────────────────────────────────────┘
```

"Recent Sessions" is not a separate entry point. It is a list inside the
dialog (`NewSessionDialog.tsx:363`), so it selects a directory in step 1.

## Backend

### Go: `claude_service.go`

Add a lister. `lastAssistantText` already does the tail read, so reuse
it per file.

```go
// ListConversations returns the Claude conversations stored for a
// directory, newest first.
func (cs *ClaudeService) ListConversations(dir string) ([]Conversation, error)
```

Rules:

- Read `claudeProjectDir(dir)`, the helper merged in `35e7042`. It folds
  every non-alphanumeric character to `-`, truncates at 200 characters
  and appends a base36 hash, matching Claude's own rule.
- **Filter on `cwd`.** Directories that differ only in non-alphanumerics
  share one project folder, so `/x/foo_bar` and `/x/foo-bar` land in the
  same place. Each JSONL entry carries `cwd`, and `firstUserText`
  already reads the head of the file, so read it there and drop entries
  from a different directory.
- Cap at the newest 20 by modification time. These files reach many MB.
- `Title` comes from the first user message. Add
  `firstUserText(path string) (text, cwd string)` alongside
  `lastAssistantText`.
- `Preview` reuses `lastAssistantText`.
- `MessageCount` is skipped. Counting means reading whole files.
  `SizeBytes` is cheap and good enough to show "long" versus "short".

Conversations that Koko did not create are listed on purpose. A plain
`claude` run in the directory is exactly the history a user wants back,
and case 3 above means Koko will never have claimed it.

### Types: `types.go`

```go
type Conversation struct {
	UUID       string `json:"uuid"`
	Title      string `json:"title"`      // first user message, truncated
	Preview    string `json:"preview"`    // last assistant message, truncated
	ModifiedAt int64  `json:"modifiedAt"` // unix millis
	SizeBytes  int64  `json:"sizeBytes"`
}
```

### Slugs

Two faults, both needed for `koko-1` to mean one thing.

**The frontend never stores a slug.** `createTab` sets `slug: ""`
(`useSessionTabs.ts:181`) and nothing fills it in. The comment claims it
is "populated from GetSessions or after capture"; no code does that. So
every persisted slug is empty, and simply adding `Slug` to
`CreateSessionOpts` would pass `""` and change nothing.

Fix: return the slug from create, or read it straight after, alongside
the existing `mergeClaudeID` call. Store it on the tab so it persists.

**The counter is keyed wrongly and starts empty.** `nextSlug` builds the
name from `dirSlug(dir)`, the basename, but counts per full path:

```go
base := dirSlug(dir)        // "koko"
tm.slugCount[dir]++         // keyed by the whole path
```

So `/a/koko` and `/b/koko` both hand out `koko-1`, as does any worktree
with the same basename. Key the counter by base slug instead.

`slugCount` also starts empty on every launch, and saved tabs reconnect
lazily, on click. That gives:

1. Restart the app.
2. Open a new session in `koko`. It takes `koko-1`.
3. The saved `koko-1` tab is still sitting there, disconnected.

Fix: seed the counters in `app.go` at startup from the persisted
records, before the frontend can create anything. Raising the counter at
recovery time is too late.

Then add `Slug` to `CreateSessionOpts`, used in place of `nextSlug` when
set.

```go
type CreateSessionOpts struct {
	...
	Slug string `json:"slug"` // reuse an existing slug on recovery
}
```

### Ownership guard

UX rule 3 is currently enforced by a disabled radio button only.
`CreateSessionWithOpts` copies `opts.ClaudeSessionID` into the session
with no check, so the API and MCP can still open a second process on one
conversation and break the uniqueness `claimUUID` relies on.

Refuse in `CreateSessionWithOpts` when `uuidClaimed` says another
session holds it.

**The trap.** `uuidClaimed` counts dead sessions on purpose, and
`reconnectTab` never closes the session it replaces. `CloseSession` runs
only from `closeTab` (`useSessionTabs.ts:280`). A naive guard would
therefore refuse every reconnect after the first. Reconnect must drop
the old entry first, either by calling `CloseSession(old)` or by adding
a `Replaces` field to the opts that the backend clears under the same
lock.

`api_server.go:123` is the only other caller, and it uses the older
`CreateSession` wrapper. It gets the guard for free.

### Drop `--continue`

Resume by explicit UUID only. `--continue` resumes the newest
conversation in the directory, which is a sibling's whenever a sibling
is live. It is also the one path that cannot refuse, because guessing is
all it does. If a caller asks to resume with no UUID, start a new
conversation and log it.

This removes the `continueUUID` snapshot and its `claimUUID` call
(`terminal_manager.go:249`, `:285`). `mergeClaudeID` in the frontend
stays: it is cheap and still closes the window for any future
synchronous path.

### Deletion

Three stores, three blast radii. The plan keeps them separate, because
conflating them is how history gets destroyed by accident.

| Store | Call | Effect |
|---|---|---|
| Koko's closed-session records | `clearHistoryEntry`, exists at `useSessionTabs.ts:335` | Forgets Koko's bookmark, reversible in effect |
| Claude's `.jsonl` files | new `DeleteConversation` | Destroys the conversation, permanent |
| Git worktrees | `RemoveWorktree`, exists at `git_service.go:468` | Removes a directory, permanent |

The picker reads Claude's project folder, not Koko's history, so
clearing Koko's records removes nothing from the list. A delete that
clears what the user sees has to delete the file.

```go
// DeleteConversation removes one Claude session file.
func (cs *ClaudeService) DeleteConversation(dir, uuid string) error

// DeleteConversations removes every conversation the picker lists for a
// directory, using the same cwd filter as ListConversations.
func (cs *ClaudeService) DeleteConversations(dir string) (int, error)
```

Rules:

- Refuse to delete a conversation any live session holds. `ClaudeService`
  has no `TerminalManager`, so the check belongs with the ownership
  guard above. Route deletion through a small method on `App`, which
  holds both.
- Both calls confirm first, and the confirmation says the conversation
  cannot be recovered.
- `DeleteConversations` reports how many it removed and how many it
  skipped because a session held them.

### Wails bindings

`ListConversations` is bound through the existing `claude` struct in
`main.go`. The deletion methods are bound through `app`, which can reach
the `TerminalManager` for the ownership check. No new binding entry
either way.

## Frontend

### A) `SessionDialog`

`NewSessionDialog` is renamed, not duplicated. Step 2 is added to it,
and the other entry points are pointed at it. The revert notes assume
that rename.

```
┌─────────────────────────────────────────────┐
│  New session                                │
│                                             │
│  Directory   ~/Projects/koko          [···] │
│  ☐ Create worktree    branch  wt-3          │
│                                             │
│  ─────────────────────────────────────────  │
│  Conversations in this directory            │
│                                             │
│  ● Start a new conversation                 │
│                                             │
│  ○ Fix the terminal resize bug       2h ago │
│    "That is the cropping fixed, the..."     │
│                                             │
│  ○ Add the search web item      1d ago   🗑 │
│    "Both guards verified, 16 tests..."      │
│                                             │
│  ○ Session picker plan          3d ago  ⊘   │
│    reconnect koko-2                         │
│                                             │
│  Clear all conversations here               │
│                                             │
│                        [ Cancel ]  [ Open ] │
└─────────────────────────────────────────────┘
```

Rules:

- Default selection is "Start a new conversation", except for a
  disconnected tab, where D1 applies.
- A conversation another tab holds is marked with that tab's slug.
  Connected or disconnected both count: a dead session still owns its
  conversation. D2 decides what choosing it does.
- Held conversations are found in the frontend, from every tab's
  `claudeSessionId`. `NewClaudeService()` has no `TerminalManager`
  reference, so the backend cannot supply the tab that holds it. The
  backend guard above is the real enforcement.
- Conversations load when the directory changes, not on dialog open.
  Twenty tail reads of 64KB is not instant, so show a loading row.
- Empty directory means no list and no heading.
- Each row carries a delete button, shown on hover, next to the date. It
  deletes Claude's session file, so the confirmation says the
  conversation cannot be recovered. A row another tab holds has no
  delete button, only the reconnect action from D2.
- "Clear all conversations here" sits under the list and calls
  `DeleteConversations`. It reports what it removed and what it skipped
  because a session held it.

### B) Entry points

| Entry point | Dialog opens with |
|---|---|
| New Session button | Nothing preselected |
| Recent Sessions row, inside the dialog | That directory, its stored conversation preselected |
| Worktrees module row | That worktree path, new conversation selected |
| Disconnected tab | That directory, its stored conversation preselected, or D1 when it has none |

The Recent Sessions row currently drops `entry.claudeSessionId`. Passing
it as the preselection is the fix.

### C) Worktrees

No change to `WorktreesModule` beyond what it hands the dialog. A
worktree is a directory with its own project key, so its conversation
list is naturally separate.

The "Create worktree" toggle stays in step 1, unchanged from plan 027.

### D) Clearing history and worktrees

Two bulk actions, in Settings rather than the dialog. They are
housekeeping, not part of starting a session.

**Clear session history.** Drops Koko's closed-session records. It does
not touch Claude's files, so the picker is unchanged. The button says so
in as many words, because the obvious reading is the opposite.
`clearHistoryEntry` already does this per directory and is wired to no
UI. Generalise it to clear everything.

**Remove Koko's worktrees.** Scoped to worktrees Koko created, which it
knows from each tab's `worktreePath`. It never passes `--force`. A
worktree with uncommitted changes is skipped and named in the result,
and the user removes it themselves from the worktrees module, which
already offers the force path through `WorktreeRemovalDialog`.

Forcing in bulk is the one action in this plan that could destroy work
the user cannot get back from git. It is deliberately not offered.

### E) Reload session

A button on the session row in the sidebar, shown on hover next to the
close button, and a "Reload Session" item in the terminal context menu.

Reload restarts the Claude process in place. Same tab, same slug, same
directory, same conversation. The user needs it when Claude hangs, when
`CLAUDE.md` or an MCP server changed and Claude must re-read it, or
after a Koko update.

It is `reconnectTab` aimed at a connected tab, which nothing does today.
`switchTab` only reconnects a tab that is already disconnected
(`useSessionTabs.ts:307`).

Steps:

1. Close the old session, so the ownership guard does not see its UUID
   as taken. This is the same requirement reconnect has.
2. Create a session with the stored UUID, `resume: true`, and the tab's
   slug.
3. Swap the new session id onto the same tab, exactly as `reconnectTab`
   does.

Rules:

- Confirm first when `GetSessionState` is not `idle`. Reload kills a
  Claude that may be mid-tool-call, and anything typed but not sent is
  lost. The conversation itself is on disk and comes back.
- A tab with no UUID cannot reload into its conversation. Open the
  picker instead, under D1, so the user chooses.
- Reload is the answer to "restart this session". Closing and reopening
  the tab is not the same: it loses the slug and the conversation.

## Data flow

```
SessionDialog
  │  directory chosen
  ▼
ClaudeService.ListConversations(dir) ──▶ Conversation[]
  │  marked against every tab's claudeSessionId
  ▼
user picks new, or a UUID
  │
  ▼
createTab({ dir, slug?, resume: uuid !== "", claudeSessionId: uuid })
  │
  ▼
CreateSessionWithOpts ──▶ uuidClaimed? refuse : claude --resume <uuid>
```

## UX rules

1. Recovery is never silent. Every resume is chosen.
2. One keystroke restores a disconnected tab that has a UUID. Clicking
   its card opens the dialog with that conversation preselected, so
   Enter accepts it. This replaces today's silent reconnect, which is
   the open TODO "Reconnect koko-1?".
3. One conversation, one tab. Enforced in the backend, surfaced in the
   dialog.
4. A recovered session keeps its slug. So does a reloaded one.
5. The newest conversation is first. Recency is the only sort.
6. Reload never silently discards work in progress. If the session is
   not idle, it asks.

## Testing

Go:

- `ListConversations` sorts newest first, caps at 20, and skips
  non-`.jsonl` files.
- `ListConversations` drops entries whose `cwd` is a different
  directory that folds to the same project key.
- `firstUserText` handles both JSONL shapes `extractAssistantText`
  covers, and a file with no user message.
- `ListConversations` on a missing directory returns empty, not an
  error. A directory with no conversations is normal.
- Slug counters seed from persisted records at startup, so a new session
  after a restart does not reuse a saved slug.
- `slugCount` keyed by base slug: `/a/koko` and `/b/koko` do not both
  get `koko-1`.
- `CreateSessionOpts.Slug` is reused when set.
- `CreateSessionWithOpts` refuses a UUID another session holds.
- Reconnect succeeds under that guard, because the replaced session is
  dropped first.
- `DeleteConversation` removes the file, and refuses when a live session
  holds that UUID.
- `DeleteConversations` honours the same cwd filter as
  `ListConversations`, so it cannot delete a neighbouring directory's
  conversations that share a project folder.
- `DeleteConversations` counts what it skipped.
- Clearing worktrees skips a dirty worktree and names it, rather than
  forcing.

Frontend:

- The dialog marks a conversation held by a connected tab, and by a
  disconnected one.
- Choosing a conversation calls `CreateSessionWithOpts` with
  `resume: true` and that UUID.
- Choosing "new" passes `resume: false` and an empty UUID.
- A Recent Sessions row preselects its stored conversation, which is the
  regression that loses the conversation today.
- A created tab stores the slug the backend assigned, and persists it.
- A row held by another tab offers reconnect and no delete button.
- Clearing session history leaves the conversation list unchanged.
- Reload closes the old session before creating the new one, so the
  ownership guard does not refuse it.
- Reload keeps the tab's slug and conversation id.
- Reload on a tab with no conversation id opens the picker instead.
- Reload asks first when the session state is not `idle`.

## Rollout

1. Backend lister, types and tests. No UI change, nothing user visible.
2. Slug round trip, counter seeding and base keying, `Slug` on
   `CreateSessionOpts`.
3. Ownership guard, with reconnect dropping the replaced session.
4. `SessionDialog` step 2, reached from the New Session button.
5. Route the worktrees module and the disconnected tab through it.
6. Reload session, on the sidebar row and in the context menu.
7. Deletion: per-row delete, clear all conversations for a directory,
   clear session history, remove Koko's worktrees.
8. Drop the `--continue` fallback.

Steps 1 to 3 are independently mergeable. Step 4 is where behaviour
changes for the user.

Step 6 needs step 2, for the slug, and step 3, for the guard. It does
not need the dialog, so it can land before step 4.

Step 7 is independent of the picker and can move earlier or later.

Step 8 must come last. Every tab saved before `35e7042` has no UUID, so
dropping the fallback earlier would make those tabs start a new
conversation on reconnect. Once step 5 lands they get the picker
instead, and the fallback has nothing left to cover.

## Revert notes

Steps 1 to 3 are additive, and so are steps 6 and 7. Steps 4 and 5
revert together, and the rename back to `NewSessionDialog` goes with
them, because the entry points call it directly. Reverting step 8
restores `--continue`, which is worse but not broken, so revert it first
if the picker has to go.

## Out of scope, future plans

- Archiving a conversation, as opposed to deleting it.
- Searching conversation text.
- Showing token or context usage per conversation.
- Reattaching a conversation to a different directory.
