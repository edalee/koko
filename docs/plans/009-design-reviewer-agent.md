# Plan 009: Upgrade /design to Full Design Reviewer Agent

**Status:** Completed

## Context

The `/design` skill (`.claude/commands/design.md`) took a single static screenshot via `screencapture`, read a hardcoded list of component files (several stale post-plan-007), and output a flat list of suggestions. It started from scratch every session.

The aentidote-portal-ui's `ui-ux-reviewer` agent demonstrated a more effective pattern: Playwright MCP for interactive browser review, persistent agent memory, multi-state capture, console/network checks, and severity-rated structured output.

## Goal

Rebuild `/design` as a proper Claude Code agent with Playwright interaction, persistent memory, and structured review output.

## Changes Made

### Created `.claude/agents/design-reviewer.md`

Full agent definition with:
- **Hybrid capture:** native `screencapture` (shows macOS chrome, traffic lights, real PTY data) + Playwright MCP (interactive review, accessibility snapshots, console/network checks)
- **Playwright Docker fallback:** tries `host.docker.internal:5173` then `localhost:5173`, graceful screencapture-only mode if both fail
- **Reference design comparison:** reads `docs/references/Screenshot 2026-03-01 at 14.44.34.png` and key reference source files, visually diffs against running app
- **Multi-state review:** detect app, read reference, native screenshot, Playwright navigate + snapshot + screenshot, interactive states (sidebar toggle, tab switch, resize 1200/900), console/network checks, dynamic component discovery, source code reading
- **13 review criteria:** reference fidelity, elevation/layering, color token compliance, terminal rendering, resizable panels, session tab states, toolbar, right sidebar, session sidebar, macOS integration, dark theme consistency, accessibility, spacing/typography
- **Structured output:** severity-rated (Critical/Major/Minor) with component, file, evidence, expected, fix columns + quality checklist
- **Memory management:** reads/updates `.claude/agent-memory/design-reviewer/MEMORY.md` each review

### Created `.claude/agent-memory/design-reviewer/MEMORY.md`

Seeded with:
- Component inventory (8 components post plan-007)
- Design token summary (hex values from globals.css)
- Reference design paths
- Empty known issues and review history sections

### Updated `.claude/commands/design.md`

Replaced 101-line inline prompt with ~15-line thin wrapper:
- Frontmatter: `allowed-tools` includes all Playwright MCP tools
- Body: reads agent definition and follows it, passes `$ARGUMENTS` as focus area
- After review: presents findings, updates memory, asks which to fix

### Updated `.claude/settings.local.json`

Added 10 Playwright MCP tool permissions:
- `browser_snapshot`, `browser_take_screenshot`, `browser_console_messages`, `browser_network_requests`
- `browser_click`, `browser_resize`, `browser_run_code`, `browser_press_key`, `browser_hover`

## First Review Results (2026-03-02)

Ran in screencapture-only mode (Playwright Docker MCP was down). Findings:
- Layout ~85% reference fidelity (intentional terminal-only divergence)
- 1 Critical: GitHubPanel badges use raw Tailwind colors instead of theme tokens
- 6 Major: components use `text-gray-*` instead of `text-muted-foreground`
- 4 Minor: animate-pulse on tabs, terminal edge padding, grab zone width, branding polish
