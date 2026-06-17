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
- An interactive TUI for exploring turns and selected-turn activity details

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

### Open the terminal dashboard

```bash
cargo run -- tui /path/to/session.jsonl
```

The TUI supports:

- `Up` and `Down` to move between turns
- `PageUp` and `PageDown`, or `[` and `]`, to scroll the Activity pane
- `q` to quit

The dashboard header shows token usage by category: input, output, and cached. When a log format does not expose a category, the value is shown as `n/a` rather than estimated from unrelated data.

The dashboard Activity pane follows the currently selected turn. It uses a Ratatui table to show turn metadata, token and credit details, tools, terminal commands, skills, and subagents for the highlighted row in the Turns pane.

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