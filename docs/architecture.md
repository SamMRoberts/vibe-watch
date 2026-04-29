# Architecture

`vibe-watch` is organized as a Cobra CLI with domain logic in `internal/`.

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

1. `cmd` parses flags and validates command inputs.
2. `internal/codex/sessions` discovers candidate JSONL files under the session root.
3. `internal/codex/events` parses each JSONL file line by line.
4. `internal/analysis` aggregates scan summary, metrics, and suggestions.
5. `internal/report` formats text or JSON output.

Commands should orchestrate. They should not own scanning, parsing, aggregation, suggestion, or privacy logic.

## CLI Rules

- `main.go` owns final process exit.
- `cmd/` wires Cobra commands and global flags.
- Use `RunE`, return errors, and keep command behavior testable.
- Write primary command output to stdout.
- Write diagnostics, skipped-file warnings, and progress to stderr when those exist.
- Keep JSON output stable enough for scripting.

## TUI Status

The current `tui` command is a placeholder. Do not add Bubble Tea, Bubbles, or Lip Gloss dependencies until implementing the first real interactive model.

When TUI work starts:

- Load summaries through bounded async work.
- Keep file I/O out of `View`.
- Track width, height, focus, selected session, active filters, loading state, errors, and pending request identity in model state.
- Add direct model transition tests.
- Run a manual terminal smoke test for resize, quit keys, filtering, focus movement, loading/error states, help display, and monochrome readability.

## Extension Points

- New Codex event shapes: add synthetic fixture lines under `testdata/codex/`, then update parser or metric extraction.
- New metrics: add fields to `analysis.Metrics`, update aggregation, reports, docs, and tests.
- New suggestions: add deterministic rules in `internal/analysis/suggest.go`, with triggering and non-triggering tests where practical.
- New output formats: add report formatters without exposing raw private content by default.
- Future agents: add a new adapter boundary after Codex ingestion is stable.
