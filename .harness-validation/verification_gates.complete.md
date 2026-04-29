# Verification Gates

Complete.

Required gates:

- `go test ./...`
- `go run . --help`
- `go run . <command> --help` for new or materially changed commands.
- `go run . <command> <bad-input>` or equivalent command tests for validation paths.

Conditional gates:

- `go mod tidy` after dependency changes.
- `go test -race ./...` when concurrency, async Bubble Tea commands, or shared state change.
- Direct Bubble Tea model tests for new or changed models.
- Manual terminal smoke tests for full-screen TUI flows.

Skipped checks must be reported with reason and residual risk.
