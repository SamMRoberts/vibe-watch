# Product Direction

`vibe-watch` is a local Go CLI/TUI for monitoring coding-agent session history.

The initial source is Codex session history stored as JSONL files under:

```text
~/.codex/sessions/YYYY/MM/DD/*.jsonl
```

The product should help the user observe coding-agent work as it happens through a user-friendly TUI over live Codex JSONL session data.

## Current Scope

Active scope:

- Real-time session file watching.
- Polling the Codex session directory for new or changed JSONL files.
- Parser improvements needed for live display.
- User-friendly TUI views.
- Tests and docs.

Parked for possible future scope:

- Analytics.
- Metrics.
- Reports.
- Rule-based suggestions.

Do not implement new analytics, metrics, report, or suggestion work unless the user explicitly reactivates that direction.

## Current Implementation

The current app is CLI-first.

Implemented commands:

- `scan`: summarizes scan quality and source coverage.
- `stats`: prints aggregate metrics. Currently parked for future scope.
- `suggest`: prints rule-based suggestions. Currently parked for future scope.
- `report`: combines scan summary, metrics, and suggestions. Currently parked for future scope.
- `tui`: opens a polling real-time Codex session monitor with sanitized recent event display. Use `--once` for non-interactive smoke checks.

Common flags:

- `--session-root`: Codex session root. Defaults to `~/.codex/sessions`.
- `--since`: include sessions on or after `YYYY-MM-DD`.
- `--until`: include sessions on or before `YYYY-MM-DD`.
- `--limit`: maximum number of session files to scan.
- `--format`: `text` or `json`.

## Current Product Assumptions

- Codex is the first supported agent source.
- Real session files are read-only inputs.
- Real-time monitoring should watch active JSONL files and poll the session directory.
- Real-time state should remain in memory for now.
- The TUI should be the primary user-friendly experience.
- CLI helpers should remain scriptable where they support parser, watcher, or TUI development.

## Non-Goals

- Mutating, deleting, or relocating `~/.codex/sessions` files.
- Uploading session data or adding telemetry.
- Committing raw session JSONL, raw prompts, answers, code snippets, command output, or secrets.
- Supporting non-Codex agents before the Codex adapter boundary is stable.
- Adding a GUI, web dashboard, daemon, persistent cache, background monitor, or scheduled scans without explicit user direction.

## Open Product Decisions

- First expanded TUI workflow beyond active session stream: session list, event detail, or data-quality inspection.
- File-watching strategy details beyond polling: exact watcher library and active-file detection refinements.
