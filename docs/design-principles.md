# Design Principles

These principles translate the agent-first and harness-engineering lessons into concrete rules for `goose-go`.

Use this file when designing new features, changing architecture, or deciding what to defer.

## 1. The Repo Is the System of Record

- Important design intent must live in repo docs, not only in chat history.
- A new agent should be able to recover project goals, boundaries, and current state from the repo alone.
- When a decision changes how future work should be designed, update the relevant root docs in the same change.

## 2. Keep the Core Narrow

- Prefer a small terminal-core runtime over broad product-surface parity.
- Build only what is needed for the current milestone and defer the rest explicitly.
- Do not port upstream Goose or external reference projects feature-for-feature unless the behavior is required for the current target.

## 3. Make Architecture Executable

- Architecture documents are not enough on their own; the repo should fail fast when code violates package boundaries.
- Keep dependency direction strict and easy to inspect.
- Do not let provider, tool, storage, CLI, or agent code leak across boundaries for short-term convenience.

## 4. Normalize at Boundaries

- Internal runtime packages should operate on normalized domain types.
- Provider-specific request and response shapes stay inside provider implementations.
- Storage-specific row shapes and schema details stay inside storage implementations.
- Tool-specific input and output validation stays inside the tool boundary.

## 5. Optimize for Agent Legibility

- Favor clear package seams, typed data, and deterministic behavior over clever abstraction.
- Make failures diagnosable from repo artifacts, logs, and tests instead of relying on memory.
- Keep docs short, specific, and linked from the root so agents can find the right context quickly.

## 6. Build the Harness Alongside the Runtime

- Evals, smoke paths, and replayable artifacts are part of the product scaffolding, not optional polish.
- Every major runtime subsystem should eventually be exercised by repeatable tests or task evals.
- When a bug escapes, improve the harness so the same class of failure is easier to catch next time.

## 7. Prefer Explicit Deferral Over Silent Scope Creep

- If a feature is not part of the current milestone, record it in the backlog instead of leaving hidden TODOs.
- Keep v1 focused even when a broader product surface is technically possible.
- A deliberate omission is better than an accidental half-implementation.

## 8. Keep Design and Cleanup Continuous

- Agents will copy the patterns the repo already contains, including bad ones.
- Remove ambiguous structure early: duplicate helpers, drifting docs, oversized files, and mixed responsibilities should not be allowed to accumulate.
- When code becomes harder to reason about, fix the structure before adding more surface area.

## 9. Default to Boring, Inspectable Solutions

- Prefer simple files, explicit interfaces, and small helper packages over framework-heavy designs.
- Reuse existing local auth state, schemas, and contracts when they fit the milestone cleanly.
- Add complexity only when the simpler option creates a concrete correctness or maintenance problem.

## 10. Design for Fresh-Agent Handoffs

- Progress files should describe what is done, what is next, and what is intentionally deferred.
- Milestones should be decision-complete enough that a fresh agent can continue without hidden assumptions.
- If a future implementer would need prior chat context, the repo documentation is not complete enough.
