<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0435 — docket run cancel leaves a stale RunEpochID on a released gate-admission slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0435-docket-run-cancel-leaves-a-stale-runepochid-on-a-released-ga.md)**
<!-- docket:backlink:end -->

# docket run cancel leaves a stale RunEpochID on a released gate-admission slot

## Problem

A gate-admission slot record (`internal/gatedrive/admission.go:105-131`, `admissionRecord`) carries
a `RunEpochID` field that identifies which dispatched run currently owns a worktree's gate. Two
fence checks refuse admission on any non-empty `RunEpochID` that doesn't match the caller's own
epoch, **regardless of the slot's `State`**:

- `internal/gatedrive/admission.go:245`, inside `reserveWorktreeExecution`
- `internal/app/gate.go:244`, `rawStaleEpochRefusal`

This State-agnostic behavior is intentional (change 0375 Task 9, covered by
`TestEpochOmissionCannotDetachOwnedWorktree`, `internal/gatedrive/epoch_test.go:55-103`): a
gate-driven run works in slices, releasing the slot briefly between them, and the fence must keep
protecting that live run's between-slice gap — not just its actively-executing window. Weakening
this check to ignore `released` slots would reopen exactly the race that test was written to
close: a different run grabbing the worktree mid-drive, between two slices of a still-live run.

The bug is on the other side of that same design: `docket run cancel`
(`internal/app/rungate_cancel.go`) is the sanctioned way to declare an epoch dead. Its own doc
comment says cancellation "touches only the epoch record and the worktree admission slot," but its
slot reconciliation (`reconcileWorktreeSlot`) treats an already-`released` slot as fully accounted
and does nothing further, and `ReleaseWorktreeExecution` (`admission.go:429-439`) only ever flips
`State` and `UpdatedAt` — neither path ever clears `RunEpochID`. Once an epoch is cancelled while
its slot is (or becomes) `released`, its `RunEpochID` fingerprint is permanently stuck on the slot.
The fence can no longer distinguish "released, but a live run will resume the next slice any
second" (must stay protected) from "released, and the owning run is provably dead" (safe to
re-admit) — both look identical on disk.

No other existing mechanism clears this field:

- `docket gate recover --root <dir>` (`internal/app/gate.go:417` → `internal/process/recover.go:59`)
  only understands raw run-slot directories (locks/manifests under a run-root); every
  `gate-admission/v1/<hash>` entry it's pointed at comes back `"foreign"` / `"not a run slot; left
  untouched"` — confirmed by direct invocation. It has no knowledge of the `gatedrive` admission
  schema at all.
- `docket gate cleanup` (`internal/app/finalize_cleanup.go:709`) only removes one exact terminal
  run directory's logs — also raw-run-directory scoped.
- Legacy drive inventory (`internal/gatedrive/history.go`, `inventoryLegacyDrives`) only runs once,
  on a worktree's very first-ever reservation, and assesses the `gate-drives` schema, not admission
  slots.

