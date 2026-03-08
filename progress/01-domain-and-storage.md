# 01 Domain and Storage

## Objective

Define the structured runtime model and persistence layer before provider or tool breadth.

## Status

done

## Dependencies

- 00 Repo Foundation

## Scope In

- conversation and message model
- session model
- storage abstraction
- persistence backend

## Scope Out

- provider network logic
- developer tools
- CLI UX depth

## Checklist

- [x] Define core message and conversation types
- [x] Define session metadata and replay requirements
- [x] Choose and implement persistence backend
- [x] Support create, load, append, replace, and replay
- [x] Add storage-focused tests

## Acceptance Criteria

- Structured sessions can be created, loaded, and replayed without provider-specific reconstruction.

## Open Questions

- How much of Goose's richer message surface should be modeled before provider/tool milestones require it.

## Notes / Findings

- Storage should preserve tool requests and tool results as first-class conversation data.
- The first backend is SQLite, with the conversation stored as validated JSON in the session row.
- The initial `internal/session.Store` interface is narrow on purpose: create, load, append, replace, and replay.
