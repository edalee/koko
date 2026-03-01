---
description: Screenshot the running Koko app, analyze against the design system, and propose specific UI/UX fixes
argument-hint: [feedback or focus area]
allowed-tools: [Bash, Read, Edit, Write, Glob, Grep, Task]
---

# /design — Design Iteration

Spawn a subagent that screenshots the running Koko app, analyzes it against the design system, and returns a concrete list of proposed changes.

## Arguments

- `$ARGUMENTS` — optional feedback or focus area (e.g. "tabs look cramped", "panel spacing feels off", "check contrast")

## Workflow

Use the **Task** tool to launch a subagent with `subagent_type: "general-purpose"` and the following prompt:

---

You are a design review agent for Koko, a Wails v2 desktop app (Go + React + Tailwind v4).

**Your job:** Screenshot the running app, read the relevant component source, and return a structured list of proposed UI/UX changes.

### Step 1 — Screenshot

Run these bash commands:
```bash
# Verify running
ps aux | grep "Koko.app/Contents/MacOS/Koko" | grep -v grep
# Bring to front and capture
osascript -e 'tell application "System Events" to tell process "Koko" to set frontmost to true' && sleep 1 && screencapture -o /tmp/koko-design.png
```
Then read `/tmp/koko-design.png` to view it.

If Koko is not running, return: "Koko is not running. Start it with `make dev` from `/Users/edward/Projects/personal/koko`."

### Step 2 — Read current source

Read these files to understand current styling:
- `/Users/edward/Projects/personal/koko/frontend/src/globals.css`
- `/Users/edward/Projects/personal/koko/frontend/src/App.tsx`
- `/Users/edward/Projects/personal/koko/frontend/src/components/TitleBar.tsx`
- `/Users/edward/Projects/personal/koko/frontend/src/components/SessionTabBar.tsx`
- `/Users/edward/Projects/personal/koko/frontend/src/components/PanelDock.tsx`
- `/Users/edward/Projects/personal/koko/frontend/src/components/GitHubPanel.tsx`
- `/Users/edward/Projects/personal/koko/frontend/src/components/SlackPanel.tsx`
- `/Users/edward/Projects/personal/koko/frontend/src/components/SummaryPanel.tsx`
- `/Users/edward/Projects/personal/koko/frontend/src/components/TerminalPane.tsx`

Also read the design memory for context:
- `/Users/edward/.claude/projects/-Users-edward-Projects-personal-koko/memory/design.md`

### Step 3 — Analyze

Compare the screenshot against the design system. Check:

1. **Elevation**: base (darkest) → titlebar → panel → surface. Do the layers read correctly?
2. **Spacing**: generous padding, no cramped elements, clean gaps between sections
3. **Borders**: minimal — prefer bg-color elevation over border lines
4. **Typography**: icy blue primary text readable? Steel blue secondary visible but subdued?
5. **Tabs**: flat (no bevels), text centered in tab area, gradient on active tab
6. **Terminal**: icy blue on navy, plum cursor visible, enough padding around terminal pane
7. **Panels**: cards use bg-surface elevation on bg-panel, clean header layout, badges visible
8. **Traffic lights**: macOS red/yellow/green visible in top-left
9. **Overall polish**: does it feel like a cohesive dark dashboard? Any harsh edges?

If user feedback was provided: "$ARGUMENTS" — focus analysis on that area first.

### Step 4 — Propose changes

Return a structured response:

```
## Screenshot Analysis

[Brief description of what you see]

## What looks good
- [list items that match the design system]

## Proposed changes

### 1. [Component] — [issue]
**File:** `frontend/src/components/Foo.tsx`
**Current:** `className="..."`
**Proposed:** `className="..."`
**Why:** [reasoning]

### 2. [Component] — [issue]
...

## Priority
[Which change would have the biggest visual impact]
```

Only propose changes where there's a clear improvement. Don't suggest changes for things that already look correct. Be specific — include exact Tailwind classes, inline styles, or CSS values.

---

After the subagent returns, present the proposed changes to the user and ask which ones to apply.
