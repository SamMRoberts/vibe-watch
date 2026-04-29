# Repository custom instructions

Use these instructions for automated coding assistants working on vibe-watch.

## Project context

vibe-watch is a Go TUI application. It watches local agentic coding tool session files and renders dashboard, detail, and analytics views in the terminal.

## Coding expectations

- Use Go idioms and keep package boundaries intact.
- Keep parser changes isolated to the relevant detector in `internal/agents/` when possible.
- Keep TUI state and rendering changes inside `internal/tui/` unless shared model changes are required.
- Avoid adding dependencies unless they clearly simplify the implementation and fit the existing stack.
- Do not add comments for obvious behavior; match the existing style.
- Never commit personal session logs, secrets, or machine-specific absolute paths.

## Validation

Before completing behavior changes, run:

```bash
go test ./...
go build ./...
go vet ./...
```

Documentation-only changes do not require the Go validation commands unless they affect generated examples or documented command output.
