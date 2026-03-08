# 05 CLI and Session Flow

## Objective

Expose the runtime through a reliable terminal interface and finish the basic session ergonomics needed before a real TUI exists.

## Status

in_progress

## Dependencies

- 04 Agent Loop and Approvals

## Scope In

- CLI session start
- session listing and resume
- interrupt handling
- terminal rendering
- stable command shape for future TUI work

## Scope Out

- full interactive TUI
- server and desktop surfaces
- nonessential terminal polish

## Checklist

- [x] Start a session from the CLI
- [x] Render agent transcript output
- [x] Add a `sessions` command
- [x] Add `run --session <id>` resume flow
- [ ] Handle interrupts and cancellation cleanly
- [x] Add CLI smoke tests

## Acceptance Criteria

- A user can start, inspect, and resume a terminal-core session from the CLI; interrupt handling remains to be added.

## Open Questions

- None yet.

## Notes / Findings

- CLI should render runtime events, not absorb runtime logic.
- `goose-go run` now creates a session, runs one prompt through the agent runtime, and prints the resulting transcript.
- Session persistence already exists in SQLite; this milestone is about making that state usable from the CLI.
- The session boundary is now documented in package-local form at `internal/session/ARCHITECTURE.md` so fresh agents can understand the store interface and SQLite relationship before editing session code.
- The provider replay path now preserves both function-call item IDs and call IDs so multi-turn CLI runs can survive tool use with the Codex backend.
- The current `run` command is still transcript-after-completion, not a live event-driven UI.
- `goose-go sessions` now lists stored sessions, and `goose-go run --session <id>` resumes an existing session while printing only the new transcript segment.
- The remaining work in this milestone is clean interrupt handling.
- The next milestone must add a live agent event stream before substantial TUI work begins.
