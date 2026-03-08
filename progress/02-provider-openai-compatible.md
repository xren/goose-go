# 02 Provider Foundation and Codex First OpenAI Provider

## Objective

Add the provider foundation needed for the core runtime, while tightening package boundaries before provider and agent code start to sprawl. The first concrete provider uses OpenAI/Codex subscription auth by reusing the existing Codex auth cache.

## Status

in_progress

## Dependencies

- 01 Domain and Storage

## Scope In

- package-boundary cleanup needed before provider growth
- provider interface around normalized conversation types only
- model config and usage metadata
- Codex auth/cache integration using `~/.codex/auth.json`
- one `openai-codex` provider
- assembled SSE streaming support
- first architecture enforcement check

## Scope Out

- provider breadth beyond the first slice
- native `goose-go` login
- PKCE or browser callback flow
- keyring-backed Codex credential support
- generic OpenAI API-key provider
- websocket transport
- raw vendor delta streaming outside the provider boundary

## Checklist

- [x] Split domain-facing session contracts from SQLite implementation by moving the SQLite backend under `internal/storage/sqlite`
- [x] Define provider interface around normalized conversation types only
- [x] Define model config and usage metadata
- [x] Add a Codex auth/cache reader with token refresh support
- [ ] Keep Codex/OpenAI request-response DTOs inside the provider implementation
- [ ] Implement one `openai-codex` provider
- [ ] Support assembled assistant streaming output over SSE
- [ ] Add provider integration tests
- [x] Add the first import-boundary architecture check to `make check`

## Acceptance Criteria

- A user with an existing `codex login` can complete a simple streaming conversation without setting an API key.
- Provider code does not depend on SQLite implementation details.
- The repo fails fast if forbidden package imports violate the intended architecture.

## Open Questions

- None yet.

## Notes / Findings

- Provider code should not know about tool execution or session persistence.
- Provider wire shapes should stay private to the provider package and be translated at the boundary.
- The clean v1 path is to reuse the existing Codex auth cache instead of porting a full OAuth or login stack.
- This milestone is where the repo should start turning architectural intent into mechanical checks.
- The SQLite backend now lives under `internal/storage/sqlite` and owns schema migration through `PRAGMA user_version`.
- `internal/provider` now defines the normalized request, event, usage, and model config types that later provider implementations must satisfy.
- `make check` now includes the first architecture import-boundary enforcement pass.
- `internal/auth/codex` now owns file-backed Codex credential loading, JWT-derived metadata, token refresh, and atomic auth-file rewrite.
- The filename still reflects older wording, but the active milestone scope is now Codex-first provider foundation.
