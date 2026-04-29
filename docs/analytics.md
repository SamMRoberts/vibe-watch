# Analytics

## Current Scope

Analytics, metrics, reports, and rule-based suggestions are currently parked. Keep this document accurate for existing code, but do not implement new analytics work unless the user explicitly reactivates this area.

Analytics are deterministic aggregates over local Codex session JSONL files.

The source of truth for result structures is `internal/analysis/types.go`.

## Scan Summary

`ScanSummary` reports scan coverage and data quality:

- `session_root`: configured root used for discovery.
- `files_scanned`: JSONL files opened and parsed.
- `sessions_scanned`: currently equivalent to parsed JSONL files.
- `lines_scanned`: total JSONL lines scanned.
- `events_scanned`: valid JSON events parsed.
- `malformed_lines`: lines that could not be decoded as JSON.
- `skipped_files`: files discovered but not parsed because they could not be opened.
- `first_date`: earliest included session date.
- `last_date`: latest included session date.

## Metrics

`Metrics` reports aggregate signals:

- `sessions_by_day`: count of parsed session files by `YYYY-MM-DD`.
- `event_types`: counts by event type extracted from `type`, `event`, or `kind`; missing type becomes `unknown`.
- `repositories`: counts by repository/workdir-like values from `cwd`, `workdir`, `repo`, or `repository`; absolute paths are reduced to basename.
- `models`: counts by `model`.
- `tools`: counts by `tool`, `tool_name`, or `recipient_name`.
- `shell_commands`: count of `exec_command` tool events that also include `cmd` or `command`.
- `failed_shell_commands`: count of `exec_command` events with non-zero `exit_code` or `exitCode`.
- `approval_events`: count of events with approval-like type text or `require_escalated` content.
- `error_events`: count of events with error-like type text or content containing `error` or `failed`.
- `verification_signals`: count of events containing `go test`, `verification`, or `passed`.
- `final_responses`: count of events with final-like type text or content containing `final answer` or `final response`.
- `unknown_event_types`: count of events with no recognized event type field.
- `data_quality`: duplicate scan-quality metrics plus missing timestamp counts.

## Suggestion Rules

Suggestions are rule-based in `internal/analysis/suggest.go`.

### No Sessions Found

Trigger:

- `sessions_scanned == 0`

Output:

- Severity: `info`
- Confidence: `1.0`
- Action: check `--session-root`, `--since`, `--until`, or `--limit`.

### Increase Explicit Verification Evidence

Trigger:

- `verification_signals < sessions_scanned`

Output:

- Severity: `medium`
- Confidence: `0.7`
- Action: end sessions with concrete validation commands and results.

### Review Repeated Shell Command Failures

Trigger:

- `shell_commands >= 3`
- failed shell commands are at least 20 percent of shell command attempts.

Output:

- Severity: `medium`
- Confidence: `0.75`
- Action: pause after repeated failures to inspect assumptions, paths, environment, or sandbox requirements.

### Track Escalation Reasons

Trigger:

- `approval_events > 0`

Output:

- Severity: `low`
- Confidence: `0.65`
- Action: group escalation causes into dependency, network, destructive, and out-of-workspace categories.

### Harden Codex Event Decoding

Trigger:

- `malformed_lines > 0` or `unknown_event_types > 0`

Output:

- Severity: `low`
- Confidence: `0.8`
- Action: add synthetic fixtures for unrecognized shapes before relying on derived metrics.

### Improve Final Handoff Detection

Trigger:

- `final_responses < sessions_scanned`

Output:

- Severity: `low`
- Confidence: `0.55`
- Action: end sessions with a clear final response or teach the parser the current final-message event shape.

## Adding Or Changing Metrics

1. Define the metric in code and in this document.
2. Identify exact source fields and fallback behavior.
3. Add synthetic fixtures for the expected shape.
4. Add or update tests in `internal/analysis`.
5. Update text and JSON report expectations if output changes.

## Adding Or Changing Suggestions

1. State the evidence threshold.
2. Include severity, confidence, affected set, evidence, and action.
3. Add triggering and non-triggering tests where practical.
4. Prefer specific workflow improvements over vague advice.
5. Suppress or lower confidence when evidence is weak.
