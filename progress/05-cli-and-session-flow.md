# 05 CLI and Session Flow

## Objective

Expose the runtime through a reliable terminal interface.

## Status

in_progress

## Dependencies

- 04 Agent Loop and Approvals

## Scope In

- CLI session start
- interrupt handling
- resume flow
- terminal rendering
- basic command shape

## Scope Out

- server and desktop surfaces
- nonessential terminal polish

## Checklist

- [x] Start a session from the CLI
- [x] Render agent transcript output
- [ ] Handle interrupts
- [ ] Resume prior sessions
- [x] Add CLI smoke tests

## Acceptance Criteria

- A user can start and inspect a terminal-core session from the CLI; interrupt and resume remain to be added.

## Open Questions

- None yet.

## Notes / Findings

- CLI should render runtime events, not absorb runtime logic.
- `goose-go run` now creates a session, runs one prompt through the agent runtime, and prints the resulting transcript.
- The remaining work in this milestone is interrupt handling and resume flow.
- The provider replay path now preserves both function-call item IDs and call IDs so multi-turn CLI runs can survive tool use with the Codex backend.
