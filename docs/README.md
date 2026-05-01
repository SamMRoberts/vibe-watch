# Knowledge Base

This directory is the committed knowledge base for `vibe-watch`.

Use it with `AGENTS.md`:

- `AGENTS.md` is the short agent entrypoint and routes agents to these docs.
- `docs/` explains product decisions, architecture, data assumptions, metric definitions, privacy rules, and runbooks.
- `testdata/codex/` captures synthetic Codex event shapes used by tests.
- `.harness-validation/` is a local ignored review workspace, not the durable knowledge base.

## Documents

- [Product Direction](product.md): mission, current scope, command surface, and open decisions.
- [Architecture](architecture.md): package layout, data flow, extension points, and current TUI status.
- [Codex Sessions](codex-sessions.md): session-root layout, discovery rules, parser behavior, fixtures, and schema-drift handling.
- [Analytics](analytics.md): current metric and suggestion definitions.
- [Privacy](privacy.md): data handling rules and output boundaries.
- [Runbooks](runbooks.md): common workflows for adding metrics, suggestions, parser support, reports, and future TUI behavior.

## Current Scope

Active scope is real-time Codex JSONL watching, parser improvements, TUI views, tests, and docs.

Analytics, metrics, suggestions, and reports are parked for possible future scope. Keep their docs accurate, but do not implement new work in those areas unless the user explicitly reactivates them.

## Maintenance Rules

- Update these docs when command behavior, data handling, metric definitions, suggestion rules, output formats, or privacy behavior changes.
- Prefer synthetic examples. Do not copy real Codex prompts, answers, code, command output, secrets, or private file contents into docs.
- When live session history reveals a useful event shape, document the shape in general terms and add a minimal synthetic fixture under `testdata/codex/`.
