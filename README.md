# vibe-watch

`vibe-watch` is a local Go CLI/TUI for monitoring coding-agent session history.

The initial source is Codex JSONL session files under:

```text
~/.codex/sessions/YYYY/MM/DD/*.jsonl
```

The active direction is a user-friendly TUI for real-time session data. Real-time monitoring should watch active Codex JSONL files and poll the session directory while keeping data in memory for now.

Analytics, metrics, and reports exist from an earlier slice but are currently parked unless explicitly reactivated.

## Commands

```bash
go run . scan --limit 5
go run . stats --since 2026-04-01 --format json
go run . suggest
go run . report --session-root ~/.codex/sessions
```

Common flags:

- `--session-root`: Codex session root, defaulting to `~/.codex/sessions`
- `--since`: include sessions on or after `YYYY-MM-DD`
- `--until`: include sessions on or before `YYYY-MM-DD`
- `--limit`: maximum number of session files to scan
- `--format`: `text` or `json`

## Privacy

Real session files are treated as private local history. Tests use synthetic fixtures under `testdata/`; real Codex session JSONL files must not be committed.

## Knowledge Base

Repo knowledge lives under [`docs/`](docs/README.md). Start there for product direction, architecture, Codex session assumptions, analytics definitions, privacy rules, and runbooks.
