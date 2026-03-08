# 07b TUI Stage 2 UX

## Objective

Expand the minimal Bubble Tea MVP into a more useful coding interface without violating the headless runtime boundary.

This stage is where the TUI becomes meaningfully more productive, closer in spirit to `pi-mono`, while still staying native to Go and the current `goose-go` architecture.

## Status

planned

## Dependencies

- 07a TUI Stage 1 MVP
- 07c TUI Architecture

## Scope In

- session picker/resume UX
- approval interaction UI
- richer tool rendering
- status/footer panes
- slash commands or command palette
- compaction visibility
- better error surfaces
- transcript scrolling and navigation

## Scope Out

- server or desktop surfaces
- branch/session tree UX
- extension/plugin-defined UI panels
- advanced theme system
- non-terminal frontends

## Checklist

- [ ] Add session picker or recent-session view
- [ ] Add approval interaction flow in the TUI
- [ ] Add richer tool rendering with collapsible or grouped output
- [ ] Add footer/status line with session and runtime state
- [ ] Add slash commands or a command palette
- [ ] Add compaction notices/history surfaces
- [ ] Add navigation and transcript scrolling improvements
- [ ] Add stronger TUI smoke and reducer coverage

## Stage 2 Focus

Stage 2 is where the TUI becomes meaningfully useful for repeated coding sessions. The goal is not visual novelty; it is operational clarity.

Recommended priorities:

1. approval interaction
2. session picker
3. richer tool rendering
4. slash commands
5. navigation improvements

That order matters because approval and session selection change the usability of the app more than UI polish.

## Execution Phases

### Phase 1: Approval UI

- render `approval_required` as a first-class modal or focused panel
- allow:
  - approve
  - deny
  - maybe inspect tool args before decision

Acceptance:

- the user can complete approval-required sessions without dropping back to the plain CLI.

### Phase 2: Session entry UX

- add recent-session list or picker
- support:
  - open recent session
  - continue most recent
  - new session

Acceptance:

- the user no longer needs to pass session ids manually.

### Phase 3: Rich tool surfaces

- group tool call + tool result together
- allow collapse/expand
- distinguish success/error visually
- show compaction/system notices separately from transcript text

Acceptance:

- long tool-using runs stay readable.

### Phase 4: Command surface

- add slash commands or command palette for:
  - new
  - resume
  - copy/export later if needed
- keep the command surface runtime-agnostic

Acceptance:

- users can drive common session actions from inside the TUI.

### Phase 5: Navigation and polish

- transcript jump/scroll improvements
- footer/status improvements
- clearer failure surfaces
- modest keyboard shortcuts

Acceptance:

- the UI feels coherent and efficient for daily use.

## Acceptance Criteria

- The TUI feels meaningfully more usable than the Stage 1 MVP for repeated coding sessions.
- Tool activity, approval state, and session state are first-class parts of the UI.
- The richer UI still consumes only normalized runtime events and session APIs, not provider internals.

## Implementation Notes

- This stage should remain event-driven. No direct provider/tool/storage shortcuts.
- Tool output grouping matters more than visual polish.
- Approval interaction should be implemented only after the event/state model has proven stable in Stage 1.
- Avoid `pi-mono`’s full tree/branch UI in this stage.
- Avoid a custom widget ecosystem until the state/reducer model proves stable.

## Open Questions

- Whether slash commands should be a text protocol in the composer or a separate command surface.
- Whether session picking belongs inside the TUI home screen or as a pre-launch view.

## Risks

- Stage 2 can sprawl into a redesign if Stage 1 state boundaries are weak.
- Approval UI can accidentally leak runtime policy into the view layer if not kept behind the existing agent/app boundaries.
- Session picker UX can drift into persistence-owned logic if the TUI starts querying raw storage structures instead of using session APIs.
