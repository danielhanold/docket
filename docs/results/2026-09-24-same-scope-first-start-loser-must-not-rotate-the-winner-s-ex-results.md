<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0452 — Same-scope first-start loser must not rotate the winner's executing worktree slot](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0452-same-scope-first-start-loser-must-not-rotate-the-winner-s-ex.md)**
<!-- docket:backlink:end -->
# Same-scope first-start loser must not rotate the winner's executing worktree slot — Results

**Human action:** None required beyond the normal PR review. The fix is one guard plus a deterministic regression test, and the flaky CI test it addresses now passes 500 of 500 runs on a single core.

## Outcome

`TestBarrierSameScopeFirstStartContention` failed intermittently on GitHub Actions runners (and about half the time locally with `GOMAXPROCS=1`). The cause was a real admission bug: when two first starts for the same scope raced and the loser arrived after the winner had already marked the worktree slot `executing`, the loser rotated the winner's slot back to `reserved` under its own token. The winner's live run was then left under a slot it could no longer release.

Now only a start that carries a predecessor receipt (a genuine successor) may rotate an executing slot. A receipt-less first start that finds the slot already executing is refused with the typed `ErrScopeSecondDrive` error and the slot is left untouched. The `admitScopedWorktree` doc comment now says rotation is successor-only.

## Verification performed

- New deterministic test `TestSameScopeFirstStartLateLoserDoesNotRotate` (late loser admits only after the winner is executing). Mutation check: run against the code without the guard, it failed at the refusal assert; with the guard it passes.
- `GOMAXPROCS=1 -race`, 500 runs each (two drives of 250) of `TestBarrierSameScopeFirstStartContention` and the new test: all passed.
- Whole `internal/gatedrive` package under `-race`: passed.
- Full repository suite via the build gate at the final head (recorded as build evidence in the PR body).
