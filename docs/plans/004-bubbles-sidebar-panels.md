# Plan 004: Interactive Sidebar Panels with Bubbles v2

**Status:** Superseded by Plan 005 (Wails migration)
**Branch:** `feat/bubbles-sidebar-panels`

## Context

The sidebar panels (GitHub, Slack, Summary) currently render hardcoded mock data as static text. To make koko useful as a workspace, these panels need:
- Real data (GitHub PRs via `gh` CLI, Slack counts as mock-with-structure for now)
- Interactive list navigation (scroll, select)
- Loading states (spinners during data fetch)
- Periodic background refresh

The `charm.land/bubbles/v2` library provides production-ready `list`, `viewport`, and `spinner` components that integrate directly with bubbletea v2.

## Architecture

```
Root
├── Terminal (unchanged)
└── Sidebar Container
    ├── GitHub Panel  → bubbles/list + custom delegate + gh CLI fetch
    ├── Slack Panel   → bubbles/list + mock data (structured for future API)
    └── Summary Panel → bubbles/viewport + aggregated data from siblings
```

**Data flow:**
```
Init() → fetchGitHubPRs Cmd (goroutine runs `gh pr list`)
       → returns GitHubDataMsg{prs}
       → GitHub panel updates list items
       → Summary panel re-aggregates

Tick every 60s → re-fetch → same flow
```

## Key Design Decisions

### Use bubbles/list for GitHub and Slack panels

The `list` component provides scrolling, keyboard navigation, filtering, and spinner out of the box. We use a compact custom delegate (1-line items, no spacing) to fit the narrow 28-char sidebar width.

Filtering and help are disabled — these are notification panels, not search interfaces.

### Use bubbles/viewport for Summary panel

The Summary panel aggregates counts from GitHub + Slack into a short scrollable text block. A `viewport` is simpler than a `list` here since the content is informational, not interactive.

### GitHub data via `gh` CLI

Run `gh pr list --repo epidemicsound/<repo> --json number,title,author,reviewDecision --limit 10` in a goroutine. Parse JSON response. This avoids managing GitHub tokens — `gh` handles auth.

Tracked repos (configurable later, hardcoded for now):
- `drumstick2`
- `drumstick-ui`
- `trigon`
- `conductor-bot`

### Slack panel stays mock (structured)

Define proper data types (`SlackChannel`, `SlackCount`) but populate with mock data. This establishes the message routing and UI patterns so real Slack API integration drops in later.

### Message routing

Async data messages (`GitHubDataMsg`, `SlackDataMsg`) are always forwarded to sidebar regardless of focus — data updates happen in the background.

Interactive messages (`tea.KeyPressMsg`, `tea.MouseMsg`) are only forwarded to the focused panel.

### Periodic refresh

A `tickMsg` fires every 60 seconds. Root catches it and dispatches a re-fetch command. The tick is started in `Init()` alongside the terminal startup.

## Changes

### New dependency

```
charm.land/bubbles/v2 v2.0.0
```

### New files

| File | Purpose |
|------|---------|
| `internal/tui/components/github/messages.go` | `GitHubDataMsg`, `GitHubPR` types |
| `internal/tui/components/github/fetch.go` | `fetchPRs() tea.Cmd` — runs `gh pr list`, returns `GitHubDataMsg` |
| `internal/tui/components/github/delegate.go` | Custom compact list delegate (1-line items) |
| `internal/tui/components/slack/messages.go` | `SlackDataMsg`, `SlackItem` types |

### Modified files

| File | Changes |
|------|---------|
| `internal/tui/components/github/model.go` | Replace static View() with `list.Model`. Add `Init()` → `fetchPRs`. Handle `GitHubDataMsg` in `Update()`. |
| `internal/tui/components/slack/model.go` | Replace static View() with `list.Model`. Populate with mock `SlackItem`s in `New()`. |
| `internal/tui/components/summary/model.go` | Replace static View() with `viewport.Model`. Add `UpdateData(github, slack)` method to re-aggregate. |
| `internal/tui/components/sidebar/model.go` | Forward messages to child panels. Propagate sizing to list/viewport components. |
| `internal/tui/root.go` | Route `GitHubDataMsg`/`SlackDataMsg` to sidebar. Add tick command for periodic refresh. Batch `Init()` with terminal + GitHub fetch + tick. |
| `internal/tui/messages.go` | Add `tickMsg` type. |
| `go.mod` | Add `charm.land/bubbles/v2`. |

## Detailed Component Changes

### GitHub Panel (`github/model.go`)

