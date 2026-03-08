# 07d Model Registry and Selection

## Objective

Implement the `pi-mono` style model-selection path for `goose-go` using a local built-in registry, auth-aware availability filtering, runtime-selectable provider/model configuration, and shared CLI/TUI selection behavior.

This stage is registry-first. It should not add custom `models.json` support or live provider model discovery yet.

## Status

done

## Dependencies

- 07a TUI Stage 1 MVP
- 07b TUI Stage 2 UX
- 07c TUI Architecture

## Scope In

- built-in local model registry
- auth-aware availability filtering
- runtime/provider factory driven by selected provider and model
- CLI `--provider` and `--model`
- model-listing command surface
- TUI `/model` picker
- session persistence of provider/model selection
- `/model` local reporting based on runtime/session metadata instead of hard-coded constants

## Scope Out

- live model fetching from OpenAI
- custom `models.json` support
- arbitrary custom providers
- remote provider capability probing
- full `pi-mono` provider breadth

## Key Decision

Follow the `pi-mono` architecture pattern, not the current hard-coded `goose-go` model path:

- local catalog first
- auth-aware filtering second
- CLI and TUI selection on top
- live discovery deferred

## Architecture Notes

### Registry shape

Add a package such as `internal/models` that owns:

- `ProviderID`
- `ModelID`
- `ModelSpec`
- `ProviderSpec`
- built-in catalog data
- lookup helpers:
  - `ListProviders()`
  - `ListModels(provider)`
  - `FindModel(provider, model)`
  - `DefaultModel(provider)`

Each `ModelSpec` should include at least:

- provider id
- model id
- display name
- context window

### Availability filtering

Add a thin availability layer over the built-in catalog.

For the first slice:

- `openai-codex` models are available only if Codex auth resolves successfully
- unavailable models may still be listed, but must carry a clear diagnostic reason

Do not fetch models from the provider backend.

### Runtime selection

Refactor runtime construction so `internal/app` takes an explicit runtime selection:

- `Provider string`
- `Model string`

`OpenRuntime(...)` should:

- validate the selection against the registry
- construct the provider from the selected provider id
- derive `provider.ModelConfig` from the selected `ModelSpec`

### Session persistence

Persist provider/model on each session so resume is deterministic.

Rules:

- new sessions inherit CLI or TUI selection
- resumed sessions default to the persisted provider/model
- explicit override on resume may be supported, but if implemented it must be intentional and documented

### UI behavior

- `goose-go run "/model"` should report the actual runtime/session selection
- `goose-go tui` `/model` should become a picker, not only a reporter
- the TUI picker should be driven by the registry and availability layer, not by provider-specific calls

## Execution Phases

### Phase 0: Registry and availability

Status: done

Tasks:

- add `internal/models`
- define built-in provider/model catalog
- add availability filtering using Codex auth readiness
- keep provider list narrow: `openai-codex` only in the first slice

Acceptance:

- the repo has one authoritative model catalog
- availability is computed locally and does not require a provider round trip

### Phase 1: Runtime and CLI selection

Status: done

Tasks:

- refactor `internal/app/runtime.go` to use selected provider/model rather than hard-coded constants
- add `--provider` and `--model` to `run` and `tui`
- add a model-listing command, preferably `goose-go models`
- reject invalid provider/model combinations cleanly

Acceptance:

- a user can select a valid model at startup
- the runtime uses registry-derived `ModelConfig`
- defaults still work without flags

### Phase 2: Session persistence and local commands

Status: done

Tasks:

- persist provider/model in session metadata
- make resume reuse persisted provider/model by default
- update `/model` local reporting to read runtime/session metadata

Implemented so far:

- `internal/models` now provides the built-in local registry and auth-aware availability filtering
- `internal/app` runtime construction now resolves provider/model through the registry instead of hard-coded model behavior
- `goose-go run` and `goose-go tui` now accept `--provider` and `--model`
- `goose-go models` now lists registry-backed models with availability state
- sessions now persist provider/model metadata
- resumed sessions now reuse their persisted provider/model by default and update the runtime selection before the next provider turn
- `goose-go run /model` now reports the actual runtime/session selection locally
- `goose-go tui /model` is now the registry-backed picker entrypoint

Acceptance:

- resumed sessions do not silently switch models
- `/model` reflects the actual active selection

### Phase 3: TUI picker

Status: done

Tasks:

- replace the TUI `/model` reporter with a picker or modal
- list available models for the current provider
- optionally show unavailable models disabled with a reason
- selecting a model updates the active runtime selection for future runs in that TUI session

Implemented:

- `/model` now opens a registry-backed picker in the TUI
- the picker preselects the current runtime selection
- unavailable models stay visible with a reason instead of disappearing silently
- selecting a model updates runtime state immediately and persists to the active session when one exists

Acceptance:

- model selection is usable inside the TUI without leaving the interactive surface
- the picker is driven entirely by the registry and availability layer

## Testing Strategy

Add coverage for:

1. registry and selection
- provider/model lookup
- default model resolution
- unknown provider/model rejection

2. availability
- Codex auth present vs missing
- unavailable-model reason propagation

3. runtime and CLI
- `OpenRuntime` uses registry-derived model config
- `run --model ...` selects the intended model
- listing command prints registry data, not hard-coded strings

4. session behavior
- provider/model are persisted on new sessions
- resumed sessions reuse persisted selection

5. TUI
- `/model` opens the picker
- selecting a model updates TUI/runtime state
- subsequent runs use the chosen model

## Acceptance Criteria

- `goose-go` no longer hard-codes `gpt-5-codex` in runtime behavior.
- Provider/model selection is backed by a local registry, not model self-reporting or live fetches.
- CLI and TUI share the same registry and runtime-selection behavior.
- Session resume is deterministic with respect to provider/model choice.

## Notes / Findings

- `pi-mono` uses a local built-in registry plus filtering, not live model fetch on each `/model` open.
- That pattern fits the current `openai-codex` subscription path better than live discovery would.
- Custom `models.json` support can be added later once the built-in registry path is stable.
