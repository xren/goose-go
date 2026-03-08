# 07a TUI Stage 1 MVP

## Objective

Build the smallest useful Bubble Tea TUI on top of the existing event-driven runtime.

The Stage 1 goal is not a rich coding environment. It is a thin interactive shell over the current `goose-go` runtime that proves:

- the TUI can submit prompts
- the TUI can render the live agent event stream
- the TUI can interrupt cleanly
- the TUI can resume an existing session through a simple entrypoint

## Status

done

## Dependencies

- 06 Agent Event Stream Evals and Hardening
- 07c TUI Architecture

## Scope In

- Bubble Tea application shell
- Bubble Tea `tea.Model` root loop
- Bubbles `textinput` for the composer
- Bubbles `viewport` for transcript scrolling
- Lip Gloss layout and styling
- event-to-state adapter
- transcript view
- input/composer field
- submit/run flow
- interrupt flow
- basic session resume by id
- tool activity rendering
- basic error/status rendering

## Scope Out

- approval interaction UI
- session picker UI
- slash commands
- multi-pane tool inspectors
- transcript search
- advanced keyboard shortcuts
- theming/polish work

## Checklist

- [x] Add Bubble Tea, Bubbles, and Lip Gloss dependencies and document the rationale
- [x] Add `cmd/goose-go tui` entrypoint
- [x] Add `internal/tui` package skeleton
- [x] Define the Stage 1 root model, Bubble Tea messages, and update loop
- [x] Build an event adapter that translates agent events into TUI messages
- [x] Render transcript items from user, assistant, system, tool, and compaction events
- [x] Add input field and submit flow with `textinput`
- [x] Add transcript viewport with `viewport`
- [x] Add running/idle/interrupted/failed status handling
- [x] Add Ctrl-C interrupt behavior for the active run
- [x] Add a minimal resume entrypoint using session id
- [x] Add reducer tests and TUI smoke coverage for start, stream, tool activity, and interrupt

Current note:

- reducer and scripted TUI smoke coverage now cover transcript replay, run start, streamed assistant output, tool activity, and interrupt behavior
- the manual TUI runbook now lives in [README.md](/Users/rex/projects/goose-go/README.md)
- additional UI polish can happen later without reopening the Stage 1 MVP acceptance criteria


## Stack Rationale

Recommended Stage 1 stack:

- Bubble Tea
  - root application loop
  - keyboard/update model
  - composable commands
- Bubbles `textinput`
  - stable input handling without building custom editing behavior
- Bubbles `viewport`
  - transcript scrolling without inventing our own scroll container first
- Lip Gloss
  - layout, color, padding, and status formatting

This is the right fit because `goose-go` already has a normalized event stream. Bubble Tea is best used here as a state machine and renderer, not as a place to re-implement runtime behavior.

## Recommended Stage 1 UX

Single-column layout:

- header
  - session id
  - cwd
  - run state
- transcript viewport
  - user messages
  - assistant text
  - tool activity
  - system notices
- composer
  - one-line or growing input field
- footer
  - shortcuts: submit, interrupt, quit

No side panes in Stage 1.
No approval interaction in Stage 1.

## Execution Phases

### Phase 0: App shell

- add dependencies
- add `cmd/goose-go tui`
- create a root Bubble Tea program with:
  - startup
  - quit
  - resize handling

Acceptance:

- `go run ./cmd/goose-go tui` opens and exits cleanly.

### Phase 1: State and reducer

- define structured TUI state
- define Bubble Tea messages for:
  - window resize
  - input submission
  - agent event arrival
  - run completion
  - run failure
- keep reducer logic pure where practical

Acceptance:

- state transitions can be tested without a real terminal.

### Phase 2: Runtime bridge

- bridge `internal/agent.ReplyStream(...)` into Bubble Tea messages
- run the stream in a goroutine
- send normalized messages back into the TUI loop
- ensure cancellation shuts down the goroutine cleanly

Acceptance:

- a submitted prompt produces TUI messages from the live runtime stream.

### Phase 3: Transcript and composer

- render transcript items in a viewport
- add text input for prompt submission
- support:
  - submit
  - clear
  - ignore submit while already running

Acceptance:

- user can submit a prompt and watch the transcript update live.

### Phase 4: Tool activity and status

- render:
  - tool requested
  - tool running
  - tool result persisted
  - compaction notices
  - failure/interrupted states
- add a small status line

Acceptance:

- tool-using runs are legible without opening traces.

### Phase 5: Resume and interrupt

- support launching the TUI against an existing session id
- support interrupting the active run
- show resumed context in the transcript view from session replay

Recommendation:

- Stage 1 resume should be via launch flag or startup parameter first, not an in-TUI picker.

Acceptance:

- start new session
- resume known session
- interrupt active run

### Phase 6: Tests and smoke coverage

- reducer tests
- event adapter tests
- smoke tests with scripted providers
- one manual runbook in docs

Acceptance:

- Stage 1 behavior is testable without live Codex access.

## Acceptance Criteria

- A user can start a TUI session, submit a prompt, and watch assistant/tool output stream live.
- The TUI can interrupt an active run cleanly without owning runtime logic.
- The TUI can resume a known session by id.
- The TUI consumes the agent event stream rather than polling SQLite or waiting for a final transcript dump.

## Implementation Notes

- Stage 1 should be single-column and intentionally boring in layout.
- The event stream remains the source of truth; the TUI is only a reducer and renderer.
- The TUI should not parse provider-specific data or inspect SQLite directly for live state.
- Approval-required events may be shown read-only in Stage 1, but interactive approval is deferred.
- Keep transcript state as structured items, not pre-rendered strings.
- Use the existing session APIs only for:
  - create/load session
  - resume
  - initial transcript replay
- Do not make Bubble Tea models call tool or provider code directly.

## Open Questions

- How much transcript history should remain live in memory before truncation/paging is needed.

## Risks

- duplicate assistant output if both deltas and final assistant messages are rendered naively
- goroutine leaks if stream cancellation is not tied cleanly to Bubble Tea lifecycle
- transcript memory growth if every line is kept forever without paging/truncation strategy
- view churn if tool output is rendered as raw append-only text instead of structured items
