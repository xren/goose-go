# Progress

## Objective

Build `goose-go` as a Go implementation of Goose terminal core: a local agent runtime with structured sessions, a provider boundary, developer tools, approvals, and an end-to-end CLI loop that can later support a proper interactive TUI.

## Current V1 Target

Terminal core first. No server or desktop parity in v1. The first provider slice is Codex-first and reuses an existing `codex login`. A minimal TUI is planned only after CLI/session ergonomics and a live agent event stream exist.

## Milestones

| Milestone | Goal | Status | Dependencies | Acceptance | Last Updated |
| --- | --- | --- | --- | --- | --- |
| 00 | Root setup, docs, and progress structure | done | none | Repo is the system of record and workflow targets are defined | 2026-03-08 |
| 01 | Domain model and storage | done | 00 | Structured sessions can be created, loaded, and replayed | 2026-03-08 |
| 02 | Provider foundation and Codex-first OpenAI provider | done | 01 | Existing `codex login` user can complete streaming chat without an API key | 2026-03-08 |
| 03 | Tool runtime and developer tools | done | 01, 02 | The runtime can list and execute the initial `shell` tool | 2026-03-08 |
| 04 | Agent loop and approvals | done | 02, 03 | Multi-turn tool-using loop works with approvals | 2026-03-08 |
| 05 | CLI and session flow | done | 04 | Terminal session can start, inspect, resume, and interrupt cleanly | 2026-03-08 |
| 06 | Agent event stream, evals, and hardening | done | 04, 05 | Runtime emits structured events and eval harness catches regressions | 2026-03-08 |
| 07 | Interactive TUI | in_progress | 05, 06 | TUI can drive sessions through event stream without owning runtime logic | 2026-03-08 |
| 99 | Later parity backlog | planned | none | Deferred work is tracked outside v1 milestones | 2026-03-08 |

## Current Focus

- Milestone 06 is complete.
- Milestone 07 is now in progress with the first Bubble Tea TUI scaffold under `internal/tui` and `cmd/goose-go tui`.
- Stage 1 of Milestone 07 is now complete: the TUI can start runs, resume sessions by id, stream transcript/tool activity, interrupt active runs, and replay persisted history through the shared runtime path.
- The next TUI work is Stage 2 richer UX, not more Stage 1 plumbing.
- The detailed Stage 2 plan is now in [progress/07b-tui-stage2-ux.md](/Users/rex/projects/goose-go/progress/07b-tui-stage2-ux.md).
- The approval continuation seam for Stage 2 is now implemented in `internal/agent` and `internal/app`.
- The Stage 2 approval UI is now implemented in `internal/tui` on top of the approval continuation seam.
- Shell execution now requires approval by default on both main user surfaces: `goose-go run` uses inline terminal approval prompts by default, and `goose-go tui` uses the in-app approval panel.
- The `pi-mono`-style local model registry and runtime-selection slice are now complete in `internal/models` and `internal/app`.
- Session persistence of provider/model is now implemented, and resumed sessions reuse that selection by default.
- The TUI `/model` picker is now implemented on top of the registry-backed selection path.
- The TUI recent-session picker is now implemented on top of `ListSessions(...)`, exposed through `/sessions` and `Ctrl-R`.
- The recent-session picker now uses a keyboard-driven windowed list and consumes `PageUp` / `PageDown`, `Home`, and `End` for long session lists.
- The TUI has been refactored to use terminal-native scrollback instead of a Bubble Tea viewport for transcript history.
- Session replay and finalized live transcript output now print into terminal scrollback, while the Bubble Tea surface stays focused on composer, status, approval, and pickers.
- The TUI now uses a transcript-first layout: session/model/cwd/status metadata render in the lower control area instead of occupying the top of the screen.
- Human messages in the TUI now render with a subdued gray background bubble instead of plain foreground-only text.
- Human message bubbles now span the full transcript width with fixed horizontal padding instead of hugging only the text width.
- Human message bubbles now use a lighter background tone, while vertical separation between turns is handled at the transcript-item level instead of inside each bubble.
- Interaction-surface polish is now underway: the composer has clearer active/inactive treatment, and pickers now show inline confirm/cancel hints with stronger selected-row emphasis.
- Tool lifecycle is now rendered as grouped transcript blocks in the TUI instead of flat request/result lines.
- The TUI now defaults to compact rendering, with `--debug` and `/debug` available when full tool args/output and verbose UI detail are needed.
- The first broader local TUI command surface is now in place through `/help`, `/session`, and `/new`.
- The TUI now exposes `/context`, which opens a persistent right-side inspector for the active next-turn provider context, including the resolved system prompt, compaction state, active messages, and estimated tokens.
- Built-in dark/light TUI themes are now implemented through `internal/tui/theme`, exposed via `goose-go tui --theme <name>` and the local `/theme` picker.
- The TUI now runs in normal screen mode without alt-screen or transcript mouse capture, so terminal-native scrolling, selection, and scrollback search work again.
- The first markdown-rendering slice from [progress/07g-tui-markdown-rendering.md](/Users/rex/projects/goose-go/progress/07g-tui-markdown-rendering.md) is now implemented: `internal/tui/markdown` uses `goldmark`, theme-driven inline styling, and width-aware wrapping for assistant/system transcript text.
- `internal/prompt` now exists as a concrete runtime package: `goose-go run` and `goose-go tui` eagerly load local `AGENTS.md` files from the working directory up to the git root and append them to the system prompt as project context.
- The root architecture diagram in [docs/architecture.md](/Users/rex/projects/goose-go/docs/architecture.md) is now synced to the current runtime shape, including `internal/app`, `internal/models`, `internal/compaction`, trace writing, and the live TUI/event-stream path.
- The default runtime max-turn limit is now 10000 instead of 8, so long CLI and TUI sessions do not stop early under normal use.
- `goose-go run /model` remains a local reporter, while `goose-go tui /model` now opens the registry-backed picker.
- The next Stage 2 TUI work is now split more explicitly: interaction features are landing already, and the next planning slice adds a `pi-mono`-informed styling/layout pass plus a token-based theme system.
- `goose-go run`, `goose-go sessions`, and `goose-go tui` no longer share a hard 5-minute root context deadline; they now use long-lived cancelable contexts suitable for multi-hour sessions.
- Milestone 07 is now split into a rollup plus supporting plan files:
  - [progress/07a-tui-stage1-mvp.md](/Users/rex/projects/goose-go/progress/07a-tui-stage1-mvp.md)
  - [progress/07b-tui-stage2-ux.md](/Users/rex/projects/goose-go/progress/07b-tui-stage2-ux.md)
  - [progress/07c-tui-architecture.md](/Users/rex/projects/goose-go/progress/07c-tui-architecture.md)
  - [progress/07d-model-registry-and-selection.md](/Users/rex/projects/goose-go/progress/07d-model-registry-and-selection.md)
  - [progress/07e-tui-styling-and-layout.md](/Users/rex/projects/goose-go/progress/07e-tui-styling-and-layout.md)
  - [progress/07f-tui-theme-system.md](/Users/rex/projects/goose-go/progress/07f-tui-theme-system.md)
  - [progress/07g-tui-markdown-rendering.md](/Users/rex/projects/goose-go/progress/07g-tui-markdown-rendering.md)
