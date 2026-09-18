---
id: 435
slug: 'docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga'
title: 'docket run cancel leaves a stale RunEpochID on a released gate-admission slot'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-18'
updated: '2026-09-18'
depends_on: []
stacked_on:
related: [413, 427]
discovered_from: [434]
adrs: []
spec: 'docs/superpowers/specs/2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-18-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga-design.md) |
<!-- docket:artifacts:end -->

## Why

Once a dispatched run's epoch is cancelled via `docket run cancel`, the worktree's gate-admission slot keeps the cancelled epoch's RunEpochID forever. The admission fence (internal/gatedrive/admission.go and internal/app/gate.go) deliberately refuses on any non-empty RunEpochID mismatch even when the slot's State is `released`, by design, to protect a live run's between-slice gaps. But `run cancel`'s slot reconciliation (internal/app/rungate_cancel.go) treats an already-released slot as fully accounted and never clears RunEpochID, and ReleaseWorktreeExecution never clears it either. Net effect: that worktree's gate is permanently fenced with `stale-run-epoch` and no CLI-level recovery exists (docket gate recover only covers raw run-slot directories, not gate-admission/v1 records). This was hit finalizing change 434 (PR #312) and only unblocked via a manual, human-approved one-off hand-edit of record.json, which is explicitly against this repo's rule that durable gate state must never be hand-edited.

**Second, independent occurrence (build path, not finalize):** the same defect recurred hours later on change 0368, on the *build* side of the gate rather than finalize. A human ran `docket run cancel` on 0368's in-flight `docket-implement-next` dispatch (epoch `523ddc4c4a7a1c715ce105d669165749`, run key `implement-next-20260918t192652z-58216-ef81`) to pause the run for session limits; `run cancel` reported clean `cancelled`. On resume, `docket run gate-before implement-next --resume 368` correctly armed a fresh reserved-replacement epoch (`6f79c99d5888658da5f166107ba640c4`). But the resumed docket-implement-next agent's own `docket gate drive start --owner build` call refused that new epoch with `stale-run-epoch: "an in-flight run owns this worktree; present that run's epoch or cancel it"` — the worktree's gate-admission record still named the superseded, cancelled predecessor epoch `523ddc4c...` with `State: released`, never rebound or cleared. The drive only started once the agent presented the stale predecessor epoch `523ddc4c...` directly (which the fence still accepted since it matched the value stuck on the slot) instead of the newly-armed replacement epoch — a working manual bypass of the same class of bug, distinct from 434's hand-edit-of-record.json recovery, but converging on the identical root cause: `run cancel` never clears `RunEpochID` on the slot it releases. Two independent occurrences on the same day, one on each side of the gate (finalize vs. build/resume), both pointing at the same fix in `run cancel`'s worktree-slot reconciliation.

## What changes

Fix `docket run cancel`'s worktree-slot reconciliation so that when it cancels the epoch owning a worktree's admission slot, it also clears that slot's RunEpochID (in both the already-released case and the release-during-cancel case), so the fence naturally re-admits future reservations. Add a regression test exercising exactly the reported gap: cancel an epoch owning a released slot, then attempt admission of a new, different-epoch reservation on the same worktree, and assert it succeeds.

## Out of scope

Do not weaken the admission fence's State-agnostic RunEpochID check itself (internal/gatedrive/admission.go:245, internal/app/gate.go:244) — that check is intentional (change 0375 Task 9) and protects a live run's between-slice gaps; removing it would reopen a real race. Do not extend `docket gate recover` to cover gate-admission slots as an alternative/additional recovery path — out of scope for this change, could be proposed separately later. No change to the takeover epoch fence (internal/gatedrive/takeover.go).
