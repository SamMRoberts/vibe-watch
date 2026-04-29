# Codex Sessions

Codex session discovery starts at:

```text
~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl
```

Discovery should walk date directories from newest to oldest. Within a day, sort rollout files by the best available recency signal, usually file modification time, then filename as a deterministic fallback.

## Parsing Rules

- Parse JSONL incrementally and tolerate unknown event shapes.
- Preserve enough raw metadata for debugging, but normalize display data into typed internal events.
- Do not fail an entire session because one line is malformed; surface parse errors as session diagnostics.
- Infer agent identity from explicit fields when available and from the Codex adapter as a fallback.
- Infer session status from the latest meaningful activity and file freshness when explicit status is unavailable.
- Extract token data when present, and distinguish missing token data from zero tokens.

## Required Fixture Coverage

Parser changes require Codex JSONL fixtures under `testdata/codex/`. Fixtures should cover:

- User prompts.
- Assistant messages.
- Tool calls.
- Tool results or outputs when present.
- Token metadata.
- Unknown event types.
- Malformed lines.
- Incomplete or still-growing sessions.
