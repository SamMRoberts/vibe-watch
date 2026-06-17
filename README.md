# vibe-watch

vibe-watch is a Rust CLI and terminal dashboard for inspecting GitHub Copilot session logs. It reads Copilot log files, reconstructs session activity, and reports token usage, AI credit usage, per-turn timing, tool activity, skills, and subagent usage.

It currently supports two log formats:

- VS Code Copilot Chat `.jsonl` session logs
- Copilot CLI `events.jsonl` session logs

## What it shows

- Session totals for output tokens and AI credits
- Full token and credit breakdowns when the input log contains them
- Per-turn output token and elapsed-time metrics
- Aggregate tool, skill, and subagent usage
- Per-model usage summaries for CLI session logs
- An interactive TUI for exploring turns and session activity

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
- `q` to quit

## Input formats

### VS Code Copilot Chat logs

These are the `.jsonl` chat session logs written by VS Code. On macOS they are typically under a path like:

```text
~/Library/Application Support/Code/User/workspaceStorage/.../chatSessions/<session-id>.jsonl
```

For this format, vibe-watch reconstructs the session from the delta journal and reports output-token-based credits. Input and cache token data are not available in the log, so credits are marked as partial.

### Copilot CLI logs

These are Copilot CLI `events.jsonl` logs, typically found under:

```text
~/.copilot/session-state/<session-id>/events.jsonl
```

When the session shutdown metrics are present, vibe-watch can report full input, output, cache, and total credit usage, plus a per-model breakdown.

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