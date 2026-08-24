<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0335 — refresh-claim fails verify-delta when the board is byte-unchanged](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-22-0335-refresh-claim-fails-verify-delta-on-a-byte-unchanged-board.md)**
<!-- docket:backlink:end -->

# Design — refresh-claim fails verify-delta when the board is byte-unchanged

Change: 0335 (`fix`). Autonomously groomed (human override: groomed although the stub was not
armed `auto_groomable`). This spec is the deferred audit trail; every design decision below was
defaulted conservatively and gated by the adversarial critic.

## Problem

`docket change refresh-claim` re-stamps only `claimed_at` (plus the `updated:` date) on an
in-progress change. When that re-stamp leaves the inline board **byte-identical** to the committed
`BOARD.md` — which is the normal case, because `claimed_at` is not a board-visible field — the
transaction still **declares** the board path in its `MutationPlan.Files`, and the engine's
two-way delta guard (`verifyActualDelta`, stage `verify-delta`) refuses with
`"a declared path is not an actual change"`. The net effect: the optional plan→build lease
re-stamps silently fail, so a live agent that wanted to refresh its claim lease cannot, purely
because its board re-render happened to produce identical bytes.

Observed 2026-08-21 during the change-0330 implement-next run (`discovered_from: 330`). It did not
break that run — the lease was already fresh and later transactions changed link-bearing fields —
but it is a real defect in the shared `change_claim` / `change refresh-claim` op.

## Root cause (verified against the running code, not just the stub hypothesis)

`internal/app/change_claim.go`, `changeClaimOp.Plan` (the single `Plan` shared by both
`change.claim` and `change.refresh-claim`), unconditionally appends the board `FileMutation` when
`o.inline` is true:

```go
if o.inline {
    boardBytes, err := render.Board(...)
    boardPath := path.Join(o.changesDir, "BOARD.md")
    kind, err := boardMutationKind(ctx, st.Tree, boardPath)   // create-or-replace; NEVER compares bytes
    files = append(files, transaction.FileMutation{Path: ..., Kind: kind, Bytes: boardBytes})
}
```

`boardMutationKind` (`internal/app/change_create.go`) reads the base-tree board blob only to decide
create-vs-replace; it never compares the rendered bytes to the committed bytes. So a byte-identical
re-render is still declared, materialization writes identical bytes, `git status` reports **no**
change to `BOARD.md`, and `verifyActualDelta` (`internal/repository/transaction/commitverify.go`,
lines 43–47) trips the declared-but-unchanged arm.

The fresh-claim transition (`proposed → in-progress`) changes the board's status column, so its
board render genuinely differs — the bug is reachable only on `refresh-claim`. Both paths share the
same `Plan`, so the fix lands once and covers both.

## The fix

In `changeClaimOp.Plan`, replace the `boardMutationKind`-based unconditional append with the
**declare-only-when-changed** switch that two sibling ops already use verbatim — `change_attach.go`
(lines ~671–692) and `change_reconcile.go` (lines ~478–504):

```go
if o.inline {
    boardBytes, err := render.Board(render.BoardInput{Snapshot: candidate})
    if err != nil { /* wrap: "change claim: rendering board" */ }
    boardPath := path.Join(o.changesDir, "BOARD.md")
    results, err := st.Tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(boardPath)})
    if err != nil { /* wrap: "change claim: probing board path" */ }
    existing := len(results) == 1 && results[0].Found
    switch {
    case !existing:
        files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(boardPath), Kind: transaction.MutationCreate, Bytes: boardBytes})
    case !bytes.Equal(results[0].Blob.Bytes, boardBytes):
        files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(boardPath), Kind: transaction.MutationReplace, Bytes: boardBytes})
    }
}
```

