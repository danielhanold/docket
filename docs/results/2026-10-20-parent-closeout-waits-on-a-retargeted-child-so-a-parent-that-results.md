<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0571 — Close out a landed stack parent without waiting on children that never merged into it](../changes/active/0571-parent-closeout-waits-on-a-retargeted-child-so-a-parent-that.md)**
<!-- docket:backlink:end -->

# Results — Close out a landed stack parent without waiting on children that never merged into it

**Human action:** None required beyond the normal PR review and merge.

## Outcome

A stack parent that has landed on main now closes out to `done` without waiting on children that never merged into it. Its closeout takes to `done` exactly the children whose PRs merged into its branch, whether they are `stacked-merged` or still `implemented` because their own closeout never ran. Each one's work must first be proven on main. A child that was retargeted to main, is not built yet, was killed, or already landed on its own no longer blocks the parent.

- One carry rule (`domain.DeriveCarriedSet`) now serves rebase, publish, merge, and root closeout. `DeriveRootCloseoutSet` is a thin wrapper that keeps only the refusal when the root's own ancestry cycles.
- A `stacked-merged` child whose PR facts are missing or point elsewhere still refuses the parent, as before. So does a carried child whose work is not proven on main (`descendant-carry-unproven`), with zero descendant writes.
- Closeout asks GitHub only about `implemented` and `stacked-merged` descendants.
- Closeout's `children-retarget-required` disposition is removed, along with its glossary row and skill text. Cleanup's disposition of the same name is a different thing and stays.
- `TestE2EStackParentLandedFirst` now runs the real parent closeout. The hand-written done record (`archiveAsDoneOnOrigin`) is deleted.
- Recorded as ADR-0156.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed on the final head, after the review fix. Budget lines were `BUDGET WATCH` screening notes only, each at parallel-overrun streak 1/5. None was a breach.
- Every build task ran mutation probes. Each of these turned the new tests red, and the tests went green again after restore:
  - putting back the old "every descendant must be stacked-merged" rule;
  - dropping the `implemented`-but-merged arm;
  - carrying an `implemented` child merged anywhere;
  - skipping the content proof for that arm;
  - removing the GitHub lookup filter.
- Review: deep tier (`docket-review-deep`). It returned one minor finding, which was fixed in-branch: the subtest `open-pr-not-carried` was renamed to `not-merged-pr-not-carried` and now asserts exactly one GitHub probe. No mutation probe was run for that new assertion.
