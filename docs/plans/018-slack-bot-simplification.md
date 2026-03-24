# Plan 018: Slack Bot Simplification

## Context
The Slack integration had two separate systems sharing one user token:
1. **Awareness panel** — fetched DMs, @mentions, threads for sidebar display
2. **Bot commands** — listened for DMs and responded to commands

This created problems:
- User token (`xoxp-`) meant the "bot" was the user themselves
- Bot responded to every DM from other people (even with `koko` prefix, confusing)
- Required 5 scopes: `im:history`, `im:read`, `users:read`, `search:read`, `chat:write`
- Awareness panel was marginally useful — Slack is always open anyway

## Decision
Remove the awareness panel entirely. Keep only the bot command handler, switched to a proper bot token (`xoxb-`). The bot gets its own Slack identity — DM the bot to control Koko sessions.

## Changes

### Removed
- `slack_service.go` — entire awareness panel backend (270 lines)
- `SlackPanel.tsx` — sidebar panel UI
- `useSlack.ts` — polling hook
- Slack notification badge from toolbar
- `koko` prefix requirement — every DM to the bot is a command
- `SetSlackToken()` method (token saved via `SaveConfig` directly)
- `SlackEnabled` config field (enabled when token is non-empty)

### Modified
- `slack_commands.go` — simplified: no prefix, bot token auth, removed IM channel caching (bot only has its own DM channels)
- `main.go` — removed SlackService binding
- `config_service.go` — removed `SlackEnabled`, `SetSlackToken()`; `SlackToken` now holds bot token
- `SettingsPanel.tsx` — Slack section simplified to bot token input + test
- `App.tsx` — removed SlackPanel, useSlack, slack-related props
- `Toolbar.tsx` — removed Slack badge/button
- `globals.css` — removed `--color-badge-slack`

### Bot setup
See `docs/references/slack-bot-setup.md` for step-by-step instructions.

### Owner-only access
The bot only responds to DMs from the configured Slack user ID (`SlackOwnerID` in config). Messages from anyone else are silently dropped. Without this, any workspace member who DMs the bot could list sessions, read output, and send input to running Claude sessions.

The owner ID is set in Settings alongside the bot token — found via Slack profile → ⋯ → "Copy member ID".

If `SlackOwnerID` is empty, the bot responds to everyone (backwards-compatible but insecure).

### Bot token scopes (3 total)
| Scope | Purpose |
|-------|---------|
| `im:history` | Read DM messages sent to the bot |
| `im:read` | List DM channels the bot is in |
| `chat:write` | Send replies back |
