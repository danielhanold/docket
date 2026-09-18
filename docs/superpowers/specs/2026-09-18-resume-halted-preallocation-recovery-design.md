<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0368 — Recover a run halted before its workspace was allocated](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0368-resume-halted-preallocation-recovery.md)**
<!-- docket:backlink:end -->

# Recover a run halted before workspace allocation

Approved design for change 0368, 2026-09-18.

## Outcome

A human-authorized `change.resume-halted` can recover an in-progress change halted before feature-workspace allocation while its lease is fresh. Recovery preserves the same claim and recorded branch, refreshes the claim, removes the halt section, and leaves feature branches, worktrees, manifests, and build evidence uncreated and unchanged. Ordinary workspace preparation occurs later in the existing implementation workflow.

## Evidence and existing machinery

The source examined is commit `3ccf9fac511f370c200315674a1edf97766b0a5b`. The fetched integration snapshot at grooming startup was `ad7f1462cd4fddbf28ced014f1b02963fe13ee8e`; its changes relative to the examined checkout did not modify the recovery or workspace code discussed here. Reconcile must check subsequent integration changes before implementation.

- `internal/workspace/manifest.go` already distinguishes `manifestAbsent`, `manifestValid`, `manifestForeign`, and `manifestUnknown` in `classifyManifest`.
- `internal/workspace/inspect.go`, `Service.Inspect`, collapses `manifestAbsent` into `StateForeign` before probing branches, paths, or registrations.
- `internal/app/change_halt.go`, `ChangeResumeHalted` and `resumeQuiescenceRefusal`, reject that state. The existing exact-version `resumeHaltedOp` already owns the required metadata transition.
- `internal/workspace/prepare.go`, `inventoryForFresh`, already checks local and remote branch absence, target-path absence with `Lstat`, and Git registration before allocation. Reuse its primitives and share the relevant probe logic rather than inventing a recovery allocator.
- `internal/app/change_reclaim.go` separately proves branch absence and applies strict lease expiry. Reclaim abandons a claim; resume preserves it. Their different lease policies are intentional.
- `TestIntegrationChangeResumeHalted` exercises a fake workspace service. `TestInspectForeignAbsent` explicitly pins the current conflation. Neither proves a real pre-allocation recovery.

History reviewed: 0313 introduced workspace ownership and fresh-allocation checks; 0316 introduced halt/resume/reclaim; 0318 supplied the reported pre-allocation incident; 0354 fixed halt-section structure; 0375 added run cancellation and admission; 0429 corrected ready-workspace ancestry checks. Neighbouring 0360 concerns coordination cost and 0366 concerns release acceptance; neither is a dependency of this fix.

ADR-0034 keeps paths anchored to the primary checkout. ADR-0035 preserves fail-closed ownership and teardown. ADR-0118 owns run cancellation and replacement admission. The learnings `probe-error-is-not-clean-absence`, `groomed-root-cause-is-a-hypothesis`, and `verify-the-claim` inform the proof and test requirements. No new lifecycle, cancellation policy, persistent record, or ADR reversal is needed.

## Design

### Preserve the absence distinction in workspace inspection

Extend the existing `StateKind` classification with `StateAbsent` (`absent`). This is a current, local observation, not a historical assertion that a workspace never existed and not evidence that a worker has stopped.

Return it only when the manifest slot is cleanly absent, the recorded local feature ref is cleanly absent, nothing exists at the canonical intended workspace path, and no Git worktree registration occupies that path or references that feature ref. A dangling symlink counts as a present path. A stale registration counts as present even when its directory is gone. Any probe failure or unresolved path identity prevents the absence result.

A missing manifest with a leftover branch/path/registration remains foreign. Malformed, wrong-owner, and unreadable manifests retain their existing classification or error behavior. Existing owned-state classification, including the 0429 ancestry correction, remains unchanged.

Factor only the overlapping local inventory checks from `inventoryForFresh` into narrow read-only helpers reused by preparation and inspection. Keep `Inspect` local-only and non-mutating. Preserve preparation's remote-branch check and allocation ownership/locking. The current `worktreeAt` skips registrations whose path cannot be canonicalized; that is insufficient proof of absence. For the shared absence proof, recognize an exact stale registration and fail closed on unresolved identity rather than copying this skip behavior. No general workspace repair or canonicalization rewrite is required.

