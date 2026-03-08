# Goose Reference

This document records how upstream Goose should inform `goose-go`.

## Upstream Role

The [goose](/Users/rex/projects/goose-go/goose) submodule is the reference implementation. It exists to help analyze architecture and behavior, not to become part of the Go runtime.

## Core for V1

These areas matter for the first Go implementation:

- agent turn loop
- structured conversation and message model
- provider abstraction
- tool and extension boundary
- developer toolset for terminal coding tasks
- session persistence and replay
- prompt building and local hint loading
- context compaction
- approval and safety gating

## Later, Not V1

These areas are valid roadmap targets but should not shape the first implementation:

- recipes beyond minimal support
- remote/stdin MCP transport breadth
- extra providers
- server and desktop parity
- subagents
- apps and scheduling

## Explicitly Deferred

These are out of the initial milestone set and should live in the later-parity backlog:

- dictation and local inference
- telemetry breadth
- OAuth breadth
- gateway integrations
- full product parity with upstream Goose

## Working Rule

When learning from upstream Goose, extract behavior and interfaces into root docs. Do not treat upstream file layout as the required Go package layout.
