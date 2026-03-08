# Agent Architecture

`internal/agent` is the orchestration layer for the terminal-core runtime.

It ties together:

- the session store
- the provider boundary
- the tool registry
- approval handling
- the multi-turn reply loop
- the live event stream used by CLI and future TUI layers

The package exists so provider, tools, and persistence stay narrow while one place owns runtime control flow.

## Package Position

`internal/agent` depends on normalized runtime boundaries:

- `internal/session`
- `internal/provider`
- `internal/tools`
- `internal/conversation`

It must not absorb provider HTTP logic, tool implementation details, or storage-specific schema logic.

## Runtime Flow

```mermaid
flowchart LR
    A["cmd/goose-go run"] --> B["internal/app.RunAgent"]
    B --> C["Agent.Reply"]
    C --> D["Agent.ReplyStream"]
    D --> E["append user message to session"]
    E --> F["provider.Request"]
    F --> G["provider.Stream"]
    G --> H["emit provider_text_delta events"]
    G --> I["final assistant message"]
    I --> J["persist assistant message"]
    J --> K{"tool requests?"}

    K -- "no" --> L["run_completed event"]
    K -- "yes" --> M["approval check"]

    M -- "pending" --> N["approval_required event"]
    M -- "deny" --> O["synthetic denied tool result"]
    M -- "allow" --> P["tools.Registry.Execute"]

    P --> Q["tool result"]
    O --> Q
    Q --> R["persist tool response as tool-role message"]
    R --> S{"max turns reached?"}
    S -- "no" --> F
    S -- "yes" --> T["run_failed event (max turns)"]
```

## Event Stream Flow

```mermaid
flowchart TD
    A["Agent.ReplyStream"] --> B["run_started"]
    B --> C["user_message_persisted"]
    C --> D["turn_started"]
    D --> E["provider_text_delta*"]
    E --> F["assistant_message_complete"]
    F --> G["assistant_message_persisted"]
    G --> H{"tool calls?"}
    H -- "no" --> I["run_completed"]
    H -- "yes" --> J["tool_call_detected"]
    J --> K{"approval needed?"}
    K -- "yes, no approver" --> L["approval_required"]
    L --> M["run_completed (awaiting approval)"]
    K -- "resolved" --> N["approval_resolved"]
    N --> O["tool_execution_started"]
    O --> P["tool_execution_finished"]
    P --> Q["tool_message_persisted"]
    Q --> R{"another turn?"}
    R -- "yes" --> D
    R -- "no" --> I
```

## Package Topology

```mermaid
flowchart TD
    A["internal/agent"] --> B["internal/provider"]
    A --> C["internal/tools"]
    A --> D["internal/session"]
    A --> E["internal/conversation"]

    B --> F["internal/provider/openaicodex"]
    D --> G["internal/storage/sqlite"]
    C --> H["internal/tools/shell"]
```

## Core Types

- `Agent`
  The orchestrator. It owns one provider, one session store, one tool registry, and one runtime config.
- `Config`
  Runtime settings for system prompt, model choice, max turns, and approval mode.
- `Result`
  The terminal state of one reply operation: completed or awaiting approval, with the updated session.
- `Event`
  The normalized live runtime fact emitted by `ReplyStream`.
- `Approver`
  Optional callback boundary for `approve` mode.
- `ApprovalRequest`
  The normalized approval payload for a pending tool call.

## Current Behavior

The first loop is intentionally narrow:

- one provider request per turn
- one final assistant message per provider turn
- tool calls are read from normalized assistant message content
- tool responses are persisted as `tool` role messages
- approval modes are limited to `auto` and `approve`
- max-turn stopping is enforced by the loop

This is enough to support:

- plain assistant replies
- tool request -> tool execution -> follow-up reply
- approval pause when no approver is present
- deny branch through a synthetic tool result
- default shell execution in the persisted session working directory when the model omits `working_dir`
- live runtime observation without reading SQLite directly

## Event Taxonomy

The current event set is intentionally narrow:

- `run_started`
- `user_message_persisted`
- `turn_started`
- `provider_text_delta`
- `assistant_message_complete`
- `assistant_message_persisted`
- `tool_call_detected`
- `approval_required`
- `approval_resolved`
- `tool_execution_started`
- `tool_execution_finished`
- `tool_message_persisted`
- `run_completed`
- `run_interrupted`
- `run_failed`

These are runtime facts, not provider wire events. The provider still handles SSE internally; the agent exposes normalized milestones the CLI and future TUI can render safely.

## Why Tool Responses Use `tool` Role

Tool responses are stored as `tool` role messages instead of assistant messages because the provider translation layer already expects tool outputs as a separate message class.

That keeps the agent loop simple:

- assistant emits tool request
- tools layer produces tool result
- session stores tool result as a `tool` role message
- provider reconstructs function-call output on the next turn

## Boundary Rules

- `internal/agent` owns orchestration, not transport details.
- `internal/agent` must only depend on normalized provider and tool contracts.
- Approval logic belongs here, not in CLI rendering or provider code.
- Tool execution must go through the registry, not through direct tool-specific calls.
- Session persistence must stay behind the `session.Store` contract.

## Near-Term Growth

Milestone 05 is now in place:

- `cmd/goose-go run` exposes the runtime through a thin app layer
- sessions can be listed and resumed
- `SIGINT` cancels the active run cleanly

The next architecture step is Milestone 06:

- keep growing the event stream into the primary live runtime interface
- keep CLI rendering on top of the event stream instead of transcript-after-completion output
- feed trace/log sinks from the same event stream so runs stay debuggable after the terminal output is gone
- make live rendering and future TUI work subscribe to agent events instead of polling persistence

`internal/agent` should remain the only runtime orchestration layer even after event streaming lands.
