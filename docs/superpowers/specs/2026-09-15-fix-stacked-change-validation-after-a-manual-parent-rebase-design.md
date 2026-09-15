<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0429 — Fix stacked-change validation after a manual parent rebase](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0429-fix-stacked-change-validation-after-a-manual-parent-rebase.md)**
<!-- docket:backlink:end -->

# Fix stacked-change validation after a manual parent rebase

## Purpose and scope

Fix change 0429's two reported false refusals after an existing stack parent is manually rebased onto the configured integration branch:

1. A ready, owned feature workspace is classified as branch-missing because its recorded creation base is no longer ancestral.
2. Carried children become unprovable because their old merge-result IDs were rewritten and later children changed the same files.

This is one bounded correction to workspace identity and the existing preservation primitive. No stack recovery subsystem, new workflow, or manual repair command is needed.

The supported rewrite preserves an exact snapshot of each child's full recorded delta somewhere in the target's reachable history. Ordinary commit-by-commit rebases and subsequent same-file child edits satisfy this when the replay retains those entries. Do not promise acceptance of arbitrary conflict resolutions or squashes that eliminate every matching snapshot. Existing ancestry and exact-at-tip successes remain supported.

## Current behavior and evidence

Inspected source: main at 97033634422c66861d97f7aa2da086c492e67299.

- `workspace.Service.classifyState` treats a false `IsAncestor(m.BaseCommit, reg.Head)` in `PhaseReady` as `StateBranchGone`, even after proving manifest ownership, registration, the recorded feature ref, and matching local heads.
- The ready-workspace checks in `internal/workspace/publish.go` and `cleanupReady` repeat that requirement. `prepareExisting` already validates a ready workspace without it.
- `FinalizeRebase` independently requires the recorded implemented change, verified PR identity and effective base, clean owned workspace, and agreement among requested, local, remote, and PR heads. These checks remain.
- `gitcli.Client.ProvePreserved` accepts original ancestry or exact entries at the target tip. Its full source delta is derived against the sole source/target merge base. Checking only the tip rejects an earlier child's snapshot once a later child edits those paths.
- Change 0327's accepted design explicitly treats historical inclusion as sufficient and excludes detecting later explicit reverts. This change applies that same meaning to exact-content evidence in rewritten history.

The reported downstream child IDs are incident details, not Docket-repository dependencies. Related changes 0298, 0316, and 0327 are done; there are no prerequisites. ADR-0092's effective-base policy remains unchanged.

## Design

### 1. Ready workspace identity

For `PhaseReady`, remove the historical-base ancestry requirement from inspection, publication verification, and cleanup verification. Do not replace it with an ancestry requirement on today's parent: a child is permitted to lag its parent until its own finalize rebase.

Retain all existing checks of repository and manifest ownership, change/slug, canonical path, recorded feature and base ref names, registration, attached branch, branch-ref existence, registered HEAD equality with the feature-ref tip, and dirty/staged/untracked state. A missing feature ref still means branch-missing; detached, wrong-branch, or inconsistent registrations retain their existing mismatch classification.

Keep `BaseCommit` in the manifest as creation provenance. Do not rewrite it, adopt a different workspace, or introduce a rebind operation. Ready-state eligibility must not depend on reading this historical commit; the internal `BaseReached` observation, if retained, applies only where allocation recovery actually checks ancestry.

Keep the creation and `PhaseAllocating` recovery ancestry checks unchanged. An unfinished allocation has not yet established a reusable ready workspace.

Keep finalize's PR/head checks, fresh effective-base resolution, explicit publication leases, owned rebase receipts, and non-forcing cleanup. A remote-only update with a stale local head must still refuse under the existing head-mismatch checks; this fix does not synchronize or reset a user's checkout.

The three ready-state sites use the same corrected rule. No broad workspace refactor, new manifest version, new state token, or new CLI option is part of this change.

### 2. Exact-content evidence in reachable history

Extend `ProvePreserved(source, target)` in place. Callers continue to supply the authoritative child merge-result ID as source and their exact immutable action/delivery commit as target.

Keep the existing validation and fast paths:

1. Validate and resolve both commit objects and reject incomplete/shallow observations.
2. Accept source ancestry of target.
3. Otherwise require exactly one merge base `B` for source and target.
4. Derive the same complete, nonempty tracked-entry delta `D = diff(B, source)`, with rename detection disabled.
5. Check every entry of D at target using the existing exact object-ID/mode/deletion comparison. An exact tip match succeeds immediately.

Only if the last comparison completed with an entry mismatch, search the target's reachable history after B for a matching snapshot:

- Enumerate the complete Git revision range `B..target` using pinned full IDs. Include reachable side histories; do not limit to first-parent, use dates, read unrelated refs, or scan reflogs. Exclude B and all its ancestors.
- Compute B and D once. Do not recompute the comparison base for individual candidates or shrink D to the target-tip mismatch list.
- Reuse the same exact-entry comparison against each candidate commit. **Every entry of D must match at one and the same candidate.** Never combine entries that match in different commits.
- A matching candidate succeeds because its exact recorded snapshot is reachable from the pinned target. Subsequent commits may evolve the shared files. Keep the existing `exact-content` proof kind, documented to include a reachable historical snapshot; no receipt, cache, or new externally serialized field is required.
- If no candidate matches, return the existing entry-differs/unproven result with bounded target-tip differing paths. A history or object-read failure returns an observation error, not a successful proof or a fabricated mismatch. Do not silently truncate the search or turn a timeout into an affirmative result.

