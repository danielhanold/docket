<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0567 — Clean up stacked children when their stack lands, and finalize a child after its parent](../../changes/active/0567-cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea.md)**
<!-- docket:backlink:end -->

# Clean up stacked children when their stack lands, and finalize a child after its parent — design

Change #567, groomed 2026-10-18.

## Why

Docket users stack changes in their own repositories, and two parts of finishing a stack are broken.

**1. A finished stack leaves every child behind.** In the normal stacked flow a child's PR merges into its parent's branch (`stacked-merged`). When the stack root lands on the integration branch, the root closeout (`finalize.closeout`, disposition `root-archived`) marks the root and every carried child `done`. Then:

- Neither `docket-finalize-change` step 10 nor the sweep's post-closeout suffix (`sweepRunCloseout`) runs cleanup for the carried children. Both clean only the root.
- When cleanup does run for a carried child (`finalize cleanup --id <child>`, or a later full sweep), `finalizeCleanupDone` refuses with `destination-mismatch`, because the child's PR merged into its parent's branch, not the integration branch. That refusal comes before the workspace leg, so the child keeps its worktree folder, its local branch, and its remote branch.
- Every full sweep re-dispatches that refused cleanup while the child's remote branch exists. Once the remote branch is gone, the sweep reports the child's workspace as blocked on every run (see the next point). A leftover local branch alone does not dispatch, because of the carried-descendant exception #562 added.

Removing the `destination-mismatch` refusal is not enough, for two more reasons:

- **The worktree reads as foreign.** `workspace.prepare` records in the workspace manifest the base the checkout was cut from (`BaseRef`; for a stacked child, its parent's branch). Every later workspace operation rebuilds its target from the *current* effective base (`domain.ResolveEffectiveBase`), and ownership (`ownsManifest`) requires `m.BaseRef == target.BaseRef`. Once the parent is `done`, ADR-0092 resolves the child's base to the integration branch, so the manifest no longer matches. `workspace.Cleanup` returns blocked ("manifest identity does not match this repository or target"), the sweep's inspection reads `StateForeign`, and cleanup keeps both branches behind the blocked workspace.
- **The local-branch proof can't pass.** For an ordinary change, the local branch is deleted only when the PR's merge-result commit is an ancestor of the freshly-fetched integration tip. Docket prefers rebase merges (`selectMergeMethod`: rebase, then merge commit, then squash), and the root's finalize rebase rewrites its branch, so a child's merge-result commit on the parent's branch is never itself on the integration branch: same content, different commit ids. Closeout already handles this. Before archiving a carried child, it proves the child's work is present, by ancestry or by exact content at the root's merge result (`ProvePreserved`).

**2. A child whose parent landed first cannot be finalized** (found by reading the code; not yet reproduced). The stacking reference lets interactive finalize merge a parent that still has open children, after retargeting their PRs (`finalize.retarget-children`). The child's PR then targets the integration branch, and so does its effective base, but its manifest still records the parent's branch. `finalize.rebase` builds its target through `resolveWorkspaceTarget` and inspects the workspace, which reads as foreign, so the rebase refuses. `finalize.publish`, `pr.publish`, `evidence.recertify`, and `workspace.inspect`/`workspace.publish` build their target the same way. Its cleanup after merging hits the same blocked workspace.

Both problems have one cause: a finished checkout's identity includes the branch it was cut from, but a stacked child's base legitimately changes when its parent lands.

## Goals

1. When a stack root lands, finalize and the sweep clean up every carried child right after the root, in the same run.
2. Cleanup removes a carried child's worktree, local branch, and remote branch once it proves the child's work reached the integration branch through its stack, under the same safety rules as an ordinary done change.
3. A finished workspace is recognized as its change's own even after its effective base changed, so a child whose parent landed first can be finalized and cleaned up.
4. Leftovers that already exist in users' repositories are cleaned on the first full sweep after upgrade, with no migration and no new command.

## Non-goals

- Changing how stacked children merge, when they become `stacked-merged` or `done`, or the rule that a child's branches are kept until its root reaches the integration branch (the child's branch is the backup copy if a parent rebase drops its work).
- Changing ordinary (unstacked) cleanup. In particular, a clean worktree folder is still removed when cleanup then keeps the branch. That was decided while grooming: nothing is lost, because the commits stay on the kept branch.
- Killed changes' branches and worktrees.
- The killed-parent policy.

