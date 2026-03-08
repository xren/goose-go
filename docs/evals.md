# Evals

`goose-go` should be built with an eval harness from the start, even before the full runtime exists.

## Goal

The project should be testable as an agent, not only as a collection of unit-tested helpers.

## Test Layers

- unit tests for domain types, storage logic, tool validation, and provider translation
- integration tests for provider streaming and session persistence
- CLI smoke tests for the terminal flow
- task evals that exercise the actual agent loop on realistic coding tasks

## Initial Smoke Checks

The first smoke path should confirm:

- the CLI starts
- a session can be created
- a simple prompt can flow through the runtime
- developer tools can be exercised once they exist
- a per-session event trace is written for later inspection

## Initial Task Eval Categories

- plain chat completion
- tool call and result round-trip
- approval required then allow
- approval required then deny
- session resume
- context compaction continuation

## Repo Workflow

- `make smoke` should remain the minimal human-readable check.
- `make eval` should run deterministic scripted scenarios against the real runtime boundaries.
- New runtime behavior should be tied to a unit, integration, smoke, or eval case before the milestone is marked done.
- Event traces should be preferred over terminal output when evals need to assert ordering or runtime state transitions.

## Current Eval Harness

The first eval harness is intentionally narrow:

- it is deterministic and does not call live providers
- it drives `internal/agent` through scripted provider scenarios
- it records normalized trace events and asserts on event ordering and terminal run status

Current baseline scenarios:

- plain chat completion
- tool call round-trip
- approval deny path
- interrupted run