### Extend the existing resume operation

For `StateAbsent`, require clean absence of the recorded remote feature ref through the existing typed `ProbeRemoteBranch` adapter before applying the existing resume transaction. Use the resolved recorded feature ref, never a reconstructed branch spelling. A remote branch blocks recovery as existing work; an errored or unrecognized probe blocks as unknown. Reuse a small existing probe primitive if extraction is needed; do not invoke reclaim or import its lease/conventional-branch policy.

Continue requiring the exact in-progress marked record, explicit quiescence acknowledgement, resolved target identity, and successful probes. Preserve existing result/reason conventions for workspace ambiguity and probe failure. Accept only the explicitly known resumable states; unknown state values must not silently authorize resume. Ready, dirty-owned, cleaned, and branch-missing retain their current recovery behavior. Allocating, foreign, and mismatched states remain refused.

The absence result grants no cancellation or launch authority. The existing gate-before/cancel/replacement protocol remains authoritative for dispatched runs under ADR-0118. Do not infer process death from missing workspace files, auto-cancel a run, clear an epoch, or allocate a workspace as part of resume. This change does not add a new cross-process atomicity promise: human acknowledgement and gate admission retain their roles, while subsequent `workspace.prepare` re-probes and locks allocation as today.

### Keep other consumers coherent

Search all workspace-state consumers at implementation time rather than maintaining a fixed allowlist. Preserve their prior behavior for the newly distinguished all-absent case: reclaim still requires strict expiry and all its branch checks; identity repair may still treat absence as no owned workspace; maintenance must not interpret absence as a cleaned tombstone or authority to delete; finalize, publish, and evidence paths still require their existing owned states. Carry the new state through workspace operation output and update any relevant schema, documentation, and test surfaces.

## Alternatives

- Match the text `no workspace manifest` in resume: rejected because detail text is not a typed API, is not forwarded by `WorkspaceInspect`, and proves neither branch nor path absence.
- Add a `--no-workspace` bypass, force reclamation before lease expiry, or allocate first: rejected because the existing resume transition already does the desired work. These add policy or effects without fixing the lost classification.
- Distinguish absence in the existing inspector and reuse allocation probes: selected. The extra state is justified by a concrete information loss at the current service boundary.

## Verification requirements

Use existing workspace and application fixtures with disposable repositories and local bare remotes; no live user change or GitHub PR is needed.

1. A real claim followed by a real halt before `workspace.prepare`, then acknowledged resume with a fresh lease and exact version, succeeds through the actual workspace service. Assert the halt section is removed, claim refreshed, status and recorded branch preserved, and no feature branch/path/manifest created. A subsequent ordinary prepare succeeds.
2. Missing acknowledgement, wrong version, wrong lifecycle, or missing halt marker retains the existing refusal and no-write behavior.
3. Cover missing manifest with a local branch, remote-only branch, existing directory/file/dangling symlink, stale registration at the target path, and registration on the feature ref elsewhere. None qualifies for pre-allocation recovery; preserve all state and authored data.
4. Retain malformed/foreign/identity-mismatched/allocating refusal tests and inject manifest/ref/remote/path/registration probe failures. Unknown never becomes absence.
5. Verify the inspector remains local-only and read-only; normal owned-state recovery, identity repair, maintenance assessment, and strict reclaim expiry preserve their contracts.
6. Mutation-test the absence conjuncts and the new resume admission branch. A removed proof must redden a targeted behavioral test; the positive regression must fail under the original conflation. Use uncached runs for mutation evidence.
7. At build time, run the complete suite through the configured `build.test_command` from the feature checkout using the canonical Go runner. Read its budget report and act on confirmed breaches. Grooming itself writes markdown only and does not run the suite.

## Scope and metadata

No reclaim TTL change, new recovery flag, new command, workspace allocation during resume, adoption or deletion, manifest format migration, gate attribution redesign, or unrelated refactoring. No unmerged dependency or stack base is required.

Keep `depends_on: []`, retain `discovered_from: [318]`, and set related changes to 313, 316, 318, 354, 366, 375, and 429. Cite ADRs 34, 35, and 118. Recovery extends the existing resume operation and preserves its safety boundary. The design resolves the original open questions: extend workspace classification, preserve reclaim policy, and add a real pre-allocation regression fixture.
