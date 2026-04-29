# Vibe Watch Documentation Map

These files are the authoritative harness for building and maintaining `vibe-watch`. After this creation pass, agents must not edit files in `docs/` unless the user explicitly asks to revise the harness or documentation.

## Product And Scope

- [product-spec.md](product-spec.md): core product goals, in-scope work, and excluded work.
- [codex-sessions.md](codex-sessions.md): Codex session storage, discovery order, parsing rules, and fixture expectations.

## TUI Areas

- [tui-dashboard.md](tui-dashboard.md): first tab, viewport layout, live counters, status, agent, and token data.
- [tui-session-list.md](tui-session-list.md): second tab, grouped session browser, metadata, and selection behavior.
- [tui-session-detail.md](tui-session-detail.md): selected-session activity view and scrollable event rendering.
- [themes.md](themes.md): theme model, styling expectations, and reduced-color behavior.

## Engineering Harness

- [architecture.md](architecture.md): Go package boundaries and framework defaults.
- [operating-phases.md](operating-phases.md): intake, discovery, planning, implementation, verification, and handoff gates.
- [boundaries.md](boundaries.md): editable areas, forbidden operations, network rules, and stop conditions.
- [verification.md](verification.md): required tests and evidence.
- [handoff-feedback.md](handoff-feedback.md): final response format and feedback loops.
