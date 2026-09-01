---
id: 390
slug: 'maintenance-sweep-scope-full-re-probes-the-remote-per-item-h'
title: 'maintenance sweep --scope full re-probes the remote per item, hanging the sweep'
status: 'done'
priority: 'critical'
type: 'fix'
created: '2026-08-31'
updated: '2026-09-01'
depends_on: []
stacked_on:
related: [389]
discovered_from: [389]
adrs: [101]
spec: 'docs/superpowers/specs/2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h-design.md'
plan: 'docs/superpowers/plans/2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/maintenance-sweep-scope-full-re-probes-the-remote-per-item-h'
pr: 'https://github.com/danielhanold/docket/pull/262'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h-design.md) |
| Plan | [2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h.md) |
| ADRs | [ADR-0101](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0101-maintenance-sweep-scope-defer-historical-cleanup-out-of-impl.md) |
<!-- docket:artifacts:end -->

## Why

`docket maintenance sweep --scope full` repeats repository setup work for every eligible
historical record, making full maintenance slow and delaying docket-status's subsequent health
report. An investigation stopped a sweep after 1m25s without completion; individual healthy remote
commands took about 0.5s, and a goroutine dump showed a remote probe in flight. These observations
support redundant network work as a contributor, not a measured completion time or proof that it
is the only cost.

The initial inventory, each sweep reload, and each dispatched operation currently pin the whole
operational context. That repeats default-branch discovery and source/configuration setup as well
as fetching metadata. Fresh metadata is necessary: another agent, or the sweep's own preceding
closeout, can change the record before the next operation. The setup probes can instead be shared
within one invocation while metadata is fetched afresh for actual operation attempts. Historical
records with no possible cleanup effect should not trigger individual remote reads at all; shared
metadata/source/ref inventory and local workspace inspection can assess them. Active PR discovery
also needs batching instead of repeated repository resolution and individual PR queries. The
linked spec corrects the original call-count diagnosis and preserves fresh mutation proofs.

Git and GitHub clients also inherit five-minute network timeouts. A stalled context read, proof
query, or write can therefore add a long wait independently of the redundant work.

## What changes

- Resolve repository identity, configuration, and source revisions once for the sweep. Batch
  active PR selection by exact number (25 unique PRs per request), and collect historical remote
  branch heads in one shared read when needed. Assess all full-scope historical candidates from
  shared metadata/source/ref snapshots and local workspace facts. Non-actionable records get
  truthful no-work/retained/blocked/unknown entries without individual remote queries; missing
  manifests are never labeled clean. Independent backlink repairs must remain discoverable.
- For candidates that warrant a dispatched operation, fetch
  metadata once for each operation attempt and share that exact observation between the sweep's
  check, the operation, and its nested readers. Remove repeated setup probes and duplicate
  metadata refreshes. The initial metadata fetch already supplies every record for selection;
  the additional refreshes protect operations, not enumeration of individual records.
- Bound every sweep Git/GitHub remote read to 30 seconds and remote write to 60 seconds, including
  the transaction and cleanup paths. Preserve mutation proofs and uncertain-outcome handling.
  Standalone command defaults stay unchanged; the spec defines separate read/write budgets and
  prevents timeout multiplication inside adapter failure classification.
- Verify reduced setup traffic through production wiring, real metadata-race detection, bounded
  read failures, and measured before/after performance. Adding non-actionable historical records
  must not add remote calls. Tests permit batched discovery and required action-specific traffic;
  neither zero total network calls nor a total sweep deadline is promised.

Shared observations can justify doing nothing, never a mutation. Work appearing after a snapshot
assessment is reconsidered next invocation; mandatory closeout suffixes always receive fresh
checks. The existing full/implementation worklists and mutation safety rules remain unchanged. Change
0389 is already done; ADR-0101 continues to govern scope selection. Design detail and acceptance
criteria are in the linked spec.

## Out of scope

- Changing scope membership, closeout/cleanup/reclaim eligibility, or their ownership and merge
  proofs; eliminating pre-dispatch metadata freshness or transaction compare-and-swap checks.
- Changing package-wide Git/GitHub defaults or retry policy; adding a total sweep timeout, circuit
  breaker, streaming output, or automatic maintenance schedule. Sweep-specific read/write
  deadlines are explicitly in scope.
- Batching mutations. Batched discovery and non-mutating historical assessment are in scope.
- Reworking the health-check pass, the separate test-suite runtime work, or general corpus
  parsing costs beyond sharing inventory and operation observations.
- Implementing code or running a live mutating sweep as part of this re-grooming.

## Reconcile log

### 2026-08-31

### 2026-08-31 — reconcile (docket-implement-next)

Design confirmed current against `origin/main` at `c95e5189`, the exact source revision the spec targets. Verified in-tree: `maintenanceSweep` / `MaintenanceSweep` (internal/app/maintenance.go), `loadOperationalContext` and `gatherRepoFacts` (internal/app/operational_context.go, repository_facts.go), `sweepReloadVersion`, the `githubFinalizeProber` PR prober, and `ProbeRemoteBranch` all still present with the structure the spec describes. Dependency change 0389 is `done` (archived); ADR-0101's full/implementation worklists and deferral rules stand. Relations (related [389], discovered_from [389], adrs [101]) already correct — left untouched. Scope unchanged: batched PR selection (25/req GraphQL aliases), single shared remote-heads advertisement, local snapshot assessment of historical `done`/`stacked-merged` candidates without per-item remote reads, one shared metadata observation per dispatched operation attempt, and sweep-only 30s read / 60s write network deadlines. No obsolescence and no fundamental invalidation found; proceeding to plan + build.
