# Slack Bot Setup for Koko

## Overview
Koko uses a Slack bot to let you control sessions via DM. The bot has its own identity — DM it commands like `sessions`, `status`, `send <id> hello`.

## Step-by-step

### 1. Create a Slack App
1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click **Create New App** → **From scratch**
3. Name: `Koko` (or whatever you like)
4. Pick your workspace
5. Click **Create App**

### 2. Add Bot Scopes
1. In the left sidebar: **OAuth & Permissions**
2. Scroll to **Bot Token Scopes**
3. Add these 3 scopes:

| Scope | Purpose |
|-------|---------|
| `im:history` | Read messages in DMs with the bot |
| `im:read` | See DM channel info |
| `chat:write` | Send replies |

### 3. Install to Workspace
1. Scroll up to **OAuth Tokens for Your Workspace**
2. Click **Install to Workspace**
3. Review permissions and click **Allow**
4. Copy the **Bot User OAuth Token** (`xoxb-...`)

### 4. Configure in Koko
The Slack tab in Settings is hidden (plan 029). It returns once the bot is dropped or rebuilt. Until then, set the bot up in the config file by hand.

1. Quit Koko.
2. Open `~/Library/Application Support/koko/config.json`.
3. Set `slackToken` to the bot token.
4. Set `slackOwnerId` to your Slack Member ID (see below).
5. Start Koko. It reads the file only at start-up.

The bot stays idle while `slackToken` is empty.

#### Finding your Slack Member ID
1. In Slack, click your profile picture → **Profile**
2. Click the **⋯** (three dots) menu
3. Click **Copy member ID**
4. Put it in `slackOwnerId` in `config.json`

This ensures only you can send commands to the bot. Without it, anyone who DMs the bot can control your sessions.

### 5. Enable DMs to the Bot
By default, Slack apps can't receive DMs until you enable it:
1. In your Slack app settings: **App Home** (left sidebar)
2. Under **Show Tabs**, enable **Messages Tab**
3. Check **Allow users to send Slash commands and messages from the messages tab**

### 6. DM the Bot
1. In Slack, find the bot in your DM list (search for its name)
2. Send `help` to see available commands

## Available Commands

| Command | Description |
|---------|-------------|
| `sessions` | List active Koko sessions |
| `status [id]` | Session state + recent output |
| `send <id> <text>` | Send raw text to a session |
| `prompt <id> <text>` | Send a prompt and wait for Claude's reply |
| `output [id]` | Last ~50 lines of output |
| `files <id>` | Git file changes for session |
| `help` | Show command list |

If you only have one session, `status` and `output` default to it (no ID needed).

## Troubleshooting

**Bot doesn't respond:**
- Check the token is a bot token (`xoxb-...`), not a user token (`xoxp-...`)
- Verify the bot is running: check Koko logs at `~/Library/Application Support/koko/koko.log`
- Make sure **Messages Tab** is enabled in App Home settings

**"missing_scope" error:**
- Go back to OAuth & Permissions and add the missing scope
- Reinstall the app to your workspace after adding scopes

**Bot responds to old messages on restart:**
- Normal — the bot processes messages from the last 30 seconds on startup, then tracks from there
