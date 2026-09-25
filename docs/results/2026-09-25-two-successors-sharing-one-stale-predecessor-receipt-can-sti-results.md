# Two successors sharing one stale predecessor receipt can still free a live worktree slot — Results

**Human action:** No action is required before merge. One optional item is worth a look: a narrow leftover race on freshly reserved successor starts (see Known issues).

## Outcome

Before this change, a gate start that presented an out-of-date predecessor receipt could still take over a worktree slot that another start was actively running a test gate in. It then failed its own scope check and released the slot, so a third start could launch a second gate in the same worktree. That breaks the rule that one worktree runs one gate at a time.

What changed, all in `internal/gatedrive`:

- **Refuse before rotating.** Before rotating an executing same-scope slot, admission now loads the scope record and runs the same ordered checks the scope reservation uses. That covers capability, closed scope, half-filled receipt, busy, pending acknowledgement and staleness. Any refusal leaves the slot untouched. If the scope record can't be read, admission fails closed. The checks now live in one shared helper, `scopeReserveRefusal`, which both call sites use, so the two can't drift apart. That also answers the review's minor finding: callers now see the same typed refusal from both paths.
- **Don't free a slot a sibling adopted (review finding, important).** The review found a second interleaving the design didn't cover. It needs admissions that aren't serialized by the run-epoch lock, which means epoch-less scopes. In it, a stale start rotates first, a sibling adopts its reservation and launches, and then the stale start's cleanup releases the sibling's live slot. A successor that loses with `ErrStalePredecessor` now keeps its reservation when the reloaded scope shows a sibling consumed the same receipt, or when the current drive holds this token. An unreadable record also keeps it. Keeping a reserved slot is the fail-closed direction.
- An existing test (`TestSuccessorAdmissionFailureLegsReleaseRotatedSlot`) had asserted the old rotate-then-release behavior on a stale receipt. It was revised to assert the new refusal. Its original purpose, releasing a rotated slot when admission fails after rotation, is still exercised through a scope closed after the rotation by a test-only hook.

## Verification performed

- Every new test was confirmed red against the unfixed code before the fix went in: the stale-receipt rotation test, the fail-closed scope read, the sibling-adopted reservation test (4 subtests, each condition mutation-checked separately), and the whole-predicate guard test (6 subtests).
- `go test -count=1 ./internal/gatedrive/` passed after each commit. The relevant subsets also passed under `-race`.
- The full suite (`go run ./cmd/docket development test`) passed at the build head before review: 54/54 files. The build-evidence block in the PR records the full-suite result for the final head.

## Known issues and follow-ups

### Leftover check-then-release race for freshly reserved successors

This needs an epoch-less scope, a successor start that reserved a released slot fresh (not by rotation), and a stall long enough for the scope to move two drives past the receipt. In that window, a new adopter can take its token between the start's check and its release. The effect would be the same one-gate-per-worktree breach, but the window is much narrower than the bug this change fixes. Rotated starts are not affected. The defect is suspected from code reading and has not been reproduced. Suggested next step: a human decides whether to capture a follow-up change that makes the release conditional under the scope lock.
