# TUI Theme Architecture

## Role

`internal/tui/theme` provides the semantic color system for the Bubble Tea frontend.

It keeps color and presentation tokens out of the main TUI reducer and renderer files.

## Current Scope

- built-in `dark` and `light` themes
- semantic tokens for header, footer, status, transcript, panels, and tool blocks
- startup theme selection via `goose-go tui --theme <name>`
- in-TUI theme switching through `/theme`

## Design Rules

- TUI components depend on semantic tokens, not raw color literals
- theme state affects only presentation, not runtime/provider behavior
- built-in themes are the current system of record; file-backed custom themes are still planned

## Flow

```mermaid
flowchart LR
    A["cmd/goose-go tui --theme <name>"] --> B["theme.Resolve(name)"]
    B --> C["built-in palette"]
    C --> D["internal/tui model"]
    D --> E["header / transcript / panels / footer"]
```

## Next Steps

- add file-backed custom theme loading
- add validation for custom themes
- add hot reload for the active custom theme
