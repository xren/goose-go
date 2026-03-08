# 06 Context Compaction

## Objective

Add context compaction to `goose-go` so long-running sessions can continue within model context limits without destroying persisted history. The runtime should preserve full session history in storage, build a compacted provider view for active turns, and emit explicit compaction events and traces.

## Status

done

## Dependencies

- 01 Domain and Storage
- 04 Agent Loop and Approvals
- 05 CLI and Session Flow
- 06 Agent Event Stream Evals and Hardening

## Design Decisions

- Preserve full history in SQLite. Compaction must not destructively rewrite or delete prior session history.
- Persist explicit compaction artifacts instead of mutating old messages into invisibility-only state.
- Build a compacted provider view at runtime from: latest compaction summary + kept recent messages + newer messages.
- Trigger compaction in two paths:
  - threshold-based before provider submission when the context budget is too full
  - overflow-recovery after provider context-limit failure
- Keep recent context verbatim. Summarize older context only.
- Emit explicit agent events for compaction so CLI, traces, evals, and a future TUI can observe it.
- Start with one built-in compaction strategy. Do not add extension-defined compaction or branch summarization in v1.

## Scope In

- session-side compaction artifact model
- runtime compaction planning and cut-point selection
- summary generation via the existing provider boundary
- threshold-triggered compaction
- overflow-recovery compaction
- compaction events and trace output
- deterministic tests and evals for continuation after compaction

## Scope Out

- branch summarization
- extension-driven custom compaction
- multiple compaction strategies selectable at runtime
- UI workflows beyond exposing the resulting events/traces
- tool-pair summarization as a separate background subsystem

## Proposed Data Model

- Add an explicit compaction artifact to the session layer.
- The artifact should capture at minimum:
  - `id`
  - `session_id`
  - `summary`
  - `first_kept_message_id`
  - `tokens_before`
  - `trigger_reason` (`manual`, `threshold`, `overflow`)
  - `created_at`
- Keep raw conversation messages intact.
- Reconstruct the active provider context by combining:
  - synthetic compaction summary message
  - messages from `first_kept_message_id` onward
- Prefer an explicit compaction table or persisted artifact record over encoding compaction only inside message metadata.

## Proposed Trigger Policy

- Add compaction settings under config with defaults similar to pi-mono's shape:
  - `enabled`
  - `reserve_tokens`
  - `keep_recent_tokens`
- Threshold trigger:
  - before provider submission, compact if estimated context tokens exceed `context_window - reserve_tokens`
- Overflow trigger:
  - if the provider returns a context-limit error, compact once and retry once
- Do not compact repeatedly in a loop on the same stale usage/error state
- Continue to support a manual compaction command later, but it is not required for the first implementation slice

## Proposed Summarization Strategy

- Summarize only the older portion of the conversation selected by the cut point
- Keep recent messages verbatim
- Serialize conversation messages into a summarization-safe text representation rather than sending them as a normal conversational transcript
- Use a structured summary prompt focused on continuation, likely with these sections:
  - Goal
  - Constraints and Preferences
  - Progress
  - Key Decisions
  - Next Steps
  - Critical Context
- Preserve exact file paths, commands, error strings, and open work items
- Keep tool-result text bounded during summarization input construction if needed to avoid the summarization request itself overflowing

## Proposed Runtime Flow

1. Load persisted conversation and latest compaction artifact
2. Build the active provider context view
3. Estimate active-context tokens
4. If over threshold, run compaction before the provider call
5. Emit `compaction_started`
6. Generate summary through the provider
7. Persist compaction artifact
8. Rebuild active context view
9. Emit `compaction_completed`
10. Continue the normal provider/tool loop
11. If the provider still overflows, surface a terminal error after one recovery attempt

## Proposed Event Model

- `compaction_started`
  - session id
  - turn number
  - trigger reason
  - tokens before
- `compaction_completed`
  - session id
  - turn number
  - trigger reason
  - tokens before
  - first kept message id
- `compaction_failed`
  - session id
  - turn number
  - trigger reason
  - error

These should flow through the same agent event stream and therefore into traces automatically.

## Implementation Plan

### Step 1: Session and storage model
- Decide whether compaction artifacts live in:
  - a dedicated SQLite table, or
  - a new session artifact abstraction backed by SQLite
- Add migrations for the chosen schema
- Add storage tests for create/load/latest-compaction retrieval

Status:
- done

Implementation notes:
- `internal/session/store.go` now defines `Compaction`, `CompactionParams`, `CompactionTrigger`, `AppendCompaction`, and `GetLatestCompaction`.
- `internal/storage/sqlite/store.go` now persists compactions in a dedicated `compactions` table via schema version 2.
- `internal/storage/sqlite/store_test.go` now covers latest-compaction retrieval, not-found behavior, and preservation of conversation history alongside compaction artifacts.

