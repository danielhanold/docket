---
id: 390
slug: 'maintenance-sweep-scope-full-re-probes-the-remote-per-item-h'
title: 'maintenance sweep --scope full re-probes the remote per item, hanging the sweep'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: [389]
discovered_from: [389]
adrs: [101]
spec: 'docs/superpowers/specs/2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-31-maintenance-sweep-scope-full-re-probes-the-remote-per-item-h-design.md) |
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
within one invocation while metadata is fetched afresh. The linked spec corrects the original
call-count diagnosis and preserves that distinction.

The reader also inherits a five-minute Git network timeout. A stalled context read can therefore
add a long wait independently of the redundant work.

## What changes

- Resolve repository identity, configuration, and source revisions once for the sweep. Give both
  its reloads and its dispatched operations an explicit reader for that invocation which performs
  a fresh metadata fetch on every pin, without repeating setup probes. Never reuse an old metadata
  revision as fresh authority.
- Set a 30-second network timeout on the sweep's separate read-only Git client, covering the
  initial context read and later metadata reloads. Preserve the existing clients and deadlines for
  mutation transactions, merge/reachability proofs, branch deletion, workspaces, and GitHub calls.
- Verify reduced setup traffic through production wiring, real metadata-race detection, bounded
  read failures, and measured before/after performance. Tests permit required metadata and
  operation traffic; neither zero total network calls nor a total sweep deadline is promised.

The existing full/implementation worklists and mutation safety rules remain unchanged. Change
0389 is already done; ADR-0101 continues to govern scope selection. Design detail and acceptance
criteria are in the linked spec.

## Out of scope

- Changing scope membership, closeout/cleanup/reclaim eligibility, or their ownership and merge
  proofs; eliminating metadata freshness checks or transaction compare-and-swap checks.
- Changing package-wide Git/GitHub defaults, mutation deadlines, retry policy, or adding a total
  sweep timeout, circuit breaker, streaming output, or automatic maintenance schedule.
- Reworking the health-check pass, the separate test-suite runtime work, or corpus parsing costs.
- Implementing code or running a live mutating sweep as part of this re-grooming.
