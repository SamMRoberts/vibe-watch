# Codex Sessions

Codex session data is treated as private local history.

## Current Scope

Active work should support real-time monitoring by watching active JSONL files and polling the session directory for new or changed files. Parsing must continue to work for historical files, but new product work should optimize for live display rather than analytics.

Default root:

```text
~/.codex/sessions
```

Expected file layout:

```text
~/.codex/sessions/YYYY/MM/DD/*.jsonl
```

Example path shape:

```text
~/.codex/sessions/2026/04/29/rollout-...jsonl
```

## Discovery Rules

Discovery is implemented in `internal/codex/sessions`.

- Walk the configured `--session-root`.
- Include only files with a `.jsonl` extension.
- Derive session date from the first three relative path components: year, month, day.
- Ignore files that do not fit the date-folder shape.
- Sort by date, then session ID.
- Apply `--since`, `--until`, and `--limit` before parsing content where possible.
- Treat unreadable files as skipped scan inputs, not as reasons to mutate source data.

## Parsing Rules

Parsing is implemented in `internal/codex/events`.

- Parse JSONL line by line.
- Keep memory bounded by avoiding whole-corpus reads.
- Count malformed lines instead of failing the whole scan.
- Preserve event maps internally for aggregation.
- Do not print raw event content by default.

## Real-Time Watching Rules

Real-time monitoring should:

- Watch the active session JSONL file for appended lines.
- Poll the session directory for new or changed JSONL files.
- Keep live state in memory for now.
- Tolerate partial writes by waiting for complete JSONL lines before parsing.
- Preserve file offset or equivalent in-memory cursor per watched file.
- Avoid persistent caches unless the user explicitly asks.
- Report bounded live JSONL scope in handoffs without pasting raw content.

## Current Event Extraction Heuristics

The current parser/analyzer code can extract these fields. Analytics use is parked, but these heuristics may still help live display:

- Event type from top-level `type`, `event`, or `kind`.
- Timestamp presence from `timestamp`, `time`, or `created_at` anywhere in the event.
- Repository/workdir values from keys such as `cwd`, `workdir`, `repo`, or `repository`.
- Model values from `model`.
- Tool names from `tool`, `tool_name`, or `recipient_name`.
- Shell command attempts from tool names containing `exec_command` and an event containing `cmd` or `command`.
- Failed shell command results from `exit_code` or `exitCode` values that are not zero.
- Approval signals from event types containing `approval` or text containing `require_escalated`.
- Error signals from event types or text containing `error` or `failed`.
- Verification signals from text containing `go test`, `verification`, or `passed`.
- Final response signals from event types containing `final` or text containing `final answer` or `final response`.

These are heuristics. When they are wrong or noisy, add a synthetic fixture and update parser behavior or display rules with the code change.

## Fixture Policy

Tests use synthetic session files under `testdata/codex/`.

Fixtures should:

- Preserve useful event structure.
- Use fake or generic content.
- Avoid real prompts, answers, code, command output, paths with sensitive details, and secrets.
- Include malformed lines when testing parser tolerance.
- Include append/polling scenarios for watcher tests when practical.

## Schema Drift Procedure

When live Codex history exposes a new shape:

1. Inspect the smallest practical sample.
2. Summarize keys or event types, not raw content.
3. Add a minimal synthetic fixture under `testdata/codex/`.
4. Update parser or aggregation logic.
5. Update live-display or parser docs if behavior changed.
6. Run `go test ./...` and any relevant bounded live JSONL check.