## Anti-goals

- Never delete a carried child's folder or branches unless both its stack relationship and its work on the integration branch are proven. A probe that errors is unknown, never proof (learning probe-error-is-not-clean-absence).
- No second copy of the carry proof that can drift from closeout's (learning duplicated-gate-copies-the-whole-predicate).
- No "delete it by hand" advice when the carry can't be proven. That branch may be the only copy of the work.

## Design

### 1. A finished checkout's identity excludes its base

`ownsManifest` stops comparing `BaseRef` for a manifest in the `ready` or `cleaned` phase. Identity stays: repository common dir, workspace id, feature ref, change id, slug, and path. Every proof that actually protects work is unchanged: registration at the exact path on the feature ref, HEAD equal to the ref tip, no dirty, staged, untracked, or conflicted path, and non-forcing removal.

*Designer's call:* an `allocating` manifest (a creation interrupted halfway) keeps the base match. `prepareResume` resumes from the recorded `BaseCommit`, so resuming it under a different base ref would mix the old commit with the new ref. A mismatched interrupted allocation stays blocked, as today.

This one rule fixes the worktree leg of cleanup for carried children, the sweep's workspace inspection for them, and every workspace operation for a child whose parent landed first. The manifest's `BaseRef` and `BaseCommit` stay recorded as history. Anything that reads them (for example `Inspection.BaseReached`) keeps reading the manifest's values. Record the rule as an ADR, since it changes what identifies a workspace.

### 2. Cleanup proves a carried child, then cleans it normally

In `finalizeCleanupDone`, when the verified merge destination is not the integration branch, cleanup tries to prove the record is a carried child before refusing.

- **Relationship.** Walk up `stacked_on` from the child. At each link, the change's live PR (probed, never read from the record) merged into its parent's recorded `branch:`, and the parent is `done`. The walk ends at the first ancestor whose PR merged into the integration branch: the root. A killed link, a cycle, a missing parent, a PR that is not merged, or a destination that matches no parent branch fails the proof. A PR that cannot be read (a probe error) is unknown, never a failed proof: it takes the probe-error row below. This is closeout's carry walk (`proveCarry`) applied to a population that is now `done` rather than `stacked-merged`. Generalize and share it rather than writing a second walk.
- **Content.** The root's merge-result commit is an ancestor of the freshly-fetched integration tip (the same reachability check closeout makes for the root, re-run live, *designer's call*). Then the child's merge-result commit is an ancestor of that tip, or `ProvePreserved(child merge result, root merge result)` proves it. This is exactly closeout's per-descendant proof in `closeoutIntegrationDestination`. Extract it into one helper that both closeout and cleanup call.

Outcomes:

| Situation | Result |
|---|---|
| Relationship fails | Today's `destination-mismatch` refusal; everything kept. |
| A probe errors (PR, fetch, ancestry, preservation) | `pending`; everything kept; quiet retryable wording. |
| Relationship holds, content unproven | `pending`; everything kept; a new reason (name left to the builder, for example `carry-unproven`) with a plain message saying the child's work could not be found on the integration branch, so its folder and branches were kept. Not a kept-branch finding, and no delete command. |
| Both proven | The normal legs run unchanged and in order: PR backlink repair, workspace (now owned, per §1), local branch, remote branch. |

For a proven carried child, the local leg's containment check is satisfied by the content proof, replacing the merge-result ancestry check that can't pass for a carried child. Its other checks stay: the tip equals the child's merged head, the branch is checked out in no worktree, and the delete is checked against that tip. The remote leg is unchanged: no open child PR targets the branch, the remote tip equals the merged head, and the delete is an exact-lease push. #562's kept-branch warnings apply as for any done change.

A child whose PR was retargeted and merged straight into the integration branch takes the ordinary path. §1 alone fixes its cleanup.

### 3. Clean carried children right after the root

