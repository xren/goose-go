# 07b TUI Stage 2 UX

## Objective

Expand the Stage 1 Bubble Tea MVP into a meaningfully more productive coding interface without violating the headless runtime boundary.

Stage 2 is not about visual polish first. It is about closing the main usability gaps that still force users back to the plain CLI:

- approval interaction
- session selection without manual ids
- grouped tool output
- richer in-TUI session controls
- in-TUI model selection backed by a local registry rather than hard-coded constants

This stage should move `goose-go` closer in spirit to `pi-mono`'s interactive coding UX while staying native to Go and preserving the current event-driven runtime architecture.

## Status

in_progress

## Dependencies

- 07a TUI Stage 1 MVP
- 07c TUI Architecture

## Scope In

- approval interaction flow in the TUI
- session picker / recent-session entry UX
- richer grouped tool rendering
- toggleable compact tool rendering for lower-noise long sessions
- better status and footer surfaces
- slash commands or a command palette for session actions
- compaction visibility beyond simple transcript notices
- model registry and model-selection UI
- styling and layout standardization for panels, transcript hierarchy, and footer surfaces
- stronger TUI reducer and smoke coverage for the richer flows

## Scope Out

- server or desktop surfaces
- branch/session tree UX
- plugin-defined UI panels
- advanced theme system beyond a narrow token-based foundation
- non-terminal frontends
- broad `pi-mono` feature parity

## Critical Constraint

Stage 2 cannot start with view work alone.

The current runtime path returns `StatusAwaitingApproval` and ends the run when approval is required. That is enough for Stage 1 read-only visibility, but it is not enough for an interactive approval modal.

So Stage 2 must begin with a runtime/app seam that lets the TUI resolve an approval and continue the run without dropping back to a different surface.

That means Phase 0 is a runtime integration phase, not a styling phase.

## Priority Order

1. model registry and runtime selection
2. session picker / recent sessions
3. grouped tool rendering
4. command surface
5. styling and layout standardization
6. navigation and status polish

## Proposed Package and Surface Changes

### Runtime / app changes

Likely additions:

- `internal/agent`
  - explicit approval continuation surface
  - or a stream-pause / resolver mechanism for pending approvals
- `internal/app`
  - TUI-friendly runtime controller around approval and session actions
  - runtime provider/model selection driven by a local model registry
  - keep provider/store/agent wiring here, not in the Bubble Tea reducer
- `internal/models`
  - built-in provider/model catalog
  - availability filtering

### TUI changes

Likely state additions inside `internal/tui`:

- approval state
- session picker state
- grouped tool-block state
- command-surface state
- model-picker state

Keep this inside `internal/tui` first. Avoid premature subpackages unless the reducer/view surface becomes unwieldy.

## Execution Phases

### Phase 0: Approval runtime seam

Status: done

This phase is required before approval UI.

Tasks:

- design and implement a continuation path for pending approvals
- avoid ending the run in a way that forces the user to restart the workflow elsewhere
- keep approval policy in `internal/agent`, not in the TUI
- make the TUI consume approval requests and send back decisions through `internal/app`

Acceptance:

- the TUI can approve or deny a pending tool request and the run continues in the same interactive surface

Implemented runtime surfaces:

- `internal/agent.PendingApproval(...)`
- `internal/agent.ResolveApproval(...)`
- `internal/agent.ResolveApprovalStream(...)`
- `internal/app.Runtime.PendingApproval(...)`
- `internal/app.Runtime.ResolveApproval(...)`
- `internal/app.Runtime.ResolveApprovalStream(...)`

The remaining Stage 2 work now starts after approval UI.

### Phase 1: Approval UI

Status: done

Implemented behavior:

- approval-required runs now open a focused panel inside the TUI
- the panel shows tool name, structured args, session id, and cwd context
- `a`/`y` approve and `d`/`n` deny
- the continuation stays on the same interactive surface through `ResolveApprovalStream(...)`

Acceptance:

- approval-required runs are usable entirely inside the TUI

### Phase 2: Model registry and selection

System of record:

- [07d-model-registry-and-selection.md](/Users/rex/projects/goose-go/progress/07d-model-registry-and-selection.md)

Tasks:

- add a local built-in model registry
- refactor runtime selection away from hard-coded provider/model constants
- add CLI `--provider` and `--model`
- add a model-listing command
- persist provider/model in session metadata
- replace TUI `/model` reporter with a picker backed by the registry

