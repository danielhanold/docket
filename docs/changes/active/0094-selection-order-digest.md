---
id: 94
slug: selection-order-digest
title: Selection-order backlog digest — implement-next selects from a digest instead of walking active/
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-19
depends_on: []
related: [69, 85, 88, 93]
adrs: [12]
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` Step 1 selects by having the **model** walk `active/`: read every change
file's frontmatter, filter to build-ready, rank by priority → created → id. That is a per-run cost
that grows with the backlog, and the ranking is model-executed rather than deterministic.

**#0088 (merged 2026-07-18) turned that from a once-per-invocation cost into a per-iteration one.**
Its loop-continuation contract is driver-agnostic prose, and `/loop docket-implement-next` — the
recommended driver, confirmed working — re-forks the skill per iteration with **fresh context**. So
draining N changes now re-walks `active/` N times. 0088 shipped no digest consumption of any kind;
Step 1 is byte-unchanged in how it acquires state.

This change gives that walk a deterministic, single-read replacement. It is deliberately **not** the
original "docket-prime" framing, which is dead: #0069 already ships the stdout-only
`render-board.sh --format digest` (backlog rollups + one line per active change), and #0093 shipped
archive decay. What survives is the narrow, genuinely missing piece — **selection order** — plus the
plumbing needed for anything to actually consume it.

Two blockers found while re-scoping, which is why this is not a pure one-script extension:

- **`render-board` is not reachable from a skill.** The `docket.sh` facade exposes `preflight env
  bootstrap docket-status board-refresh archive-change …` — no `render-board`. The digest exists
  only as internal report output inside `docket-status.sh`.
- **The one path that emits it, `docket-status --board-only`, also commits and pushes `BOARD.md`.**
  A selection read must not be a write.

Without addressing those, a selection-order queue ships with no way to reach it.

## What changes

1. **Selection-order build-ready queue** in `render-board.sh --format digest` — the build-ready set
   emitted in the order `implement-next` selects by (priority → created → id), where today's digest
   is id-ascending. Static frontmatter fields only, so determinism and the golden byte-compare hold.
2. **Claim-age signal** — the in-progress `updated:` / `claimed_at:` value carried in the digest as
   the **raw date**, never a computed "N days stale" (a wall-clock read would break `render-board`'s
   determinism).
3. **A read-only entry point** so a skill can obtain the digest without a write: either expose
   `render-board` in the `docket.sh` facade, or a `docket-status` flag that emits the digest and
   skips the board write. Shape settled in brainstorm.
4. **Rewire `docket-implement-next` Step 1** to select from the queue instead of walking `active/`,
   with the change files staying authoritative for the change it actually operates on (digest for
   orientation, file reads for action). This is the entire payoff — items 1–3 have no consumer
   without it.

Net token cost of items 1–2 must stay at or near zero per digest: the queue reorders and annotates
lines the digest already emits rather than adding a new section.

## Out of scope

- **ADR index titles and learnings-index hooks in the digest** — they require reading sources
  `render-board` must not own (ADR-0012) and would *add* per-digest tokens, contradicting the
  purpose. Explicitly dropped at the 2026-07-17 groom and still dropped.
- Replacing `BOARD.md` (the human-facing board stays; this is the agent-facing projection).
- A new committed or cached surface — the digest stays stdout-only and always-fresh.
- Semantic/embedding relevance ranking of anything.
- Rewiring skills other than `docket-implement-next`; whether the interactive skills or
  `docket-status`'s own report adopt the queue is a follow-up, not this change.
- Changing *what* any skill does with the state — only how it acquires it.

## Open questions

- Item 3's shape: expose `render-board` through the `docket.sh` facade, or add a write-free flag to
  `docket-status`? The facade route is thinner but widens the supported-operations surface; the
  flag route keeps one entry point but adds a mode to an already multi-mode script.
- Does the queue reorder the existing `change …` lines in place, or ride as a distinct
  `ready <n> <id> …` line set? In-place keeps the line count flat but changes an existing contract
  #0069's consumers may depend on.
- How much of Step 1's prose survives — does the skill still describe the ranking (as the
  authoritative definition the script implements), or defer entirely to the digest's order?
- Does the claim-age date belong in this change at all, or is it a separable nicety with no named
  consumer yet? (Same "ships with no adopter" trap that deferred the original.)

## Reconcile log
