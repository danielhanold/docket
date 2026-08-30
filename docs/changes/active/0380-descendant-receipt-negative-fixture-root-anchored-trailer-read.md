---
id: 380
slug: 'descendant-receipt-negative-fixture-root-anchored-trailer-read'
title: 'Add a descendant-receipt negative fixture pinning the root-anchored trailer read'
status: proposed
priority: medium
type: chore
created: '2026-08-30'
updated: '2026-08-30'
depends_on: []
stacked_on:
related: [378]
discovered_from: [378]
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

Change 0378's metadata-ownership verifier reads its native `OpInitRoot`/`OpMigrateSeed` receipt
trailers strictly at the sole parentless seed root. Nothing currently pins that anchoring: a
regression that read a trailer from a *descendant* commit instead would go uncaught. Both the
mutation audit and the deep reviewer flagged the gap, and the fixture that would close it was
authored then reverted along with 0378's other non-blocker fixes when the fix-loop gate reddened.
It should be added deliberately so the root-anchored read is a proven invariant, not an implicit one.

## What changes

Add a negative test fixture placing a receipt trailer on a descendant commit (not the seed root)
and assert the verifier does NOT treat it as an ownership proof — pinning that the trailer read is
anchored at the parentless root. Cherry-pick the reverted 0378 fixture as the starting point.

## Out of scope

- Changing the verifier's behavior — this is test coverage only; the behavior is already correct.
- The other 0378 follow-ups (SHA-256 width fix; internal/process flake) — separate changes.

## Open questions

<!-- None yet — resolve exact fixture shape and placement during reconcile. -->

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
