<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0438 — finalize.rebase-abort can't recover a completed-but-unmerged rebase whose base later moved](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-20-0438-finalize-rebase-abort-can-t-recover-a-completed-but-unmerged.md)**
<!-- docket:backlink:end -->

# Rebase a completed finalize attempt onto an advanced base and retest

## Intent and agreed behavior

When finalize resumes a completed local rebase and its effective base has advanced, check the current state, rebase the current feature branch onto the new base, and retest the resulting code. Preserve the prior conflict resolutions rather than resetting the branch to its pre-rebase head. The user approved this flow during grooming of change 0438.

For this repository the effective base is normally `main`; use the existing effective-base resolver so stacked changes retain their established destination semantics. Unchanged-base recovery retains its existing receipt/checkpoint/continuation machinery. A moved base invalidates the earlier gate checkpoint; it does not justify discarding the local work.

Keep the stub's secondary finalize-block idempotency repair, including the same defect in clear-block. Grooming stops at this specification.

## Grounding and correction of the initial hypothesis

Inspected source: `main` at `57794104aef05dc02ca7e3a7f0d1658ee067234d`, 2026-09-20. The incident on change 0368 was not independently replayed during grooming; the reported failure paths are present in the source.

- In `internal/app/finalize_rebase.go`, `FinalizeRebase` resolves current metadata, PR identity, base and remote feature heads. `recoverFromReceipt` rejects a changed `BaseHead` before inspecting the live rebase. That unconditional refusal is the forward-progress gap.
- `FinalizeRebaseAbort` delegates to `gitcli.Client.AbortRebase`, which executes only `git rebase --abort`. A completed rebase has no Git rebase state to abort. Extending abort to reset a completed branch would repair the proposed workaround, but would unnecessarily discard prior resolutions.
- `workspace.RebaseReceipt` already distinguishes `OrigHead` (local starting point) from `OrigRemoteHead` (publication lease), and stores `BaseRef`, `BaseHead`, the rewrite token, resolver accounting, a live gate continuation, and a completed-gate checkpoint. No completion-head field is needed for forward rebasing.
- `gitcli.Client.BeginRebase` already accepts an exact expected local head and exact target base, checks clean state, writes owned orig/base anchors before Git, and classifies completion/conflict. `workspace.PublishRewrite` already publishes under the exact separately recorded remote lease.
- `finalizeRebaseContinueBudgeted` supplies the established workspace-lock, reload-under-lock, durable-before-Git, and interrupted-effect reconciliation pattern. `recordGatePassedReceipt` and `clearGateContinuation` own gate evidence lifetime.
- `finalizeBlockOp.Plan` and `finalizeClearBlockOp.Plan` return zero-valued no-op plans. The transaction engine calls `validatePlan` before its empty-`Files` no-op path, so the missing subject fails validation. Supported empty-file plans still carry valid subject/receipt metadata. The current block test checks planning alone and misses that integration failure.

Historical context reviewed: change 0316's original finalize/recovery specification; 0349's resolver reservations; 0396 and ADR-0105's receipt-owned gate continuation; 0408 and ADR-0112's completed-gate checkpoint; 0411's post-continuation reconciliation-write recovery; 0368's incident context; and 0439's separate admission-slot diagnostics. These are done. Active 0291 concerns reference-loading order and is adjacent, not a dependency.

ADR-0010 preserves the conflict-resolution/semantic-repair boundary. ADR-0105, ADR-0112, and ADR-0113 place recovery facts in the existing receipt. ADR-0118 keeps execution admission and cancellation authoritative. Relevant learnings: `groomed-root-cause-is-a-hypothesis`, `moving-base`, `idempotency-keying`, `decide-and-act-on-the-same-copy`, `probe-error-is-not-clean-absence`, and `printed-remedy-state-validity`.

## Design

### 1. Classify before refreshing the recorded base

Extend the existing `finalize.rebase` receipt-recovery path. A different remote base head is a signal to inspect, not permission to reset or overwrite an arbitrary attempt.

Admit forward rebasing only when the change/version, recorded feature branch, PR destination, manifest, registered checkout, and receipt repo/change/base-ref identities agree; the workspace is clean; no Git rebase is in progress; and its current head descends from the receipt's old base. Reuse the existing completed-rewrite proof. Re-prove carried descendant preservation at the starting head and resulting head through the established checks.

