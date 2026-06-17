# vibe-watch

vibe-watch is a Rust CLI and terminal dashboard for inspecting GitHub Copilot session logs. It reads Copilot log files, reconstructs session activity, and reports token usage, AI credit usage, per-turn timing, tool activity, skills, and subagent usage.

It currently supports two log formats:

- VS Code Copilot Chat `.jsonl` session logs
- Copilot CLI `events.jsonl` session logs

## What it shows

- Session totals for output tokens and AI credits
- Repository path for the workspace that produced the session
- Token category totals for input, output, and cached tokens when the input log contains them
- Reported AI credit usage when logs provide it, with pricing-based estimates as a fallback
- Per-turn output token and elapsed-time metrics
- Aggregate tool, skill, and subagent usage
- Request/turn-associated usage breakdowns for tools, skills, and subagents
- Per-model usage summaries for CLI session logs
- An interactive TUI for browsing repositories, sessions, selected-turn activity sequences, and progressive scan progress

## Build

```bash
cargo build
```

Run the test suite:

```bash
cargo test
```

You can also run the binary directly with Cargo:

```bash
cargo run -- --help
```

## Usage

```bash
vibe-watch <COMMAND>
```

Available commands:

- `analyze`: print a text report or JSON summary for a session log
- `tui`: open an interactive terminal dashboard for a session log

### Analyze a log

```bash
cargo run -- analyze /path/to/session.jsonl
```

Output JSON instead of the text table:

```bash
cargo run -- analyze /path/to/session.jsonl --json
```

Force the input format when auto-detection is not enough:

```bash
cargo run -- analyze /path/to/session.jsonl --format vscode
cargo run -- analyze /path/to/events.jsonl --format cli
```

Session analytics are cached in a local SQLite database by default. Disable the
cache for one run, force a refresh, or use an explicit cache database path with:

```bash
cargo run -- analyze /path/to/session.jsonl --no-cache
cargo run -- analyze /path/to/session.jsonl --refresh-cache
cargo run -- analyze /path/to/session.jsonl --cache-db /tmp/vibe-watch.sqlite
```

### Open the terminal dashboard

```bash
cargo run -- tui
cargo run -- tui /path/to/session.jsonl
cargo run -- tui /path/to/session-root
```

With no path, the TUI scans the default local Copilot session roots when they exist:

- VS Code Copilot Chat sessions under `~/Library/Application Support/Code/User/workspaceStorage`
- Copilot CLI sessions under `~/.copilot/session-state`

Passing a file opens the browser around that session. Passing a directory recursively scans supported session logs under that directory. The `--format auto|vscode|cli` flag still controls format detection; forcing `vscode` or `cli` limits discovery to that log family.

The TUI supports:

- `Up` and `Down`, or `j` and `k`, to move through repositories, sessions, or turns
- `Home` and `End` to jump to the first or last repository, session, or turn in the current view
- `Enter` or `Right` to drill from repositories to sessions, then into a session detail view
- `Left` or `Backspace` to go back
- `PageUp` and `PageDown`, or `[` and `]`, to scroll the Activity pane in a session detail view
- `q` to quit, and `Esc` to go back or quit from the top level

The initial dashboard lists repositories as they are loaded. Each repository row shows its loaded session count, total turns, and available AI Credit summary. The selected repository or session row is marked with `>` in addition to color highlighting, so selection remains visible in low-color terminals. Selecting a repository opens the session list for that repository; each loaded session row shows its title or session id, turn count, AI Credit value, and path. Sessions that cannot be read or parsed appear as error rows with their path and error message, so one bad log does not stop the rest of the scan.

The TUI loads sessions progressively. Candidates are discovered and processed newest-first by modified time, and the Progress bar shows whether it is loading, loaded, or finished with no sessions found, plus the number of visible error rows. The dashboard updates as each session finishes loading. While motion is enabled, repository/session browser changes use short TachyonFX fades, and scan errors get a brief muted pulse. Set `VIBE_WATCH_NO_MOTION=1` or `NO_COLOR=1` to disable those effects.

The dashboard header shows token usage by category: input, output, and cached. When a log format does not expose a category, the value is shown as `n/a` rather than estimated from unrelated data.

The dashboard Activity pane follows the currently selected turn. It uses a Ratatui table to show the turn's recorded tools, terminal commands, skills, and subagents in source-log order, with rows color-coded by activity kind. Consecutive activities with the same kind and name are grouped into one row with a count, while non-consecutive repeats stay separate so the action sequence remains clear. The pane focuses on the action sequence and does not show AI Credit values; credit totals remain in the dashboard header and reports.

The Turns table and Timeline now share the same per-turn color palette. Each turn keeps the same color in both places so it is easier to match the selected row with its timeline segment at a glance. Long turn lists keep the selected turn visible and show a scrollbar when rows are clipped. The selected timeline slice also uses explicit marker glyphs, so the active turn stands out even when adjacent slices have similar widths.

