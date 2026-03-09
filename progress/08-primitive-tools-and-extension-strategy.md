# 08 Primitive Tools and Extension Strategy

## Objective

Define the next runtime-capability layer after the current `shell`-only baseline:

- a first-party set of primitive built-in tools
- a default permission policy by capability class
- a later extension transport layer for MCP and other external tools

This plan keeps `goose-go` useful without requiring external servers, while preserving a clean path to future extensibility.

## Status

in_progress

## Dependencies

- 04 Agent Loop and Approvals
- 05 CLI and Session Flow
- 06 Agent Event Stream Evals and Hardening

## Scope In

- first-party primitive tool set definition
- tool categorization by capability class
- default permission policy by category
- implementation order for primitive tools
- future MCP / extension transport position in the architecture

## Scope Out

- full MCP implementation
- browser automation implementation
- provider-specific internet access
- broad remote-tool ecosystem support
- server/desktop parity work

## Tool Categories

### Primitive Read Tools

These are low-risk, high-frequency tools that should be built in and approved by default.

Target set:

- `read_file`
- `list_dir`
- `find_files`
- `grep`
- `fetch_url`

Properties:

- no local mutation
- no arbitrary command execution
- deterministic inputs and outputs
- suitable for routine autonomous use

Default permission:

- `allow`

### Primitive Write and Exec Tools

These are mutation or execution boundaries and should require approval by default.

Target set:

- `write_file`
- `edit_file`
- `shell`

Properties:

- local file mutation or arbitrary command execution
- higher risk of unintended or destructive changes
- should remain explicit in approval and trace surfaces

Default permission:

- `ask`

### Extension Tools

These are tools delivered through a later extension layer, such as MCP or other remote/local tool providers.

Examples later:

- browser automation
- SaaS API integrations
- remote context systems
- custom enterprise tools

Properties:

- larger trust surface
- extra auth/process/runtime complexity
- tool semantics not fully controlled by `goose-go`

Default permission:

- `ask`
- later configurable per extension/tool when the extension system exists

## Core Design Rules

- Web access is a tool capability, not a provider feature.
- Primitive tools come first; extension transport comes later.
- Permission policy is attached by tool capability, not by trying to infer intent from shell commands.
- The agent/runtime stays provider-agnostic with respect to tool categories.
- All tools must emit structured lifecycle events and traces through the existing runtime event stream.

## Primitive Tool Rollout Order

### Phase 1: Read-first baseline

Implement the read tools first:

- `read_file`
- `list_dir`
- `find_files`
- `grep`
- `fetch_url`

Current state:

- `read_file`, `list_dir`, `find_files`, `grep`, and `fetch_url` are now implemented
- all current primitive read tools are registered in the main runtime and modeled as `read + allow`
- approval-mode runs now auto-allow them through the tool metadata path

Why first:

- these reduce overuse of `shell`
- they improve observability and validation immediately
- they fit the default `allow` policy cleanly
- they give the agent a safer way to inspect repos and web content

Acceptance:

- the agent can inspect the workspace and fetch documentation pages without needing `shell`
- these tools are approved by default and represented clearly in traces/TUI

### Phase 2: Structured mutation tools

Add mutation tools next:

- `write_file`
- `edit_file`

Why second:

- these reduce heredoc and ad hoc shell edits
- they improve replayability and reviewability of file changes
- they fit the default `ask` policy cleanly

Acceptance:

- file creation and targeted edits can be expressed without `shell`
- approval prompts clearly distinguish read vs mutate vs exec operations

### Phase 3: Rebalance `shell`

Keep `shell` as the escape hatch, but narrow how often the agent needs it.

Goals:

- use `shell` primarily for commands that are genuinely execution-oriented
- stop using `shell` for routine reads and simple file operations once primitives exist

Acceptance:

- common repository inspection and file editing flows no longer depend on `shell`
- `shell` remains approval-gated by default

## Web Access Plan

### Phase 1: `fetch_url`

Implement a first-party read-only web tool:

- input: URL
- output: normalized response payload
  - final URL
  - status code
  - content type
  - extracted text or markdown-friendly body

Rules:

- no browser automation
- no JS interaction
- no login/session handling
- treat it like a read tool
- default permission `allow`

Why:

- this covers documentation pages, articles, and simple web research cheaply
- it avoids coupling browser automation into the core runtime too early

### Phase 2: Browser automation later

Browser automation should be a separate capability later, not folded into `fetch_url`.

Possible later implementations:

- MCP-backed browser server
- Playwright-backed extension/tool

Default permission:

- `ask`

## Extension Transport Plan

### MCP and external tools

MCP support is explicitly lower priority than primitive tools.

When implemented, it should:

- live as a separate extension transport layer
- not replace primitive tools
- not be required for core coding workflows
- integrate with the same agent event stream and approval surfaces

Desired architecture:

- primitive built-in tools for core workflows
- extension tools for ecosystem breadth

This follows the same broad split seen in Goose and `pi-mono`:

- web/browser/API access should be tooling
- not provider logic
- not model magic

## Implementation Requirements

### Runtime

- tool metadata must include category/capability class
- approval policy should be driven by that metadata
- traces and TUI rendering should reflect the category clearly enough for users to reason about risk

Current foundation in place:

- tool definitions now carry capability and default approval metadata
- the tools registry stores metadata alongside each registered tool
- the agent now reads `ApprovalDefault` from registry metadata when deciding whether approval is required
- `shell` is now explicitly modeled as `exec + ask`

### TUI / CLI

- read tools should not interrupt the user by default
- write/exec tools should continue to surface approval clearly
- future extension tools should show their origin distinctly once the extension layer exists

### Tests

Add coverage for:

- permission defaults by category
- agent/tool loop with read tools approved by default
- agent/tool loop with write/exec tools requiring approval
- `fetch_url` normalization and failure behavior
- regression tests that `shell` is no longer used for flows covered by the new primitives where applicable

## Acceptance Criteria

- `goose-go` has a documented and tracked primitive tool strategy.
- Primitive read tools are approved by default.
- Primitive write/exec tools require approval by default.
- Extension transport is clearly deferred and architected as additive, not foundational.
- A fresh agent can pick up this file and know the rollout order without chat history.

## Open Questions

- Whether `edit_file` should begin as simple find/replace only or support a patch-style format in its first version.
- Whether `fetch_url` should return raw HTML plus extracted text, or extracted text only in v1.
- Whether the first extension transport slice should be MCP-only or a more generic external-tool abstraction.

## Notes / Findings

- The current `shell`-only baseline is functional but too blunt for long-term agent ergonomics and permission control.
- The right split is primitive built-ins first, extension breadth later.
- Internet access should be treated as a tool capability; the first version should be read-only `fetch_url`, not full browser automation.
