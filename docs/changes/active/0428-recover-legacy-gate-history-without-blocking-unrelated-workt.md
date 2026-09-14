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
claimed_at: '2026-09-14T18:05:33Z'
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