- The TUI planning files are now populated with the execution plan; implementation should start from `07c` and `07a`, not from the rollup alone.
- `internal/agent` now exposes a live event stream through `ReplyStream`, and `Reply` is now a wrapper over that streaming runtime.
- `internal/app` and `cmd/goose-go run` now consume the live agent event stream instead of rendering only after completion.
- `goose-go run` now writes per-session JSONL traces from the same event stream under `.goose-go/traces/`.
- `make eval` now runs a first deterministic trace-based harness over scripted runtime scenarios.
- `make eval` now covers plain chat, tool round-trip, approval deny, interrupt, resume, awaiting-approval, and max-turn runtime scenarios.
- `provider-smoke` now classifies failures into normalized diagnostics and preserves low-level causes behind `--debug`.
- `goose-go run` now classifies provider/auth failures through the same diagnostic model used by `provider-smoke`.
- `internal/archcheck` now holds the executable boundary rules, with `cmd/archcheck` reduced to a thin entrypoint.
- `cmd/goose-go sessions` now exposes stored sessions from the session store abstraction.
- `cmd/goose-go run --session <id>` now resumes an existing session and prints only the new transcript segment.
- `cmd/goose-go run` now cancels cleanly on `SIGINT` and renders the persisted transcript instead of a raw context error.
- Context compaction storage/model groundwork is now in place through explicit compaction artifacts in the session store and SQLite schema version 2.
- The compaction planner groundwork is now in place in `internal/compaction`, including token estimation, cut-point selection, active-context reconstruction, and summarization-safe serialization.
- The compaction summarizer groundwork is now in place in `internal/compaction`, including the first prompt template, provider-backed summary generation, previous-summary updates, and usage capture.
- Context compaction is now integrated into `internal/agent`: threshold checks run before provider turns, overflow recovery compacts once and retries once, and compaction artifacts are persisted during live runs.
- `cmd/goose-go run` now renders compaction notices from the event stream and records the same compaction events into per-session JSONL traces.
- The detailed compaction plan in [progress/06a-context-compaction-plan.md](/Users/rex/projects/goose-go/progress/06a-context-compaction-plan.md) is now complete end to end.
- The agent compaction path now also handles prior-summary token accounting, forced reduction when the initial cut point would be a no-op, and explicit `Compaction.Enabled=false` configs.
- `internal/repocheck` now closes out the remaining Milestone 06 hardening work with oversized-file checks and local Markdown link validation.
- Stage 1 TUI work should use Bubble Tea.
- The initial TUI scaffold now reuses the same runtime/session path as `goose-go run`, and consumes `ReplyStream(...)` directly rather than inventing a separate runtime path.
- Stage 1 TUI coverage now includes reducer and scripted smoke tests for replay, run start, streamed assistant output, tool activity, and interrupt behavior.
- The manual TUI runbook now lives in [README.md](/Users/rex/projects/goose-go/README.md).
- The next TUI work is Stage 2 UX expansion.
- A dedicated primitive-tool and permission rollout plan now exists in [progress/08-primitive-tools-and-extension-strategy.md](/Users/rex/projects/goose-go/progress/08-primitive-tools-and-extension-strategy.md).
- The primitive-tool rollout plan is now in progress: tool definitions carry capability and default-approval metadata, the registry stores that metadata alongside each tool, and the agent approval path now reads approval defaults from the registry instead of relying on tool-name conventions.
- The first primitive read tool, `read_file`, is now implemented and registered in the main runtime as `read + allow`, which proves the metadata-driven permission path end to end.
- The full Phase 1 primitive read-tool baseline is now implemented in the main runtime: `read_file`, `list_dir`, `find_files`, `grep`, and `fetch_url` all run as `read + allow`.
- Stage 2 planning now explicitly treats approval as a runtime/app integration problem before it becomes a TUI modal problem.
- Keep `docs/design-principles.md` as the default design checklist for new feature work and architecture changes.
- The first concrete provider is documented in `internal/provider/openaicodex/ARCHITECTURE.md` so fresh agents can understand the provider shape without reading implementation first.
- The tools runtime is documented in `internal/tools/ARCHITECTURE.md` so fresh agents can pick up the tool execution model without prior chat context.
- The agent runtime is documented in `internal/agent/ARCHITECTURE.md` so fresh agents can pick up the control flow without prior chat context.
- The session boundary is documented in `internal/session/ARCHITECTURE.md` so fresh agents can see the store interface and SQLite boundary without reading implementation first.
- The eval harness is documented in `internal/evals/ARCHITECTURE.md` so fresh agents can understand what `make eval` actually exercises without reading the test file first.
- The detailed compaction implementation plan now lives in [progress/06a-context-compaction-plan.md](/Users/rex/projects/goose-go/progress/06a-context-compaction-plan.md) so fresh agents can pick up the remaining Milestone 06 work without prior chat context.
- The compaction planner is now documented in [internal/compaction/ARCHITECTURE.md](/Users/rex/projects/goose-go/internal/compaction/ARCHITECTURE.md) so fresh agents can understand the checkpoint model and cut-point logic before touching the agent loop.
- The root, agent, and session architecture diagrams are updated to reflect the current CLI/session surface and the Milestone 06 event-stream direction.
- `cmd/goose-go run` now exposes the agent runtime through a minimal CLI session path.
- The Codex provider replay path now preserves function-call item IDs separately from call IDs, which fixes multi-turn CLI runs after tool use.
- Tool execution now defaults to the persisted session working directory when the model omits `working_dir`, which keeps resumed and cross-repo runs scoped to the right workspace.
- After Milestone 05, refactor `internal/agent` around a live event stream before building any substantial TUI.
- The future TUI must subscribe to agent events; it must not be built directly on the current blocking `agent.Reply()` path.
- The future TUI must not use SQLite as its primary live UI transport.
- Keep native `goose-go` login out of the first slice.
- Keep the `goose/` submodule as reference-only material.

