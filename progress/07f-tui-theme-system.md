# 07f TUI Theme System

## Objective

Introduce a token-based theme system for `goose-go`'s TUI, following the useful parts of `pi-mono`'s theme architecture:

- built-in named themes
- shared semantic tokens instead of scattered colors
- component rendering driven by tokens
- later support for file-based custom themes and hot reload

## Status

planned

## Dependencies

- 07c TUI Architecture
- 07e TUI Styling and Layout

## Pi-Mono Findings

`pi-mono` does not hard-code colors per component. It has:

- JSON theme files
- a theme schema
- semantic tokens for:
  - core UI
  - message backgrounds
  - tool lifecycle states
  - markdown
  - syntax highlighting
  - thinking-level borders
  - bash mode
- a theme runtime that resolves variables and terminal color capabilities
- built-in dark and light themes
- hot reload for custom theme files

`goose-go` does not need all of that immediately, but it should copy the semantic-token direction now.

## Scope In

- semantic theme token set for current TUI surfaces
- built-in dark and light themes
- a small theme package consumed by `internal/tui`
- theme-driven colors for transcript, tool blocks, panels, footer, and status text
- future-compatible shape for custom theme files

## Scope Out

- immediate full custom-theme loading
- syntax highlighting theme parity
- extension-defined theme tokens
- export-specific themes
- hot reload in the first slice

## Proposed Token Groups

### Core UI

- `accent`
- `muted`
- `dim`
- `text`
- `success`
- `warning`
- `error`
- `border`
- `border_active`
- `selected_bg`

### Transcript

- `user_bg`
- `user_text`
- `assistant_text`
- `system_text`
- `notice_text`

### Tool Blocks

- `tool_pending_bg`
- `tool_running_bg`
- `tool_success_bg`
- `tool_error_bg`
- `tool_title`
- `tool_output`

### Panels

- `panel_title`
- `panel_hint`
- `panel_border`
- `panel_selected`

### Footer And Status

- `footer_text`
- `footer_muted`
- `status_idle`
- `status_running`
- `status_waiting`
- `status_error`

Do not start with dozens of tokens. Start with the surfaces we actually render today.

## Proposed Package Shape

- `internal/tui/theme`
  - token definitions
  - built-in theme values
  - theme selection and access helpers

The TUI renderer should consume semantic helpers, not raw ANSI constants spread through many files.

## Execution Phases

### Phase 1: Internal Tokenization

Tasks:

- define semantic tokens
- replace ad hoc style decisions in `internal/tui` with theme lookups
- ship built-in `dark` and `light`

Acceptance:

- TUI colors are driven from one shared theme source

### Phase 2: Startup Selection

Tasks:

- allow `goose-go tui --theme <name>`
- persist active theme for the session if useful, or keep it per-launch first
- expose current theme in `/session` or a future `/settings`

Acceptance:

- the user can switch between built-in themes without code changes

### Phase 3: File-Backed Custom Themes

Tasks:

- define a JSON theme file shape
- load from a repo-local or user-global theme directory
- validate shape before applying

Acceptance:

- custom themes can be added without changing code

### Phase 4: Hot Reload

Tasks:

- watch the active custom theme file
- re-render the TUI when it changes

Acceptance:

- theme edits apply live during a TUI session

## Testing Plan

- unit tests for token lookup and built-in themes
- TUI view tests ensuring current surfaces consume semantic tokens
- config/startup tests for theme selection once `--theme` exists

## Acceptance Criteria

- No major TUI surface depends on one-off hard-coded color choices.
- Built-in dark/light themes render the same interface structure with different token values.
- The theme package is narrow enough that adding file-backed themes later does not require a TUI rewrite.

## Notes

- This should follow `pi-mono`'s tokenized approach, not its full schema size on day one.
- Layout and hierarchy should be stabilized first; otherwise the theme system will encode unstable surfaces.
