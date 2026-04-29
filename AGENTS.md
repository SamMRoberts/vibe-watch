# Vibe Watch Agent Session Harness

## Harness Purpose

This harness governs agents that build or modify `vibe-watch`: a local Go CLI/TUI for monitoring coding-agent sessions, starting with Codex session history stored as JSONL under `~/.codex/sessions/<year>/<month>/<day>/`, and producing statistics, analytics, and improvement suggestions from that history.

## Supported Work

- Build a Go CLI/TUI that scans local Codex session files without mutating them.
- Add Cobra commands for scriptable workflows such as `scan`, `stats`, `report`, `suggest`, and `tui`.
- Add Bubble Tea, Bubbles, and Lip Gloss screens for browsing sessions, metrics, trends, warnings, and suggestions.
- Implement a Codex session adapter that discovers files under `~/.codex/sessions/YYYY/MM/DD/*.jsonl`.
- Parse JSONL incrementally with tolerant event decoding so unknown or changed event shapes do not break the whole scan.
- Extract stable metadata and aggregate metrics such as session counts, date ranges, repositories, models, tools, command outcomes, approvals, errors, verification evidence, interruptions, and handoff quality when present in the data.
- Generate suggestions from observable patterns, such as missing verification, repeated failing commands, long idle loops, frequent escalations, weak handoffs, or recurring repo-specific friction.
- Add privacy-preserving reports that default to aggregate metrics and avoid storing raw prompts, code, tool outputs, or secrets.
- Add synthetic fixtures and tests for parser behavior, metric aggregation, suggestion rules, and TUI model transitions.
- Update README or docs when command usage, data locations, privacy behavior, or analytics definitions change.

## Out Of Scope

- Do not modify, delete, rewrite, or relocate files under `~/.codex/sessions`.
- Do not ingest non-Codex agents until the Codex adapter boundary is stable.
- Do not upload session data, send analytics over the network, or add telemetry without explicit user approval.
- Do not store raw conversation text, code snippets, command output, secrets, or file contents in generated caches by default.
- Do not treat current Codex JSONL field names as a permanent schema; schema-specific logic must be isolated behind the Codex adapter.
- Do not make unsupported quality judgments about agent behavior unless they are tied to observable evidence and a named heuristic.
- Do not build a GUI, web dashboard, daemon, or background monitor unless the user explicitly expands the scope.
- Do not replace Go, Cobra, Bubble Tea, Bubbles, or Lip Gloss without user approval.
- Do not perform destructive filesystem or Git operations without explicit user approval.

## Required Context

Before planning or editing, inspect the files that exist for the current task:

- `AGENTS.md` for this harness and any nested guidance.
- `README.md`, `docs/`, issue notes, or task-specific specs if present.
- `go.mod`, `go.sum`, `main.go`, `cmd/`, `internal/`, `pkg/`, `testdata/`, and existing tests if present.
- Codex adapter, parser, analytics, suggestion, report, and TUI packages if present.
- CI or workflow files such as `.github/workflows/*` if present.

For session-source context:

- Use `~/.codex/sessions` as the default Codex session root.
- Expect date folders shaped like `~/.codex/sessions/2026/04/29/`.
- Inspect directory and filename shape before reading content.
- When schema discovery is needed, read the smallest practical sample and summarize event keys or types rather than copying private conversation content into chat.
- Prefer synthetic fixtures in `testdata/` for tests. Do not commit real Codex session files.

If the repository is empty, assume the default implementation layout below until the user says otherwise:

```text
main.go
cmd/
  root.go
  scan.go
  stats.go
  report.go
  suggest.go
  tui.go
internal/
  codex/
    sessions/
    events/
  analysis/
    metrics/
    suggestions/
  privacy/
  report/
  tui/
testdata/
  codex/
```

Summarize the constraints that matter before editing: data source, privacy posture, Codex schema assumptions, command surface, output format, tests, and any user changes already present.

## Operating Phases

### Intake

Entry criteria:

- The user request concerns session monitoring, analytics, suggestions, Codex JSONL parsing, CLI commands, or TUI views.

Exit criteria:

- The agent identifies whether the task touches ingestion, parsing, metrics, suggestions, reporting, TUI, docs, or tests.
- The agent states any assumptions about the Codex JSONL schema, privacy behavior, date range, or output format.
- The agent asks questions only when a reasonable default would create data-loss, privacy, or product-direction risk.

### Discovery

Entry criteria:

- The task scope is clear enough to inspect repository and data-source context.

Exit criteria:

- Relevant project files, tests, docs, and overlapping user changes have been read.
- The agent has inspected the session directory shape when the task depends on real Codex history.
- Any live-session sampling is minimal and avoids exposing raw private content in chat.
- Existing package boundaries and command conventions have been preferred over generic defaults.

### Plan

Entry criteria:

- The agent has enough repository and data-source context to choose files, packages, and verification gates.

Exit criteria:

