# Compaction Architecture

`internal/compaction` is the runtime planning and summarization layer for context compaction.

It does not persist compaction artifacts directly and it does not own agent-loop orchestration. Its job is to answer:

- how large the active conversation context is
- whether compaction should trigger
- where the cut point should be
- what older messages should be summarized
- what active provider view should be reconstructed after a compaction artifact exists
- how to ask the provider for a compaction summary through the normalized provider boundary

This keeps compaction logic explicit, testable, and separate from CLI or storage concerns.

## Package Position

`internal/compaction` currently depends on:

- `internal/conversation`
- `internal/session`
- `internal/provider`

It must not depend on:

- provider implementations
- SQLite details
- CLI rendering
- app-layer orchestration

The agent runtime calls into this package, then persists the resulting compaction artifact through the session store.

## High-Level Flow

```mermaid
flowchart LR
    A["persisted session conversation"] --> B["internal/compaction"]
    C["latest compaction artifact"] --> B
    D["provider model context window"] --> B

    B --> E["EstimateConversationTokens"]
    E --> F{"ShouldCompact?"}
    F -- "no" --> G["active provider view = full conversation"]
    F -- "yes" --> H["FindCutPoint"]
    H --> I["MessagesToSummarize + KeptMessages"]
    I --> J["SerializeForSummarization"]
    J --> K["Summarizer.Summarize"]
    K --> L["persist compaction artifact (future step)"]
    L --> M["BuildActiveMessages"]
    M --> N["active provider view = summary + kept messages"]
```

## Current Responsibilities

The current package owns:

- token estimation for normalized conversation messages
- threshold checks from `context_window - reserve_tokens`
- cut-point selection anchored to user-turn boundaries
- preparation of:
  - `MessagesToSummarize`
  - `KeptMessages`
  - `FirstKeptMessageID`
- reconstruction of the active provider context from:
  - latest compaction artifact
  - kept recent messages
- serialization of messages into a summarization-safe text format
- provider-backed summary generation through the normalized provider interface

## What It Does Not Own

This package does not:

- persist compaction artifacts
- emit agent events
- render CLI notices
- own the retry or threshold orchestration in the live run loop
- build provider HTTP payloads or transport logic

Those belong to:

- `internal/agent`
- `internal/provider` for transport details, while `internal/compaction` owns the high-level summarizer request shape
- `internal/session` / `internal/storage/sqlite`
- `internal/app`

## Runtime Planning Model

```mermaid
flowchart TD
    A["messages[]"] --> B["EstimateConversationTokens"]
    B --> C["total token estimate"]
    C --> D["ShouldCompact(total, contextWindow, settings)"]
    D -- "false" --> E["Preparation{NeedsCompaction:false, KeptMessages:all}"]
    D -- "true" --> F["FindCutPoint(messages, keepRecentTokens)"]
    F --> G["Preparation{MessagesToSummarize, KeptMessages, FirstKeptMessageID}"]
```

The key idea is:

- summarize old context
- keep recent context verbatim
- make the cut point deterministic and easy to test

## Active Context Reconstruction

After a compaction artifact exists, the provider should not receive the full historical conversation.

It should receive:

1. a synthetic summary message built from the compaction artifact
2. the kept messages from `FirstKeptMessageID` onward

That reconstruction is handled by `BuildActiveMessages(...)`.

```mermaid
flowchart LR
    A["full persisted message history"] --> B["find FirstKeptMessageID"]
    C["session.Compaction"] --> D["SummaryMessage(compaction)"]
    B --> E["kept recent messages"]
    D --> F["active provider messages"]
    E --> F
```

This is the architectural reason compaction artifacts are explicit session records rather than invisible in-memory rewrites.

### Example

If a session has 500 persisted messages and compaction chooses:

- summarize `m1` through `m420`
- keep `m421` through `m500`

then the next provider turn should receive:

```text
[
  summary_message(m1..m420),
  m421,
  m422,
  ...,
  m500,
  new_user_message
]
```

Important:

- SQLite still keeps all 500 original messages.
- The provider does not receive all 500 messages again.
- The provider receives the compacted active view: summary + kept suffix + newer messages.

This is the intended runtime contract for compaction in `goose-go`.

## Cut-Point Rule

The current implementation deliberately uses a simple rule:

- walk backward from the newest message
- accumulate estimated tokens
- when the `keep_recent_tokens` budget is crossed, move the cut point back to the nearest user message

This means:

- recent context remains intact
- compaction is aligned to a user-turn boundary
- tool results do not get split off arbitrarily from later context

This is narrower than pi-mono’s split-turn handling and is intentional for the first implementation slice.

## Summary Serialization

`SerializeForSummarization(...)` converts normalized `conversation.Message` values into plain text markers such as:

- `[USER]: ...`
- `[ASSISTANT]: ...`
- `tool_request(name): ...`
- `tool_response: ...`
- `system_notification(level): ...`

This is done so the summarizer sees an explicit transcript to summarize rather than a live conversation to continue.

## Summarizer Flow

The summarizer uses:

- embedded prompt template in `compaction_prompt.md`
- serialized conversation transcript
- optional previous summary block
- optional additional focus instructions

It calls the normalized provider interface and extracts:

- final summary text
- usage metadata from provider events

This keeps the summarization call consistent with the rest of the runtime and avoids adding a provider-specific side channel just for compaction.

## Current Types

- `Settings`
  Controls `enabled`, `reserve_tokens`, and `keep_recent_tokens`.
- `CutPoint`
  The first kept message id and its index.
- `Preparation`
  The planning output for a pending compaction run.
- `SummaryRequest`
  The summarizer input, including optional previous summary and custom instructions.
- `SummaryResult`
  The summarizer output, including summary text and provider usage.
- `Summarizer`
  Provider-backed summarizer over the normalized provider boundary.

Key functions:

- `DefaultSettings()`
- `EstimateMessageTokens(...)`
- `EstimateConversationTokens(...)`
- `ShouldCompact(...)`
- `FindCutPoint(...)`
- `Prepare(...)`
- `BuildActiveMessages(...)`
- `SummaryMessage(...)`
- `SerializeForSummarization(...)`
- `NewSummarizer(...)`
- `(*Summarizer).Summarize(...)`

## Design Tradeoffs

### Why this is simpler than pi-mono

Current `goose-go` compaction planning does not yet support:

- split-turn compaction
- iterative summary updates across multiple compacted checkpoints in the runtime loop
- branch summarization
- extension-defined compaction
- cumulative file-operation tracking

That is intentional. The first goal is a correct, explicit checkpoint model that fits the existing Go runtime.

### Why this is more explicit than Goose

Upstream Goose mostly rewrites conversation visibility state in-place.

This package is designed around explicit compaction artifacts and reconstructed active context instead. That fits:

- the event-driven runtime
- explicit traces
- resume/replay
- future TUI rendering

## Next Integration Step

The next implementation step is agent integration:

1. have `internal/agent` call the compaction planner before provider turns
2. run threshold compaction before normal provider submission
3. run overflow recovery compaction after context-limit failures
4. persist compaction artifacts through the session store
5. emit compaction events into the existing event stream

At that point, `internal/compaction` remains the compaction planning and summarization layer and `internal/agent` remains the orchestration layer.
