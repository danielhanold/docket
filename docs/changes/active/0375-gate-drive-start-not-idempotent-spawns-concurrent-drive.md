---
id: 375
slug: gate-drive-start-not-idempotent-spawns-concurrent-drive
title: '`docket gate drive start` is not idempotent — a re-run spawns a second concurrent drive'
status: 'in-progress'
priority: critical
type: fix
created: 2026-08-30
updated: '2026-09-10'
depends_on: [405]
stacked_on:
related: [359, 376, 405, 412, 420, 421, 422]
discovered_from: [372]
adrs: [87, 95, 107, 111, 115, 116, 117]
spec: 'docs/superpowers/specs/2026-09-10-gate-drive-start-not-idempotent-spawns-concurrent-drive-design.md'
plan:
results:
trivial: false
auto_groomable:
branch: 'fix/gate-drive-start-not-idempotent-spawns-concurrent-drive'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-10T16:28:31Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-10-gate-drive-start-not-idempotent-spawns-concurrent-drive-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-10-gate-drive-start-not-idempotent-spawns-concurrent-drive-design.md) |
| ADRs | [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0115](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0115-outer-run-gate-retry-budget-is-a-counted-config-snapshotted.md), [ADR-0116](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0116-build-full-suite-repair-bound-is-a-durable-scope-owned-suite.md), [ADR-0117](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0117-sequential-test-drives-within-one-worker-recovery-scope.md) |
<!-- docket:artifacts:end -->

## Why

Repeated gate starts and interrupted implementation runs can leave two executions or two surviving writers acting on one worktree. Change 0405 is now merged: it prevents duplicate starts within one recovery scope and supports sequential tests through explicit acknowledgement. Separate scopes, scopeless starts, and competing automatic relaunches still need worktree-wide admission.

The reported manual-Stop incidents also expose a workflow ownership gap. The detached gate supervisor executes the test command; later commits, pushes and PR creation come from surviving workflow participants. A safe resume must revoke the previous run's authority and establish that its tasks and processes have stopped before admitting a replacement.

## What changes

Build on change 0405's reservation and acknowledgement model. Reserve one gate execution per canonical worktree before launch, including separate scopes, scopeless/raw starts in that worktree, and automatic relaunches. Refuse conflicting starts with safe diagnostics; preserve intentional sequential tests and independent worktrees.

Add explicit run cancellation and durable ownership fencing. Human Stop cancels owned work; cancellation is confirmed only when the registered tasks, processes and admitted workflow actions are accounted for. Connect supported native Stop events to that operation, provide an explicit operator cancellation path, and report unavailable lifecycle integration honestly. Resume admits exactly one replacement after verified shutdown, while ordinary authorized continuation keeps its existing ownership and budgets.

Cover concurrency, interrupted launch/cancellation transitions, stale writers, legacy records, process identity failures and accounting with behavioral regression tests. Update the maintained caller, adapter, schema and generated instruction surfaces together. The linked spec defines the detailed contract and acceptance criteria.

## Out of scope

The underlying test-load flake; changing suite budgets or retry limits; redesigning the foreground/yield behavior tracked by 0412; cross-machine ownership; policing arbitrary programs outside registered Docket participants; undoing completed commits, pushes or PRs; recovering credentials by repeating start; and implementation or implementation planning during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
