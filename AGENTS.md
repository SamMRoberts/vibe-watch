# Go CLI/TUI Agent Harness

## Harness Purpose

This harness governs agents that create or modify a Go command-line or terminal UI application in this repository, using Cobra for scriptable command entrypoints and Bubble Tea, Bubbles, and Lip Gloss for interactive terminal surfaces.

## Supported Work

- Scaffold a new Go module for a CLI-only, TUI-only, or hybrid CLI/TUI app.
- Add Cobra command trees, flags, argument validation, shell completion, and help output.
- Add Bubble Tea models, Bubbles components, Lip Gloss styling, key bindings, and terminal layouts.
- Implement domain behavior in testable packages under `internal/` or established project packages.
- Add focused tests for command behavior, domain logic, Bubble Tea model transitions, and layout helpers.
- Improve usability through clear help text, useful errors, accessible key help, resize handling, and monochrome-safe output.
- Update README or developer docs when commands, app setup, or verification steps change.

## Out Of Scope

- Do not build a GUI, web frontend, daemon, or service unless the user explicitly expands the task.
- Do not replace Cobra, Bubble Tea, Bubbles, or Lip Gloss with another framework without user approval.
- Do not put business logic in anonymous Cobra `RunE` closures or Bubble Tea `View` methods.
- Do not perform destructive filesystem, Git history, or branch operations without explicit user approval.
- Do not add network-backed features, telemetry, or secret handling unless the user asks for them and verification boundaries are clear.
- Do not introduce broad architecture rewrites when a small command, model, or package change is sufficient.

## Required Context

Before planning or editing, inspect the files that exist for the current task:

- `AGENTS.md` for this harness and any nested guidance.
- `README.md`, `docs/`, issue notes, or task-specific specs if present.
- `go.mod`, `go.sum`, `main.go`, `cmd/`, `internal/`, `pkg/`, and existing tests if present.
- Cobra command files for command naming, flag style, output conventions, and dependency wiring.
- Bubble Tea model files for message types, key maps, child component routing, and layout patterns.
- CI or workflow files such as `.github/workflows/*` if present.

If the repository is empty, assume the default layout below until the user says otherwise:

```text
main.go
cmd/
  root.go
  <command>.go
internal/
  app/
  config/
  tui/
```

Summarize the implementation constraints that matter before making edits: app surface, package layout, dependency choices, output contract, tests, and any user changes already present.

## Operating Phases

### Intake

Entry criteria:

- The user request describes a CLI, TUI, or hybrid terminal app task.
- The agent can identify likely touched areas or state what is missing.

Exit criteria:

- The agent classifies the surface as CLI-only, TUI-only, or hybrid.
- The agent states assumptions only when they affect behavior or verification.
- The agent asks questions only when a reasonable default would create meaningful risk.

### Discovery

Entry criteria:

- The task scope is clear enough to inspect repository context.

Exit criteria:

- Relevant project files, tests, and documentation have been read.
- Existing user changes have been identified when they overlap the task.
- Framework conventions in the repo have been preferred over generic defaults.

### Plan

Entry criteria:

- The agent has enough context to choose files, packages, and verification gates.

Exit criteria:

- The plan identifies files to change, domain packages to add or reuse, dependencies, tests, and risk areas.
- For hybrid apps, the plan preserves noninteractive commands for core operations and uses explicit commands to launch the TUI.

### Implementation

Entry criteria:

- The plan is specific enough for scoped edits.

Exit criteria:

- Cobra command handlers parse and validate input, then delegate real work to testable helpers.
- Bubble Tea models keep state explicit, use `tea.Cmd` for async work, and keep `View` deterministic.
- Bubbles components are used for standard widgets instead of custom one-off widgets.
- Lip Gloss styles are pure values applied at render boundaries.
- No unrelated files or user changes were reverted.

### Verification

Entry criteria:

- Implementation is complete.

Required gates:

- `go test ./...`
- `go run . --help`
- `go run . <command> --help` for each new or materially changed command.
- `go run . <command> <bad-input>` or an equivalent command test for each new validation path.

Conditional gates:

- `go mod tidy` after dependency or module changes.
- `go test -race ./...` when goroutines, channels, async Bubble Tea commands, or shared state are changed.
- Direct Bubble Tea model tests for new or changed models by calling `Update` with key, window-size, success, and error messages.
- Manual terminal smoke test for new full-screen TUI flows, including resize, quit keys, focus movement, loading/error states, and monochrome readability.