- Add the `"bytes"` import to `change_claim.go` (currently absent).
- Carry a short header comment mirroring the sibling ops, naming the `verify-delta` refuse string so
  the reason is greppable (per this repo's cross-reference-by-quoted-clause convention).
- `boardMutationKind` stays — it is still the correct helper for the other 8 call sites (create,
  groom, implemented, kill, lifecycle, reclaim, finalize-closeout ×2), each of which changes a
  board-visible status so their board render always differs. Only the claim op stops calling it.

Comparing the rendered bytes against the **base-tree** blob (`st.Tree.ReadBlobs`) is the same
baseline `verifyActualDelta` diffs the worktree against, so the declaration decision and the guard
decision read the same copy (the repo's `decide-and-act-on-the-same-copy` learning).

## Regression test

Add to `internal/app/change_claim_test.go`, alongside `TestChangeRefreshClaimStampsOnly`: a case
that runs `refresh-claim` on an in-progress change whose board render is byte-identical to the
committed `BOARD.md`, and asserts the transaction **applies** (disposition `applied`, no
`verify-delta` failure) and that the board path is **not** among the committed files. A mutation
check: with the fix reverted, this test must redden with the `verify-delta` refusal — that is the
proof the guard-shaped fix is load-bearing, per this repo's mutation-test-every-guard rule.

Run the whole suite at the gate (`scripts/run-tests.sh`, the resolved `finalize.test_command`), not
only the claim file — the change touches a shared op.

## Assumptions (deferred audit trail — every decision, the default taken, the rejected alternatives)

1. **Approach: declare-the-board-only-when-changed (chosen) vs. relax `verify-delta` to tolerate a
   declared-but-unchanged derived-view path (rejected).** The stub's own Open question posed this
   fork. Chosen the former: it is narrower, matches the "derived view — commit only if changed"
   contract used across the board writers, and — decisively — is **already the in-repo precedent**:
   `change_attach.go` and `change_reconcile.go` each fixed exactly this byte-identical-board case on
   their own op with this switch. Relaxing the guard would weaken a safety predicate globally to
   paper over one op's over-declaration; rejected.

2. **Scope: the claim op family (claim + refresh-claim) only; other `boardMutationKind` callers left
   alone.** The stub's Out-of-scope forbids a broader `verify-delta` redesign. The established house
   pattern is to fix this per-op as each byte-identical case is discovered (attach did, reconcile
   did). `change_groom` is a plausible latent carrier of the same bug (grooming sets `spec:`/
   `trivial:`, which may not be board-visible), but confirming and fixing that is a **separate**
   change, not this one. Noted here as a recommendation, not built.

3. **Inline the switch (chosen) vs. extract a shared helper for all board-declaring ops (rejected
   for this change).** The repo already carries two near-verbatim inline copies (attach, reconcile);
   a third in the claim op is the minimal, consistency-preserving diff. A shared helper is a
   worthwhile follow-up refactor but would touch attach and reconcile too, exceeding this fix's
   scope and risk budget. Rejected here; recorded as a candidate refactor.

4. **The change record always changes on a refresh, so the plan is never empty.** Refresh re-stamps
   `claimed_at` to `clock.Now()` and `updated:` to today; the record blob therefore differs and is
   always declared. The theoretical true-no-op (two refreshes within the same clock-second landing
   identical record bytes AND identical board) would leave an empty declared set — an
   engine-empty-plan edge unrelated to the board. It was not observed, is not what the stub reports,
   and is out of scope; if it ever surfaces it is a distinct defect against the engine's empty-plan
   handling, not this board-declaration fix.

5. **`bytes.Equal` (exact byte comparison) is the right equality, not a semantic/normalized board
   compare.** `verifyActualDelta` keys on Git's byte-level changed-path set, so the declaration
   decision must use the identical byte equality to stay consistent with the guard. Any looser
   comparison could declare a path Git sees as unchanged (re-introducing the bug) or skip one Git
   sees as changed (undeclared-path refusal). Rejected any normalization.

6. **Dependency state.** `depends_on: []` — no blockers; build-ready on merge once specced. The
   `discovered_from: 330` link is informational only.

## Out of scope

- Any broader `verify-delta` redesign beyond this no-op-board case (stub Out-of-scope).
- Changing when or how often claim leases are re-stamped (stub Out-of-scope).
- Generalizing the byte-compare switch to `change_groom` or the other `boardMutationKind` callers,
  or extracting a shared helper — recommended follow-ups, not this change.
