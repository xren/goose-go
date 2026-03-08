# 03 Tool Runtime and Developer Tools

## Objective

Create the first tool runtime and the first-party developer toolset used by the terminal agent.

## Status

planned

## Dependencies

- 01 Domain and Storage
- 02 Provider OpenAI Compatible

## Scope In

- tool contract
- tool registry
- in-process developer tools
- tool validation and result shape

## Scope Out

- remote/stdin MCP transport breadth
- tool marketplace breadth

## Checklist

- [ ] Define tool interfaces and registry
- [ ] Implement `shell`
- [ ] Implement `write`
- [ ] Implement `edit`
- [ ] Implement `tree`
- [ ] Add unit and integration tests for tool execution

## Acceptance Criteria

- Tools can be listed, executed, validated, and returned to the runtime as structured results.

## Open Questions

- None yet.

## Notes / Findings

- V1 should favor in-process tools over transport breadth.
