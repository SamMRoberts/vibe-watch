# ⚡ vibe-watch

A graphical terminal UI (TUI) for monitoring and analyzing session data from agentic coding agents and CLIs — Claude Code, Codex CLI, GitHub Copilot CLI, GitHub Copilot Chat for VS Code, and Amazon Q Developer CLI.

Run it from your repository in a **separate terminal** alongside your AI coding agent to get real-time dashboards of session activity and token usage for the current repo.

![vibe-watch dashboard](https://via.placeholder.com/800x400?text=vibe-watch+TUI+Dashboard)

## Features

- **Real-time session monitoring** — auto-polls every 2 seconds for new/updated sessions
- **Dashboard view** — sortable table of current-repository sessions with token counts, duration, and status
- **Detail view** — full message history with token-level breakdowns per message
- **Analytics view** — aggregate stats, agent comparisons, bar charts, top projects
- **Multi-agent support** — Claude Code, Codex CLI, GitHub Copilot CLI, GitHub Copilot Chat for VS Code, Amazon Q
- **Filtering** — filter sessions by agent type or project name
- **No config required** — detects sessions automatically from standard log locations

## Supported Agents

| Agent | Log Location |
|---|---|
| **Claude Code** | `~/.claude/projects/` (JSONL) |
| **Codex CLI** | `~/.codex/sessions/` (JSON) |
| **GitHub Copilot CLI** | `~/.copilot/session-state/` |
| **GitHub Copilot Chat for VS Code** | `~/Library/Application Support/Code/User/workspaceStorage/` |
| **Amazon Q Developer** | `~/.aws/amazonq/` |

## Installation

### From source (requires Go 1.21+)

```bash
git clone https://github.com/SamMRoberts/vibe-watch
cd vibe-watch
go build -o vibe-watch .
./vibe-watch
```

### Go install

```bash
go install github.com/SamMRoberts/vibe-watch@latest
```

## Usage

```bash
# Start the TUI dashboard (default command)
vibe-watch

# Explicit watch subcommand
vibe-watch watch

# Filter to a specific agent
vibe-watch watch --agent claude
vibe-watch watch --agent codex
vibe-watch watch --agent copilot
vibe-watch watch --agent chat
vibe-watch watch --agent amazonq

# Set refresh interval (default: 2 seconds)
vibe-watch watch --refresh 5
```

## Key Bindings

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Cycle between views (Dashboard ↔ Analytics) |
| `↑` / `↓` or `k` / `j` | Navigate session list |
| `enter` | Open session detail |
| `esc` | Return to dashboard |
| `r` | Force refresh |
| `/` | Filter sessions by name |
| `pgup` / `b` | Scroll up (detail view) |
| `pgdown` / `f` | Scroll down (detail view) |
| `q` / `ctrl+c` | Quit |

## Tech Stack

- **[Go](https://golang.org)** — language
- **[Cobra](https://github.com/spf13/cobra)** — CLI structure
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — TUI framework
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** — terminal styling
- **[Bubbles](https://github.com/charmbracelet/bubbles)** — TUI components (table, viewport)

## Project Structure

```
vibe-watch/
├── main.go                        # Entry point
├── cmd/
│   ├── root.go                    # Root Cobra command
│   └── watch.go                   # watch subcommand
└── internal/
    ├── models/
    │   └── session.go             # Session, Message, TokenUsage models
    ├── agents/
    │   ├── detector.go            # AgentDetector interface & registry
    │   ├── claude.go              # Claude Code JSONL parser
    │   ├── codex.go               # Codex CLI JSON parser
    │   ├── copilot.go             # GitHub Copilot CLI session scanner
    │   ├── copilot_chat.go        # GitHub Copilot Chat for VS Code scanner
    │   └── amazonq.go             # Amazon Q log scanner
    ├── watcher/
    │   └── watcher.go             # Polling-based file watcher
    └── tui/
        ├── app.go                 # Main Bubble Tea model
        ├── dashboard.go           # Dashboard (session table) view
        ├── detail.go              # Session detail (scrollable) view
        ├── analytics.go           # Analytics (charts & stats) view
        ├── keys.go                # Key bindings
        └── styles.go              # Lip Gloss color scheme & styles
```
