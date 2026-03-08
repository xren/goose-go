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
- [ ] Add context compaction logic
- [ ] Add task eval runner
- [ ] Add regression cases for streaming agent flows
- [ ] Add architecture and boundary checks
- [ ] Add per-session structured logs and transcript artifacts for debugging
- [ ] Add provider smoke coverage and diagnostics for Codex auth/cache failures
- [ ] Add repo hygiene checks for drift, duplication, or oversized files
- [ ] Promote smoke and eval commands into regular workflow

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
- Eval quality will depend on runtime legibility, not only on test count.
- Runtime diagnostics must cover failures caused by shared external auth state, not only agent-loop logic.
- Architecture docs should stay synchronized with the current runtime shape so fresh agents can start Milestone 06 without reconstructing the current system from code first.
