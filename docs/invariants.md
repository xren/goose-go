# Invariants

These rules are non-negotiable unless this file is updated as part of the same change.

## Repo Rules

- Root docs are the system of record for project intent and architecture.
- Important design decisions must be written in the repo, not only explained in chat.
- Progress is tracked at the root through [progress.md](/Users/rex/projects/goose-go/progress.md) and the files under `/progress`.
- [goose](/Users/rex/projects/goose-go/goose) is read-only reference material for learning and comparison.

## Runtime Rules

- Structured conversation state is the source of truth, not ad hoc prompt strings.
- The agent loop owns orchestration.
- Providers only translate between internal conversation/tool structures and model APIs.
- Tool execution is isolated behind a tool registry and tool contracts.
- Session persistence must be able to replay a conversation without provider-specific reconstruction.

## Boundary Rules

- `provider` must not execute tools.
- `tools` must not contain provider request or response translation logic.
- `cli` must not become the home of core runtime behavior.
- `session` must not depend on terminal presentation.
- New cross-package shortcuts are not allowed unless the architecture doc is updated first.

## Delivery Rules

- A change that alters public runtime behavior must update docs and tests in the same change.
- New milestone work must update its corresponding file under `/progress`.
- Deferred ideas belong in the later-parity backlog, not hidden TODOs spread through the repo.
