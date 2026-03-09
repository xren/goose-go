# 07e TUI Styling and Layout

## Objective

Translate the useful terminal presentation patterns from `pi-mono` into a Go-native plan for `goose-go` without copying its full surface area or extension system.

This plan focuses on layout structure, visual hierarchy, and reusable rendering rules. It is not a theme-engine implementation plan by itself.

## Status

in_progress

## Dependencies

- 07a TUI Stage 1 MVP
- 07b TUI Stage 2 UX
- 07c TUI Architecture

## Pi-Mono Findings

`pi-mono` styles the terminal as a system, not as scattered color calls:

- explicit layout slots: startup header, messages, editor, footer
- reusable selector surfaces with the same border, search, list, and hint treatment
- state-specific tool rendering with different backgrounds for pending, success, and error
- footer as a dense operational surface: cwd, session, tokens, cost, context usage, model
- border color communicates mode, especially thinking level and bash mode
- theme tokens drive component rendering instead of each component inventing colors
- keyboard hints are rendered as part of the UI, not left only to docs

These are the parts worth porting.

## Scope In

- establish stable TUI layout slots
- tighten visual hierarchy for transcript, selectors, approval panel, and footer
- standardize selector and panel rendering across `/model`, `/sessions`, approvals, and future commands
- make grouped tool blocks visually distinct by lifecycle state
- add visible keyboard-hint surfaces where they materially reduce friction
- add transcript and footer polish that improves long interactive sessions
- make transcript history navigable with mouse-wheel scrolling plus explicit scroll and jump bindings

## Scope Out

- extension-provided widgets or overlays
- full `pi-mono` feature parity
- custom HTML export styling
- branch/tree UI
- full theme engine and hot reload

## Proposed Layout Model

Adopt the same top-to-bottom mental model as `pi-mono`, but narrower:

1. Header
- session id or `new`
- cwd
- selected provider/model
- concise status

2. Transcript viewport
- user messages
- assistant output
- grouped tool blocks
- compaction notices
- local system notices

3. Active surface
- composer when idle
- approval panel when awaiting approval
- selector panel when `/model` or `/sessions` is open

4. Footer
- key hints
- mode hints
- later: session metrics and context usage once the surface is worth the density

The key structural rule is:
- only one primary interactive surface is active at a time
- transcript remains visually stable above it

## Visual Hierarchy Rules

### Transcript

- user messages should remain visually distinct but low-noise
- assistant text should stay the primary reading surface
- tool blocks should look like operational cards, not plain transcript lines
- compaction and system notices should be visibly secondary
- all transcript roles should wrap inside the viewport width rather than relying on the terminal to wrap oversized lines

### Tool Blocks

Mirror `pi-mono`'s lifecycle treatment:

- pending/requested: neutral-muted background
- running: accent or active border state
- success: success-tinted background
- error/deny: error-tinted background

Within each block:

- title row: tool name + status
- metadata row: args or selected fields
- body: output preview
- width-capped card layout so long outputs wrap inside the viewport instead of producing oversized full-width blocks

Do not flatten all tool output into the same transcript style as assistant text.

### Selectors And Panels

Standardize these surfaces:

- `/model` picker
- `/sessions` picker
- approval panel
- future command palette or settings panel

Shared traits:

- top border
- title row
- muted hint row
- searchable/selectable list or action area
- bottom border

This is one of the clearest `pi-mono` patterns and should be copied directly in spirit.

### Footer

`pi-mono`'s footer is operationally dense. `goose-go` should adopt that gradually.

Recommended order:

1. keep concise key hints and mode indicators
2. add session/model/cwd overflow handling
3. later add token/context/compaction status once the runtime values are reliable and worth the noise

Do not jump straight to a crowded footer before the basic layout settles.

## Execution Phases

### Phase 1: Shared Panel Styling

Status: done

Tasks:

- introduce common panel rendering helpers for bordered overlays and pickers
- unify title, subtitle, and hint rows across approval, model picker, and session picker
- normalize spacing and empty-state rendering

Acceptance:

- picker and approval surfaces look intentionally related instead of separately improvised

### Phase 2: Transcript Hierarchy Pass

Status: done

Tasks:

- refine user/assistant/system spacing
- add lifecycle-aware tool block visuals
- ensure replayed transcript uses the same grouped-tool rendering rules as live runs

Acceptance:

- long tool-using runs remain readable without flattening everything into one text stream

### Phase 3: Footer And Status Polish

Status: done

Tasks:

- make the footer a stable operational surface
- improve truncation and alignment of session/model/cwd
- surface compact but useful hints for active mode and key actions

Acceptance:

- users can orient themselves without opening `/session` repeatedly

### Phase 4: Interaction-Surface Polish

Status: in_progress

Tasks:

- improve focus visibility
- make active panel/composer clearly obvious
- add consistent cancel/confirm hint treatment

Acceptance:

- the active interaction target is always visually obvious

Implemented so far:

- the composer now renders as a clearer active/inactive control surface instead of raw textinput output
- picker panels now include inline confirm/cancel hints instead of relying only on the footer
- selected rows in model/session/theme pickers now have stronger emphasis than cursor-only treatment

## Testing Plan

- reducer tests for panel state transitions
- view tests for grouped tool rendering states
- smoke checks for:
  - approval panel
  - model picker
  - session picker
  - long transcript with grouped tool blocks

## Acceptance Criteria

- The TUI has a stable, intentional layout model instead of incremental one-off styling.
- Approval, model selection, and session selection look like parts of one interface family.
- Tool-heavy sessions are materially easier to read than the current flat transcript treatment.

Current note:

- shared panel styling is now in place for approval, model, session, and theme pickers
- transcript rendering now applies stronger hierarchy for user, assistant, system, and grouped tool blocks
- footer and header surfaces are now more structured and carry model/theme/session context
- the session picker now uses a windowed, scrollable list so long histories do not collapse the main transcript area
- the layout is now transcript-first, with session/model/cwd/status metadata moved to the lower control area near the composer
- user messages now render as gray-background bubbles, while assistant messages stay lower-noise foreground text
- user message bubbles now fill the full transcript width with consistent horizontal padding
- user message bubbles now use a lighter background shade, and vertical separation between turns is now handled between transcript items rather than inside each bubble
- transcript items now have explicit blank-line spacing between turns, which keeps the user bubble tighter while preserving readability

## Notes

- This plan intentionally copies `pi-mono`'s structure, not its TypeScript component count.
- Styling should remain subordinate to the existing headless runtime and Bubble Tea reducer model.
