# 05 CLI and Session Flow

## Objective

Expose the runtime through a reliable terminal interface.

## Status

planned

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

- [ ] Start a session from the CLI
- [ ] Render streamed agent output
- [ ] Handle interrupts
- [ ] Resume prior sessions
- [ ] Add CLI smoke tests

## Acceptance Criteria

- A user can start, interrupt, resume, and inspect a terminal-core session.

## Open Questions

- None yet.

## Notes / Findings

- CLI should render runtime events, not absorb runtime logic.
