# 00 Repo Foundation

## Objective

Turn the repo root into the system of record for humans and agents before runtime implementation begins.

## Status

done

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

- [x] Replace bootstrap README with project README
- [x] Add root AGENTS navigation file
- [x] Add architecture, invariants, Goose reference, and eval docs
- [x] Add progress rollup
- [x] Add milestone files under `/progress`
- [x] Add `check`, `smoke`, and `eval` targets to the root Makefile
- [x] Verify `make run`, `make test`, `make lint`, and `make check`
- [x] Verify `make smoke`
- [x] Verify `make eval` fails with the intended placeholder message

## Acceptance Criteria

- A new contributor can understand the project objective, v1 target, and milestone structure from the repo alone.
- The root docs point to valid files.
- The new workflow targets exist and execute with clear behavior.

## Open Questions

- When to switch the module path from local to the real import path.

## Notes / Findings

- `goose/` remains read-only reference material.
- `make eval` exists as a stable entrypoint but intentionally exits with a placeholder failure until the harness is implemented.
- `docs/design-principles.md` extends the root system-of-record docs so fresh agents can apply the project’s agent-first design rules without prior chat context.
