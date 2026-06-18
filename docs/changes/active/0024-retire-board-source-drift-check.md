---
id: 24
slug: retire-board-source-drift-check
title: Retire or downgrade the inline board/source-drift health check once rendering is deterministic
status: proposed
priority: low
created: 2026-06-18
updated: 2026-06-18
depends_on: [22]
related: [23]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Why

The **board/source-drift** health check exists because the `inline` board is
rendered by the model: a writer skill could regenerate `BOARD.md` inconsistently
with the change files, so `docket-status` re-renders in-memory and warns on any
disagreement. Once change 0022 makes `inline` rendering **deterministic** (a
script that emits byte-identical output from the same change files), that whole
failure class for `inline` largely disappears — a script cannot "render the board
wrong" the way a model can.

That leaves a question worth its own decision (spun out of 0023): does the
`inline` board/source-drift check still earn its keep, and if so in what reduced
form?

## What changes

To be decided at brainstorm, after 0022 lands. The candidate shapes:

- **Retire** the `inline` drift check entirely — rendering is now a pure function
  of the change files, run by the script, so there is nothing to drift.
- **Downgrade** it to a narrower "a writer skipped the mandatory board-refresh
  invariant" check: detect that a `status:` write landed without the
  corresponding board-refresh commit (the committed `BOARD.md` is stale relative
  to the change files), rather than re-rendering to compare byte-for-byte.
- Leave the **`github`** surface's drift/visibility flag untouched — that surface
  is best-effort and self-healing and is not affected by 0022.

Whichever is chosen, update `docket-status`'s Health-checks section and the
convention accordingly.

## Out of scope

- The board render script itself (change 0022) and the sweep/health-check
  scripting decision (change 0023).
- The `github`-surface mirror-reachability flag.

## Open questions

- Is a "stale-`BOARD.md`-vs-change-files" staleness check still useful once a
  deterministic script renders the board, or does the must-land board-refresh
  commit discipline already guarantee freshness?
- Does retiring the check need an ADR, or is it a convention edit?

## Reconcile log
