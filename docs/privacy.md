# Privacy

`vibe-watch` analyzes private local Codex history. Privacy is part of the product contract, not an optional enhancement.

## Source Data

Real Codex sessions live under:

```text
~/.codex/sessions/YYYY/MM/DD/*.jsonl
```

The app may read these files for local analysis. It must not modify, delete, rewrite, relocate, upload, or commit them.

## Default Output Contract

Default command output may include:

- Counts.
- Date ranges.
- Session identifiers or derived aggregate categories.
- Data-quality signals.
- Rule-based suggestions with evidence expressed as aggregate numbers.

Default command output must not include:

- Raw user prompts.
- Raw assistant answers.
- Code snippets copied from sessions.
- Command output copied from sessions.
- Secrets, tokens, credentials, or private file contents.
- Large raw event payloads.

## Live Smoke Tests

Live scans against `~/.codex/sessions` should be bounded:

```bash
go run . scan --session-root ~/.codex/sessions --limit 2
```

or constrained with a narrow date range.

Handoffs should describe the live scope without exposing content. Example:

```text
bounded live scan of 2 local Codex session files; output was aggregate counts only
```

## Fixtures

Tests must use synthetic fixtures under `testdata/codex/`.

Do not create fixtures by copying real session lines. Instead, preserve only the structural properties needed for the parser or metric.

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
