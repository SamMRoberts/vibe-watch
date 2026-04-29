# Boundaries

Complete.

Allowed edits include `AGENTS.md`, `README.md`, `docs/`, Go module files, `main.go`, `cmd/`, `internal/`, `pkg/`, `testdata/`, workflows, and focused tests.

Disallowed without explicit request: unrelated domains, generated vendored trees, Git internals, editor settings, secrets, local machine configuration, and broad rewrites.

Allowed commands include repository inspection, Go formatting, tests, vetting, module tidy, and representative `go run` smoke checks. Approval is required for dependency downloads when network access is needed, destructive commands, writes outside the workspace, and any command that exposes or transmits secrets.

Git operations that mutate history, branches, remotes, staging, or commits require explicit user request.
