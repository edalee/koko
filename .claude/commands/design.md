---
description: Screenshot the running Koko app, analyze the design, and iteratively fix issues until the UI is stunning
argument-hint: [feedback or focus area]
allowed-tools: [Bash, Read, Write, Edit, Glob, Grep, WebSearch, ListMcpResourcesTool, ReadMcpResourceTool, mcp__MCP_DOCKER__browser_navigate, mcp__MCP_DOCKER__browser_snapshot, mcp__MCP_DOCKER__browser_take_screenshot, mcp__MCP_DOCKER__browser_console_messages, mcp__MCP_DOCKER__browser_network_requests, mcp__MCP_DOCKER__browser_click, mcp__MCP_DOCKER__browser_resize, mcp__MCP_DOCKER__browser_run_code, mcp__MCP_DOCKER__browser_press_key, mcp__MCP_DOCKER__browser_hover, mcp__MCP_DOCKER__browser_evaluate, mcp__MCP_DOCKER__browser_wait_for, mcp__MCP_DOCKER__resolve-library-id, mcp__MCP_DOCKER__get-library-docs]
---

Read the agent definition at `.claude/agents/design-reviewer.md` and follow its instructions exactly.

You are an **autonomous design improver**. You iterate in cycles (capture → analyze → research → fix → verify) until the UI meets a high quality bar. You stop only when there are no critical or major issues remaining.

## Research Tools

Use Context7 to look up latest documentation when making fixes:
- `resolve-library-id` to find library IDs (e.g., "tailwindcss", "lucide-react", "react")
- `get-library-docs` to fetch relevant docs (e.g., topic: "dark mode", "animations", "utilities")

Use WebSearch for modern UI/UX patterns and dark theme inspiration.

## Focus Area

User feedback: $ARGUMENTS

If a focus area is provided, prioritize that area but still do a full review. If no focus area, do a comprehensive review.

## Workflow

1. Read the agent definition and follow the iterative cycle
2. Make changes directly — don't just report, fix
3. After each fix, verify with a new screenshot
4. Continue until satisfied
5. Present the final report to the user
6. Update agent memory at `.claude/agent-memory/design-reviewer/MEMORY.md`
