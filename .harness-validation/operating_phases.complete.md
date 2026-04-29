# Operating Phases

Complete.

- Intake: classify the task as CLI-only, TUI-only, or hybrid; ask questions only when defaults would create meaningful risk.
- Discovery: read relevant project files, tests, docs, and overlapping user changes.
- Plan: identify files, packages, dependencies, tests, and risk areas; preserve noninteractive command paths for hybrid apps.
- Implementation: wire Cobra commands, keep domain logic testable, keep Bubble Tea `View` deterministic, route child component updates, and avoid unrelated edits.
- Verification: run required and conditional checks, including help, bad-input paths, tests, race checks when relevant, and manual TUI smoke tests when needed.
- Handoff: report changed files, validation, skipped checks, residual risk, and next steps.