Extract only the small entry-comparison helper needed to reuse the existing semantics. Put the revision-range read in the typed Git adapter; do not repeatedly invoke the whole preservation operation on candidates. Keep Git-object reads, NUL-safe paths, full IDs, modes, deletions, binary entries, and rename-as-delete-plus-add handling. No patch IDs, whitespace normalization, merge drivers, cherry-pick probes, or semantic equivalence.

This is historical inclusion, consistent with the ancestry arm. It does not detect an intentional later revert, or prove that an earlier child's bytes are still the final bytes. A stale-worktree rewrite that never carries the child's complete recorded delta has no valid witness and remains blocked.

### 3. Existing enforcement and closeout

All current callers receive the corrected preservation behavior through the shared primitive:

- before rebase and after completion, continuation, or receipt recovery;
- before publish and merge;
- stacked-closeout marking/replay;
- root closeout, including the existing maintenance path.

Do not move, remove, or duplicate these gates. Preserve authoritative PR destination/relationship checks and the full carried-child set. A successful content comparison cannot rescue a failed relationship check.

For root closeout, keep the root merge-result ancestry check against freshly pinned integration. The exact-content fallback searches only history reachable from the root's verified merge-result commit, never later integration commits. A matching snapshot that appears only after that root merge must not retroactively prove its delivery.

Only proven stacks enter the existing atomic archive transaction. Preserve current lease/version checks, lifecycle transitions, retained-on-refusal behavior, merge-method policy, and cleanup eligibility. Fix the duplicated ready-workspace ancestry predicate; do not expand which child branches cleanup elects to delete.

## Acceptance and validation

Extend the existing topical Go fixtures. No downstream repair or live GitHub mutation is needed to validate the fix.

1. **Updated stacked workspace:** allocate a child on an old parent tip, rebase the parent onto newer integration, and update the child so the old tip is no longer ancestral. With the owned ready workspace and local/remote/PR/requested heads agreeing, inspection reports ready and local finalize reaches its normal rebase/gate path. Publication and otherwise-eligible non-forcing workspace cleanup do not fail on the old base. The manifest's recorded base remains unchanged.
2. **Workspace refusal controls:** actual missing branch, wrong/detached registration, dirty worktree, foreign manifest, and stale local versus remote/PR head still refuse for their actual reasons. Rewritten unfinished allocation remains blocked.
3. **Shared-file stack:** build a real multi-child stack where one child changes several paths and a later child evolves a shared file. Rebase it commit by commit onto advanced integration, then simulate rebase-and-merge into integration. Assert the original child IDs are non-ancestors, the child's complete delta mismatches the final tip, and one rewritten historical snapshot matches. Rebase/publish/merge and root archive succeed through their existing paths, without synthetic merge commits. Use at least two sequential edits to the same file; do not model this as disjoint-file siblings.
4. **Dropped work:** the same stack rewritten from a stale parent that omits one child's work must fail. Keep all head identities and PR relationship facts otherwise valid so an unrelated guard cannot make the test pass. Publish/merge have zero unsafe external effects; closeout changes none of the records or board.
5. **One-snapshot requirement:** arrange for each required entry to appear at some target ancestor but never together. The fallback must refuse. A complete match on an unrelated branch or only after the root's merge must also refuse.
6. **Existing compatibility:** keep ancestry and exact-tip successes, unavailable-object/shallow/multiple-base/empty-delta refusals, exact entry semantics, and probe-error distinctions. Add targeted history-walk failure coverage. Preserve the existing historical-inclusion treatment of later explicit reverts.
7. **Guard verification:** mutation-test omission of the all-entries/same-snapshot requirement and the target-history restriction; preserve or mutation-check the existing head/identity and per-boundary preservation guards before removing the obsolete ready-base assertions. Test names and fixtures must distinguish false refusals from genuine lost-work refusals.

At implementation's build and finalize gates, run the whole suite through the configured source-entered Go runner, reading each command from its own configuration. Inspect the budget report even when green. Keep additions in topical shards and obey existing runtime budgets; no new benchmark infrastructure.

## YAGNI exclusions

- Automatic restacking, remote/local synchronization, generic workspace rebinding, or history repair.
- Force/skip-proof overrides, new configuration, new persistence, proof registries, or a stack-management redesign.
- Semantic comparison, arbitrary conflict-resolution support, or acceptance of every squash.
- Merge-policy changes, synthetic merge commits, altered branch-retention policy, or repair of the original downstream repositories.
- Unrelated diagnostics/refactoring, metadata migrations, or changes to frozen specs/ADRs/results.

Update only maintained workspace and preservation comments/documentation that describe the corrected predicates. Leave change 0327's historical records immutable.

## Design alternatives

- Keeping only exact-tip comparison reproduces the reported failure.
- Dropping preservation or trusting PR/CI metadata admits the stale-worktree data-loss case and is rejected.
- Reusing exact comparison at one reachable historical snapshot is selected: it handles later child edits using the existing evidence model and requires no new state.

No unresolved design decisions remain for this defined scope. The implementation must demonstrate both modeled regressions and the dropped-work negative before claiming the fix complete.
