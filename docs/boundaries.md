# Boundaries

## Editable Areas

Agents may edit repo files outside `docs/`, including Go source, tests, fixtures, build files, scripts, README, and `AGENTS.md`.

`docs/` is editable during this harness creation pass only. After creation, do not edit files in `docs/` unless the user explicitly asks to revise the harness or documentation.

## Forbidden Work

Do not:

- Modify files under `~/.codex/sessions/`.
- Edit, rewrite, delete, move, redact, normalize, or repair Codex `.jsonl` files.
- Add cloud sync.
- Add a web UI.
- Store secrets or credentials in the repo.
- Perform destructive git or filesystem operations without explicit approval.

## Network And Dependencies

Network access is allowed. Dependency installation is allowed when it supports the task. Prefer established Go libraries for Cobra, Bubble Tea, Bubbles, and Lip Gloss rather than hand-rolled replacements.

## Stop Conditions

Stop and ask the user before continuing when:

- A task requires modifying source session data.
- A task conflicts with the no-`docs/`-edits rule after harness creation.
- A destructive command is needed.
- Required verification cannot run and the change has meaningful risk.
- Session data appears to contain secrets that would need copying into fixtures or reports.