## Token and credit mapping

vibe-watch normalizes known token fields into three display categories:

- Input: prompt/input token fields, such as VS Code `result.metadata.promptTokens` and CLI `usage.inputTokens`
- Output: completion/output token fields, such as VS Code `completionTokens` and CLI `usage.outputTokens`
- Cached: cache read and cache write token fields, such as CLI `usage.cacheReadTokens` plus `usage.cacheWriteTokens`

Reasoning tokens, when present as CLI `usage.reasoningTokens`, are preserved as a separate detail and are not added to output tokens unless a future log format proves output tokens exclude them.

Credit values use this precedence:

1. Reported credits/costs from the log, such as VS Code `result.details` credit text or CLI `requests.cost`
2. Estimated credits from token usage and known model pricing when no reported value is available
3. Output-only credits for formats that expose only output usage

## Activity usage breakdowns

The text and JSON reports include two levels of activity reporting:

- Count-only aggregates: how many times each tool, skill, or subagent appeared
- Associated usage breakdowns: the tokens and credits recorded on the enclosing request or turn for each activity name

Associated usage is not exact per-call metering. Current VS Code and Copilot CLI logs attach token and credit usage to requests, turns, or session/model summaries rather than to each individual tool, skill, or subagent call. vibe-watch therefore counts every call, but adds an enclosing request or turn's usage only once per activity name in that request or turn. This prevents repeated calls to the same activity from multiplying the entire request cost.

For VS Code chat logs, activity usage can include request input tokens, output tokens, output-only credits, and reported request credits when those fields are present. For Copilot CLI logs, activity usage currently includes turn output tokens and output-only credits; session shutdown input, cache, and reasoning totals remain session/model-level data and are not attributed to individual activities.

## Input formats

### VS Code Copilot Chat logs

These are the `.jsonl` chat session logs written by VS Code. On macOS they are typically under a path like:

```text
~/Library/Application Support/Code/User/workspaceStorage/.../chatSessions/<session-id>.jsonl
```

For this format, vibe-watch reconstructs the session from the delta journal and reports output token usage from `completionTokens`. If a request includes `result.metadata.promptTokens`, vibe-watch counts those as input tokens. When a request includes reported credit text in `result.details` such as `GPT-5.4 • 148.8 credits`, vibe-watch uses that reported value for credits. If no reported credit details are available, it falls back to output-token-based credits and marks them as output-only. Cache token counts are not available in this session log format; input tokens are shown as `n/a` when prompt token metadata is absent.

When `workspace.json` is present next to the containing workspace storage directory, vibe-watch also reports the repository path from its `folder` field.

### Copilot CLI logs

These are Copilot CLI `events.jsonl` logs, typically found under:

```text
~/.copilot/session-state/<session-id>/events.jsonl
```

When the session shutdown metrics are present, vibe-watch reports full input, output, cached, cache-read, cache-write, reasoning, and credit usage, plus a per-model breakdown. If `requests.cost` is present, that reported value is used as the credit total; otherwise vibe-watch estimates credits from the recorded token usage and built-in model pricing.

When `workspace.yaml` is present in the session directory, vibe-watch reports the repository path from `git_root`, falling back to `cwd`.

## Example workflow

Inspect a VS Code session log:

```bash
cargo run -- analyze "$HOME/Library/Application Support/Code/User/workspaceStorage/.../chatSessions/session.jsonl"
```

Inspect a Copilot CLI session with full metrics:

```bash
cargo run -- analyze "$HOME/.copilot/session-state/<session-id>/events.jsonl" --format cli
```

Open the interactive dashboard for the same file:

```bash
cargo run -- tui "$HOME/.copilot/session-state/<session-id>/events.jsonl" --format cli
```

## Local cache

vibe-watch stores derived session analytics in a local SQLite database so
unchanged logs do not need to be reparsed on each launch. The cache is enabled
by default for both `analyze` and `tui`; pass `--no-cache` to bypass it, or
`--refresh-cache` to reparse matching logs and update their cached rows. Use
`--cache-db <path>` to point a command at a specific database, which is useful
for tests or temporary experiments.

By default, the cache lives under the operating system cache directory. On
macOS this is:

```text
~/Library/Caches/vibe-watch/sessions.sqlite
```

The cache stores derived analytics and validity metadata: source log paths,
repository paths, session/model summaries, token and credit totals, the same
activity details already shown by reports, and metadata for workspace sidecar
files. It does not store raw JSONL log records. Cached rows are invalidated when
the source log changes, workspace sidecars such as `workspace.json`,
`.code-workspace`, or `workspace.yaml` change, the cache schema changes, or the
embedded pricing/log-field configuration in the binary changes.