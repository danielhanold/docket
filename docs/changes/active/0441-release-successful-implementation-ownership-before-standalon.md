---
id: 441
slug: 'release-successful-implementation-ownership-before-standalon'
title: 'Release successful implementation ownership before standalone finalize'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-09-21'
updated: '2026-09-21'
depends_on: []
stacked_on:
related: [375, 407, 433, 435, 437]
discovered_from: []
adrs: [87, 95, 105, 111, 118, 124]
spec: 'docs/superpowers/specs/2026-09-21-release-successful-implementation-ownership-before-standalon-design.md'
plan: 'docs/superpowers/plans/2026-09-21-release-successful-implementation-ownership-before-standalon.md'
results: 'docs/results/2026-09-21-release-successful-implementation-ownership-before-standalon-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/release-successful-implementation-ownership-before-standalon'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-21T22:38:51Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-21-release-successful-implementation-ownership-before-standalon-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-21-release-successful-implementation-ownership-before-standalon-design.md) |
| Plan | [2026-09-21-release-successful-implementation-ownership-before-standalon.md](https://github.com/danielhanold/docket/blob/fix/release-successful-implementation-ownership-before-standalon/docs/superpowers/plans/2026-09-21-release-successful-implementation-ownership-before-standalon.md) |
| Results | [2026-09-21-release-successful-implementation-ownership-before-standalon-results.md](https://github.com/danielhanold/docket/blob/fix/release-successful-implementation-ownership-before-standalon/docs/results/2026-09-21-release-successful-implementation-ownership-before-standalon-results.md) |
| ADRs | [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md), [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md) |
<!-- docket:artifacts:end -->

## Why

A successful implementation can record run-complete while leaving its run epoch active and its released worktree execution slot owned by that epoch. A subsequent standalone finalize gate then receives stale-run-epoch. The reported local history contains 15 successful reports with active epochs and 14 released slots retaining epochs; these are historical observations, not a count of current blockages.

## What changes

Finish successful implementation ownership at the existing keyed run-verdict boundary. Reuse epoch fencing, launch/process/mutation accounting, and ownership-checked slot retirement. Persist observed native-task completion in the existing participant records, and distinguish successful closeout from cancellation so finalize can proceed without inheriting stale authority. Preserve ordinary between-drive exclusion and fail closed on missing evidence. The linked spec records the implementation trace, prior decisions, minimal extensions, replay behavior, and acceptance tests.

## Out of scope

No daemon, background sweeper, new CLI command, configuration policy, generic coordination framework, retry layer, coordinator-topology redesign, or bulk historical cleanup. Do not bypass finalize checks, weaken between-drive ownership, or automatically call human cancellation. Implementation and planning are outside this grooming change.

## Reconcile log

### 2026-09-21

2026-09-21: Reconciled at claim. Design baseline (main 329fa0a0) equals current integration HEAD, so the investigation trace, referenced symbols (RunGateVerdict, ReleaseWorktreeExecution, RetireWorktreeExecutionEpoch, EpochParticipant, findEpochByWorktree, processFinalizeGate.RunLocalGate), and cited ADRs (0087, 0095, 0105, 0111, 0118) are all current. Dependencies 0375/0407/0435/0437 are done; related 0433 remains proposed and is deliberately out of scope. No scope adjustment, relation change, or spec revision required.

### 2026-09-21

2026-09-21: Recorded ADR-0124 (successful-run ownership closeout extends the run-epoch lifecycle with completing/completed) in adrs:, produced by this build per the change's exclusions-and-implementation-boundary. ADR-0118 was extended by cross-link, not rewritten.
