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

## Scope Out

- large product-surface expansions

## Checklist

- [ ] Add compaction logic
- [ ] Add task eval runner
- [ ] Add regression cases for terminal-core flows
- [ ] Add architecture and boundary checks
- [ ] Promote smoke and eval commands into regular workflow

## Acceptance Criteria

- The repo can catch regressions in terminal-core behavior through repeatable smoke and eval runs.

## Open Questions

- None yet.

## Notes / Findings

- Hardening starts only after a working terminal-core loop exists.
