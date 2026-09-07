<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0388 — Reimplement post-merge fast-forward integration-branch sync as a native Go verb](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0388-reimplement-post-merge-fast-forward-integration-branch-sync.md)**
<!-- docket:backlink:end -->

# Native post-merge integration checkout sync

## Purpose and current context

Restore the post-merge fast-forward behavior removed with the Bash facade in change 0370. After finalize or maintenance completes its work, a clean primary checkout on the configured integration branch should follow the fetched integration tip. Local work that prevents a safe fast-forward is retained with an explicit reason.

The design was approved interactively for change 0388. It is based on main at effc9a6dc549f1c7b1fa888707d3303681c80cad.

- Changes 0029 and 0041 established the behavior and corrected its target to the consuming repository's primary worktree.
- Change 0364 has since landed `gitcli.Client.FastForwardWorktree`. It checks tracked and non-ignored untracked dirt and applies `merge --ff-only` to an object ID. Reuse this primitive; add the operation's branch, ancestry, repository-state, and diagnostic checks around it.
- ADR-0099 removed main-mode. The stub's former main-mode no-op is obsolete: legacy topology follows the existing typed migration refusal, without restoring compatibility behavior.
- `maintenanceSweep` processes merged-PR recovery, cleanup, and reclaim; it does not merge open PRs. Its full and implementation scopes remain governed by ADR-0101.
- Closeout and cleanup may advance integration through existing artifact-backlink repair. Sync therefore follows that suffix and fetches the final tip, rather than using the merge commit alone.

## Operation and ownership

Introduce the semantic operation `repository.sync-integration`, exposed as a native Go repository subcommand with `--repo-dir <dir>` and the standard JSON switch. The omitted directory follows the CLI's existing current-directory convention. There is no change ID, caller-supplied branch, remote override, force flag, or configuration knob.

Register its capability as `local-write`: it fetches objects/remote-tracking state and may advance the primary checkout, but neither pushes nor changes planning metadata. Bind its request/result contract to the reflected schema surface and closed vocabularies according to ADR-0104 and ADR-0109. Maintained skills name the semantic operation and resolve executable argv from the catalog.

Keep the mechanics in one app-layer service over the existing config resolver and Git adapter. The public CLI and the maintenance composition call that same service; maintenance does not launch a nested CLI process or duplicate predicates. Use an injectable seam for orchestration tests.

Resolve the consuming repository using Git discovery, including invocation from its metadata or feature worktree. Select its canonical `PrimaryWorktree`, not the invocation worktree, binary install directory, or Docket's source clone. Resolve the integration branch through the existing configuration path (configuration remains based on the default branch). Do not call the mutating repository preparation workflow merely to perform this sync. Normal topology/configuration validation still applies.

## Safe advancement

The service performs these checks against the same discovered primary checkout and fresh target:

1. Resolve repository and configuration; refuse unsupported/legacy topology through the existing typed contract. Failed discovery or configuration is an error, never an absent/defaulted repository.
2. Read the primary's symbolic branch and state. Require exactly the configured integration branch, no in-progress merge/rebase or equivalent unfinished Git operation, and a pristine index/worktree including non-ignored untracked files. Detached or other-branch checkouts are explicit skips. Ignored files do not by themselves count as dirt.
3. Fetch the configured integration branch from origin through the existing Git adapter, then resolve and pin its fetched commit object ID. A failed fetch or unresolved target must never fall back to stale remote-tracking data.
4. Compare local HEAD with that pinned target. Equality is already current. A strictly ahead local branch is skipped; divergence is skipped. Only strict local-ancestor-of-target admits advancement. Distinguish a negative ancestry result from a probe error.
5. Recheck branch identity and unfinished-operation state immediately before the advance; leave a changed or unobservable state alone. Let `FastForwardWorktree` recheck cleanliness and enforce the actual FF-only update. Pass the pinned object ID, not a movable ref name.
6. Verify the resulting branch/HEAD before claiming advancement. A post-check failure reports an unknown/failed observation with the known before/target facts; it does not roll back or claim the tree was untouched.

Never switch branches, stash, commit user edits, reset, rebase, force a ref, delete resources, or retry using another Git strategy. Detected races are reported and left for a later invocation; no claim of atomic exclusion against arbitrary external Git processes is introduced. Existing Git locking and FF-only semantics remain the mutation boundary.

## Results and failure behavior

Return the standard protocol-v1 envelope plus a Docket-owned structured sync outcome: disposition, reason, primary path, integration branch, and before/target/after object IDs when known. Define the closed disposition vocabulary as `advanced`, `already-current`, `skipped`, `refused`, and `failed`; human messages are explanatory, never decision inputs.

Normal reasons distinguish dirty state, detached HEAD, another branch, unfinished Git operation, local-ahead, divergence, and a detected checkout change. Dirty-state messages explicitly mention that non-ignored untracked files can block sync and tell the user to inspect/resolve the dirt before retrying. Error reasons distinguish discovery/configuration, state probe, fetch/target resolution, ancestry, update, and post-update verification failures, reusing existing typed findings where they match.

