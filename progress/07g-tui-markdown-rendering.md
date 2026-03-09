# 07g TUI Markdown Rendering

## Objective

Render assistant and system message content as markdown-aware terminal text without surrendering transcript layout control to a full document renderer.

This slice exists to improve readability for common model output such as:

- `**bold**`
- `` `inline code` ``
- `_italic_`
- `[links](https://example.com)`

while preserving the existing `goose-go` TUI structure:

- full-width user bubbles
- transcript-controlled spacing
- grouped tool rendering
- width-bounded transcript rows

## Status

done

## Dependencies

- 07b TUI Stage 2 UX
- 07e TUI Styling and Layout
- 07f TUI Theme System

## Research Findings

### Goose

Upstream Goose uses renderer delegation, not a custom inline transcript renderer:

- Rust CLI renders markdown text through `bat` in [output.rs](/Users/rex/projects/goose-go/goose/crates/goose-cli/src/session/output.rs)
- the Ink text UI uses `marked` + `marked-terminal` in [markdown.tsx](/Users/rex/projects/goose-go/goose/ui/text/src/markdown.tsx)

That works for document-style output, but it is not a good direct fit for `goose-go`'s current Bubble Tea transcript because `goose-go` already owns layout tightly.

### pi-mono

`pi-mono` has the better architecture for `goose-go` to copy:

- a dedicated markdown component in [/Users/rex/projects/pi-mono/packages/tui/src/components/markdown.ts](/Users/rex/projects/pi-mono/packages/tui/src/components/markdown.ts)
- assistant and user message components consume that renderer while still owning message layout:
  - [/Users/rex/projects/pi-mono/packages/coding-agent/src/modes/interactive/components/assistant-message.ts](/Users/rex/projects/pi-mono/packages/coding-agent/src/modes/interactive/components/assistant-message.ts)
  - [/Users/rex/projects/pi-mono/packages/coding-agent/src/modes/interactive/components/user-message.ts](/Users/rex/projects/pi-mono/packages/coding-agent/src/modes/interactive/components/user-message.ts)

The key lesson is:

- markdown rendering should be a reusable content-rendering layer
- transcript layout should remain outside that layer

## Chosen Direction

Implement a small dedicated markdown renderer for the TUI, starting with inline markdown only.

Use:

- `goldmark` for parsing
- a custom renderer that maps parsed inline nodes to ANSI-styled spans using the existing theme tokens
- `goose-go`'s own transcript renderer for width, spacing, grouping, and bubble layout

Do not start with:

- regex-only formatting
- `glamour` as the primary transcript renderer
- full block markdown rendering in the first slice

## Why This Direction

### Why not regex

Regex is too brittle for:

- nested emphasis
- escaping
- links
- malformed backticks
- future extension to blocks

It is acceptable only as a throwaway spike, not as the actual architecture.

### Why not `glamour`

`glamour` is optimized for rendering complete markdown documents into ANSI text. That is useful for CLI pages, but it is the wrong abstraction for the current TUI transcript because:

- it is block/document oriented
- it wants to own layout
- it is awkward to combine with full-width user bubbles and grouped tool rendering
- it would fight the current transcript surface instead of integrating cleanly into it

### Why `goldmark`

`goldmark` gives:

- a real CommonMark parser
- clean AST access
- future growth path to block rendering
- no forced document-level renderer decisions

It is the right base if `goose-go` wants controlled markdown rendering rather than all-or-nothing document formatting.

## Scope In

- assistant message inline markdown rendering
- system message inline markdown rendering where appropriate
- theme tokens for markdown emphasis and inline code
- width-aware ANSI-safe wrapping of styled inline content
- transcript integration without changing tool grouping or user bubble layout

## Scope Out

- full block markdown in the first slice
- fenced code blocks
- syntax highlighting
- markdown tables
- blockquotes and lists in the first slice
- markdown rendering for grouped tool output
- replacing the current transcript layout model

## Package Plan

Add:

- `internal/tui/markdown`

Proposed files:

- `internal/tui/markdown/renderer.go`
  - public render entrypoints
- `internal/tui/markdown/inline.go`
  - inline-node rendering
- `internal/tui/markdown/wrap.go`
  - ANSI-safe wrapping helpers
- `internal/tui/markdown/renderer_test.go`
  - parsing/rendering coverage
- `internal/tui/markdown/ARCHITECTURE.md`
  - package-local design notes once the first slice lands

## Integration Plan

### Phase 1: Theme tokens

Add semantic tokens in [theme.go](/Users/rex/projects/goose-go/internal/tui/theme/theme.go) for:

- `MarkdownBold`
- `MarkdownItalic`
- `MarkdownCodeFG`
- `MarkdownCodeBG`
- `MarkdownLink`
- optionally `MarkdownMuted`

Acceptance:

- markdown styling is theme-driven rather than hard-coded in the renderer

### Phase 2: Inline renderer package

Build the new renderer around `goldmark`:

- parse inline markdown
- map nodes to styled spans
- preserve plain text when parsing is incomplete or unsupported

Initial supported constructs:

- strong emphasis
- emphasis
- inline code
- links

Acceptance:

- inline markdown converts to styled terminal text without changing transcript ownership

### Phase 3: Transcript integration

Update [transcript.go](/Users/rex/projects/goose-go/internal/tui/transcript.go):

- assistant messages use the markdown renderer
- system messages use the markdown renderer where that improves readability
- user messages stay plain for now unless explicitly expanded later

Keep:

- `renderUserText(...)` as the bubble/layout owner
- tool rendering unchanged
- transcript spacing unchanged

Acceptance:

- assistant/system messages visibly style inline markdown while the transcript layout stays stable

### Phase 4: Width and wrapping hardening

Add regression coverage for:

- long bold spans
- long inline code spans
- mixed styled/unstyled content
- links
- narrow viewport widths

Acceptance:

- styled transcript lines still honor viewport width
- wrapping does not break ANSI state or overflow the transcript

### Phase 5: Optional expansion

Only after inline markdown is solid:

- consider fenced code blocks
- consider lists and block quotes
- consider user-message markdown rendering

This is explicitly later work, not part of the first implementation.

## Integration Rules

- transcript owns layout
- markdown renderer owns content styling
- tool rendering remains separate
- user bubbles remain separate
- no provider-specific formatting logic in the markdown package

## Testing Plan

Add tests for:

1. inline bold rendering
2. inline code rendering
3. mixed plain + markdown content
4. links
5. unsupported markdown degrades safely
6. rendered output stays within viewport width
7. assistant transcript items render markdown, but tool items do not route through the markdown renderer

## Acceptance Criteria

- `**goose-go**` and `` `asdfasdf` `` visibly render differently inside assistant/system transcript text
- transcript width rules still hold
- grouped tool rendering is unchanged
- theme controls markdown colors
- the new renderer is a package boundary, not ad hoc formatting inside `renderItem(...)`

## Notes

- Copy `pi-mono`'s layering, not Goose's whole-document renderer choice.
- Goose is useful as proof that markdown-aware terminal rendering is expected, but not as the exact implementation model for the current `goose-go` TUI.
- This slice should start small and correct. Full markdown support can come later if the inline path is solid.
- Implemented first slice:
  - `internal/tui/markdown` now exists as a dedicated package
  - `goldmark` is the parser
  - assistant and system transcript content now route through the markdown renderer
  - theme tokens now control bold, italic, inline code, and link styling, with stronger orange-toned inline-code text and no forced code background in the built-in themes
  - transcript width rules still hold through markdown-aware wrapping tests
