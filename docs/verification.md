# Verification

## Required Checks

Run this before handoff:

```bash
go test ./...
```

## Parser Fixture Requirement

Changes to Codex discovery, JSONL parsing, metadata extraction, status inference, token accounting, or activity rendering require parser tests with Codex JSONL fixtures under `testdata/codex/`.

Fixtures must be synthetic or sanitized. Do not copy private real session content into the repo.

## Additional Expected Checks

Use focused tests when relevant:

- Cobra command tests for help, invalid flags, and scriptable output.
- Bubble Tea model update tests for key handling, tab switching, progressive load messages, and viewport scroll messages.
- Token aggregation tests for present, missing, and malformed token metadata.
- Watcher/progressive loading tests for newest-first ordering and nonblocking refresh behavior.

Manual TUI smoke checks are useful for layout, mouse wheel scrolling, theme selection, resize behavior, and live refresh, but they do not replace `go test ./...`.
