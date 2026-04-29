# TUI Session List

The second tab is a polished grouped list of sessions.

## Grouping

Group sessions by date and agent. Dates should be newest first. Within a group, sessions should appear newest first unless the user explicitly selects another sort.

## Metadata

Each session row should show compact metadata:

- Start time or best known timestamp.
- Agent.
- Inferred status.
- Prompt or title preview when available.
- Tool-call count when available.
- Token summary when available.
- Parse or diagnostic indicator when needed.

## Interaction

The list should support keyboard navigation, selection, resize handling, and mouse wheel scrolling. Selecting a session switches to the detailed activity view for that session.

Keep the list responsive while session loading continues. Newly loaded sessions should merge into groups without resetting the user's current selection unless the selected item disappears.
