# TUI Dashboard

The dashboard is the initial tab. It should be useful while data is still loading.

## Viewports

Use multiple Bubble Tea viewports or equivalent Bubbles components for live sections:

- Session counts by total, active, completed, errored, and unknown.
- Session status summaries.
- Session agent summaries.
- Token summaries, including known, missing, and aggregate token values.
- Recent activity or refresh state.

Viewport sizes must respond to terminal resize messages. Avoid hard-coded dimensions that break narrow terminals.

## Real-Time Behavior

The dashboard should update as progressive loading discovers and parses sessions. Show partial data clearly without blocking keyboard or mouse input.

Refresh work must run outside `View`. Use Bubble Tea commands/messages to deliver batches, progress updates, errors, and completion state.

## Interaction

- Provide keyboard navigation between tabs.
- Keep help discoverable through Bubbles key bindings.
- Enable mouse wheel scrolling for any dashboard viewport whose content can overflow.
