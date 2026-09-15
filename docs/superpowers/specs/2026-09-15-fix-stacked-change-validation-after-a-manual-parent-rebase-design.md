<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0429 — Fix stacked-change validation after a manual parent rebase](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-15-0429-fix-stacked-change-validation-after-a-manual-parent-rebase.md)**
<!-- docket:backlink:end -->

# Fix stacked-change validation after a parent rebase

## Problem

A manual parent rebase changes commit IDs. Two existing checks then falsely block the reported stacks:

- Ready-workspace validation requires the original `BaseCommit` to remain ancestral.
- `ProvePreserved` accepts a child's original commit ancestry or exact content at the final tip, overlooking an exact snapshot earlier in the rebased history when later children edit shared files.

## Minimal fix

### Workspace check

Remove the original-base ancestry requirement for **ready** workspaces in inspection, publication verification, and cleanup verification. These are three copies of the same faulty condition.

Keep the existing manifest ownership, recorded refs, registration, clean-tree, and local/remote/PR head checks. Keep original-base ancestry checks for unfinished allocation. Leave the recorded base unchanged; add no rebinding or synchronization command.

### Child preservation

Extend the existing `ProvePreserved(source, target)` exact-content fallback:

1. Keep original-ancestry and exact-at-tip successes.
2. If exact-at-tip comparison fails, inspect commits reachable in `B..target`, where B is the existing sole merge base of source and target.
3. Accept if **one commit** matches **every entry** of the existing nonempty `B → source` delta, using the same exact object/mode/deletion comparison. Compute that delta once; never combine matches from different commits.
4. Otherwise retain the existing unproven result. Git observation errors remain errors.

This makes rewritten history follow the existing historical-inclusion rule: later edits do not invalidate an already-included child. No patch matching, semantic comparison, or persistent proof state is needed.

All current preservation callers inherit this correction. Keep their enforcement points and PR relationship checks unchanged. For root closeout, the target remains the verified root merge-result commit, so later integration history cannot supply the proof. Keep current archive and branch-retention behavior.

## Acceptance

Use existing Go fixtures to demonstrate:

- A ready stacked workspace remains usable after a parent/child rebase makes its creation base non-ancestral; genuinely mismatched, dirty, or stale-head workspaces still refuse.
- A rebased multi-child stack with later edits to shared files passes preservation and normal finalize/root closeout without merge commits.
- A stale-parent rewrite that drops child work still refuses. Entries matching only across different snapshots, on an unrelated branch, or after the root merge cannot establish proof.

Keep existing negative coverage. Mutation-test the new same-snapshot/history restriction. Run the configured whole-suite gates and inspect their budget reports during implementation.

## Scope limit

The added fallback requires an exact snapshot to survive in reachable history. Arbitrary conflict resolutions or squashes that eliminate every matching snapshot remain unproven. This preserves the existing treatment of historical inclusion, including later explicit reverts.

No automatic restacking, history repair, new configuration, recovery framework, merge-policy changes, or unrelated refactoring. Update only comments/docs describing the changed checks.

Related: changes 0298, 0316, 0327; ADR-0092. No dependencies.