### Step 2: Context planning
- Add a compaction planner package or module, likely under `internal/agent` or a dedicated `internal/compaction`
- Implement:
  - token estimation for active context
  - cut-point selection
  - active-context reconstruction from compaction artifact + recent messages
- Keep the logic provider-agnostic except where model context window is needed

Status:
- done

Implementation notes:
- `internal/compaction/compaction.go` now owns the first provider-agnostic planner slice.
- The planner now provides:
  - token estimation for conversation messages
  - threshold checks from `context_window - reserve_tokens`
  - cut-point selection anchored to user-turn boundaries
  - active-context reconstruction from a compaction artifact plus kept messages
  - summarization-safe conversation serialization helpers
- `internal/compaction/compaction_test.go` covers token estimation, cut-point selection, reconstruction, and serialization.

### Step 3: Summarization prompt and summarizer
- Add the first compaction prompt template to the repo
- Implement conversation serialization for summarization input
- Implement the compaction summarizer using the existing provider boundary
- Ensure provider usage is tracked for compaction runs separately from normal turns

Status:
- done

Implementation notes:
- `internal/compaction/compaction_prompt.md` now holds the first compaction summary prompt template.
- `internal/compaction/summarizer.go` now provides a provider-backed summarizer over the existing `internal/provider` boundary.
- The summarizer supports:
  - serialized conversation input
  - previous-summary updates
  - additional focus instructions
  - usage capture from provider events
- `internal/compaction/summarizer_test.go` now covers request construction, previous-summary handling, provider error propagation, and empty-summary rejection.

### Step 4: Agent integration
- Integrate threshold checks before provider submission
- Integrate overflow recovery after provider context-length errors
- Persist compaction artifacts and rebuild the active provider view
- Emit compaction events on the live event stream

Status:
- done

Implementation notes:
- `internal/agent/agent.go` now checks active context size before provider submission and runs threshold compaction when the estimated budget is exceeded.
- The agent now performs one overflow-recovery compaction attempt when the provider returns a context-length style error, then retries the provider turn once with the compacted context.
- Compaction artifacts are now persisted during live runs through `session.Store.AppendCompaction(...)`.
- The live agent event stream now emits `compaction_started`, `compaction_completed`, and `compaction_failed`.
- `internal/agent/agent_test.go` now covers both threshold compaction and overflow-recovery compaction paths.

### Step 5: CLI and trace integration
- Ensure traces capture compaction events
- Render minimal inline compaction notices in the CLI
- Keep the rendered UX narrow; event correctness matters more than UI polish in this phase

Status:
- done

Implementation notes:
- `internal/app/run.go` now records compaction events into the existing per-session JSONL trace files.
- The CLI renderer now emits narrow `system>` notices for compaction start, completion, and failure.
- No new compaction-specific transport path was added; the existing event stream remains the single source for live rendering and traces.

### Step 6: Tests and evals
- Unit tests:
  - cut-point selection
  - provider-view reconstruction
  - compaction persistence
- Agent tests:
  - threshold compaction path
  - overflow-recovery compaction path
  - no double-compaction on stale usage
- Eval cases:
  - continue after threshold compaction
  - continue after overflow recovery
- resumed session after prior compaction

Status:
- done

Implementation notes:
- Unit and agent coverage now exist for planner behavior, summary generation, threshold compaction, and overflow recovery.
- `internal/evals/evals_test.go` now covers:
  - threshold compaction continuation
  - resume after prior compaction
- The agent integration now also handles three important edge cases:
  - persisted summaries count toward future compaction planning
  - threshold/overflow compaction no longer silently retries unchanged context when the initial cut point would summarize nothing
  - explicit `Compaction.Enabled=false` configuration is preserved instead of being overwritten by defaults

## Acceptance Criteria

- Full history remains persisted and replayable after compaction.
- The provider receives a compacted context view instead of the full historical conversation.
- Threshold compaction works before normal provider turns.
- Overflow compaction works as a one-time recovery path.
- Compaction emits explicit runtime events and appears in traces.
- Eval coverage proves that the session can continue correctly after compaction.

## Open Questions

- Do we want one generic token estimator at first, or provider-aware estimators per provider?
- Do we want manual compaction in the first slice or only automatic compaction plus later CLI support?

## Notes / Findings

- Goose is simpler but rewrites conversation visibility in place.
- pi-mono's explicit compaction artifact model fits `goose-go`'s event-driven architecture better.
- The first `goose-go` implementation should copy pi-mono's explicit checkpointing idea without copying its full tree/branch summarization complexity.
- Compaction should be implemented before substantial TUI work, because the TUI will need to observe and render compaction events as part of long-running sessions.
- The first compaction slice is now end-to-end complete: persistence, planning, summarization, agent integration, CLI/trace integration, and eval coverage are all in place.
