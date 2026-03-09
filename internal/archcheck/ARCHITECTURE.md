# Architecture Check Architecture

`internal/archcheck` turns architecture boundary rules into executable repository checks.

Its purpose is simple: architecture docs should not be aspirational only. The repo should be able to fail when import directions drift.

## Code Map

- package loader
  Collects Go package import data from `go list`.
- rule set
  Encodes forbidden dependency directions between runtime layers.
- violation finder
  Matches loaded package imports against the declared rules and returns human-readable failures.

## Check Flow

```mermaid
flowchart LR
    A["go list -json ./..."] --> B["loaded package imports"]
    B --> C["archcheck rules"]
    C --> D["FindViolations"]
    D --> E["cmd/archcheck"]
    E --> F["CI / make check / local runs"]
```

## Boundaries

- `internal/archcheck` owns mechanical dependency checks, not broader repo hygiene
- the rule set should encode architectural intent already documented elsewhere in the repo
- package-specific exception logic should stay rare and explicit

## Cross-Cutting Concerns

- doc/code alignment: boundary rules should reflect the root architecture docs so drift is visible quickly
- production isolation: the checker deliberately keeps `cmd/*` and `internal/evals` off production dependency paths
- maintainability: rules should stay legible enough that a fresh agent can extend them without reverse-engineering hidden policy

## Current Constraints

- the rule set is still intentionally narrower than the full intended architecture
- deeper semantic checks belong in future expansions, not in ad hoc import shortcuts