- The plan identifies files to change, parser or adapter assumptions, analytics definitions, suggestion rules, tests, and risk areas.
- For new analytics, the plan states which observed fields drive each metric.
- For new suggestions, the plan states the evidence threshold and why the suggestion is actionable.

### Implementation

Entry criteria:

- The plan is specific enough for scoped edits.

Exit criteria:

- Codex session discovery handles year/month/day folders and `.jsonl` files without mutating the source tree.
- JSONL parsing is streaming or bounded-memory and tolerates malformed lines, unknown event types, and missing optional fields.
- Domain logic for scanning, parsing, metrics, suggestions, and privacy is outside `cmd/` and outside Bubble Tea models.
- Cobra command handlers parse flags, validate inputs, and delegate to testable helpers.
- Bubble Tea models keep state explicit and render aggregate information without reading files from `View`.
- Reports and caches omit raw private content by default and document any opt-in detail modes.
- No unrelated files or user changes were reverted.

### Verification

Entry criteria:

- Implementation is complete.

Required gates:

- `go test ./...`
- `go run . --help`
- `go run . <command> --help` for each new or materially changed command.
- Parser, metric, and suggestion tests using synthetic JSONL fixtures for changed ingestion or analysis logic.

Conditional gates:

- `go mod tidy` after dependency or module changes.
- `go test -race ./...` when goroutines, channels, async Bubble Tea commands, caches, or shared state are changed.
- `go run . scan --session-root ~/.codex/sessions --limit <small-number>` or an equivalent bounded smoke test when live Codex history behavior changes.
- Direct Bubble Tea model tests for new or changed TUI models by calling `Update` with key, window-size, load-success, and load-error messages.
- Manual terminal smoke test for new full-screen TUI flows, including resize, quit keys, filtering, focus movement, loading/error states, and monochrome readability.

Exit criteria:

- Required checks pass, or failures are explained with the command, important output, data/privacy impact, and next action.

### Handoff

Entry criteria:

- Verification evidence has been collected or explicitly skipped with rationale.

Exit criteria:

- The user receives a concise summary, changed files, validation results, residual risk, privacy implications, and useful next steps.

## Boundaries

Edit boundaries:

- Allowed: `AGENTS.md`, `README.md`, `docs/`, `go.mod`, `go.sum`, `main.go`, `cmd/`, `internal/`, `pkg/`, `testdata/`, `.github/workflows/`, and focused test files.
- Disallowed without explicit request: real files under `~/.codex/sessions`, unrelated application domains, generated vendored trees, Git internals, editor settings, secrets, local machine configuration, and broad rewrites outside the task.

Command boundaries:

- Allowed without approval when relevant: `rg`, `find` or `ls` against the session directory, `go test ./...`, `go test -race ./...`, `go run . --help`, bounded `go run . scan --session-root ~/.codex/sessions --limit <n>`, `go mod tidy`, `go fmt ./...`, and `go vet ./...`.
- Requires approval: dependency downloads when network access is needed, destructive commands, commands that write outside the workspace, commands that read large amounts of private session content unnecessarily, and commands that expose or transmit secrets.

Git boundaries:

- Do not commit, push, reset, rebase, delete branches, or discard changes unless the user explicitly asks.
- Preserve unrelated user changes in the worktree.
- If asked to commit, use the repository's commit policy if present; otherwise use a concise Conventional Commit subject.

Dependency and network boundaries:

- Prefer existing dependencies and package versions.
- For new app work, Cobra, Bubble Tea, Bubbles, and Lip Gloss are acceptable dependencies when the user has requested this stack.
- Prefer the Go standard library for JSONL scanning and filesystem traversal unless a dependency removes real complexity.
- Run `go mod tidy` after dependency changes.
- If dependency installation fails because network access is restricted, request approval rather than working around the restriction.

Privacy and data boundaries:

- Treat `~/.codex/sessions` as private local history.
- Never commit real session JSONL files.
- Never print raw prompts, answers, code, command outputs, secrets, or file contents in chat unless the user explicitly asks for a specific excerpt.
- Redact or omit sensitive values in logs, reports, fixtures, snapshots, and failure output.
- Default generated reports to aggregate metrics and session identifiers, not raw content.
- Any cache or export format must document what is stored and provide a way to regenerate from source.

## Product Rules

### Codex Session Ingestion

- Discover sessions from `~/.codex/sessions/<year>/<month>/<day>/*.jsonl`.
- Keep the session root configurable with a flag such as `--session-root`.
- Support date range and limit options before reading file contents.
- Stream JSONL line by line; do not load an unbounded session corpus into memory.
- Preserve enough raw event metadata internally to debug parser gaps, but do not expose raw content by default.
- Record malformed-line counts and unknown-event counts as metrics rather than failing the whole scan.

### Analytics

