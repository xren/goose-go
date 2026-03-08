# 04 Agent Loop and Approvals

## Objective

Implement the multi-turn runtime that ties provider, tools, and sessions together.

## Status

planned

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

- [ ] Implement the turn loop
- [ ] Parse and dispatch tool calls
- [ ] Add approval flow
- [ ] Support allow and deny branches
- [ ] Add agent-loop tests

## Acceptance Criteria

- The agent can solve a multi-turn terminal task that requires tools and approvals.

## Open Questions

- Whether `smart_approve` lands here or immediately after.

## Notes / Findings

- The agent loop should remain the only orchestration layer.
