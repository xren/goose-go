# goose-go

`goose-go` is a Go implementation of Goose terminal core.

The goal is not full product parity with upstream Goose. The first target is a local terminal agent runtime with a structured conversation model, one provider boundary, developer tools, approvals, session persistence, and an end-to-end CLI loop.

## V1 Target

V1 is terminal core only:

- one OpenAI-compatible provider
- structured conversations and sessions
- in-process developer tools
- multi-turn agent loop
- approval flow
- smoke tests and task evals

Not in v1:

- server parity
- desktop parity
- broad provider parity
- remote MCP transport breadth
- full upstream Goose feature parity

## Upstream Reference

The [goose](/Users/rex/projects/goose-go/goose) submodule is the reference implementation. It is read-only in this repo and exists for architecture study, behavior comparison, and implementation notes.

## Repo Map

- [AGENTS.md](/Users/rex/projects/goose-go/AGENTS.md): short navigation guide for agents
- [docs/design-principles.md](/Users/rex/projects/goose-go/docs/design-principles.md): project design rules derived from the agent-first harness approach
- [docs/architecture.md](/Users/rex/projects/goose-go/docs/architecture.md): target package layout and boundaries
- [internal/provider/openaicodex/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/provider/openaicodex/ARCHITECTURE.md): high-level architecture of the first concrete provider
- [docs/invariants.md](/Users/rex/projects/goose-go/docs/invariants.md): hard rules for the project
- [docs/goose-reference.md](/Users/rex/projects/goose-go/docs/goose-reference.md): what to copy, defer, or ignore from upstream Goose
- [docs/evals.md](/Users/rex/projects/goose-go/docs/evals.md): future smoke and eval strategy
- [progress.md](/Users/rex/projects/goose-go/progress.md): project rollup and milestone status
- `progress/`: milestone-by-milestone implementation tracking

## Prerequisites

This repo is currently set up around:

- Go `1.26.1`
- `golangci-lint` `2.11.2`

Install them with Homebrew:

```sh
brew install go golangci-lint
```

## Workflow

Use the root make targets:

```sh
make run
make test
make lint
make check
make smoke
make eval
```

`make eval` is a stable command shape for the future harness. In Milestone 00 it is intentionally a stub.

## Current State

The repo now has the first runtime foundation in place:

- root docs and progress tracking are set up
- design principles for future feature work are documented at the root
- structured conversation types exist
- a SQLite-backed session store exists with tests for create, load, append, replace, and replay

The next milestone is the first provider implementation.
