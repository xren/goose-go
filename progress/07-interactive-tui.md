# 07 Interactive TUI

## Objective

Build a minimal interactive TUI on top of the event-driven runtime, borrowing the architecture pattern from `pi-mono` without porting its TypeScript implementation directly.

## Status

planned

## Dependencies

- 05 CLI and Session Flow
- 06 Agent Event Stream Evals and Hardening

## Scope In

- TUI view model that consumes agent events
- live transcript rendering
- input editor / composer
- status and footer rendering
- approval prompts
- session picker or resume entrypoint
- tool execution rendering

## Scope Out

- extension system
- advanced overlays and theming
- session tree or branch UI
- server and desktop surfaces

## Checklist

- [x] Choose the Go TUI stack and document the rationale
- [ ] Build an adapter from agent events to UI state
- [ ] Render live assistant text and tool lifecycle events
- [ ] Add an input box and submit flow
- [ ] Surface approval-required state as first-class UI
- [ ] Add session picker or resume UI
- [ ] Add TUI smoke tests

## Acceptance Criteria

- A user can drive an interactive terminal session through the TUI without the TUI owning runtime logic.
- The TUI consumes live agent events instead of polling SQLite or waiting for a final transcript dump.

## Open Questions

- How much transcript history should remain live in memory versus being paged from session state.

## Notes / Findings

- Copy the `pi-mono` layering, not the implementation language or component code.
- The agent core must stay headless and event-driven.
- The TUI should subscribe to runtime events and maintain its own view state.
- Do not make the TUI read SQLite directly as its primary live data source.
- Stage 1 should use Bubble Tea as the Go TUI stack.
- Stage 1 should remain a minimal event-driven MVP first; richer `pi-mono`-style UX belongs in a later stage.
