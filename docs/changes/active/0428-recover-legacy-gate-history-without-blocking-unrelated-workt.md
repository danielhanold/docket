---
id: 428
slug: 'recover-legacy-gate-history-without-blocking-unrelated-workt'
title: 'Recover legacy gate history without blocking unrelated worktree admission'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-09-14'
updated: '2026-09-14'
depends_on: []
stacked_on:
related: [375, 427]
discovered_from: [375]
adrs: [87, 95, 118]
spec: 'docs/superpowers/specs/2026-09-14-recover-legacy-gate-history-without-blocking-unrelated-workt-design.md'
plan: 'docs/superpowers/plans/2026-09-14-recover-legacy-gate-history-without-blocking-unrelated-workt.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/recover-legacy-gate-history-without-blocking-unrelated-workt'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-14T18:17:08Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-14-recover-legacy-gate-history-without-blocking-unrelated-workt-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-14-recover-legacy-gate-history-without-blocking-unrelated-workt-design.md) |
| Plan | [2026-09-14-recover-legacy-gate-history-without-blocking-unrelated-workt.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-14-recover-legacy-gate-history-without-blocking-unrelated-workt.md) |
| ADRs | [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Testing Docket after change 0375 on an existing consumer repository exposed a first-admission blocker: schema-2 gate-drive history under the repository's Git common directory is rejected by the newer reader, and the repository-wide legacy inventory reports unresolved-execution before unrelated work can start. The current source writes schema 4 and accepts schema 3; schema 2 remains unsupported. The reported baseline gate.drive.start returned invalid-input / unresolved-execution with the misleading message 'a prior execution in this worktree is unresolved; recover it through the parent or run.cancel, never a blind re-start' and no incumbent drive locator. No TDD cycle, edits, or commit occurred. Scope 08ae86c490d6d19fc55602e39ba0482d had drive_count:0 and closed:false. Empty workspace operation.lock (workspace for consumer change 16, c610db33…) and gate-admission/v1/c4725d99…/lock files were observed and left untouched; empty lock files alone do not establish a live or abandoned execution. Source inspection confirms inventoryLegacyDrives fails on unsupported records before filtering by worktree, while the application discards the internal inventory-legacy-drive-<id> operation locator in its public refusal.

## What changes

Unblock implementation starts in repositories containing completed schema-2 gate history from before change 0375. Give the existing admission inventory an explicit historical reader, recognize trustworthy completed drives before resolving potentially removed worktree paths, and reuse existing process recovery evidence for safely recoverable HALTED history. Perform this assessment inside the original admission so the implementation can reach its baseline without manual cleanup or a second start attempt. Share that assessment with a small, idempotent gate.history.cleanup command offering a specific drive selector and dry-run. Preserve original records, execution-reader schema boundaries, current admission and cancellation authority, and suite budgets. Carry the exact credential-free inventory-legacy-drive-<id> locator and a compact recovery summary through successful and refused starts. The approved spec defines the safety conditions and behavioral acceptance tests.

## Out of scope

Implementation or implementation planning during grooming; manually changing a consumer repository's .git state; drive retirement receipts or a new retired-drive lifecycle; a registry-wide reference census; age filters, log pruning, general garbage collection, or disk-space reclamation; schema rewrites or making schema-2 records executable; new start retry controllers, recursive cleanup, process cancellation, or changes to suite budgets and ownership fences; treating unknown or unprovable state as safely inactive; the separate verdict-path epoch binding fix tracked by 0427.

## Reconcile log

### 2026-09-14

2026-09-14 — Reconciled against current main (06ebb52c, identical to the spec's stated baseline; no drift since grooming). Confirmed the spec's cause is still true in source: internal/gatedrive/admission.go inventoryLegacyDrives loads each pre-admission record via s.Load(id), which fails closed on the unsupported schema-2 record (ErrUnknownSchema) before filtering by worktree, and resolves the historical worktree path (admissionKeyFor) before recognizing a terminal PASSED/FAILED drive — so a removed historical worktree or a schema-2 record blocks an unrelated admission. internal/app/gate_drive.go:667-668 emits the misleading generic 'a prior execution in this worktree is unresolved' message and drops the internal inventory-legacy-drive-<id> locator. No gate.history.cleanup operation exists yet in internal/app, internal/gatedrive, or cmd. Execution reader still accepts schemas 3/4 and rejects 2 (driver.go). ADR-0087/0095/0118 invariants unchanged and preserved. Related 375 is done; 427 remains a separate proposed change (verdict-path epoch binding) left out of scope. Scope, relations, and design remain valid as authored — no section or relation edits required.

## Run halted

### 2026-09-14

Autonomous implement-next run halted at the build gate (Step 5). A human decision is required to clear a self-hosting environment blocker; all work through plan attachment is intact and the change stays in-progress.

### What is done

- Claimed, reconciled (design confirmed valid against current main 06ebb52c — no drift), workspace prepared, and the implementation plan (10 TDD tasks) authored, backlinked, committed on `fix/recover-legacy-gate-history-without-blocking-unrelated-workt` (plan commit `c8e350d3`), and attached (`plan:` set).
- Task 1's failing tests are staged (untracked) in the feature worktree at `internal/process/recover_classify_test.go` by the build worker; no production code changed.

### The blocker (a self-hosting bootstrap of change 428's own bug)

The build worker for Task 1 returned BLOCKED: every `gate.drive.start` in the feature worktree is refused `invalid-input / unresolved-execution` before any test runs. Root cause, verified by direct inspection:

- The installed docket binary (`v0.9.3-1016-g06ebb52c`) runs `inventoryLegacyDrives` on first admission, which calls the executable drive reader `s.Load(id)` over every record in the repository's `.git/docket/gate-drives/v1` store.
- That store holds 989 pre-0375 records — 201 at schema 1 and 788 at schema 2, and zero at the executable schemas 3/4. The reader fails closed with `ErrUnknownSchema` on the first schema-2 record and returns `ErrUnresolvedExecution` before ever filtering by worktree.
- This is exactly the defect change 428 fixes (recognize/handle legacy schema-2 history without blocking unrelated worktree admission), now reproduced live on the change's own repository. It blocks the gate that the build workers must use to run their focused tests — so the fix cannot be TDD-built through the gate until the environment is unblocked.
- No live process holds the store; all 989 records are historical/terminal pre-0375 state (an active drive would be schema 4, of which there are none). A stale, record-less admission lock dir for this worktree (`.git/docket/gate-admission/v1/c14e739…/lock`, born 2026-09-14 14:20:56) is also present.

### Remedy (needs human authorization)

Either is sufficient; both are safe and reversible, and neither touches the change's product scope:

1. Preferred, reversible: move the disposable pre-0375 gate history aside so the current binary's inventory finds an empty store, e.g. `mv .git/docket/gate-drives/v1 .git/docket/gate-drives/v1.pre0428-backup` and remove the empty stale admission lock dir `.git/docket/gate-admission/v1/c14e73912c64f0cac9ea9d83cd1a91aa0d125f68eb87304c96b832aed593d628`. This is local, gitignored run-state; the backup preserves history. Then re-dispatch this change by id to resume the build from Task 1.
2. Or install a docket binary carrying this fix once implemented, and re-run the gate with it.

The autonomous run could not perform option 1: the fs mutation of `.git/docket` was denied by the harness auto-mode permission classifier (Modify Shared Resources), which directs that the user decide. No safer permitted path exists, since focused tests must run through the blocked gate driver.