Net effect: once hit, the only "fix" is a hand-edit of `record.json` — which this repo's own rules
say durable gate state must never receive. This exact case blocked finalizing change 434 (PR #312):
the epoch `13d9c78f54853811831e415dafe6dc64` (run key
`implement-next-20260918t182257z-6946-e48f`) was cleanly cancelled via `docket run cancel`, but the
worktree's admission slot kept that `RunEpochID` with `State: released`, permanently refusing the
finalize gate with `stale-run-epoch` until a human explicitly authorized a one-off manual edit to
unblock it. No test anywhere exercises this exact sequence — `internal/app/rungate_cancel_test.go`
has thorough cancel-path coverage (happy path, already-cancelled, wrong epoch, fencing before
stopping, repeat/resume, never-charges-or-resets) but nothing asserts on the admission slot's
`RunEpochID` after cancel, nor exercises "cancel an epoch owning a released slot, then attempt
admission of a new, different-epoch reservation on the same worktree."

**Second, independent occurrence — build path, not finalize.** The same defect recurred hours
later on change 0368, on the *build* side of the gate rather than finalize. A human ran
`docket run cancel` on 0368's in-flight `docket-implement-next` dispatch (epoch
`523ddc4c4a7a1c715ce105d669165749`, run key `implement-next-20260918t192652z-58216-ef81`) to pause
the run for session limits; `run cancel` reported clean `cancelled`. On resume,
`docket run gate-before implement-next --resume 368` correctly armed a fresh reserved-replacement
epoch (`6f79c99d5888658da5f166107ba640c4`). But the resumed `docket-implement-next` agent's own
`docket gate drive start --owner build` call refused that new epoch with `stale-run-epoch: "an
in-flight run owns this worktree; present that run's epoch or cancel it"` — the worktree's
gate-admission record still named the superseded, cancelled predecessor epoch `523ddc4c...` with
`State: released`, never rebound or cleared. The drive only started once the agent presented the
stale predecessor epoch `523ddc4c...` directly (which the fence still accepted, since it matched
the value stuck on the slot) instead of the newly-armed replacement epoch — a working manual
bypass of the same class of bug, distinct from 434's hand-edit-of-`record.json` recovery, but
converging on the identical root cause: `run cancel` never clears `RunEpochID` on the slot it
releases. Two independent occurrences on the same day, one on each side of the gate (finalize vs.
build/resume), both pointing at the same fix in `run cancel`'s worktree-slot reconciliation.

## Decision

Fix `docket run cancel`'s worktree-slot reconciliation (`reconcileWorktreeSlot` and the paths that
call `ReleaseWorktreeExecution`, in `internal/app/rungate_cancel.go` /
`internal/gatedrive/admission.go`) so that when cancel tears down the epoch owning a worktree's
admission slot, it also clears that slot's `RunEpochID` — in both the "already released before
cancel ran" case and the "released as part of this cancel" case. Clearing the field lets the
existing fence checks work exactly as designed: an empty `RunEpochID` naturally admits the next
reservation, with zero change to `admission.go:245` or `gate.go:244` and zero weakening of the
live-run between-slice protection those checks exist for.

Explicitly rejected alternatives (see `## Out of scope` on the change record for the authoritative
list): weakening the fence itself to ignore `released` state, and extending `docket gate recover`
to cover gate-admission slots as an alternative recovery path. Both were considered and set aside
in favor of fixing the actual root cause in `run cancel`.

## What changes

- `reconcileWorktreeSlot` (and/or `ReleaseWorktreeExecution`, whichever is the more correct
  ownership boundary — the implementer picks based on which call sites need the field cleared)
  clears `RunEpochID` on the slot record whenever cancel determines the slot belongs to the epoch
  being cancelled, regardless of whether the slot's `State` was already `released` or becomes
  `released` as part of this cancel call.
- Preserve every other field cancel currently leaves alone (`DriveID`, `RawRunID`, etc. stay
  historical evidence per the package's existing release semantics) — only `RunEpochID` is cleared,
  and only when it matches the epoch being cancelled (never a different, still-live epoch's
  `RunEpochID` on the same slot, if that's even representable).
- New regression test(s) in `internal/app/rungate_cancel_test.go` (and/or
  `internal/gatedrive/epoch_test.go` if the fix lands at that layer) covering exactly the reported
  gap: cancel an epoch owning an already-`released` slot, then attempt a fresh reservation with a
  different epoch on the same worktree, and assert it succeeds (previously refused
  `ErrStaleRunEpoch` / `stale-run-epoch`).
- No change to `admission.go:245`, `gate.go:244`, or the takeover epoch fence
  (`internal/gatedrive/takeover.go:88-99`).

## Out of scope

- Weakening the admission fence's State-agnostic `RunEpochID` check itself — that protects a live
  run's between-slice gap and removing it reopens a real race (see `## Problem`).
- Extending `docket gate recover` to also cover gate-admission slots.
- Any change to the takeover epoch fence.
