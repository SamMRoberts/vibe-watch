# Product Spec

`vibe-watch` is a local Go CLI/TUI for monitoring coding-agent sessions from local storage. It should give an operator a live, low-latency view of recent agent activity without altering any source session data.

## Supported Agents

The first supported agent is Codex. Codex session files are JSONL files stored under:

```text
~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl
```

The architecture may allow future agent adapters, but current implementation work must keep Codex behavior correct before generalizing.

## Core Experience

- Start with a TUI dashboard that updates from local session data.
- Load sessions progressively from newest to oldest so initial refreshes do not block the interface.
- Show real-time counts, inferred session status, agent identity, and token data.
- Provide a second tab with a polished grouped session list.
- Open a selected session into a detailed scrollable activity view.
- Support mouse wheel scrolling in all scrollable views.
- Provide polished user-selectable themes.

## Out Of Scope

- Cloud sync.
- Web UI.
- Editing, rewriting, deleting, moving, or normalizing source session data.
- Writing back to Codex session JSONL files.
- Modifying any files under `~/.codex/sessions/`.
- Treating non-Codex agents as supported before Codex parsing and display are reliable.
