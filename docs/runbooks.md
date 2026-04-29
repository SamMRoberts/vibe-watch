# Runbooks

Use these workflows when changing `vibe-watch`.

## Current Scope

Active work is real-time JSONL watching, parser improvements, TUI views, tests, and docs.

Analytics, metrics, reports, and suggestions are parked. The runbooks for those areas remain for future reuse, but do not use them for new implementation unless the user explicitly reactivates that scope.

## Add Real-Time Watching

1. Define whether the change watches an active file, polls the session directory, or both.
2. Keep live state in memory.
3. Add synthetic JSONL fixtures for append, new-file, malformed-line, and partial-line cases where relevant.
4. Add focused watcher tests under `internal/watcher/`.
5. Keep raw private content out of logs and output.
6. Update [Codex Sessions](codex-sessions.md), [Architecture](architecture.md), and [Privacy](privacy.md) if behavior changes.
7. Run `go test ./...`.
8. Run `scripts/harness_compliance.sh`.
9. Run a bounded live JSONL check only if needed for compatibility.

## Add TUI Behavior

1. Keep CLI helpers scriptable.
2. Add Bubble Tea, Bubbles, and Lip Gloss dependencies only with a real interactive model.
3. Put TUI logic under `internal/tui/` or another narrow package.
4. Keep file I/O out of `View`.
5. Model live session state, width, height, active tab, selected session, current event stream, loading state, errors, and watch identity explicitly.
6. Add direct model transition tests under `internal/tui/`.
7. Run `go test ./...`.
8. Run `scripts/harness_compliance.sh`.
9. Run `go run . tui --session-root testdata/codex --once`.
10. Run a real terminal smoke test for alternate-screen startup, resize, quit keys, focus movement, tab navigation, detail/back flows, mouse-wheel movement when enabled, live updates, loading/error states, help, and monochrome readability when interactive behavior changes.

## Add A Metric

1. Identify the source event field or heuristic.
2. Add or update a synthetic fixture under `testdata/codex/`.
3. Update aggregation in `internal/analysis/build.go`.
4. Add the field to `internal/analysis/types.go` if it is part of public output.
5. Update text reporting in `internal/report/text.go` if needed.
6. Update [Analytics](analytics.md).
7. Run `go test ./...`.
8. Run `scripts/harness_compliance.sh`.
9. Run a relevant CLI smoke check, such as `go run . stats --session-root testdata/codex`.

## Add A Suggestion

1. Define the trigger threshold from existing metrics.
2. Choose severity and confidence.
3. Include aggregate evidence and a concrete action.
4. Add the rule in `internal/analysis/suggest.go`.
5. Add tests for triggering and non-triggering cases where practical.
6. Update [Analytics](analytics.md).
7. Run `go test ./...`.
8. Run `scripts/harness_compliance.sh`.
9. Check `go run . suggest --session-root testdata/codex`.

## Add Codex Event Shape Support

1. Inspect the smallest practical live sample and note field names or event types only.
2. Add a minimal synthetic fixture that models the new structure.
3. Update parser or extraction logic.
4. Add tests that fail without the new support.
5. Update [Codex Sessions](codex-sessions.md) if heuristics changed.
6. Run `go test ./...`.
7. Run `scripts/harness_compliance.sh`.
8. Run a bounded live check only if the change affects live compatibility.

## Change Command Behavior

1. Keep behavior scriptable through Cobra.
2. Validate flags in command or helper code.
3. Keep domain logic out of `cmd/`.
4. Add or update command tests under `cmd/`.
5. Update `README.md` and [Product Direction](product.md) if user-facing usage changes.
6. Run `go test ./...`.
7. Run `scripts/harness_compliance.sh`.
8. Run `go run . <command> --help`.

## Add A Report Or Output Format

1. Keep raw private content out of default output.
2. Define whether the output is intended for humans or scripts.
3. Add report logic under `internal/report/`.
4. Add tests or golden checks for privacy-sensitive output where practical.
5. Update [Privacy](privacy.md) and [Analytics](analytics.md) if fields change.
6. Run `go test ./...`.
7. Run `scripts/harness_compliance.sh`.

## Bounded Live JSONL Check

Use live checks only to validate compatibility with local Codex history.

```bash
go run . scan --session-root ~/.codex/sessions --limit 2
```

Rules:

- Use `--limit` or a narrow date range.
- Report aggregate or structural output only.
- Do not paste raw session content into handoffs.
- Convert useful schema observations into synthetic fixtures.

## Harness Compliance

Run the compliance script before reporting completion for harness, parser, watcher, TUI, docs, and privacy-sensitive work:

```bash
scripts/harness_compliance.sh
```

The script currently checks:

- Every `docs/*.md` file has a `Current Scope` section.
- Committed JSONL files only live under `testdata/codex/`.
- Docs and fixtures do not contain obvious private session paths or credential patterns.
- `go test ./...` passes.

If it fails, make another pass until it succeeds.

## Documentation Update Checklist

Update docs when changing:

- Command names, flags, or output behavior.
- Session discovery or parser heuristics.
- Watcher behavior.
- TUI interaction model.
- Metric definitions if analytics are reactivated.
- Suggestion rules if suggestions are reactivated.
- Privacy behavior, cache paths, or export paths.

Docs to consider:

- `README.md`
- `AGENTS.md`
- `docs/product.md`
- `docs/architecture.md`
- `docs/codex-sessions.md`
- `docs/analytics.md`
- `docs/privacy.md`
- `docs/runbooks.md`
