# Progress

## Objective

Build `goose-go` as a Go implementation of Goose terminal core: a local agent runtime with structured sessions, a provider boundary, developer tools, approvals, and an end-to-end CLI loop.

## Current V1 Target

Terminal core only. No server or desktop parity in v1.

## Milestones

| Milestone | Goal | Status | Dependencies | Acceptance | Last Updated |
| --- | --- | --- | --- | --- | --- |
| 00 | Root setup, docs, and progress structure | in_progress | none | Repo is the system of record and workflow targets are defined | 2026-03-08 |
| 01 | Domain model and storage | planned | 00 | Structured sessions can be created, loaded, and replayed | 2026-03-08 |
| 02 | OpenAI-compatible provider | planned | 01 | One provider supports streaming chat without tools | 2026-03-08 |
| 03 | Tool runtime and developer tools | planned | 01, 02 | In-process tools can be listed and executed | 2026-03-08 |
| 04 | Agent loop and approvals | planned | 02, 03 | Multi-turn tool-using loop works with approvals | 2026-03-08 |
| 05 | CLI and session flow | planned | 04 | Terminal session can start, interrupt, resume, and render output | 2026-03-08 |
| 06 | Compaction, evals, and hardening | planned | 04, 05 | Eval harness catches terminal-core regressions | 2026-03-08 |
| 99 | Later parity backlog | planned | none | Deferred work is tracked outside v1 milestones | 2026-03-08 |

## Current Focus

- Finish Milestone 00.
- Establish root docs before creating runtime packages.
- Keep the `goose/` submodule as reference-only material.

## Blocked / Risks

- The module path is still local (`goose-go`) and will need a real import path later.
- Upstream Goose has broader product surface area than this repo should target in v1.
- If root docs drift from implementation, agents will start making incorrect assumptions.
