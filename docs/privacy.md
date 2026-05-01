# Privacy

`vibe-watch` analyzes private local Codex history. Privacy is part of the product contract, not an optional enhancement.

## Source Data

Real Codex sessions live under:

```text
~/.codex/sessions/YYYY/MM/DD/*.jsonl
```

The app may read these files for local analysis. It must not modify, delete, rewrite, relocate, upload, or commit them.

## Current Scope

Active work should focus on live JSONL watching and TUI display. Live state should be in memory only for now.

## Default Output Contract

Default command or TUI output may include:

- Counts.
- Date ranges.
- Session identifiers or derived aggregate categories.
- Repo or workdir path metadata when it is needed for session identification.
- Data-quality signals.
- Sanitized event type, timestamp, tool, model, and repository basename fields.
- Rule-based suggestions with evidence expressed as aggregate numbers.

The interactive TUI detail view is an explicit local inspection surface. When the user opens a selected session detail view, it may show bounded excerpts extracted from that session, including prompts, assistant messages, reasoning summaries, goals, descriptions, and tool-call text. This content must remain local, in memory, and out of docs, tests, reports, logs, and handoffs unless the user explicitly asks for a specific excerpt.

Default command output must not include:

- Raw user prompts.
- Raw assistant answers.
- Code snippets copied from sessions.
- Command output copied from sessions.
- Secrets, tokens, credentials, or private file contents.
- Large raw event payloads.

Repo/workdir paths can reveal local project names. Keep them limited to session metadata displays and do not copy them into docs, fixtures, or handoffs unless explicitly needed.

## Live Smoke Tests

Live JSONL checks against `~/.codex/sessions` should be bounded:

```bash
go run . scan --session-root ~/.codex/sessions --limit 2
```

or constrained with a narrow date range or a known active test file.

Handoffs should describe the live scope without exposing content. Example:

```text
bounded live JSONL check of 2 local Codex session files; output was aggregate or structural only
```

## Fixtures

Tests must use synthetic fixtures under `testdata/codex/`.

Do not create fixtures by copying real session lines. Instead, preserve only the structural properties needed for parser, watcher, TUI, or privacy behavior.

## Generated Files

Generated local outputs should stay out of Git. Current ignored locations include:

- `.harness-validation/`
- `.vibe-watch/`
- `reports/`
- `exports/`
- `*.session.jsonl`
- `*.sessions.jsonl`

If future commands add cache or export paths, document what is stored and add ignore rules before generating private data.

## Redaction Rule

If a failure requires showing examples, show field names, event types, counts, or synthetic payloads. Do not paste real raw session content unless the user explicitly asks for a specific excerpt.
