---
id: 459
slug: 'worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte'
title: 'Worker''s gate.drive.acknowledge is refused scope-closed after the parent claims its WAITING drive'
status: 'done'
priority: 'high'
type: 'fix'
created: '2026-09-25'
updated: '2026-09-26'
depends_on: []
stacked_on:
related: [460]
discovered_from: [458]
adrs: []
spec: 'docs/superpowers/specs/2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte-design.md'
plan: 'docs/superpowers/plans/2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte.md'
results: 'docs/results/2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte'
pr: 'https://github.com/danielhanold/docket/pull/337'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte-design.md) |
| Plan | [2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte.md) |
| Results | [2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-09-26-worker-s-gate-drive-acknowledge-is-refused-scope-closed-afte-results.md) |
<!-- docket:artifacts:end -->

## Why

During change 0458's implement-next run, Task 2's worker ran a package-test drive that handed off as WAITING. The parent claimed the drive and advanced it to PASSED. The worker then made its planned commit (0c2534e1), but its `gate.drive.acknowledge` was refused with `scope-closed` because the claim had moved authority over the scope to the parent. The worker reported BLOCKED even though its work had landed and the tree was clean. A parent that trusts the worker's status could then treat a completed task as failed, re-dispatch it, or halt the run.

## What changes

Keep the gate-drive authority model strict and fix the contracts around it. A parent's claim of a handed-off drive still closes the worker's recovery scope.

- **Worker contract** (docket-build-task): a worker that handed off (`WAITING`) never acknowledges or reuses its original scope when continued. It reports on the terminal verdict the continuation supplies, and runs further test drives only under a fresh scope bundle.
- **Parent contract** (docket-build): the continuation carries the claimed drive's id and terminal verdict, says the original scope is closed, and includes a freshly prepared scope bundle when more drives may be needed.
- **Binary**: `gate.drive.acknowledge` and a scoped `gate.drive.start` on a scope closed by a claim or takeover return a distinct typed refusal, `scope-transferred`, whose message names the real state and never says "return BLOCKED". A normally finished scope keeps `scope-closed`. The parent-side takeover path is unchanged.
- A regression test reproduces the handoff → claim → advance → acknowledge sequence, alongside contract guards in `internal/repoguard`.

## Out of scope

Broader gate-drive handoff/claim redesign (claim keeps closing the scope); the parent-side takeover path and its `scope-closed` semantics; the artifact.backlink absolute-path issue found in the same run (change 0460).

## Reconcile log

### 2026-09-26

2026-09-26 — Reconciled against main d1ca501b. Every code site the spec names still matches: ErrScopeClosed in ownership.go, the Acknowledge closed-scope branch (acknowledge.go), the scoped-start closed-scope check (driver.go), the ownershipNextAction scope-closed message (internal/app/gate_drive.go), and the takeover / bind-scope-change paths that must keep scope-closed. The worker contract still mandates acknowledge-or-BLOCKED. Change 0458 (discovered_from) is merged; 0460 stays separate. No scope change.
