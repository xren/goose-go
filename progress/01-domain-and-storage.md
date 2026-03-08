# 01 Domain and Storage

## Objective

Define the structured runtime model and persistence layer before provider or tool breadth.

## Status

planned

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

- [ ] Define core message and conversation types
- [ ] Define session metadata and replay requirements
- [ ] Choose and implement persistence backend
- [ ] Support create, load, append, replace, and replay
- [ ] Add storage-focused tests

## Acceptance Criteria

- Structured sessions can be created, loaded, and replayed without provider-specific reconstruction.

## Open Questions

- None yet.

## Notes / Findings

- Storage should preserve tool requests and tool results as first-class conversation data.
