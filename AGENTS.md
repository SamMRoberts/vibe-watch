# Vibe Watch Agent Guide

This file is the agent entrypoint for `vibe-watch`. Keep it short. Durable product and technical knowledge belongs in `docs/`, with this file pointing agents to the right source.

## Start Here

- Product scope and open decisions: [docs/product.md](docs/product.md)
- Architecture and package boundaries: [docs/architecture.md](docs/architecture.md)
- Codex session layout and parser assumptions: [docs/codex-sessions.md](docs/codex-sessions.md)
- Metric and suggestion definitions: [docs/analytics.md](docs/analytics.md)
- Privacy and data handling: [docs/privacy.md](docs/privacy.md)
- Change workflows and verification recipes: [docs/runbooks.md](docs/runbooks.md)

## Current Mission

`vibe-watch` is a local Go CLI/TUI for monitoring coding-agent sessions. The first source is Codex JSONL history under `~/.codex/sessions/YYYY/MM/DD/*.jsonl`. The app produces aggregate scan summaries, analytics, and rule-based suggestions without exposing raw private session content by default.

The current implementation is CLI-first. `scan`, `stats`, `report`, and `suggest` are active commands; `tui` is a placeholder until real interactive behavior is implemented.

## Non-Negotiable Rules

- Treat `~/.codex/sessions` as private, read-only local history. Do not modify, delete, relocate, upload, or commit real session files.
- Do not print raw prompts, answers, code, command output, secrets, or private file contents unless the user explicitly asks for a specific excerpt.
- Use synthetic fixtures under `testdata/codex/` for tests. When live history reveals a new event shape, capture only a minimal synthetic structure.
- Keep Cobra command handlers thin. Scanning, parsing, metrics, suggestions, reports, privacy behavior, and future TUI logic belong outside `cmd/`.
- Do not add Bubble Tea, Bubbles, or Lip Gloss just to keep the placeholder `tui` command alive. Add them with the first real interactive model.
- Keep `.harness-validation/` local and ignored. It is a review workspace, not committed knowledge.

## Working Flow

1. Classify the task: ingestion, parsing, metrics, suggestions, reporting, TUI, docs, tests, or harness work.
2. Read the relevant docs listed above before editing. Prefer repo docs and existing package patterns over re-inventing guidance in this file.
3. State the constraints that affect the change: data source, privacy posture, schema assumptions, command surface, output format, tests, and overlapping user changes.
4. Make scoped edits. Preserve unrelated user changes.
5. Update docs when behavior, data handling, metric definitions, suggestion rules, output formats, privacy behavior, or TUI status changes.
6. Verify using the gates below and report skipped checks with residual risk.

## Verification

Minimum for code changes:

```bash
go test ./...
go run . --help
go run . <command> --help
```

Run as relevant:

```bash
go fmt ./...
go vet ./...
go test -race ./...
go mod tidy
go run . scan --session-root ~/.codex/sessions --limit 2
```

Use bounded live scans only for compatibility checks, and report aggregate output only. For parser, metric, suggestion, report, or privacy changes, add focused synthetic fixture tests.

## Handoff

Final responses should include:

- What changed.
- Files touched.
- Verification run and pass/fail status.
- Data/privacy scope for any live scan.
- Residual risk or skipped checks.

For review-only work, lead with findings ordered by severity and include file/line references.
