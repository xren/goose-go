# 03 Tool Runtime and Developer Tools

## Objective

Create the first tool runtime needed by the terminal agent, with `shell` as the initial and sufficient tool for the next milestone.

## Status

done

## Dependencies

- 01 Domain and Storage
- 02 Provider OpenAI Compatible

## Scope In

- tool contract
- tool registry
- in-process tool runtime
- one initial developer tool (`shell`)
- tool validation and result shape

## Scope Out

- remote/stdin MCP transport breadth
- tool marketplace breadth

## Checklist

- [x] Define tool interfaces and registry
- [x] Implement `shell`
- [x] Add unit tests for tool registration and shell execution

## Acceptance Criteria

- The runtime can list, execute, validate, and return structured results for the initial `shell` tool.

## Open Questions

- None yet.

## Notes / Findings

- V1 should favor in-process tools over transport breadth.
- `internal/tools` now owns the normalized tool contract and registry.
- `internal/tools/shell` is the first concrete tool and is enough to unblock the initial agent-loop milestone.
- The tools runtime is now documented in package-local form at `internal/tools/ARCHITECTURE.md` so fresh agents can understand the execution model before reading implementation details.
- `write`, `edit`, and `tree` are no longer on the critical path; they are deferred until the agent loop proves a real need for more structured tools.
