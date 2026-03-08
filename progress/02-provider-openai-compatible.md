# 02 Provider OpenAI Compatible

## Objective

Add one narrow provider implementation that can support the core runtime.

## Status

planned

## Dependencies

- 01 Domain and Storage

## Scope In

- provider interface
- model config
- one OpenAI-compatible implementation
- streaming support

## Scope Out

- provider breadth
- advanced auth breadth
- non-core provider parity

## Checklist

- [ ] Define provider interface
- [ ] Define model config and usage metadata
- [ ] Implement one OpenAI-compatible provider
- [ ] Support streaming assistant output
- [ ] Add provider integration tests

## Acceptance Criteria

- One provider can complete a simple conversation through the runtime with streaming output.

## Open Questions

- None yet.

## Notes / Findings

- Provider code should not know about tool execution or session persistence.