```go
type Model struct {
    list    list.Model
    width   int
    height  int
    focused bool
    prs     []GitHubPR // raw data for Summary access
}

func New() Model {
    l := list.New([]list.Item{}, newCompactDelegate(), 0, 0)
    l.SetShowTitle(true)
    l.Title = "GitHub"
    l.SetShowHelp(false)
    l.SetShowFilter(false)
    l.SetShowStatusBar(false)
    return Model{list: l}
}

func (m Model) Init() tea.Cmd {
    return fetchPRs() // async gh CLI call
}

// Update handles GitHubDataMsg → update list items + spinner
// Update handles key/mouse when focused → delegate to list
```

### GitHub Fetch (`github/fetch.go`)

```go
func fetchPRs() tea.Cmd {
    return func() tea.Msg {
        repos := []string{"drumstick2", "drumstick-ui", "trigon", "conductor-bot"}
        var allPRs []GitHubPR
        for _, repo := range repos {
            out, err := exec.Command("gh", "pr", "list",
                "--repo", "epidemicsound/"+repo,
                "--json", "number,title,author,reviewDecision",
                "--limit", "10",
            ).Output()
            if err != nil { continue }
            // parse JSON, append to allPRs
        }
        return GitHubDataMsg{PRs: allPRs}
    }
}
```

### Compact List Delegate (`github/delegate.go`)

Single-line items for narrow sidebar:
```
#142 Fix claim batch processing  ← truncated to fit width
#139 Add retry logic for YDS     ← repo name as dim prefix if space
```

### Slack Panel (`slack/model.go`)

Same `list.Model` pattern as GitHub but populated with mock data:
```go
func New() Model {
    items := []list.Item{
        SlackItem{Channel: "#product-drm-internal", Count: 3, Type: "DM"},
        SlackItem{Channel: "#product-drm", Count: 1, Type: "thread"},
        SlackItem{Channel: "#drm", Count: 2, Type: "mention"},
    }
    l := list.New(items, newCompactDelegate(), 0, 0)
    l.Title = "Slack"
    // disable filter, help, status bar
    return Model{list: l}
}
```

### Summary Panel (`summary/model.go`)

```go
type Model struct {
    viewport viewport.Model
    width    int
    height   int
    focused  bool
}

func (m Model) UpdateData(ghPRs []github.GitHubPR, slackItems []slack.SlackItem) Model {
    // Count PRs needing review, unread DMs, etc.
    // Render as styled text into viewport content
    content := fmt.Sprintf("  %d PRs need review\n  %d unread messages\n  ...", prCount, msgCount)
    m.viewport.SetContent(content)
    return m
}
```

### Root Changes (`root.go`)

```go
func (m Model) Init() tea.Cmd {
    return tea.Batch(
        m.terminal.Init(),
        m.sidebar.GitHub.Init(),  // triggers fetchPRs
        tickCmd(),                 // starts 60s refresh cycle
    )
}

// New message routing in Update():
case github.GitHubDataMsg:
    m.sidebar.GitHub, cmd = m.sidebar.GitHub.Update(msg)
    m.sidebar.Summary = m.sidebar.Summary.UpdateData(m.sidebar.GitHub.PRs(), ...)
    return m, cmd
case tickMsg:
    return m, tea.Batch(m.sidebar.GitHub.Init(), tickCmd())
```

## Implementation Order

1. **Add bubbles dependency** — `go get charm.land/bubbles/v2` + verify build
2. **GitHub panel** — messages, fetch, delegate, model rewrite (most complex)
3. **Slack panel** — model rewrite with mock list items (simpler, same pattern)
4. **Summary panel** — viewport + UpdateData aggregation
5. **Sidebar + Root** — message routing, sizing propagation, tick setup
6. **Polish** — styling consistency, error handling for `gh` failures

## Known Limitations

- **No Slack API yet** — mock data only; real integration deferred
- **Sequential gh calls** — fetches repos one at a time; could parallelize later
- **No item actions** — selecting a PR doesn't open it (future: `gh pr view --web`)
- **Hardcoded repo list** — configurable repos deferred to config file plan

## Verification

1. `go build ./...` compiles with bubbles v2 dependency
2. `make run` → sidebar shows GitHub panel with real PR data (or "No PRs" if none open)
3. `ctrl+g` → focus GitHub panel → arrow keys scroll through PRs
4. `ctrl+s` → focus Slack panel → see mock notification items
5. Summary panel shows aggregated counts
6. Wait 60s → GitHub data refreshes automatically
7. Sidebar toggle (`ctrl+\`) still works
8. Terminal pane still fully functional (PTY, key input, resize)
9. `gh` CLI not installed → GitHub panel shows error message gracefully
