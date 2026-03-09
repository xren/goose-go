# goose-go Agent Guide

Use this file as a starting point, not the full spec.

## Read First

- [README.md](/Users/rex/projects/goose-go/README.md)
- [docs/design-principles.md](/Users/rex/projects/goose-go/docs/design-principles.md)
- [docs/architecture.md](/Users/rex/projects/goose-go/docs/architecture.md)
- [internal/app/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/app/ARCHITECTURE.md)
- [internal/archcheck/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/archcheck/ARCHITECTURE.md)
- [internal/agent/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/agent/ARCHITECTURE.md)
- [internal/auth/codex/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/auth/codex/ARCHITECTURE.md)
- [internal/conversation/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/conversation/ARCHITECTURE.md)
- [internal/models/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/models/ARCHITECTURE.md)
- [internal/provider/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/provider/ARCHITECTURE.md)
- [internal/session/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/session/ARCHITECTURE.md)
- [internal/storage/sqlite/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/storage/sqlite/ARCHITECTURE.md)
- [internal/tools/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/tools/ARCHITECTURE.md)
- [internal/compaction/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/compaction/ARCHITECTURE.md)
- [internal/provider/openaicodex/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/provider/openaicodex/ARCHITECTURE.md)
- [internal/evals/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/evals/ARCHITECTURE.md)
- [internal/repocheck/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/repocheck/ARCHITECTURE.md)
- [internal/tui/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/tui/ARCHITECTURE.md)
- [internal/tui/markdown/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/tui/markdown/ARCHITECTURE.md)
- [internal/tui/theme/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/tui/theme/ARCHITECTURE.md)
- [docs/invariants.md](/Users/rex/projects/goose-go/docs/invariants.md)
- [docs/goose-reference.md](/Users/rex/projects/goose-go/docs/goose-reference.md)
- [docs/evals.md](/Users/rex/projects/goose-go/docs/evals.md)
- [progress.md](/Users/rex/projects/goose-go/progress.md)

## Working Rules

- Treat [goose](/Users/rex/projects/goose-go/goose) as read-only reference material.
- Keep important design intent in repo docs, not only in chat history.
- Use [docs/design-principles.md](/Users/rex/projects/goose-go/docs/design-principles.md) when designing new features or changing architecture.
- Track milestone status in [progress.md](/Users/rex/projects/goose-go/progress.md) and the files under `/progress`.
- Do not create implementation packages outside the documented architecture without updating the root docs first.
