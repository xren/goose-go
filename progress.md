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
| 05 | CLI and session flow | done | 04 | Terminal session can start, inspect, resume, and interrupt cleanly | 2026-03-08 |
| 06 | Agent event stream, evals, and hardening | in_progress | 04, 05 | Runtime emits structured events and eval harness catches regressions | 2026-03-08 |
| 07 | Interactive TUI | planned | 05, 06 | TUI can drive sessions through event stream without owning runtime logic | 2026-03-08 |
| 99 | Later parity backlog | planned | none | Deferred work is tracked outside v1 milestones | 2026-03-08 |

## Current Focus

- Start Milestone 06.
- `internal/agent` now exposes a live event stream through `ReplyStream`, and `Reply` is now a wrapper over that streaming runtime.
- `internal/app` and `cmd/goose-go run` now consume the live agent event stream instead of rendering only after completion.
- `goose-go run` now writes per-session JSONL traces from the same event stream under `.goose-go/traces/`.
- `make eval` now runs a first deterministic trace-based harness over scripted runtime scenarios.
- `make eval` now covers plain chat, tool round-trip, approval deny, interrupt, resume, awaiting-approval, and max-turn runtime scenarios.
- `provider-smoke` now classifies failures into normalized diagnostics and preserves low-level causes behind `--debug`.
- `goose-go run` now classifies provider/auth failures through the same diagnostic model used by `provider-smoke`.
- `internal/archcheck` now holds the executable boundary rules, with `cmd/archcheck` reduced to a thin entrypoint.
- `cmd/goose-go sessions` now exposes stored sessions from the session store abstraction.
- `cmd/goose-go run --session <id>` now resumes an existing session and prints only the new transcript segment.
- `cmd/goose-go run` now cancels cleanly on `SIGINT` and renders the persisted transcript instead of a raw context error.
- Context compaction storage/model groundwork is now in place through explicit compaction artifacts in the session store and SQLite schema version 2.
- The compaction planner groundwork is now in place in `internal/compaction`, including token estimation, cut-point selection, active-context reconstruction, and summarization-safe serialization.
- The compaction summarizer groundwork is now in place in `internal/compaction`, including the first prompt template, provider-backed summary generation, previous-summary updates, and usage capture.
- The next compaction work is Step 4 from [progress/06a-context-compaction-plan.md](/Users/rex/projects/goose-go/progress/06a-context-compaction-plan.md): wire threshold and overflow compaction into `internal/agent`, persist artifacts during runs, and emit compaction events.
- Keep `docs/design-principles.md` as the default design checklist for new feature work and architecture changes.
- The first concrete provider is documented in `internal/provider/openaicodex/ARCHITECTURE.md` so fresh agents can understand the provider shape without reading implementation first.
- The tools runtime is documented in `internal/tools/ARCHITECTURE.md` so fresh agents can pick up the tool execution model without prior chat context.
- The agent runtime is documented in `internal/agent/ARCHITECTURE.md` so fresh agents can pick up the control flow without prior chat context.
- The session boundary is documented in `internal/session/ARCHITECTURE.md` so fresh agents can see the store interface and SQLite boundary without reading implementation first.
- The eval harness is documented in `internal/evals/ARCHITECTURE.md` so fresh agents can understand what `make eval` actually exercises without reading the test file first.
- The detailed compaction implementation plan now lives in [progress/06a-context-compaction-plan.md](/Users/rex/projects/goose-go/progress/06a-context-compaction-plan.md) so fresh agents can pick up the remaining Milestone 06 work without prior chat context.
- The compaction planner is now documented in [internal/compaction/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/compaction/ARCHITECTURE.md) so fresh agents can understand the checkpoint model and cut-point logic before touching the agent loop.
- The root, agent, and session architecture diagrams are updated to reflect the current CLI/session surface and the Milestone 06 event-stream direction.
- `cmd/goose-go run` now exposes the agent runtime through a minimal CLI session path.
- The Codex provider replay path now preserves function-call item IDs separately from call IDs, which fixes multi-turn CLI runs after tool use.
- Tool execution now defaults to the persisted session working directory when the model omits `working_dir`, which keeps resumed and cross-repo runs scoped to the right workspace.
- After Milestone 05, refactor `internal/agent` around a live event stream before building any substantial TUI.
- The future TUI must subscribe to agent events; it must not be built directly on the current blocking `agent.Reply()` path.
- The future TUI must not use SQLite as its primary live UI transport.
- Keep native `goose-go` login out of the first slice.
- Keep the `goose/` submodule as reference-only material.

## Blocked / Risks

- The module path is still local (`goose-go`) and will need a real import path later.
- Upstream Goose has broader product surface area than this repo should target in v1.
- If root docs drift from implementation, agents will start making incorrect assumptions.
- The first persistence backend is SQLite with JSON-encoded conversations; if that shape changes later, migration work will be needed.
- The repo now has a first architecture enforcement check, but the rules are still narrow and will need to expand with the runtime.
- The first provider slice assumes file-backed Codex credentials in `~/.codex/auth.json`; keyring-backed credentials are deferred.
- Shared Codex auth cache refresh now exists, but it still depends on the current file-backed cache shape and not keyring-backed credentials.
- The current provider implementation is intentionally narrow: SSE only, no websocket transport, and no broader Responses surface yet.
- Generic OpenAI API-key provider support is deferred until after the Codex-first slice is stable.
- Structured file tools beyond `shell` are deferred; if the agent loop becomes too opaque or too permissive with shell-only execution, that scope cut may need to be revisited.
- If we stop short of using the event stream for real CLI and eval flows, the TUI work will still end up debugging an unproven integration seam.
- The current eval harness is intentionally narrow and scripted; it still needs broader scenario coverage and CLI-facing smoke integration.
- The architecture checks are stronger now, but they still do not cover every intended dependency rule in the repo.
