---
id: 434
slug: 'move-load-sensitive-tests-out-of-the-default-parallel-suite'
title: 'Move load-sensitive tests out of the default parallel suite lane'
status: 'in-progress'
priority: 'medium'
type: 'chore'
created: '2026-09-18'
updated: '2026-09-18'
depends_on: []
stacked_on:
related: [273, 333, 362, 373, 411]
discovered_from: [411]
adrs: [108]
spec: 'docs/superpowers/specs/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite-design.md'
plan: 'docs/superpowers/plans/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/move-load-sensitive-tests-out-of-the-default-parallel-suite'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-18T18:29:55Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite-design.md) |
| Plan | [2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite.md](https://github.com/danielhanold/docket/blob/chore/move-load-sensitive-tests-out-of-the-default-parallel-suite/docs/superpowers/plans/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite.md) |
| ADRs | [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

Change 0411 reported internal/app integration timeouts under -j11 load and a gatedrive concurrent-relaunch failure in both whole-module test wrappers. Isolated passes do not establish that these are environmental false failures. Such failures consume bounded build attempts and block unrelated changes.

The existing integration-tag partition already covers the app shards. The work should extend or rebalance that machinery based on measurements, while independently proving and repairing the gatedrive terminal interleaving.

## What changes

- Measure the implicated app shards and default-corpus gatedrive coverage under comparable parallel and isolated conditions.
- Move only measured slow, eligible default tests behind the existing integration build tag; split or rebalance app tests that are already tagged using the current feature-shard helper and completeness contract.
- Keep all wrappers parallel and every scenario mandatory in the full suite, with concurrency-bearing integration tests retaining race instrumentation.
- Deterministically reproduce and resolve the gatedrive terminal-relaunch failure, preserving its assertions and fast regression coverage.
- Record final-head whole-suite reliability, per-shard and total timing, partition fidelity, and budget findings. Preserve ADR-0108.

## Out of scope

No new lanes or scheduler, serial wrapper pins, automatic retries, timeout increases, attempt-limit changes, weakened assertions, skipped coverage, host-relative budget redesign, or unrelated gofmt cleanup. Reuse the existing tag/shard/concurrency machinery; an unresolved limitation must be reported rather than silently expanding that scope.

## Reconcile log

### 2026-09-18

2026-09-18: Reconciled at claim. Spec was groomed today against the current main HEAD (3ccf9fac), so the design is fresh. Verified current reality: the integration test-shard machinery is present (tests/lib/go-integration-shard.sh and the tests/test_go_integration_*.sh wrappers, including app_rebase, app_concurrency, and gitcli_concurrency), tests/runtime-budgets.tsv carries the per-wrapper ceilings, and the gatedrive symbols named by the spec exist as described (internal/gatedrive/driver.go: reserveRelaunch, errAlreadyTerminal, errRelaunchRaceLost; the driveSlice reserveRelaunch error branch recognizes errRelaunchRaceLost but propagates errAlreadyTerminal; TestConcurrentSameOwnerAdvanceRelaunchesOnce/terminal_relaunch_winner_fails lives in internal/gatedrive/driver_concurrency_test.go). Related/discovered_from/adrs relations (273,333,362,373,411 / 411 / 108) remain accurate. No scope adjustment or relation change required; proceeding to plan and build under the settled design.