Exit criteria:

- Required checks pass, or failures are explained with the command, important output, and next action.

### Handoff

Entry criteria:

- Verification evidence has been collected or explicitly skipped with rationale.

Exit criteria:

- The user receives a concise summary, changed files, verification results, residual risk, and useful next steps.

## Boundaries

Edit boundaries:

- Allowed: `AGENTS.md`, `README.md`, `docs/`, `go.mod`, `go.sum`, `main.go`, `cmd/`, `internal/`, `pkg/`, `testdata/`, `.github/workflows/`, and focused test files.
- Disallowed without explicit request: unrelated application domains, generated vendored trees, Git internals, editor settings, secrets, local machine configuration, and broad rewrites outside the task.

Command boundaries:

- Allowed without approval when relevant: `rg`, `go test ./...`, `go test -race ./...`, `go run . --help`, `go run . <command> --help`, `go run . <command> <bad-input>`, `go mod tidy`, `go fmt ./...`, and `go vet ./...`.
- Requires approval: dependency downloads when network access is needed, destructive commands, commands that write outside the workspace, and commands that expose or transmit secrets.

Git boundaries:

- Do not commit, push, reset, rebase, delete branches, or discard changes unless the user explicitly asks.
- Preserve unrelated user changes in the worktree.
- If asked to commit, use the repository's commit policy if present; otherwise use a concise Conventional Commit subject.

Dependency and network boundaries:

- Prefer existing dependencies and package versions.
- For new apps, Cobra, Bubble Tea, Bubbles, and Lip Gloss are acceptable dependencies when the user has requested this stack.
- Run `go mod tidy` after dependency changes.
- If dependency installation fails because network access is restricted, request approval rather than working around the restriction.

Secrets boundaries:

- Never print, commit, or persist secrets.
- Keep config precedence explicit when config is added: flags, environment, config file, defaults.
- Do not add telemetry, analytics, or network calls without explicit scope.

## Framework Rules

### Cobra

- `main.go` owns final startup and process exit.
- `cmd/` wires root and subcommands.
- Prefer `RunE`, `PreRunE`, and `Args` validators so errors are returned and testable.
- Use command-local flags for command-specific behavior and persistent flags only for true global settings.
- Keep primary command output on stdout; diagnostics, prompts, progress, and logs go to stderr.
- Keep `--json` or other structured output stable if it may be scripted.

### Bubble Tea

- Model state should explicitly track size, focus, selection, loading state, active errors, child components, and pending operation identity where relevant.
- `Init` returns startup commands.
- `Update` handles messages, delegates to child component `Update` methods, stores returned models, and batches commands where needed.
- `View` renders without I/O, mutation, goroutines, filesystem access, network access, timers, or stdout logging.
- Treat terminal resize as a first-class message and recompute component sizes from the latest dimensions.
- Use `tea.Cmd` for asynchronous work and ignore stale async responses when multiple requests can overlap.
- Bubble Tea owns stdout during a program; debug logs must go to a file.

### Bubbles And Lip Gloss

- Use Bubbles for standard primitives such as text inputs, text areas, lists, tables, viewports, spinners, progress, paginators, file pickers, help, and key bindings.
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
- Command-specific help and bad-input smoke commands.
- Direct Bubble Tea model transition tests for new or changed TUI models.

Manual checks:

- For new full-screen TUI flows, run the app in a real terminal.
- Verify resize, quit keys, focus movement, loading and error states, help display, and monochrome readability.

Skipped-check policy:

- The agent must state the skipped check, why it was skipped, and the residual risk.

## Evidence Requirements

The handoff must include:

- Commands run and pass/fail status.
- Important failure lines or artifact paths for failed checks.
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

Risk:
- [residual risk or "No known residual risk."]
```

For review-only work, lead with findings ordered by severity and include file and line references.

## Automation Plan

- Keep this harness in `AGENTS.md` so future agents discover it before editing.
- Keep section validation files in `.harness-validation/` when auditing harness completeness.
- Add scripts only after repeated manual checks become stable enough to automate.
- Consider a companion skill only if this repo develops reusable CLI/TUI conventions beyond this project.
- Do not add hooks or MCP servers unless the user asks for persistent memory, enforcement, or cross-repo reuse.

## Open Questions

- None for the initial empty-repository harness. Future product requirements should define the command set, TUI screens, persistence model, and release targets.
