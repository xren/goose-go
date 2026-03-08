# 00 Repo Foundation

## Objective

Turn the repo root into the system of record for humans and agents before runtime implementation begins.

## Status

in_progress

## Dependencies

none

## Scope In

- root README
- root AGENTS guide
- docs folder
- progress rollup
- milestone files
- repo-level workflow targets

## Scope Out

- runtime packages
- provider implementation
- tool implementation
- session storage

## Checklist

- [ ] Replace bootstrap README with project README
- [ ] Add root AGENTS navigation file
- [ ] Add architecture, invariants, Goose reference, and eval docs
- [ ] Add progress rollup
- [ ] Add milestone files under `/progress`
- [ ] Add `check`, `smoke`, and `eval` targets to the root Makefile
- [ ] Verify `make run`, `make test`, `make lint`, and `make check`

## Acceptance Criteria

- A new contributor can understand the project objective, v1 target, and milestone structure from the repo alone.
- The root docs point to valid files.
- The new workflow targets exist and execute with clear behavior.

## Open Questions

- When to switch the module path from local to the real import path.

## Notes / Findings

- `goose/` remains read-only reference material.