- Define every metric in code or docs by source event fields and fallback behavior.
- Prefer counts, rates, trends, and distributions that can be explained from local evidence.
- Useful initial metrics include sessions by day, active repositories, models used, tool-call counts, shell command outcomes, approval/escalation counts, test/verification mentions, error frequency, interruption/resume markers, and final-handoff presence.
- Include data quality metrics: files scanned, lines scanned, malformed lines, unknown event types, missing timestamps, and skipped files.
- Keep analytics deterministic so the same input fixture produces the same report.

### Suggestions

- Suggestions must be rule-based until the user asks for model-generated advice.
- Each suggestion must include evidence, severity, confidence, affected sessions or date range, and a concrete action.
- Prefer suggestions that improve agent workflow: add verification, tighten prompts, reduce repeated failing commands, record durable decisions, split oversized tasks, avoid unnecessary escalations, or improve final handoffs.
- Avoid moralizing or vague coaching. If evidence is weak, lower confidence or suppress the suggestion.

### Cobra

- `main.go` owns final startup and process exit.
- `cmd/` wires root and subcommands.
- Prefer `RunE`, `PreRunE`, and `Args` validators so errors are returned and testable.
- Use command-local flags for command-specific behavior and persistent flags only for true global settings such as `--session-root`, `--since`, `--until`, `--format`, and `--limit`.
- Keep primary command output on stdout; diagnostics, skipped-file warnings, progress, and logs go to stderr.
- Keep `--json` or other structured output stable if users may script against it.

### Bubble Tea

- Use the TUI for interactive exploration of already-loaded summaries or bounded async loads.
- Model state should explicitly track size, focus, selected session, active filters, loading state, active errors, child components, and pending operation identity where relevant.
- `Init` returns startup commands.
- `Update` handles messages, delegates to child component `Update` methods, stores returned models, and batches commands where needed.
- `View` renders without I/O, mutation, goroutines, filesystem access, network access, timers, or stdout logging.
- Treat terminal resize as a first-class message and recompute component sizes from the latest dimensions.
- Use `tea.Cmd` for asynchronous work and ignore stale async responses when multiple requests can overlap.
- Bubble Tea owns stdout during a program; debug logs must go to a file.

### Bubbles And Lip Gloss

- Use Bubbles for standard primitives such as lists, tables, viewports, spinners, progress, paginators, help, filters, text inputs, and key bindings.
- Keep key maps centralized and derive help text from bindings where practical.
- Define Lip Gloss styles as pure values and apply them at render boundaries.
- Design layouts from terminal cell widths, using Lip Gloss sizing helpers rather than assuming one rune equals one cell.
- Use color sparingly and ensure the app remains usable in monochrome or reduced-color terminals.

## Verification Gates

Fast checks:

- `go fmt ./...`
- `go test ./...`
- `go run . --help`

Full checks:

- `go vet ./...`
- `go test -race ./...`
- Command-specific help and bounded scan smoke commands.
- Parser, aggregation, suggestion, privacy, and report tests with synthetic fixtures.
- Direct Bubble Tea model transition tests for new or changed TUI models.

Manual checks:

- For live Codex-history features, run a bounded scan against `~/.codex/sessions` and confirm the output is aggregate and privacy-preserving.
- For new full-screen TUI flows, run the app in a real terminal and verify resize, quit keys, filtering, focus movement, loading and error states, help display, and monochrome readability.

Skipped-check policy:

- The agent must state the skipped check, why it was skipped, and the residual risk, including any data-quality or privacy risk.

## Evidence Requirements

The handoff must include:

- Commands run and pass/fail status.
- Important failure lines or artifact paths for failed checks.
- The data source used for smoke tests, described without exposing raw session content.
- Fixture coverage for parser, metric, suggestion, and privacy behavior when those areas change.
- Manual TUI smoke-test notes when a TUI flow changed.
- Any skipped checks and residual risk.

## Handoff Format

Use this shape unless the user asks for another format:

```markdown
Implemented [short summary].

Changed:
- path/to/file

Verification:
- `command`: passed
- `command`: not run because [reason]

Data and privacy:
- [session source, fixture/live scope, redaction or aggregation behavior]

Risk:
- [residual risk or "No known residual risk."]
```

For review-only work, lead with findings ordered by severity and include file and line references.

## Automation Plan

- Keep this harness in `AGENTS.md` so future agents discover it before editing.
- Keep section validation files in `.harness-validation/` when auditing harness completeness.
- Start with guidance and tests, then add scripts after parsing and reporting workflows stabilize.
- Candidate scripts: fixture validator, bounded live-scan smoke test, report golden updater, and privacy-leak checker for generated outputs.
- Consider a companion skill only after the Codex adapter and suggestion taxonomy become reusable across repos.
- Do not add hooks, MCP servers, background monitors, or scheduled scans unless the user asks for persistent monitoring or cross-session automation.

## Open Questions

- What product name, command names, and first-report format should be user-facing once implementation begins?
- Which analytics should be treated as the MVP: activity trends, tool usage, verification quality, error loops, handoff quality, or suggestion generation?
- Should generated reports or caches live only in the workspace, under a user data directory, or be opt-in per command?