## Blocked / Risks

- The module path is still local (`goose-go`) and will need a real import path later.
- Upstream Goose has broader product surface area than this repo should target in v1.
- If root docs drift from implementation, agents will start making incorrect assumptions.
- The first persistence backend is SQLite with JSON-encoded conversations; if that shape changes later, migration work will be needed.
- The repo now has a first architecture enforcement check, but the rules are still narrow and will need to expand with the runtime.
- The first provider slice assumes file-backed Codex credentials in `~/.codex/auth.json`; keyring-backed credentials are deferred.
- Shared Codex auth cache refresh now exists, but it still depends on the current file-backed cache shape and not keyring-backed credentials.
- The current provider implementation is intentionally narrow: SSE only, no websocket transport, and no broader Responses surface yet.
- Generic OpenAI API-key provider support is deferred until after the Codex-first slice is stable.
- Structured file tools beyond `shell` are deferred; if the agent loop becomes too opaque or too permissive with shell-only execution, that scope cut may need to be revisited.
- If we stop short of using the event stream for real CLI and eval flows, the TUI work will still end up debugging an unproven integration seam.
- The current eval harness is intentionally narrow and scripted; it still needs broader scenario coverage and CLI-facing smoke integration.
- The architecture checks are stronger now, but they still do not cover every intended dependency rule in the repo.
- The new `repocheck` is intentionally narrow; it does not yet attempt clone detection or deep documentation-consistency analysis.
