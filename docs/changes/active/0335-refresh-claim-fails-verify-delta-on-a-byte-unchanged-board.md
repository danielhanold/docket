---
id: 335
slug: refresh-claim-fails-verify-delta-on-a-byte-unchanged-board
title: refresh-claim fails verify-delta when the board is byte-unchanged
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-21
updated: '2026-08-22'
depends_on: []
stacked_on:
related: []
discovered_from: [330]
adrs: []
spec: docs/superpowers/specs/2026-08-21-refresh-claim-fails-verify-delta-on-a-byte-unchanged-board-design.md
plan: 'docs/superpowers/plans/2026-08-22-refresh-claim-fails-verify-delta-on-a-byte-unchanged-board.md'
results:
trivial: false
auto_groomable:
branch: 'feat/refresh-claim-fails-verify-delta-on-a-byte-unchanged-board'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-22T15:57:11Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-21-refresh-claim-fails-verify-delta-on-a-byte-unchanged-board-design.md` |
| Plan | `docs/superpowers/plans/2026-08-22-refresh-claim-fails-verify-delta-on-a-byte-unchanged-board.md` |
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

Fix docket's shared `change_claim` / `change refresh-claim` op (`changeClaimOp.Plan` in
`internal/app/change_claim.go`) so a `claimed_at` re-stamp that leaves the inline board
byte-identical does not declare the unchanged board path — and therefore does not trip
`verify-delta`. Concretely: replace the op's unconditional board `FileMutation` (appended via
`boardMutationKind`) with the **declare-only-when-changed** switch that two sibling ops —
`change_attach` and `change_reconcile` — already use verbatim: read the base-tree board blob and
declare the board mutation only when it is absent (create) or `!bytes.Equal` to the render
(replace). A byte-identical re-render is not declared, so the transaction applies cleanly. Both
claim and refresh share one `Plan`, so the fix covers both; the fresh-claim path (which flips a
board-visible status) is unaffected. Ships with a mutation-tested regression test in
`change_claim_test.go`. Design, precedent citations, and the assumptions audit trail are in the
linked spec.

## Out of scope

- Broader `verify-delta` redesign beyond this no-op-board case.
- Changing when or how often claim leases are re-stamped.
- Generalizing the byte-compare switch to `change_groom` (a plausible latent carrier) or the other
  `boardMutationKind` callers, or extracting a shared helper — recommended follow-ups, not this
  change.

## Open questions

- ~~Should the fix drop the board from the declared set when its render is byte-identical, or should
  `verify-delta` tolerate a declared-but-unchanged derived-view path?~~ **Resolved (grooming):** drop
  the board from the declared set — narrower, matches the "derived view, commit only if changed"
  contract, and is the established in-repo precedent (`change_attach`, `change_reconcile`).
  Relaxing the global `verify-delta` safety predicate for one op is rejected.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-22

2026-08-22 — Reconciled against current `main`. Root cause confirmed live: `internal/app/change_claim.go` `changeClaimOp.Plan` (shared by claim + refresh-claim) still appends the board `FileMutation` unconditionally via `boardMutationKind` (line ~559), which never byte-compares the render. The declare-only-when-changed precedent the spec cites is present and current in both `change_reconcile.go` (lines ~478-504, imports `bytes`) and `change_attach.go`. Spec matches reality with no scope drift; `depends_on: []` still empty. Proceeding to plan + build as specified.
