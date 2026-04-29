# AGENTS.md

Guidance for automated coding agents and contributors working in this repository.

## Repository overview

vibe-watch is a Go terminal UI for monitoring local agentic coding sessions. It detects sessions from Claude Code, Codex CLI, GitHub Copilot CLI, GitHub Copilot Chat for VS Code, and Amazon Q Developer CLI, then renders dashboards, detail timelines, and analytics in a Bubble Tea TUI.

## Important paths

- `main.go` — application entry point.
- `cmd/` — Cobra commands and flags.
- `internal/agents/` — agent session detectors and parsers.
- `internal/models/` — shared session, message, and token usage models.
- `internal/watcher/` — polling watcher that refreshes detected sessions.
- `internal/tui/` — Bubble Tea models, views, key bindings, and Lip Gloss styling.
- `screenshots/` — images used by README and release documentation.

## Development commands

Run these from the repository root:

```bash
go test ./...
go build ./...
go vet ./...
```

## Contribution guidance

- Keep changes small and focused on the requested task.
- Prefer standard Go tooling and existing project dependencies.
- Add or update tests when changing parser, watcher, model, or TUI behavior.
- Do not commit generated binaries; `vibe-watch` and `vibe-watch.exe` are ignored.
- Keep README screenshots and feature descriptions aligned with current TUI behavior.
- Preserve privacy: do not commit real local agent transcripts, credentials, or user-specific paths.

## Architecture notes

- `agents.AgentDetector` implementations return `[]*models.Session` from local log/session sources.
- `agents.Registry` owns detector registration and sorted aggregation.
- `watcher.Watcher` polls detectors and publishes refreshed session slices.
- `tui.App` coordinates dashboard, detail, analytics, filter, and focused modes.
- The TUI stack is Bubble Tea for runtime/state updates, Bubbles for reusable widgets, Lip Gloss for styling, and Cobra for CLI commands.
