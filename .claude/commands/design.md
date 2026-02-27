---
description: Render a headless snapshot of the TUI layout for visual inspection and design iteration
argument-hint: [WIDTHxHEIGHT] [hidden]
allowed-tools: [Bash, Read]
---

# /design — Headless Layout Snapshot

Render the Koko TUI at a given size and display the output for visual inspection.

## Arguments

- `$ARGUMENTS` — optional size like `120x40` (default: `100x30`). Append `hidden` to render with sidebar hidden.

## Steps

1. Parse arguments: extract WIDTHxHEIGHT from `$ARGUMENTS` (default 100x30). Check if `hidden` is in the arguments.
2. Run the snapshot test:
   ```
   cd /Users/edward/Projects/personal/koko && go test ./internal/tui/ -run TestSnapshot -v -count=1 -args -snapshot-size=WIDTHxHEIGHT [-snapshot-hidden]
   ```
3. Display the visual output to the user — show the full numbered-line layout.
4. Also report:
   - Measured width x height vs expected
   - Any lines that overflow the expected width
   - Whether sidebar is visible and which panel is focused
5. If there are layout issues, suggest specific fixes with file paths and line numbers.

## Design context

Refer to the layout structure:
- Terminal pane (left) + sidebar (right, 30 cols wide)
- Sidebar has 3 panels: Slack, GitHub, Summary — separated by `─` lines
- Status bar at bottom (1 line)
- All borders are rounded (`╭╮╰╯│─`)
- Active border: color 62 (blue), inactive: color 240 (gray)
- Lipgloss v2: Width/Height are TOTAL dimensions (border-box model)
