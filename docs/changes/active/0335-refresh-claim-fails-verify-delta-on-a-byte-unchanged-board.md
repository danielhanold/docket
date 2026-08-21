---
id: 335
slug: refresh-claim-fails-verify-delta-on-a-byte-unchanged-board
title: refresh-claim fails verify-delta when the board is byte-unchanged
status: proposed
priority: medium
type: fix
created: 2026-08-21
updated: 2026-08-21
depends_on: []
stacked_on:
related: []
discovered_from: [330]
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

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`docket change refresh-claim` fails its `verify-delta` guard whenever its pure `claimed_at`
re-stamp leaves the inline board byte-identical: it declares the (unchanged) board path as
part of the transaction, and the engine refuses with
`verify-delta: "a declared path is not an actual change"`. The net effect is that the
optional plan→build lease re-stamps get skipped — a live agent that wanted to refresh its
claim lease cannot, purely because its re-stamp happened to leave the board bytes unchanged.

Observed 2026-08-21 during the change-0330 implement-next run. It did not affect that run's
correctness (the claim lease was already fresh and downstream transactions changed
link-bearing fields anyway), but it is a real defect in the `change_claim` op: a no-op board
re-stamp should never make the transaction declare a path that has no delta.

## What changes

Fix docket's `change_claim` / refresh-claim op so a `claimed_at` re-stamp that leaves the
board byte-unchanged does not declare the unchanged board path (and therefore does not trip
`verify-delta`). The board is a derived view; declaring it only when it actually changed is
the intended contract.

## Out of scope

- Broader `verify-delta` redesign beyond this no-op-board case.
- Changing when or how often claim leases are re-stamped.

## Open questions

- Should the fix drop the board from the declared set when its render is byte-identical, or
  should `verify-delta` tolerate a declared-but-unchanged derived-view path? (The former is
  narrower and matches the "derived view, commit only if changed" rule elsewhere.)

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