- **Sweep:** `sweepRunCloseout`, after the root's cleanup, runs the same per-id cleanup (`sweepRunCleanup`, each with its own fresh prepare) for every id in the closeout result's `CarriedIDs`, appending one entry per child. Both the full and the implementation-scope sweep reach closeout through it. A replayed closeout (`already-closed-out`) that carries no list leaves its children to the next full sweep (§4).
- **Finalize:** `docket-finalize-change` step 10 runs `finalize.cleanup` for the root, then for each id in step 9's `carried_ids`, and reports each result the way it reports the root's. That is one clause in the skill. Run `go generate ./internal/assets/` after the edit.
- A child's cleanup result never changes the root's result or another child's.
- *Designer's call:* in a full sweep, a carried child also sits in the cleanup worklist from the pinned snapshot, where it was still `stacked-merged` and reads "retained". When the closeout suffix has already run that child's cleanup in the same invocation, the sweep omits the stale "retained" entry, so each child reports one cleanup outcome per run. The mechanism is the builder's call.

### 4. The sweep treats a carried child's leftover like any other

Remove #562's carried-descendant exception (`sweepMaybeCarriedDescendant` and its use in the presence trigger). With §2, a carried child's cleanup no longer refuses by design, so a leftover local branch is ordinary work. A done record whose carry can't be proven re-dispatches on each full sweep and stays pending, like any other pending cleanup today.

### 5. Docs

Docs describe current behavior only, with no change citations:

- `skills/docket-convention/references/stacked-changes.md`, *The stack close-out is idempotent*: once the root lands, each carried child's folder and branches are cleaned up right after the root's, once cleanup proves the child's work reached the integration branch.
- `skills/docket-finalize-change/SKILL.md` step 10 (§3).
- `skills/docket-convention/references/close-out.md` and `docs/reference/glossary.md`, wherever they describe what cleanup or the sweep keeps for stacked children.

## Constraints

- Every destructive leg still runs through a fresh prepare and `FinalizeCleanup`'s live proofs. Records, rendered tables (*Stack carried*, *Stacked children*), and the sweep's inventory never authorize a delete.
- A carried child's cleanup adds one PR probe per ancestor up to the root, plus the content proof. An unstacked change pays nothing new. The #390 request-budget tests still hold for records with nothing left to clean.

## Testing

Write the reproductions first and watch each go red before the fix.

- **Parent landed first (end to end, integration fixture):** stack B on A and build both. Finalize A first, retargeting B's PR, then finalize B through rebase, publish, merge, closeout, and cleanup. Today this is expected to refuse at the workspace inspection. If the reproduction shows further blockers caused by the same base mismatch, fix them here. Anything else goes in the results file as a follow-up.
- **Carried child (end to end):** a two-level stack and a three-level stack (a grandchild through an intermediate), with the root merged by rebase. After the root's finalize, every child's folder, local branch, and remote branch are gone. The sweep's closeout path does the same. Extend the existing carried-descendant cleanup fixture (`TestIntegrationFinalizeCleanupPRBacklinkRetryCarriedDescendant`).
- **Existing leftover:** a carried child already `done`, with its folder and both branches present, is cleaned by one full sweep.
- **Fail-closed:**
  - content unproven keeps everything, with the new reason and no delete advice
  - an ancestor PR probe error is pending
  - a destination that matches no parent branch still refuses with `destination-mismatch`
  - a killed link fails the relationship
  - a child whose branch moved after its merge is kept with #562's named warning
  - a child with an open stacked PR keeps its remote branch
- **Ownership:**
  - a `ready` manifest whose `BaseRef` differs from the target's is owned for cleanup, inspect, publish, and prepare
  - an `allocating` manifest with a different base stays blocked
  - mutation test: restore the base comparison and watch the parent-landed-first and carried-child tests go red
- **One shared proof:** closeout and cleanup call the same content-proof helper and the same relationship walk. Mutate the helper and watch both closeout's and cleanup's tests go red.
- **Sweep:** replace `TestAssessCarriedDescendantByPRFactsGetsNoPresenceDispatch`. A carried child with only its local branch left now dispatches.

## Related

- #562 added the kept-branch warnings and the carried-descendant exception this change removes.
- ADR-0092 explains why a child's effective base becomes the integration branch once its parent is done.
- ADR-0035: cleanup teardown is fail-closed.

## Left to the builder

- The new reason code's name and message wording, in the house finding style: what happened, why it matters, the remedy.
- How `proveCarry` is generalized.
- The shape of the extracted content-proof helper.
- Whether `ownsManifest` takes the manifest phase, or a separate predicate covers the allocating case.
