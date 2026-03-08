# goose-go Agent Guide

Use this file as a starting point, not the full spec.

## Read First

- [README.md](/Users/rex/projects/goose-go/README.md)
- [docs/design-principles.md](/Users/rex/projects/goose-go/docs/design-principles.md)
- [docs/architecture.md](/Users/rex/projects/goose-go/docs/architecture.md)
- [internal/tools/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/tools/ARCHITECTURE.md)
- [internal/provider/openaicodex/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/provider/openaicodex/ARCHITECTURE.md)
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
