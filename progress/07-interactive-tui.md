# 07 Interactive TUI

## Objective

Build a minimal interactive TUI on top of the event-driven runtime, borrowing the architecture pattern from `pi-mono` without porting its TypeScript implementation directly.

## Status

in_progress

## Dependencies

- 05 CLI and Session Flow
- 06 Agent Event Stream Evals and Hardening

## Scope In

- Stage 1 MVP over the event-driven runtime
- Stage 2 richer coding UX after the MVP is stable

## Scope Out

- extension system
- advanced overlays and theming
- session tree or branch UI
- server and desktop surfaces

## Checklist

- [x] Choose the Go TUI stack and document the rationale
- [x] Finalize the TUI architecture plan
- [x] Complete Stage 1 MVP plan
- [x] Complete Stage 2 richer UX plan
- [x] Implement Stage 1 MVP
- [ ] Implement Stage 2 richer UX

Current note:

- the Stage 1 Bubble Tea MVP is now in the repo under `internal/tui` and `cmd/goose-go tui`
- Stage 2 richer UX remains the next TUI milestone
- the detailed Stage 2 execution plan now lives in [07b-tui-stage2-ux.md](/Users/rex/projects/goose-go/progress/07b-tui-stage2-ux.md)
- the first Stage 2 step, the approval continuation seam in `internal/agent` and `internal/app`, is now implemented
- the Stage 2 approval UI is now implemented in `internal/tui`
- the local model-registry and runtime-selection work tracked in [07d-model-registry-and-selection.md](/Users/rex/projects/goose-go/progress/07d-model-registry-and-selection.md) is now complete
- session persistence of provider/model is now implemented
- the TUI `/model` picker is now implemented on top of the registry-backed selection path
- the TUI recent-session picker is now implemented on top of `ListSessions(...)` through `/sessions` and `Ctrl-R`
- tool lifecycle is now rendered as grouped transcript blocks in the TUI instead of flat request/result lines
- grouped tool blocks are now width-capped and wrapped inside the viewport so long outputs do not mangle the transcript layout
- the TUI now defaults to compact rendering, with `--debug` and `/debug` available when full tool args/output and verbose UI detail are needed
- the first broader local TUI command surface is now in place through `/help`, `/session`, and `/new`
- built-in dark/light theme selection is now available through `--theme` and the TUI-local `/theme` picker
- the transcript viewport now supports mouse-wheel scrolling plus explicit history navigation through `PageUp` / `PageDown` and `Home` / `End`, and it no longer auto-snaps to bottom while the user is reading older output
- `goose-go run /model` remains a local reporter, while `goose-go tui /model` now opens the picker
- the next Stage 2 work now includes a dedicated styling/layout pass and a token-based theme plan inspired by `pi-mono`

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
- Supporting plans now live in:
  - the detailed plans are now populated and should be treated as the system of record for implementation
  - [07a-tui-stage1-mvp.md](/Users/rex/projects/goose-go/progress/07a-tui-stage1-mvp.md)
  - [07b-tui-stage2-ux.md](/Users/rex/projects/goose-go/progress/07b-tui-stage2-ux.md)
  - [07c-tui-architecture.md](/Users/rex/projects/goose-go/progress/07c-tui-architecture.md)
  - [07d-model-registry-and-selection.md](/Users/rex/projects/goose-go/progress/07d-model-registry-and-selection.md)
  - [07e-tui-styling-and-layout.md](/Users/rex/projects/goose-go/progress/07e-tui-styling-and-layout.md)
  - [07f-tui-theme-system.md](/Users/rex/projects/goose-go/progress/07f-tui-theme-system.md)
