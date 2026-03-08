# Progress

## Objective

Build `goose-go` as a Go implementation of Goose terminal core: a local agent runtime with structured sessions, a provider boundary, developer tools, approvals, and an end-to-end CLI loop.

## Current V1 Target

Terminal core only. No server or desktop parity in v1. The first provider slice is Codex-first and reuses an existing `codex login`.

## Milestones

| Milestone | Goal | Status | Dependencies | Acceptance | Last Updated |
| --- | --- | --- | --- | --- | --- |
| 00 | Root setup, docs, and progress structure | done | none | Repo is the system of record and workflow targets are defined | 2026-03-08 |
| 01 | Domain model and storage | done | 00 | Structured sessions can be created, loaded, and replayed | 2026-03-08 |
| 02 | Provider foundation and Codex-first OpenAI provider | in_progress | 01 | Existing `codex login` user can complete streaming chat without an API key | 2026-03-08 |
| 03 | Tool runtime and developer tools | planned | 01, 02 | In-process tools can be listed and executed | 2026-03-08 |
| 04 | Agent loop and approvals | planned | 02, 03 | Multi-turn tool-using loop works with approvals | 2026-03-08 |
| 05 | CLI and session flow | planned | 04 | Terminal session can start, interrupt, resume, and render output | 2026-03-08 |
| 06 | Compaction, evals, and hardening | planned | 04, 05 | Eval harness catches terminal-core regressions | 2026-03-08 |
| 99 | Later parity backlog | planned | none | Deferred work is tracked outside v1 milestones | 2026-03-08 |

## Current Focus

- Continue Milestone 02.
- Make the architecture executable before provider and agent code grow.
- Use `docs/design-principles.md` as the default design checklist for new feature work and architecture changes.
- The first concrete provider is now documented in `internal/provider/openaicodex/ARCHITECTURE.md` so fresh agents can understand the provider shape without reading implementation first.
- The SQLite backend now lives under `internal/storage/sqlite`; the next work is provider and auth foundation on top of that split.
- The provider interface, model config, usage metadata, architecture check, Codex auth/cache reader, and the first `openai-codex` provider now exist.
- The next work is to tighten provider behavior around richer tool-call/event cases and then move up into the tool runtime and agent loop.
- Keep native `goose-go` login out of the first slice.
- Keep the `goose/` submodule as reference-only material.

## Blocked / Risks

- The module path is still local (`goose-go`) and will need a real import path later.
- Upstream Goose has broader product surface area than this repo should target in v1.
- If root docs drift from implementation, agents will start making incorrect assumptions.
- `make eval` is intentionally a stub in Milestone 00 and does not represent a working harness yet.
- The first persistence backend is SQLite with JSON-encoded conversations; if that shape changes later, migration work will be needed.
- The repo now has a first architecture enforcement check, but the rules are still narrow and will need to expand with the runtime.
- The runtime does not yet emit the traces or artifacts an agent will need for later debugging and eval work.
- The first provider slice assumes file-backed Codex credentials in `~/.codex/auth.json`; keyring-backed credentials are deferred.
- Shared Codex auth cache refresh now exists, but it still depends on the current file-backed cache shape and not keyring-backed credentials.
- The current provider implementation is intentionally narrow: SSE only, no websocket transport, and no broader Responses surface yet.
- Generic OpenAI API-key provider support is deferred until after the Codex-first slice is stable.
