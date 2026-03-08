# 06 Agent Event Stream Evals and Hardening

## Objective

Refactor the runtime to emit structured live events that both the CLI and a future TUI can consume, while adding observability, evals, and hardening around that event-driven runtime.

## Status

in_progress

## Dependencies

- 04 Agent Loop and Approvals
- 05 CLI and Session Flow

## Scope In

- structured agent event stream
- thin blocking wrappers over the streaming runtime
- context compaction
- eval harness
- smoke task expansion
- regression coverage
- architecture hardening
- runtime observability for agent debugging
- provider and auth diagnostics
- repo hygiene and drift checks

## Scope Out

- full interactive TUI
- large product-surface expansions

## Checklist

- [x] Refactor `internal/agent` to emit structured live events
- [x] Keep blocking reply and CLI wrappers as thin adapters over the streaming runtime
- [x] Define a stable event taxonomy for turns, assistant deltas, tool lifecycle, approvals, and termination
- [x] Add context compaction logic (see [06a-context-compaction-plan.md](/Users/rex/projects/goose-go/progress/06a-context-compaction-plan.md))
- [x] Add task eval runner
- [x] Add regression cases for streaming agent flows
- [x] Add architecture and boundary checks
- [x] Add per-session structured logs and transcript artifacts for debugging
- [x] Add provider smoke coverage and diagnostics for Codex auth/cache failures
- [ ] Add repo hygiene checks for drift, duplication, or oversized files
- [x] Promote smoke and eval commands into regular workflow

## Acceptance Criteria

- The runtime emits structured events that a CLI or TUI can subscribe to without reading SQLite directly.
- The repo can catch regressions in terminal-core behavior through repeatable smoke and eval runs.
- Agent runs and provider failures produce enough artifacts to debug failures without reconstructing state from memory.

## Open Questions

- What is the smallest event set that can support both transcript rendering and future approval UI without churn.

## Notes / Findings

- TUI work should subscribe to agent events rather than drive provider, tool, or session logic directly.
- This milestone exists to avoid coupling future UI work to the current blocking `agent.Reply()` path.
- Event streaming should become the source of truth for live rendering; SQLite remains the persistence layer, not the live UI transport.
- `internal/agent` now exposes `ReplyStream`, and `Reply` consumes that stream as a compatibility wrapper.
- `internal/app` now renders `goose-go run` from agent events directly, which makes the event stream a real integration seam instead of test-only infrastructure.
- `internal/app` now records the same event stream into per-session JSONL traces for postmortem debugging and future eval assertions.
- `make eval` now runs a deterministic scripted harness under `internal/evals` that asserts on trace/event outcomes for baseline runtime scenarios.
- The eval harness now covers resumed sessions, awaiting-approval runs, and max-turn termination in addition to the original baseline scenarios.
- `provider-smoke` now maps low-level Codex/auth failures into a generic diagnostic model that can be reused by future providers.
- `internal/archcheck` now holds the stronger dependency rules for auth, provider implementations, storage, evals, and CLI boundaries, with `cmd/archcheck` acting as a thin wrapper.
- Eval quality will depend on runtime legibility, not only on test count.
- Runtime diagnostics must cover failures caused by shared external auth state, not only agent-loop logic.
- Detailed compaction planning now lives in [06a-context-compaction-plan.md](/Users/rex/projects/goose-go/progress/06a-context-compaction-plan.md) and should be treated as the implementation plan for this remaining Milestone 06 item.
- The compaction persistence/model slice is now complete: the session store exposes compaction artifacts, SQLite schema version 2 persists them in a dedicated table, and storage tests cover latest-compaction retrieval and history preservation.
- The compaction planner slice is now complete: `internal/compaction` estimates context size, selects cut points, reconstructs active context from compaction artifacts, and serializes messages for future summarization input.
- The compaction summarizer slice is now complete: `internal/compaction` has an embedded prompt template and a provider-backed summarizer that captures summary text and usage through the normalized provider interface.
- The compaction planner is now documented in [internal/compaction/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/compaction/ARCHITECTURE.md) so the remaining agent integration work has a package-local design reference.
- Threshold and overflow compaction are now integrated into `internal/agent`, persisted as explicit compaction artifacts, and exposed through the live event stream and JSONL traces.
- Remaining compaction work is now mostly validation breadth: add eval coverage for continuation and resume after compaction, rather than building the core runtime path itself.
- Architecture docs should stay synchronized with the current runtime shape so fresh agents can start Milestone 06 without reconstructing the current system from code first.
