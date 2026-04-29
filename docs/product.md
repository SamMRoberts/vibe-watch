# Product Direction

`vibe-watch` is a local Go CLI/TUI for monitoring coding-agent session history.

The initial source is Codex session history stored as JSONL files under:

```text
~/.codex/sessions/YYYY/MM/DD/*.jsonl
```

The product should help the user understand agent work over time through aggregate statistics, analytics, data-quality signals, and evidence-backed workflow suggestions.

## Current Slice

The current app is CLI-first.

Implemented commands:

- `scan`: summarizes scan quality and source coverage.
- `stats`: prints aggregate metrics.
- `suggest`: prints rule-based suggestions.
- `report`: combines scan summary, metrics, and suggestions.
- `tui`: placeholder that directs users to the CLI workflow.

Common flags:

- `--session-root`: Codex session root. Defaults to `~/.codex/sessions`.
- `--since`: include sessions on or after `YYYY-MM-DD`.
- `--until`: include sessions on or before `YYYY-MM-DD`.
- `--limit`: maximum number of session files to scan.
- `--format`: `text` or `json`.

## Current Product Assumptions

- Codex is the first supported agent source.
- Real session files are read-only inputs.
- Reports are aggregate and privacy-preserving by default.
- Suggestions are deterministic rules over observed aggregates, not model-generated advice.
- The CLI remains scriptable even after a full TUI is added.

## Non-Goals

- Mutating, deleting, or relocating `~/.codex/sessions` files.
- Uploading session data or adding telemetry.
- Committing raw session JSONL, raw prompts, answers, code snippets, command output, or secrets.
- Supporting non-Codex agents before the Codex adapter boundary is stable.
- Adding a GUI, web dashboard, daemon, background monitor, or scheduled scans without explicit user direction.

## Open Product Decisions

- MVP analytics priority: activity trends, tool usage, verification quality, error loops, handoff quality, or suggestion generation.
- Report/cache location: workspace-only, user data directory, or opt-in per command.
- First full-screen TUI workflow: session list, trend dashboard, suggestion review, or data-quality inspection.
