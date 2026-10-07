---
id: 490
slug: 'replace-the-durable-worktree-admission-slot-with-a-superviso'
title: 'Replace the durable worktree admission slot with a supervisor-held kernel lock'
status: 'done'
priority: 'critical'
type: 'refactor'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: [489]
stacked_on:
related: [375, 428, 435, 437, 439, 441, 446, 452, 453, 457, 488, 491, 492]
discovered_from: []
adrs: [95, 118, 120, 124, 125, 132]
spec: 'docs/superpowers/specs/2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso-design.md'
plan: 'docs/superpowers/plans/2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso.md'
results: 'docs/results/2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/replace-the-durable-worktree-admission-slot-with-a-superviso'
pr: 'https://github.com/danielhanold/docket/pull/366'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso-design.md](../../superpowers/specs/2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso-design.md) |
| Plan | [2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso.md) |
| Results | [2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-10-02-replace-the-durable-worktree-admission-slot-with-a-superviso-results.md) |
| ADRs | [ADR-0095](../../adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0118](../../adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0120](../../adrs/0120-historical-gate-drive-schemas-are-assessed-never-executed.md), [ADR-0124](../../adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md), [ADR-0125](../../adrs/0125-historical-gate-discovery-has-no-global-veto-relevance-to-th.md), [ADR-0132](../../adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md) |
<!-- docket:artifacts:end -->

## Why

Change 0375 (ADR-0118) enforces "one live gate per worktree" with a durable JSON state machine under `<git-common-dir>/docket/gate-admission/`. Its states are reserved, executing, stopping, unresolved, and released.

The state machine fails closed. Any ambiguous outcome leaves the slot closed: a lost launch response, a crash between reserving and launching, an interrupted release, or a HALTED drive whose stop can't be proven. From then on every later gate in that worktree is refused, including the build suite and finalize, until someone recovers the slot. Several of those states have no recovery path at all. The fixes that followed form the longest chain in the repo: 0428, 0435, 0437, 0439, 0441, 0446, 0452, and 0453. (0457 was killed as superseded by 0489.)

The kernel already provides what the slot reconstructs by hand. The gate supervisor holds its run's `live.lock` with flock for its whole life, handed over by the launching CLI, and the kernel releases it when the process dies. A per-worktree lock held the same way makes "busy" mean "a live supervisor holds it", and a dead run frees the worktree with no recovery step.

The earlier objections don't apply:

- 0375's spec rejected "a worktree flock alone" because short-lived CLI calls released it while detached work survived. Here the long-running supervisor holds it.
- ADR-0118 rejected releasing on process death as fail-open. Here the kernel proves the holder is gone.

What remains open is a process-tree gap the slot never closed either: its release also trusted the supervisor's terminal record or its disappearance. Test processes can outlive a supervisor that died alone, and test targets can survive a KILL escalation.

The slot also carries the run id that `run.cancel`, `run.verdict`'s closeout, resume, and the mutation fence read. This change removes those slot readers too. Retiring the run id itself stays with 0491.

## What changes

- **The lock.** Every gate launch first takes a non-blocking exclusive flock on a per-worktree lock file under the git common dir. The key is the worktree root as git reports it, with every symlink resolved, never the caller's spelling. The launching CLI hands the lock to the gate supervisor, which holds it alone (close-on-exec) until its suite ends and releases it last.
  - A held lock refuses the start with `worktree-busy` and charges no suite attempt.
  - A dead run frees the worktree automatically.
  - Finalize's one automatic relaunch takes the lock again, and halts with `worktree-busy` if another gate got there first.
- **Busy diagnostics.** Whoever takes the lock writes a small holder note. A busy refusal names the holder only after confirming that the holder's run is still running.
- **What to retire.**
  - the slot store and its states;
  - `launch-unconfirmed` as an admission refusal;
  - finished-incumbent reconciliation;
  - the first-admission legacy inventory;
  - `gate.history.cleanup`;
  - every reader of the slot's run stamp: between-gate run ownership, the relaunch and recovery run checks, and the mutation fence's slot fallback.
- **Run tracker.** `run.cancel` and the death guardian find a run's drives by the run context the drives already record. A drive counts as torn down once its supervisor is gone. Resume and the success closeout drop their slot steps.
- **ADRs.** A new ADR supersedes ADR-0118 and records the accepted losses and the known process-tree gaps. ADR-0120, ADR-0124, and ADR-0125 get Update notes for the parts it replaces.
- **Follow-up.** The process-tree teardown gaps go to 0492.

Accepted losses: between-gate run ownership, the mutation fence's fallback when a run record is corrupted, and the record of who last held the worktree.

## Out of scope

- Retiring the run id, its start-time check, the `--run-id` flags, and the run-tracker prose (0491).
- The build-owned drive protocol: start/advance, owner generation, handoff/claim.
- Fingerprinting, and the stored terminal result record.
- Finalize's rebase gate logic, apart from its admission call.
- Making suite teardown complete when a supervisor dies alone or a stop escalates to KILL (0492).
- Deleting the old `gate-admission` directory from disk.

## Reconcile log

### 2026-10-02

Reconciled against main at 1fb39868f (change 0489 merged and archived). Every slot symbol the spec names to delete or rewire (admissionRecord, ReserveWorktreeExecution*, releaseAdmissionIfProven, inventoryLegacyDrives, GateHistoryCleanup, reconcileWorktreeSlot, censusReferences, resolveDriveRun, accountCompletionSlot, slotNamedRunUnresolved, authorizeRelaunch, recoveryRunRevoked, settleStaleReleasedRun, rawStaleRunRefusal, ReconcileFinishedIncumbent) is still present, and FindScopeDriveIDs (the context-hash attribution path) exists. The spec was groomed today against post-0489 reality; no scope change. 0491 and 0492 remain follow-ups.