Acceptance:

- provider/model selection works the `pi-mono` way: local catalog first, auth-aware filtering, no live fetches
- CLI and TUI share the same selection behavior
- resumed sessions preserve provider/model choice deterministically

### Phase 3: Session entry UX
Status: done

Implemented:

- `/sessions` now opens a recent-session picker inside the TUI
- `Ctrl-R` opens the same picker as a keyboard shortcut
- selecting a session replays persisted conversation and adopts its persisted provider/model through the runtime boundary
- the session picker now uses a scrollable window for long lists, with wheel and page-key navigation

Acceptance:
- users no longer need to copy session ids to resume work in the TUI

### Phase 4: Grouped tool rendering
Status: done

Implemented:

- tool lifecycle is now grouped into one transcript block per tool call in the TUI
- grouped blocks preserve tool args, running/completed/error state, and final output
- replayed conversations rebuild the same grouped tool blocks instead of flattening request/result lines

Acceptance:

- long tool-using runs remain readable without scrolling through a flat append-only transcript

### Phase 5: Command surface

Status: in_progress

Tasks:

- add slash commands or a command palette for common session actions
- keep command handling at the TUI/app layer
- do not couple command parsing to provider or agent logic

Implemented so far:

- `/help` now lists the current local TUI command surface
- `/session` reports current session id, cwd, provider, and model
- `/new` resets the interactive surface to a fresh session state
- `/model` and `/sessions` remain local entry points for their pickers

Acceptance:

- common session actions are available without leaving the TUI

Current note:
- approval, model selection, recent-session picking, grouped tool blocks, and the first local command set are now in place
- shared panel styling, transcript hierarchy, and built-in tokenized dark/light themes are now in place
- remaining Stage 2 work is about interaction-surface polish, richer command surfaces, and eventual custom themes

### Phase 6: Styling and layout standardization

System of record:

- [07e-tui-styling-and-layout.md](/Users/rex/projects/goose-go/progress/07e-tui-styling-and-layout.md)
- [07f-tui-theme-system.md](/Users/rex/projects/goose-go/progress/07f-tui-theme-system.md)

Tasks:

- standardize panel rendering across approval, model picker, and session picker
- improve transcript hierarchy for user, assistant, system, and grouped tool blocks
- make footer and status surfaces more intentionally structured
- introduce a token-based theme direction instead of continuing with scattered ad hoc styling

Acceptance:

- the TUI looks like one interface system rather than a collection of separate widgets

## Approval Architecture Notes

This is the highest-risk part of Stage 2.

Rules:

- approval policy stays in `internal/agent`
- approval transport stays in `internal/app`
- approval presentation stays in `internal/tui`

Do not:

- make the TUI decide which tools require approval
- let the TUI call tools directly after approval
- encode provider-specific assumptions in the approval modal

## Model Selection Notes

Follow the `pi-mono` pattern:

- local built-in model catalog
- auth-aware filtering
- UI and CLI selection on top
- live provider discovery deferred

Do not:

- fetch model lists from OpenAI every time `/model` opens
- depend on model self-identification for runtime truth
- keep runtime behavior pinned to hard-coded `gpt-5-codex` constants once this phase starts

## Styling Notes

`pi-mono`'s styling strength comes from stable layout slots, reusable selector surfaces, and semantic theme tokens. `goose-go` should copy that structure before it chases broader feature parity or cosmetic flourishes.

## Session Picker Architecture Notes

Use only the session abstraction:

- `ListSessions(...)`
- `GetSession(...)` / replay through runtime/session APIs

Do not:

- read SQLite tables directly from the TUI
- couple picker logic to current database schema details

## Tool Rendering Notes

Keep grouped tool state reducer-driven. Do not reparse rendered strings, and do not fall back to flat transcript lines for replayed sessions.

## Testing Strategy

Stage 2 should add:

1. approval flow tests
2. model registry and picker tests
3. session picker tests
4. grouped tool rendering tests
5. TUI smoke tests

## Acceptance Criteria

- The TUI can handle approval-required runs without falling back to a different interface.
- Users can select the runtime model from CLI and inside the TUI without live provider fetches.
- Users can resume recent sessions without typing raw session ids.
- Tool activity is grouped and readable as a first-class UI surface.
- The richer UI still consumes only normalized runtime events, registry data, and session APIs, not provider internals.
