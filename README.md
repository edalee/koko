<p align="center">
  <img src="build/appicon.png" alt="Kõkõ" width="128" height="128" style="border-radius: 22%;" />
</p>

<h1 align="center">Kõkõ</h1>

<p align="center">
  <strong>A desktop workspace for Claude Code</strong><br/>
  Run multiple Claude sessions side-by-side with GitHub, Slack, and git awareness — all in one window.
</p>

<p align="center">
  <a href="https://github.com/edalee/koko/releases/latest"><img src="https://img.shields.io/github/v/release/edalee/koko?style=flat-square&color=4ade80" alt="Release" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-BSL_1.1-blue?style=flat-square" alt="License" /></a>
  <img src="https://img.shields.io/badge/platform-macOS_%7C_Linux-lightgrey?style=flat-square" alt="Platform" />
</p>

<br/>

<p align="center">
  <img src="docs/screenshots/koko-main.png" alt="Kõkõ — Claude Code session with terminal and sidebar panels" width="800" />
</p>

---

## What is Kõkõ?

Kõkõ (named after the [Tui bird](https://en.wikipedia.org/wiki/Tui_(bird))) is a native desktop app that wraps Claude Code's interactive terminal in a purpose-built workspace. Instead of switching between your terminal, GitHub, and Slack, Kõkõ keeps everything you need visible while you work.

Each session launches Claude Code in a directory you choose. The left sidebar shows your open sessions, and the right sidebar surfaces GitHub PRs, Slack DMs, git file changes, and session context — so you never lose context.

## Features

### Claude Code Sessions
- **Named sessions** — Each session defaults to the directory name, or give it a custom name
- **Full interactive TUI** — Claude Code renders natively in xterm.js with WebGL
- **Session persistence** — Sessions survive app restarts; reconnect with `claude --continue`
- **Session history** — Recently closed sessions shown in the new session dialog with last message preview
- **Context display** — Live context window usage percentage and model name per session
- **Approval detection** — Amber pulse on session icons when Claude is waiting for tool approval
- **Clipboard support** — Full Cmd+C/V copy-paste in terminal sessions
- **Keyboard shortcuts** — `Cmd+N` new session, `Cmd+W` close, `Cmd+1-9` switch

### Session Context
- **MCP Servers** — Connection status of configured MCP servers
- **Agents** — Built-in Claude agents with their models
- **Commands** — Project, global, and plugin slash commands — click to inject into the active terminal
- **Subagent Monitor** — See Claude's spawned child processes in real time

### Awareness Panels
- **GitHub PRs** — Live PR list from your repos with review status, approve/merge actions
- **GitHub Notifications** — Unread notifications with participating/all filter, mark-as-read
- **Slack DMs** — Unread direct messages via bot token, deep-links to Slack app
- **File Changes** — Git diff for the active session's directory (staged/unstaged)

### Safe Working

Kõkõ includes built-in features to help you maintain healthy working habits:

- **Quiet Hours** — Set a time window (e.g. 23:00–07:00) when the app blocks access with a full-screen overlay. A countdown shows when work resumes. Delay 30 minutes if you're wrapping up.
- **Break Reminders** — Configure a work/break cycle (e.g. 90 min work, 15 min break). When the timer fires, a visual overlay with a circular progress ring nudges you to step away. Skip if you're in flow, but you'll be reminded again next cycle.

<p align="center">
  <img src="docs/screenshots/quiet-hours.png" alt="Kõkõ — Quiet hours blocker encouraging rest" width="800" />
</p>

### Workspace
- **Quick Terminal** — `Cmd+`` ` slides up a per-session zsh shell for quick commands
- **Glassmorphism UI** — Dark theme with frosted glass panels and mint accents
- **Auto-updates** — Checks for new releases and shows a toolbar notification

<p align="center">
  <img src="docs/screenshots/new-session-dialog.png" alt="Kõkõ — New session dialog with name and directory picker" width="800" />
</p>

## Install

### macOS (Homebrew)

```bash
brew tap edalee/koko
brew install koko
```

Update:

```bash
brew upgrade koko
```

> **First launch:** The app is not notarized. macOS may block it — go to **System Settings → Privacy & Security → Open Anyway**.

### Manual download

| Platform | Download |
|----------|----------|
| **macOS** (Universal) | [Koko-vX.X.X-macOS.dmg](https://github.com/edalee/koko/releases/latest) |
| **Linux** (x86_64) | [Koko-vX.X.X-linux-x86_64.AppImage](https://github.com/edalee/koko/releases/latest) |

For macOS: open the DMG, drag to Applications, then `sudo xattr -cr /Applications/Koko.app`.

### Prerequisites

Kõkõ launches Claude Code for you, so you need it installed:

```bash
# Install Claude Code (requires Node.js)
npm install -g @anthropic-ai/claude-code
```

You also need an Anthropic API key or active Claude subscription configured for Claude Code.

### Optional: GitHub & Slack

- **GitHub PRs** — Requires [`gh` CLI](https://cli.github.com/) authenticated (`gh auth login`)
- **Slack DMs** — Requires a Slack bot token with `im:history`, `im:read`, `users:read` scopes (configure in Settings)

## Build from Source

Requires Go 1.24+, Node.js 22+, and the [Wails CLI](https://wails.io/docs/gettingstarted/installation).

```bash
git clone https://github.com/edalee/koko.git
cd koko

# Install frontend dependencies
make install-fe

# Run in development mode (hot reload)
make dev

# Build production app bundle
make build

# Run tests
make test
```

## How It Works

Kõkõ is built with [Wails v2](https://wails.io/) — a Go backend connected to a React frontend running in a native webview.

```
┌─────────────────────────────────────────────────┐
│  Kõkõ Window                                    │
│                                                 │
│  ┌──────┐  ┌──────────────────┐  ┌───────────┐  │
│  │      │  │                  │  │  File      │  │
│  │  S   │  │   Claude Code    │  │  Changes   │  │
│  │  e   │  │   Terminal       │  ├───────────┤  │
│  │  s   │  │   (xterm.js)     │  │  Session   │  │
│  │  s   │  │                  │  │  Context   │  │
│  │  i   │  │                  │  ├───────────┤  │
│  │  o   │  ├──────────────────┤  │  Subagent  │  │
│  │  n   │  │  Mode │ Ctx: 42% │  │  Monitor   │  │
│  │  s   │  │                  │  │            │  │
│  └──────┘  └──────────────────┘  └───────────┘  │
│  ┌──────────────────────────────────────────┐   │
│  │  Quick Terminal (Cmd+`)                  │   │
│  └──────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

- **Go backend** manages PTY sessions, GitHub API calls (via `gh`), Slack API, git operations, Claude CLI parsing, and app config
- **React frontend** renders xterm.js terminals, glassmorphism panels, and overlay pages
- **Wails IPC** bridges Go ↔ JavaScript with type-safe bindings (Go structs become TypeScript classes)
- **PTY sessions** stream base64-encoded terminal data over Wails events
- **Vitest** tests guard against terminal resize and state regressions

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop shell | Wails v2 |
| Backend | Go 1.24 |
| Frontend | React 19, TypeScript |
| Terminal | xterm.js v5 + WebGL |
| Styling | Tailwind CSS v4, OKLCH dark theme |
| Testing | Vitest, React Testing Library |
| Icons | Lucide React |
| Panels | react-resizable-panels |
| PTY | creack/pty |

## Project Structure

```
main.go                    Wails entry point + macOS Edit menu
app.go                     App lifecycle, update checker, status line
terminal_manager.go        PTY session management, spawns claude per session
claude_service.go          MCP servers, agents, commands, plugin skills
github_service.go          GitHub PR + notification fetching via gh CLI
git_service.go             Git file changes + branch info
slack_service.go           Slack DM fetching via bot token
config_service.go          App config persistence
process_monitor.go         Child process tree scanning for subagents
types.go                   Shared Go types

frontend/src/
  App.tsx                  App shell with session sidebar + overlay routing
  globals.css              Glassmorphism theme tokens (OKLCH)
  components/
    Toolbar.tsx            Title bar with notification badges + update banner
    SessionSidebar.tsx     Session list with approval detection + inline rename
    RightSidebar.tsx       File changes, session context, subagent monitor
    TerminalPane.tsx       xterm.js terminal wrapper (one per session)
    QuickTerminal.tsx      Per-session slide-up zsh shell
    ClaudeModeSwitcher.tsx Context usage bar + mode buttons
    GitHubPanel.tsx        PR cards with approve/merge actions
    SlackPanel.tsx         DM list with deep links
    NotificationsPanel.tsx GitHub notifications with filter + mark-read
    NewSessionDialog.tsx   Session creation with history + directory picker
    SettingsPanel.tsx      App configuration (Slack, safe working)
    SafeWorkingOverlay.tsx Quiet hours + break reminder overlays
    OverlayPage.tsx        Glassmorphism floating overlay wrapper
  hooks/
    useSessionTabs.ts      Session state, persistence, history
    useSessionActivity.ts  PTY activity monitoring + approval detection
    useSessionContext.ts   MCP servers, agents, commands fetching
    useFileChanges.ts      Git diff polling for active session
    useSubagents.ts        Child process tree polling
    useGitHub.ts           PR fetching
    useNotifications.ts    GitHub notification fetching
    useSlack.ts            Slack DM fetching
    useSafeWorking.ts      Quiet hours + break timer logic
    useOverlay.ts          Floating overlay page management
    useKeyboardShortcuts.ts  Cmd+N/W/1-9 bindings
    useUpdateCheck.ts      Release update polling
  test/
    setup.ts               Vitest setup with Wails binding mocks
    terminal-resize-guard.test.tsx  Scroll position preservation tests
    quick-terminal-state.test.tsx   Per-session QT state tests
    session-activity.test.ts        Activity tracking tests
```

## License

[Business Source License 1.1](LICENSE) — source available, non-competing production use allowed. Converts to GPL 2.0+ on 2030-02-27.

## Credits

Built by [Edward Lee](https://github.com/edalee).

Named after the [Tui](https://en.wikipedia.org/wiki/Tui_(bird)) (Prosthemadera novaeseelandiae) — a New Zealand songbird known for its complex vocalisations and iridescent plumage.
