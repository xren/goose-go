# 04 Agent Loop and Approvals

## Objective

Implement the multi-turn runtime that ties provider, tools, and sessions together.

## Status

done

## Dependencies

- 02 Provider OpenAI Compatible
- 03 Tool Runtime and Developer Tools

## Scope In

- turn loop
- tool call parsing
- tool execution
- tool result reinjection
- max-turn limits
- approval modes

## Scope Out

- broad parity with all upstream behaviors

## Checklist

- [x] Implement the turn loop
- [x] Parse and dispatch tool calls
- [x] Add approval flow
- [x] Support allow and deny branches
- [x] Add agent-loop tests

## Acceptance Criteria

- The runtime can complete multi-turn tool-using interactions with `shell`, max-turn limits, and `auto` or `approve` approval handling.

## Open Questions

- Whether `smart_approve` lands here or immediately after.

## Notes / Findings

- The agent loop should remain the only orchestration layer.
- The agent runtime is now documented in package-local form at `internal/agent/ARCHITECTURE.md` so fresh agents can understand the control flow before reading implementation details.
- `internal/agent` now owns provider orchestration, tool dispatch, approval handling, and conversation persistence.
- Tool responses are persisted as `tool` role messages so the provider can reconstruct function-call output on the next turn.
- Tool execution now receives runtime defaults from the session context, including the default working directory for shell execution.
