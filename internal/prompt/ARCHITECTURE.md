# Prompt Architecture

`internal/prompt` owns runtime prompt composition for `goose-go`.

It currently owns:

- the base run-system prompt
- eager `AGENTS.md` discovery from the working directory
- ancestor walking up to the git root
- prompt composition that appends discovered `AGENTS.md` files as project context

It does not own:

- provider request translation
- session persistence
- `.goosehints` compatibility
- configurable context-file names
- `@file` expansion inside context files

## Package Position

`internal/app` should call into `internal/prompt` when constructing runtime configuration.

`internal/agent` and `internal/provider` should continue to treat the final system prompt as an opaque string.

## Current Behavior

- only `AGENTS.md` is loaded
- if the working directory is inside a git repo, discovery walks from git root to the working directory
- if no git root is found, only the working directory is checked
- files are appended outermost to innermost under a `Project Context` section
- unreadable paths are skipped so prompt construction stays non-fatal

## Deferred

- global hint files
- `.goosehints`
- `CONTEXT_FILE_NAMES`
- file-reference expansion
