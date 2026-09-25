---
id: 457
slug: 'a-freshly-reserved-successor-on-an-epoch-less-scope-can-stil'
title: 'A freshly reserved successor on an epoch-less scope can still release a slot a later drive adopted'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-09-25'
updated: '2026-09-25'
depends_on: []
stacked_on:
related: [453]
discovered_from: [453]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0453 stopped a successor start holding a stale predecessor receipt from rotating and then releasing a live worktree slot, and made a start that loses with ErrStalePredecessor keep its reservation when the scope shows a sibling adopted it (siblingMayHoldReservation in internal/gatedrive/driver.go). A narrower ordering is still open, suspected from code reading and not reproduced. It needs (1) an epoch-less scope, where admissions are not serialized under the epoch lock; (2) a successor start that freshly reserved a released slot rather than rotating an executing one; and (3) a stall long enough for the scope to move two drives past the start's receipt. In that window a new adopter can take the start's token between siblingMayHoldReservation's unlocked check and the ReleaseWorktreeExecution call in admitScoped's reserveScopeDrive failure leg. The release then frees a live slot, the same one-gate-per-worktree breach 0453 fixed, in a much narrower window. Epoch-backed runs (implement-next armed through run.gate-before) serialize admission and are not exposed. In practice this needs duplicated or retried successor starts on a standalone gate drive, so the priority is low.

## What changes

Make the release in admitScoped's reserveScopeDrive failure leg conditional under the lock that serializes adoption, so the 'was this token adopted?' check and the release are one atomic step instead of an unlocked read followed by a separate write. Candidate shape: a store-level release-unless-adopted (compare-and-release against the scope's current drive AdmissionToken and PriorDriveID) evaluated inside the same critical section as ReleaseWorktreeExecution. Before fixing, add a deterministic reproduction using the scopedAdmissionHook test seam from 0453 (a fresh-reserve successor stalled while the scope advances two drives and a later drive adopts its token), and confirm it fails against current main.

## Out of scope

Epoch-backed scopes, which are already serialized. Rotated successor starts, which 0453 covers. Recovering a leaked reserved slot left by siblingMayHoldReservation's fail-closed keep, unless the atomic release makes it trivial to address together. Any change to reserveScopeDrive's ordered refusal predicate (scopeReserveRefusal).
