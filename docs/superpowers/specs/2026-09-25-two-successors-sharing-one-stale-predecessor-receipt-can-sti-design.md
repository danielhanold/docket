<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0453 — Two successors sharing one stale predecessor receipt can still free a live worktree slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0453-two-successors-sharing-one-stale-predecessor-receipt-can-sti.md)**
<!-- docket:backlink:end -->

# Successor stale-receipt must not rotate a live worktree slot — design

## Problem

`admitScopedWorktree` (`internal/gatedrive/driver.go`) rotates an executing same-scope worktree slot for any start that carries a predecessor receipt. It checks only that the slot is same-scope and `executing`; whether the receipt still names the scope's CURRENT drive is checked later, inside `reserveScopeDrive`, after the worktree admission step has already mutated the slot. ADR-0118 (change 0375) fixed that order: worktree admission is the outermost authority, and this change keeps it.

Traced interleaving on `main` (post-0452):

1. Successors S1 and S2 both present predecessor P's receipt; both pass `precheckScopedStart` before S1 retires P.
2. S1 completes admission: rotates P's executing slot to its token T1, `reserveScopeDrive` (scope current = S1), `retirePredecessor`, launch, `confirmScopeLaunch`, `ConfirmWorktreeExecution` (slot `executing`, T1).
3. S2 enters `admitScopedWorktree`: the reserve is busy, the slot is same-scope and `executing`, and S2 carries a receipt, so it calls `rotateWorktreeExecutionForSuccessor` and rotates S1's live slot to T2 (state `reserved`, S1's run identity cleared).
4. S2's `reserveScopeDrive` refuses `ErrStalePredecessor` (P is not current). `releasable` is true (rotated) and the error is not a same-scope race loss, so S2 calls `ReleaseWorktreeExecution(T2)`. The slot is now free while S1's gate is running, which breaks one-execution-per-worktree.

Change 0452 closed the receipt-less (first-start) version of this in the same branch of `admitScopedWorktree`; this is the successor-side sibling it recorded as a follow-up.

## Design

In `admitScopedWorktree`'s `admissionExecuting` case, after the existing receipt-less `ErrScopeSecondDrive` refusal and BEFORE `rotateWorktreeExecutionForSuccessor`:

- Load the scope record (`d.store.LoadScope(req.ScopeID)`).
- If the load fails, fail closed: return the error without touching the slot. A store/IO error from `LoadScope` is returned as-is; the function already fails closed on an unreadable slot, and this follows that pattern.
- If `scope.CurrentDriveID != req.PredecessorDriveID`, return `ownershipErr(ErrStalePredecessor, "start")` without touching the slot.
- Otherwise rotate exactly as today.

This is the same predicate `reserveScopeDrive` applies (`receipt.DriveID != rec.CurrentDriveID` → `ErrStalePredecessor`), evaluated earlier to guard the one mutating step that precedes it. `reserveScopeDrive` stays the authority for the scope slot; nothing about its checks, the admission order, `releasable`, or `isSameScopeRaceLoss` changes. No new field, lock, error kind, or store function.

### Why an unlocked scope read is sufficient

Ordering requirement: the scope read MUST happen after `LoadWorktreeExecution` returned the slot whose token will be passed to the rotation. Given that ordering:

- A slot that is `executing` under a successor S1's token is only written by S1's `ConfirmWorktreeExecution`, which S1 performs after its `reserveScopeDrive` set the scope's current drive to S1. The scope's current drive never moves back to an earlier drive. So a scope read that follows the slot read observes S1 (or later), never P, and the stale start is refused.
- If S2 loaded the slot while it still held P's token and read the scope while P was still current, and S1 then rotates the slot, S2's rotation presents P's old token. The existing token check in `rotateWorktreeExecutionForSuccessor` (`verifyAdmissionToken`) refuses that. `admitScopedWorktree` returns the rotation error with `rotated=false`, so `admitScoped` releases nothing.

The alternative, checking the predecessor drive's run id against `slot.RawRunID` inside the locked rotate CAS, was rejected: it changes the store function's signature and makes the drive record's run identity part of admission, for no safety gain over the argument above (YAGNI).

## Testing

Add a deterministic regression test beside `TestSameScopeFirstStartLateLoserDoesNotRotate` in `internal/gatedrive/driver_concurrency_test.go` (suggested name `TestSameScopeSuccessorStaleReceiptDoesNotRotate`):

1. A first start launches; drive it to a terminal PASSED result in the terminal-before-release window (the pattern `TestBarrierSuccessorUnderCancel` uses) so it is predecessor P.
2. Successor S1 with P's receipt starts through `d.Start`, launches, and leaves the worktree slot `executing` under S1's token.
3. Build a second request carrying the same P receipt and call `d.admitScopedWorktree` directly. This models S2, which passed its precheck before P was retired.
4. Assert the error is `ErrStalePredecessor`, and that the slot's state, `ReservationToken`, and `ExecutionGen` are unchanged from after step 2.

Mutation check: with the new comparison removed, the test must fail (the slot rotates). Existing successor coverage (`admission_successor_test.go`, `TestBarrierSuccessorUnderCancel`, and 0452's test) must stay green. Run the whole suite at the build gate.

## Out of scope

- Other admission or arbitration paths that don't involve receipt staleness during rotation.
- Reworking change 0452's receipt-less fix.
- Changing the admission order ADR-0118 fixed, or moving scope arbitration ahead of worktree admission.
