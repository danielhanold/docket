<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0452 — Same-scope first-start loser must not rotate the winner's executing worktree slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-24-0452-same-scope-first-start-loser-must-not-rotate-the-winner-s-ex.md)**
<!-- docket:backlink:end -->
# Same-scope first-start loser must not rotate the winner's executing worktree slot — Results

**Human action:** None required beyond the normal PR review; optionally capture the follow-up below as a new change. The fix is one guard plus a deterministic regression test, and the flaky CI test it addresses now passes 500 of 500 runs on a single core.

## Outcome

`TestBarrierSameScopeFirstStartContention` failed intermittently on GitHub Actions runners (and about half the time locally with `GOMAXPROCS=1`). The cause was a real admission bug: when two first starts for the same scope raced and the loser arrived after the winner had already marked the worktree slot `executing`, the loser rotated the winner's slot back to `reserved` under its own token. The winner's live run was then left under a slot it could no longer release.

Now only a start that carries a predecessor receipt (a genuine successor) may rotate an executing slot. A receipt-less first start that finds the slot already executing is refused with the typed `ErrScopeSecondDrive` error and the slot is left untouched. The `admitScopedWorktree` doc comment now says rotation is successor-only.

## Verification performed

- New deterministic test `TestSameScopeFirstStartLateLoserDoesNotRotate` (late loser admits only after the winner is executing). Mutation check: run against the code without the guard, it failed at the refusal assert; with the guard it passes.
- `GOMAXPROCS=1 -race`, 500 runs each (two drives of 250) of `TestBarrierSameScopeFirstStartContention` and the new test: all passed.
- Whole `internal/gatedrive` package under `-race`: passed.
- Full repository suite via the build gate at the final head (recorded as build evidence in the PR body).
- Whole-branch review (standard rung): one minor finding (doc comment not re-wrapped) fixed in 3417e9a8; one important finding reported as follow-up (below).

## Known issues and follow-ups

### Two successors sharing one stale receipt can still free a live slot

This is suspected from code reading, not reproduced. It predates this change, which does not touch that path. Two successor starts can present the same predecessor receipt. If one of them launches and marks the worktree slot executing, the other can still rotate that slot to its own token. The scope check then refuses the second start (`ErrStalePredecessor`), and its cleanup releases the slot while the first successor's run is still live. After that, another start could take the worktree while a gate is running in it. This is the same late-loser pattern this change fixes, but on the successor side. There is no workaround. Suggested next step: capture a follow-up change with `docket change create`. The fix would check, before rotating, that the receipt names the scope's current drive, and would refuse with `ErrStalePredecessor` without touching the slot otherwise. It needs a deterministic two-successor test modeled on `TestSameScopeFirstStartLateLoserDoesNotRotate`.
