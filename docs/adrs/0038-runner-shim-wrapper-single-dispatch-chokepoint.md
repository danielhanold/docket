---
id: 38
slug: runner-shim-wrapper-single-dispatch-chokepoint
title: Runner delegation rides a generated shim wrapper body, not per-skill dispatch branching
status: Accepted
date: 2026-07-15
supersedes: []
reverses: []
relates_to: [12, 15, 20, 24, 37]
change: 79
---

## Context

Given the explicit `runner:` switch (ADR-0037), the mechanism question remains: WHERE does
a delegated agent's run get rerouted to the child harness? Every docket agent invocation —
Claude Code's `context: fork` self-dispatch, an explicit `@docket-<name>` dispatch, and the
composition dispatches other skills make (implement-next → status/adr, auto-groom → critic)
— already lands on the generated wrapper file, whose body normally instructs executing the
skill. The alternative considered was skill-body branching: teach each skill's dispatch
sites to check the resolved runner and call the child harness themselves. That duplicates
delegation prose across seven skills, drifts, and breaks the agent layer's "wrappers own
the pin" rule (change 0016) — a skill body would suddenly own routing decisions the wrapper
was built to encapsulate.

## Decision

When an agent resolves a `runner:`, `sync-agents.sh` generates the SAME wrapper file with
the SAME frontmatter (name, description, and the `model:` line kept for bookkeeping — the
effective pin is the baked argument) but swaps the BODY for a runner-parameterized **shim**:
one foreground Bash call to the dispatch facade —
`docket.sh runner-dispatch --runner <name> --agent <agent> [--model <m>] [--effort <e>]` —
followed by relay-and-verify rules (relay the child's final message; verify the child's
contract exactly as a native caller would; abort-and-report on nonzero; never fall back to
running the skill inline). One emission chokepoint (`emit_wrapper`) feeds both generation
passes AND `--check` leg (c), so shim staleness is detected by the same drift gate as any
generated file. Deterministic mechanics live in scripts with co-located contracts
(`runner-dispatch.sh` facade, `scripts/runners/<name>.sh` adapters) per ADR-0012's
script-vs-model boundary; the shim text is one template in the runner registry, never
runner-specific prose.

## Consequences

- Every invocation path inherits delegation unchanged and unknowingly — no skill body knows
  runners exist, and un-delegating an agent is a config flip + resync.
- The wrapper stays the single place a pin (model, effort, and now execution venue) is
  enforced; ADR-0020's machine-local regime applies to shims unchanged.
- The shim's frontmatter `skills:` preload still injects the skill content into the parent-
  side shim agent even though the child re-loads it — small context waste, accepted to keep
  frontmatter byte-stable with the native wrapper shape.
- A shim is one more indirection when debugging a delegated run: the parent transcript
  shows only the facade call and the relayed final message; child-side detail lives in the
  child harness's own session artifacts.