An actual advance maps to envelope `applied`. Already-current and deliberate safety skips map to `no-op` and a successful CLI exit. Invalid input/configuration, inability to observe, and failed Git operations use the existing corresponding non-success envelope/exit semantics. The standalone command must not disguise a failed fetch as successful synchronization.

Best effort belongs to the workflow callers: a sync refusal or failure is visible but does not undo or replace completed merge, closeout, or cleanup results, create a finalize-blocked marker, or suppress unrelated work. Preserve cancellation rather than spawning detached work or looping indefinitely.

## Workflow integration

### Finalize

Add one end-of-run sync step to the maintained finalize workflow, after its closeout and cleanup attempts for the selected batch. Invoke once for the consuming repository using a path that survives removal of feature worktrees. It also runs on already-merged recovery and when cleanup reports pending/retained state. Surface the sync outcome separately.

If a batch stops after earlier verified merges, perform the best-effort suffix when repository context remains valid and execution has not been cancelled; preserve the original halt. Do not proceed with sync through failed bootstrap or an unknown repository identity. A stacked child never makes its parent branch the integration-sync target: this command always resolves the configured integration branch.

Do not embed the sync inside `finalize.merge` or each per-change closeout. That repeats work and can precede later integration backlink commits.

### Maintenance and status

After the maintenance item loop and its cleanup suffixes, invoke the shared sync service exactly once for both `full` and `implementation` scope, including a successfully initialized sweep with no actionable items. This supports recovery of externally merged or previously skipped work without a historical per-record scan. Whole-sweep initialization refusals and cancellation do not trigger new sync work.

Expose an additive top-level `integration_sync` outcome in the maintenance result/schema, with a concise human summary. It is not a change-ID entry and does not inflate applied-change counts. An advance can make an otherwise no-op sweep `applied`; a sync failure is reported separately and does not overwrite existing sweep outcomes or their failure semantics. Preserve deferred historical cleanup counts and the one-fresh-metadata-observation-per-dispatched-operation contract; sync adds bounded repository-level Git work, independent of archive size.

Keep `status` read-only. The status skill relies on maintenance's embedded sync when it requests a sweep and must not invoke a duplicate sync. Update the convention's obsolete manual-only sentence and affected finalize/status prose together, plus README and generated embedded assets through their normal generation path. Do not rewrite archived decisions or historical build records.

## Validation and acceptance

Use hermetic Go tests with local bare origins and isolated fixtures, covering:

- Successful FF to the fetched tip and idempotent repeat; configured integration branch differing from the default branch.
- Invocation from the primary, metadata, and feature worktrees, including a consuming repository distinct from the Docket source clone. Only the correct primary advances.
- Dirty tracked/index state, non-ignored untracked files, ignored-only files, detached HEAD, another branch, unfinished Git operations, local-ahead, and divergence. Assert actual branch, HEAD, index, and file preservation for skips.
- Failed discovery/state/ancestry probes, missing target, failed fetch with a stale remote-tracking tip still present, failed update, and uncertain post-update verification. None fabricates advancement; unknown never authorizes a write.
- A changed checkout detected before advancement; update failure never falls back to a reset or another strategy.
- Finalize ordering and once-per-batch wiring, already-merged recovery, pending cleanup, and preservation of an earlier halt.
- Both maintenance scopes, empty worklists, multiple changes, per-item failure, and whole-sweep refusal. Assert exactly one terminal sync attempt where permitted, no per-history amplification, separate diagnostics, correct applied counts, and no duplicate status-skill call.
- Capability annotation, schema/result fidelity, CLI exits, and human diagnostics.

Mutation-test newly introduced guards by removing the protected condition and proving the relevant tests fail. Derive workflow invocation sites from a whole-repository search and classify maintained executable sites separately from history and generated copies. Run the entire configured build suite via the source-entered development test operation, resolve the command from configuration, and act on authoritative serial budget breaches.

The change is complete when a safe primary checkout follows the final fetched integration tip after both workflows, every unsafe/unknown case is accurately reported without destructive recovery, and capability/schema/docs describe the shipped behavior consistently.

## Scope and alternatives

This change adds the native sync service/CLI, its two workflow integrations, focused documentation/generated assets, and meaningful regression coverage. It adds no background scheduler, historical retry queue, new lifecycle state, automatic binary reinstall, main-mode support, terminal publication, Bash restoration, or separate update of the Docket installation used by another repository. Newly refreshed skill files affect future sessions; no live-session reload is promised.

Manual-only operation was rejected because it leaves the original post-merge drift. Per-merge synchronization was rejected because it repeats work and misses later closeout commits. One shared service invoked at each workflow's end restores the requested behavior with explicit diagnostics.

Related changes 0029, 0041, 0364, 0370, and 0389 are already done. No unresolved dependency or stacking relationship is introduced. Architecture references: ADR-0099 (single topology), ADR-0101 (maintenance scope), ADR-0104 (capability authority), ADR-0109 (payload schema).