Require the remote feature and matching open PR to remain at the receipt's recorded `OrigRemoteHead` for this unpublished-rewrite recovery. Preserve the exact lease; a missing/moved remote, changed PR destination, merged/closed PR, foreign workspace, failed probe, or malformed receipt stays a refusal with local work retained. Require the new base to descend from the old recorded base; a rewritten/divergent base is outside this bounded change.

A changed head is not automatically an error: committed work after the earlier rebase is carried forward from the actual clean current head. No previous completed-head identity is needed because this operation preserves that work instead of rolling it back. An uncommitted edit is still a refusal.

Do not refresh over a running or unresolved gate, live resolver child, unresolved reservation, or continuation reconciliation. Preserve their existing recovery authority and route. A persisted live gate continuation must settle through the existing finalize continuation machinery before starting another rewrite; no stale gate result may be returned as permission to publish against the new base. A still-conflicted rebase continues against its recorded base through the ordinary resolver path, then may be refreshed after completion. Do not clear continuations or stop work merely because the base moved. Reuse existing execution-slot facts with fail-closed probe handling; the advisory `WorktreeAdmissionRefusal` alone is not proof of quiescence because it intentionally suppresses read errors.

### 2. Reuse the receipt and Git rebase primitive

Under the existing per-workspace operation lock, reload and compare the receipt and recheck the relevant local/remote facts before changing its rewrite identity. Share ordinary begin/recovery helpers where practical; keep policy in the app layer, receipt ownership in workspace, and Git execution in gitcli.

Persist the refreshed rewrite using existing fields:

- `OrigHead`: the current clean local head, including previous resolutions.
- `OrigRemoteHead`: unchanged, retaining the precise publication lease.
- `BaseRef`: unchanged; `BaseHead`: the newly fetched base commit.
- `Attempt`: a fresh rewrite token, rejecting reports/publication requests for the superseded rewrite.
- Resolver budget: preserve its version, snapshotted limit, and used count. Base movement and repeated observations do not replenish dispatch capacity. Only a settled reservation can cross this boundary; legacy receipts are never silently given a budget.
- Gate continuation: empty only after its prior work has settled. Clear the entire old completed-gate checkpoint before the new rewrite can authorize publication.

Write and verify the receipt before calling `BeginRebase` with the pinned local head and new base. Reuse its owned orig/base refs and normal conflict/result routing. Release the operation lock before any suite slice. A successful forward rebase uses normal gate, evidence, publication, and merge paths; it does not push or merge as part of recovery itself.

On conflict, resolver reservations continue from the preserved budget. The existing in-progress abort restores the starting head of this latest rewrite, which includes the prior completed rebase. No completed-rebase reset or new abort primitive is introduced.

### 3. Recover interruptions without another rewrite generation

The refreshed receipt is the durable intent for one pinned rewrite. Identical re-entry must inspect and resume that intent, not mint another token, reset a budget, or blindly run Git twice.

Cover the existing durable-before-effect window explicitly: if the receipt was persisted but Git has not started, no rebase is in progress, and the clean local head still equals its `OrigHead`, resume `BeginRebase` against that receipt's target. Owned scratch anchors are derived from this validated intent and may be recreated/reconciled in this proven pre-start state. This also covers interruption while the two anchor writes are being made. In-progress Git state must agree with the recorded rewrite before it is adopted. Completion is recovered through the existing no-rebase/ancestry proof.

Settle a persisted rewrite before considering a still newer base. If the base advances again during work, a later observation can refresh the next completed, quiescent rewrite; do not introduce a busy-loop, daemon, configurable retry layer, or budget reset. Concurrent re-entry must produce one owned rewrite: the loser reloads the winner's receipt. A late gate or resolver write for the prior token must not overwrite the refreshed receipt; affected receipt writers must compare the rewrite identity they observed before modifying it.

### 4. Test the updated base and scope evidence to it

A base refresh requires the full configured finalize suite on the resulting head, even when Git reports no textual conflicts or an already-contained target. Never reuse the superseded base's completed-gate checkpoint or let PR-body evidence bypass this retest.

