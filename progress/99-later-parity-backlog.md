# 99 Later Parity Backlog

## Objective

Track valid future work without polluting the v1 milestone sequence.

## Status

planned

## Dependencies

none

## Scope In

- native `goose-go` login
- keyring-backed Codex credential support
- websocket transport for the Codex provider
- generic OpenAI API-key provider
- structured file tools beyond `shell` (`write`, `edit`, `tree`)
- advanced Pi-like TUI features beyond the minimal interactive shell
- recipes beyond minimal support
- remote/stdin MCP transport breadth
- server parity
- desktop parity
- extra providers beyond the Codex-first path
- subagents
- dictation and local inference
- telemetry breadth

## Scope Out

- anything required for terminal-core v1

## Checklist

- [ ] Decide whether to add native `goose-go` login instead of reusing existing Codex auth state
- [ ] Evaluate keyring-backed Codex credential support
- [ ] Evaluate websocket transport for the Codex provider
- [ ] Evaluate generic OpenAI API-key provider support after the Codex-first slice is stable
- [ ] Revisit whether `write`, `edit`, and `tree` are needed after the first agent-loop milestone
- [ ] Revisit advanced TUI features such as slash commands, queued messages, tree view, overlays, and richer theming after the minimal TUI lands
- [ ] Revisit recipes once terminal core is stable
- [ ] Evaluate MCP transport support after in-process tools are solid
- [ ] Decide whether server parity is worth adding
- [ ] Decide whether desktop parity is worth adding
- [ ] Evaluate additional providers after one provider is stable
- [ ] Revisit subagents after the base loop is hardened
- [ ] Revisit dictation and local inference later
- [ ] Revisit broader telemetry later

## Acceptance Criteria

- Deferred work is tracked in one place and does not distort the v1 plan.

## Open Questions

- Which deferred areas provide the highest leverage after terminal-core v1.

## Notes / Findings

- This file is explicitly non-v1.
- V1 intentionally reuses existing `codex login` state from `~/.codex/auth.json`.
- Broader provider and auth surface area is intentionally deferred.
- Structured file tools beyond `shell` are intentionally deferred until the agent loop demonstrates a need for tighter permissions or better observability.
- Advanced TUI features should come only after the runtime is event-driven and the minimal interactive shell is stable.
