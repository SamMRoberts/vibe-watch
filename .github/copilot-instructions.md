# Copilot instructions

Follow the repository guidance in [`../AGENTS.md`](../AGENTS.md) and [`../custom-instructions.md`](../custom-instructions.md).

Key reminders:

- This is a Go TUI built with Cobra, Bubble Tea, Bubbles, and Lip Gloss.
- Keep changes focused and preserve existing architecture under `cmd/`, `internal/agents/`, `internal/watcher/`, `internal/models/`, and `internal/tui/`.
- Validate behavior changes with `go test ./...`, `go build ./...`, and `go vet ./...`.
- Do not commit local agent transcripts, secrets, generated binaries, or user-specific paths.