Use the existing checkpoint as the durable proof of a completed gate for the refreshed head/base/command/policy/PR identity. Receipt-based completion recovery with no valid checkpoint runs the gate; a live continuation advances the same drive; a fully matching new checkpoint can be reused after a lost response or denied publish. Apply that checkpoint path also when the required gate ran after a mechanically unchanged rebase, so interruption does not require a new persistent flag. Keep the fresh, unchanged-base no-op shortcut where its existing conditions hold.

Existing passed/failed/waiting/halted distinctions, configured gate-off behavior, evidence validation, merge checks, and admission/cancellation policy remain authoritative. Stale evidence and old rewrite tokens cannot authorize publishing the refreshed attempt.

### 5. Repair finalize no-op producers

Make the repeated same-attempt block plan and absent-marker clear-block plan carry valid operation-specific subject/receipt metadata with zero file mutations. Keep transaction validation and its no-commit/no-push no-op behavior. Do not weaken the engine or invent a new no-op protocol.

Retain comment-first idempotency, exact-version checks, and clear-block's PR/head/evidence checks. Return the requested change id even when the engine persists no receipt for a no-op. A repeated request must not create another comment, marker, metadata commit, or board change.

### 6. Documentation and compatibility

Update the finalize workflow's moved-base guidance to describe forward recovery, normal conflict handling, and retesting. Retained ambiguity still halts; generic `blocked` is never permission to reset. Preserve change 0411's same-report `rebase-continue` remedy for reconciliation-write failures and 0439's admission diagnostics. Regenerate embedded assets through the established process.

No CLI operation, argument, configuration key, receipt field, lifecycle status, or independent store is added. Existing receipt schemas remain readable. This refines recovery by starting a validated rewrite with a new target; it does not reuse an old-base checkpoint contrary to ADR-0112. Existing ADRs remain unchanged.

## Acceptance and validation

Extend the existing real-Git finalize/workspace fixtures and controlled GitHub/gate seams:

1. Complete a rebase, including a resolver-authored resolution; interrupt before publication; advance the base; invoke `finalize.rebase` again. Verify forward rebasing preserves the resolution, includes the new base, runs the suite on the new head, and never resets to the original pre-first-rebase head.
2. Verify the receipt's local starting head changes, remote lease does not, token changes once, resolver limit/usage are preserved, and old evidence/report/token cannot publish or continue the refreshed rewrite. Exercise unchanged-base recovery and new checkpoint reuse as well.
3. Interrupt after refreshed receipt persistence, during anchor preparation, after Git completion, and during/after the gate. Re-entry must resume the recorded target, avoid duplicate Git continuation or suite launch, and never accept an earlier base's evidence. Cover another base advancement and concurrent re-entry.
4. Preserve refusal behavior for dirty/foreign state, invalid ownership, divergent base, changed remote/PR, malformed receipt, unresolved work, and probe failures. Verify head, files, receipt, and remote remain unchanged when admission refuses. Keep in-progress conflict/abort and carried-descendant coverage.
5. Verify the base-changed but mechanically unchanged rebase still retests, including response-loss recovery; valid current checkpoint reuse remains possible afterward.
6. Exercise repeated block and clear-block through the real transaction engine, checking no-op dispositions, correct change id, stable metadata/board bytes and remote revision, no duplicate comments/markers, and stale-version refusal. Planning-only tests are insufficient.

Mutation-test new safety guards against the behavior they protect. At the build gate, run the whole suite through the source entry resolved from `build.test_command` (currently `go run ./cmd/docket development test`); inspect budget findings even on green. Use existing topical test shards per `tests/README.md`. Grooming runs no suite and modifies no product code.

## Alternatives and scope

Forward rebasing through the existing operation is chosen because it preserves work and uses existing receipt, Git, gate, and publication machinery. Reset-then-rebase is rejected as unnecessary loss of prior resolution work. Merely removing the moved-base refusal is insufficient: the target, interruption recovery, gate proof, and publication authority must move together.

Out of scope: completed-rebase rollback, the withdrawn completion-head field, new recovery commands, generalized scratch-cleanup repair, automatic cancellation, new retry policy, budget replenishment, transaction-engine relaxation, force-rewritten-base recovery, recovery after the local rewrite has already been published, and repair of the already-closed 0368 incident. No implementation plan or implementation is part of this grooming.

There are no dependencies or stacked parent. Preserve medium priority and fix type. The change's historical title describes the incident; its proposal and this spec define the agreed forward-rebase solution.
