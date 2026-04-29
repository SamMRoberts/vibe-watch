# Required Context

Complete.

Before planning or editing, the agent must inspect the task-relevant subset of `AGENTS.md`, `README.md`, docs, `go.mod`, `go.sum`, `main.go`, `cmd/`, `internal/`, `pkg/`, existing tests, TUI model files, and CI workflows if present.

If the repository is empty, use this default shape until directed otherwise:

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

The agent must summarize the implementation constraints that matter before editing.
