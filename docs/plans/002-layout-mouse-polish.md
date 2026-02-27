# Plan 002: Layout, Mouse, Polish

**Status:** Completed
**Date:** 2026-02-27

## Context
After first run, the TUI felt clunky: sidebar panels didn't fill height cleanly (double borders between each panel wasted space), there was no mouse support, and designs didn't align well.

## Problems Solved
1. Each sidebar panel had its own rounded border — stacking created double border lines at junctions, wasting 4 vertical lines
2. No mouse support — couldn't click panels to focus them
3. Focused vs unfocused panels not visually distinct enough

## Changes

### Unified sidebar border
- Removed individual borders from Slack, GitHub, Summary panels
- Sidebar container now renders ONE rounded border with `─` separator lines between sections
- Sidebar border color changes to active (blue) when any child panel is focused
- Height distribution accounts for border (2 lines) and separators (2 lines)

### Mouse support
- Enabled `tea.MouseModeCellMotion` on the View
- `tea.MouseClickMsg` handled in root Update — determines clicked panel by X/Y coordinates
- Click terminal area → focus terminal; click sidebar area → focus appropriate panel by Y-zone

### Focus polish
- Focused panel title rendered in bold + active color
- Unfocused panel titles rendered in dim color
- Added `Focused() bool` accessor to all panel components

## Files Modified
- `internal/tui/components/slack/model.go` — borderless, focus-aware title, `Focused()` accessor
- `internal/tui/components/github/model.go` — same
- `internal/tui/components/summary/model.go` — same
- `internal/tui/components/sidebar/model.go` — single border, separators, focus-aware border color
- `internal/tui/root.go` — mouse mode, MouseClickMsg handler
