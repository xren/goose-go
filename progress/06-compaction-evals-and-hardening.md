# 06 Compaction Evals and Hardening

## Objective

Add the first hardening pass once the terminal core works end to end.

## Status

planned

## Dependencies

- 04 Agent Loop and Approvals
- 05 CLI and Session Flow

## Scope In

- context compaction
- eval harness
- smoke task expansion
- regression coverage
- architecture hardening
- runtime observability for agent debugging
- provider and auth diagnostics
- repo hygiene and drift checks

## Scope Out

- large product-surface expansions

## Checklist

- [ ] Add compaction logic
- [ ] Add task eval runner
- [ ] Add regression cases for terminal-core flows
- [ ] Add architecture and boundary checks
- [ ] Add per-session structured logs and transcript artifacts for debugging
- [ ] Add provider smoke coverage and diagnostics for Codex auth/cache failures
- [ ] Add repo hygiene checks for drift, duplication, or oversized files
- [ ] Promote smoke and eval commands into regular workflow

## Acceptance Criteria

- The repo can catch regressions in terminal-core behavior through repeatable smoke and eval runs.
- Agent runs and provider failures produce enough artifacts to debug failures without reconstructing state from memory.

## Open Questions

- None yet.

## Notes / Findings

- Hardening starts only after a working terminal-core loop exists.
- Eval quality will depend on runtime legibility, not only on test count.
- Runtime diagnostics must cover failures caused by shared external auth state, not only agent-loop logic.
