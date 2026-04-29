# Vibe Watch Agent Harness

This repo is for `vibe-watch`: a Go CLI/TUI that monitors local coding-agent session files without modifying them. The first supported agent is Codex, whose sessions live at `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`.

## Required Reading

Before planning or editing, read the relevant authoritative docs:

- [docs/README.md](docs/README.md) for the documentation map.
- [docs/product-spec.md](docs/product-spec.md) for product scope and feature expectations.
- [docs/architecture.md](docs/architecture.md) for package boundaries and implementation shape.
- [docs/codex-sessions.md](docs/codex-sessions.md) before changing session discovery or parsing.
- [docs/tui-dashboard.md](docs/tui-dashboard.md) before changing the dashboard tab.
- [docs/tui-session-list.md](docs/tui-session-list.md) before changing grouped session browsing.
- [docs/tui-session-detail.md](docs/tui-session-detail.md) before changing activity rendering.
- [docs/themes.md](docs/themes.md) before changing theme behavior.
- [docs/verification.md](docs/verification.md) before claiming completion.
- [docs/boundaries.md](docs/boundaries.md) for hard stop conditions.

## Operating Contract

- Keep `AGENTS.md` under 100 lines.
- Treat `docs/` as the authoritative harness after this creation pass. Do not edit files in `docs/` unless the user explicitly asks to revise the harness or docs.
- You may edit other repo files as needed, including Go source, tests, fixtures, build files, README, and scripts.
- Network access and dependency installation are allowed when useful for the task.
- Never modify source session data, including files under `~/.codex/sessions/`.
- Never write, rewrite, normalize, redact, rename, move, or delete Codex `.jsonl` session files.
- Do not build cloud sync, a web UI, or any feature that writes back to agent session stores.

## Implementation Defaults

- Use Cobra for command entrypoints, flags, help, and scriptable commands.
- Use Bubble Tea heavily for the TUI event loop and asynchronous refresh behavior.
- Use Bubbles for viewports, lists, tables, spinners, help, key bindings, and other standard widgets.
- Use Lip Gloss for themes and layout styling.
- Keep parsing, indexing, aggregation, and filesystem watching outside Bubble Tea models.
- Load sessions progressively, newest first, so refreshes do not lock the TUI.
- Enable mouse wheel scrolling in every scrollable Bubble Tea viewport or list.
- Keep Bubble Tea debug logs out of stdout.

## Required Verification

Before handoff, run:

```bash
go test ./...
```

Also include parser tests with Codex JSONL fixtures for changes that touch discovery, parsing, metadata extraction, token accounting, status inference, or activity rendering.

## Handoff

Final responses must include changed files, verification run, skipped checks with reasons, known risks, and concrete follow-up work when relevant.
