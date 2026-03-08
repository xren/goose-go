# Progress

## Objective

Build `goose-go` as a Go implementation of Goose terminal core: a local agent runtime with structured sessions, a provider boundary, developer tools, approvals, and an end-to-end CLI loop that can later support a proper interactive TUI.

## Current V1 Target

Terminal core first. No server or desktop parity in v1. The first provider slice is Codex-first and reuses an existing `codex login`. A minimal TUI is planned only after CLI/session ergonomics and a live agent event stream exist.

## Milestones

| Milestone | Goal | Status | Dependencies | Acceptance | Last Updated |
| --- | --- | --- | --- | --- | --- |
| 00 | Root setup, docs, and progress structure | done | none | Repo is the system of record and workflow targets are defined | 2026-03-08 |
| 01 | Domain model and storage | done | 00 | Structured sessions can be created, loaded, and replayed | 2026-03-08 |
| 02 | Provider foundation and Codex-first OpenAI provider | done | 01 | Existing `codex login` user can complete streaming chat without an API key | 2026-03-08 |
| 03 | Tool runtime and developer tools | done | 01, 02 | The runtime can list and execute the initial `shell` tool | 2026-03-08 |
| 04 | Agent loop and approvals | done | 02, 03 | Multi-turn tool-using loop works with approvals | 2026-03-08 |
| 05 | CLI and session flow | in_progress | 04 | Terminal session can start, inspect, resume, and interrupt cleanly | 2026-03-08 |
| 06 | Agent event stream, evals, and hardening | planned | 04, 05 | Runtime emits structured events and eval harness catches regressions | 2026-03-08 |
| 07 | Interactive TUI | planned | 05, 06 | TUI can drive sessions through event stream without owning runtime logic | 2026-03-08 |
| 99 | Later parity backlog | planned | none | Deferred work is tracked outside v1 milestones | 2026-03-08 |

## Current Focus

- Finish Milestone 05.
- Add a `sessions` command and `run --session <id>` so persisted sessions become usable from the CLI.
- Add clean interrupt handling so the CLI can cancel provider and tool work without corrupting session state.
- Keep `docs/design-principles.md` as the default design checklist for new feature work and architecture changes.
- The first concrete provider is documented in `internal/provider/openaicodex/ARCHITECTURE.md` so fresh agents can understand the provider shape without reading implementation first.
- The tools runtime is documented in `internal/tools/ARCHITECTURE.md` so fresh agents can pick up the tool execution model without prior chat context.
- The agent runtime is documented in `internal/agent/ARCHITECTURE.md` so fresh agents can pick up the control flow without prior chat context.
- `cmd/goose-go run` now exposes the agent runtime through a minimal CLI session path.
- The Codex provider replay path now preserves function-call item IDs separately from call IDs, which fixes multi-turn CLI runs after tool use.
- After Milestone 05, refactor `internal/agent` around a live event stream before building any substantial TUI.
- The future TUI must subscribe to agent events; it must not be built directly on the current blocking `agent.Reply()` path.
- The future TUI must not use SQLite as its primary live UI transport.
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
- Structured file tools beyond `shell` are deferred; if the agent loop becomes too opaque or too permissive with shell-only execution, that scope cut may need to be revisited.
- If we build the TUI on top of the current blocking transcript-after-completion path, we will likely rewrite it once event streaming lands.
- The runtime still lacks a stable event taxonomy for turns, assistant deltas, tool lifecycle, approvals, and termination.
