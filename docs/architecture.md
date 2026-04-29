# Architecture

`vibe-watch` is organized as a Go CLI/TUI with domain logic in `internal/`.

## Current Scope

Active architecture work should prioritize real-time JSONL watching, parser improvements, TUI views, tests, and docs.

Analytics, metrics, suggestions, and reports are parked for possible future scope. Existing packages may remain, but new work should not expand those areas unless the user explicitly reactivates them.

## Package Map

```text
main.go
cmd/
  root.go
  scan.go
  stats.go
  report.go
  suggest.go
  tui.go
internal/
  codex/
    sessions/
      discover.go
    events/
      parse.go
  watcher/
    snapshot.go
  tui/
    model.go
    run.go
  analysis/
    build.go
    run.go
    suggest.go
    types.go
  report/
    json.go
    text.go
testdata/
  codex/
```

## Data Flow

Current historical CLI flow:

1. `cmd` parses flags and validates command inputs.
2. `internal/codex/sessions` discovers candidate JSONL files under the session root.
3. `internal/codex/events` parses each JSONL file line by line.
4. `internal/analysis` aggregates scan summary, metrics, and suggestions.
5. `internal/report` formats text or JSON output.

Current real-time TUI flow:

1. Poll the session directory for JSONL files.
2. Poll the session directory for new or changed JSONL files.
3. Parse discovered JSONL files into sanitized in-memory snapshots.
4. Maintain in-memory session state.
5. Render TUI views from in-memory state.

Commands should orchestrate. They should not own watching, parsing, TUI state, aggregation, suggestion, or privacy logic.

## CLI Rules

- `main.go` owns final process exit.
- `cmd/` wires Cobra commands and global flags.
- Use `RunE`, return errors, and keep command behavior testable.
- Write primary command output to stdout.
- Write diagnostics, skipped-file warnings, and progress to stderr when those exist.
- Keep JSON output stable enough for scripting.

## TUI Direction

The `tui` command opens a real-time, user-friendly session monitor backed by periodic snapshot loads.

Current behavior:

- Uses Bubble Tea, Bubbles, and Lip Gloss.
- Loads live session state through bounded async work.
- Keep file I/O out of `View`.
- Tracks width, height, loaded state, errors, and current sanitized watcher snapshot.
- Supports manual refresh and quit keys.
- Supports `--once` for non-interactive sanitized snapshot rendering.
- Renders a Lip Gloss dashboard with status cards, an active-session panel, and a recent-events panel.

Future behavior:

- Track focus, selected session, active filters, current event stream, and pending watch identity in model state.
- Add richer direct model transition tests as views become interactive.
- Run a manual terminal smoke test for resize, quit keys, filtering, focus movement, loading/error states, help display, and monochrome readability.

## Extension Points

- New Codex event shapes: add synthetic fixture lines under `testdata/codex/`, then update parser or metric extraction.
- New watcher behavior: add synthetic JSONL append/polling tests before live checks.
- New TUI behavior: add model tests and a manual terminal smoke check.
- New metrics, suggestions, or output formats: currently out of scope unless explicitly reactivated.
- Future agents: add a new adapter boundary after Codex ingestion is stable.
