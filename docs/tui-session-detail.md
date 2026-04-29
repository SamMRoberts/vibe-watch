# TUI Session Detail

The detail view shows activity for the selected session in a scrollable viewport.

## Activity Types

Render these normalized activity types:

- User prompt.
- Assistant message.
- Tool call.
- Tool result or output when available.
- Parser diagnostic.
- Session metadata and token summary.

Use clear visual hierarchy so long assistant messages and tool payloads remain readable in a terminal.

## Scrolling

The activity list must be scrollable with keyboard controls and mouse wheel events. Preserve scroll position across refreshes when the selected session remains the same.

## Loading And Errors

If a session is still loading, show partial activity and loading state. If a line cannot be parsed, show a diagnostic entry without dropping valid surrounding events.
