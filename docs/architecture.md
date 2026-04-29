# Architecture

Use a hybrid CLI/TUI structure:

- Cobra owns command entrypoints, help, flags, completion, and scriptable commands.
- Bubble Tea owns interactive TUI state, message routing, update loops, and view rendering.
- Bubbles provides standard UI components such as lists, viewports, tables, spinners, timers, key bindings, and help.
- Lip Gloss owns styling, layout, and theme rendering.

## Package Boundaries

Keep business logic outside `cmd/` and outside Bubble Tea models.

Recommended package responsibilities:

- `cmd/`: Cobra command wiring, flags, validation, and program launch.
- `internal/sessions/`: session discovery, ordering, indexing, status inference, and shared metadata.
- `internal/codex/`: Codex-specific JSONL parsing and event normalization.
- `internal/tokens/`: token extraction and aggregation.
- `internal/watch/`: filesystem polling or watching and progressive refresh orchestration.
- `internal/tui/`: Bubble Tea root model, tabs, keymaps, child models, messages, and layout.
- `internal/theme/`: theme definitions and style composition.
- `testdata/codex/`: representative Codex JSONL fixtures.

## Progressive Loading

Session loading must prefer newest sessions first. Expensive filesystem scans, parsing, and aggregation should run through Bubble Tea commands or other asynchronous workers so the TUI can continue rendering.

The UI should expose partial results while refresh continues. Avoid designs that require parsing all historical sessions before the dashboard becomes usable.

## Data Safety

All session readers must be read-only. Treat session files as external source data owned by Codex, not by this application.
