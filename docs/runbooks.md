# Runbooks

Use these workflows when changing `vibe-watch`.

## Add A Metric

1. Identify the source event field or heuristic.
2. Add or update a synthetic fixture under `testdata/codex/`.
3. Update aggregation in `internal/analysis/build.go`.
4. Add the field to `internal/analysis/types.go` if it is part of public output.
5. Update text reporting in `internal/report/text.go` if needed.
6. Update [Analytics](analytics.md).
7. Run `go test ./...`.
8. Run a relevant CLI smoke check, such as `go run . stats --session-root testdata/codex`.

## Add A Suggestion

1. Define the trigger threshold from existing metrics.
2. Choose severity and confidence.
3. Include aggregate evidence and a concrete action.
4. Add the rule in `internal/analysis/suggest.go`.
5. Add tests for triggering and non-triggering cases where practical.
6. Update [Analytics](analytics.md).
7. Run `go test ./...`.
8. Check `go run . suggest --session-root testdata/codex`.

## Add Codex Event Shape Support

1. Inspect the smallest practical live sample and note field names or event types only.
2. Add a minimal synthetic fixture that models the new structure.
3. Update parser or extraction logic.
4. Add tests that fail without the new support.
5. Update [Codex Sessions](codex-sessions.md) if heuristics changed.
6. Run `go test ./...`.
7. Run a bounded live scan only if the change affects live compatibility.

## Change Command Behavior

1. Keep behavior scriptable through Cobra.
2. Validate flags in command or helper code.
3. Keep domain logic out of `cmd/`.
4. Add or update command tests under `cmd/`.
5. Update `README.md` and [Product Direction](product.md) if user-facing usage changes.
6. Run `go test ./...`.
7. Run `go run . <command> --help`.

## Add A Report Or Output Format

1. Keep raw private content out of default output.
2. Define whether the output is intended for humans or scripts.
3. Add report logic under `internal/report/`.
4. Add tests or golden checks for privacy-sensitive output where practical.
5. Update [Privacy](privacy.md) and [Analytics](analytics.md) if fields change.
6. Run `go test ./...`.

## Start The Real TUI

1. Keep the CLI commands working; the TUI should not replace scriptable workflows.
2. Add Bubble Tea, Bubbles, and Lip Gloss dependencies only when implementing an actual model.
3. Put TUI domain-independent logic under `internal/tui/` or another narrow package.
4. Keep file I/O out of `View`.
5. Model width, height, focus, filters, selected item, loading state, errors, and pending request identity explicitly.
6. Test `Update` with key, resize, load-success, and load-error messages.
7. Run `go test ./...` and `go test -race ./...` when async commands or shared state are involved.
8. Run a real terminal smoke test for resize, quit keys, filtering, focus movement, loading/error states, help, and monochrome readability.

## Bounded Live Scan

Use live scans only to validate compatibility with local Codex history.

```bash
go run . scan --session-root ~/.codex/sessions --limit 2
```

Rules:

- Use `--limit` or a narrow date range.
- Report aggregate output only.
- Do not paste raw session content into handoffs.
- Convert useful schema observations into synthetic fixtures.

## Documentation Update Checklist

Update docs when changing:

- Command names, flags, or output behavior.
- Session discovery or parser heuristics.
- Metric definitions.
- Suggestion rules.
- Privacy behavior, cache paths, or export paths.
- TUI status or interaction model.

Docs to consider:

- `README.md`
- `AGENTS.md`
- `docs/product.md`
- `docs/architecture.md`
- `docs/codex-sessions.md`
- `docs/analytics.md`
- `docs/privacy.md`
- `docs/runbooks.md`
