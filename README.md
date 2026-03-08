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
- [internal/agent/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/agent/ARCHITECTURE.md): high-level architecture of the runtime loop and approval flow
- [internal/session/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/session/ARCHITECTURE.md): session store boundary, summaries, and SQLite relationship
- [internal/tools/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/tools/ARCHITECTURE.md): high-level architecture of the tool runtime and first concrete tool
- [internal/provider/openaicodex/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/provider/openaicodex/ARCHITECTURE.md): high-level architecture of the first concrete provider
- [internal/evals/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/evals/ARCHITECTURE.md): deterministic runtime eval harness and trace-based regression model
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

`make eval` now runs a minimal deterministic trace-based eval suite over scripted runtime scenarios.

To prove the current Codex provider path end to end, run:

```sh
go run ./cmd/goose-go provider-smoke
```

This uses the real `openai-codex` provider, reads the existing `codex login` cache, sends a tiny prompt, and streams the result to the terminal.

To inspect the translated request, redacted headers, raw SSE events, and normalized provider events:

```sh
go run ./cmd/goose-go provider-smoke --debug
```

`provider-smoke` now reports normalized failure categories such as:

- `auth_missing`
- `auth_invalid`
- `auth_refresh_failed`
- `provider_request_failed`
- `provider_http_error`
- `provider_stream_error`
- `provider_empty_response`

Use `--debug` when you need the low-level cause appended to the concise diagnostic.

The main `goose-go run` path now uses the same diagnostic model for provider/auth failures.

To run the first real agent loop from the CLI:

```sh
go run ./cmd/goose-go run "list my home directory"
```

To require approval before each tool execution:

```sh
go run ./cmd/goose-go run --approve "list my home directory"
```

To list stored sessions:

```sh
go run ./cmd/goose-go sessions
```

To resume an existing session:

```sh
go run ./cmd/goose-go run --session <session-id> "continue from here"
```

Press `Ctrl-C` during `goose-go run` to cancel the active run cleanly. The current persisted session state is kept and the CLI renders the transcript captured so far.
Each `goose-go run` also writes a per-session JSONL trace under `.goose-go/traces/` by default.

## Current State

The repo now has the first runtime foundation in place:

- root docs and progress tracking are set up
- design principles for future feature work are documented at the root
- structured conversation types exist
- a SQLite-backed session store exists with tests for create, load, append, replace, and replay
- a real `openai-codex` provider exists with a minimal runtime smoke path
- an initial `internal/agent` loop exists for multi-turn replies, tool dispatch, max-turn limits, and approval handling

The current milestone is now the event-stream and hardening layer; the basic CLI/session surface is in place, including interrupt handling and per-session event traces.
`goose-go run` now renders from the live agent event stream rather than waiting for the full transcript at the end of a run.
