---
id: 569
slug: 'flaky-and-failing-tests-standing-bucket'
title: 'Fix the flaky run-cancel worktree test'
status: 'proposed'
priority: 'low'
type: 'chore'
deliverable: 'pr'
created: '2026-10-18'
updated: '2026-10-20'
depends_on: []
stacked_on:
related: []
discovered_from: [562]
adrs: [132]
spec: 'docs/superpowers/specs/2026-10-20-flaky-and-failing-tests-standing-bucket-design.md'
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
| Spec | [2026-10-20-flaky-and-failing-tests-standing-bucket-design.md](../../superpowers/specs/2026-10-20-flaky-and-failing-tests-standing-bucket-design.md) |
| ADRs | [ADR-0132](../../adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md) |
<!-- docket:artifacts:end -->

## Why

`TestIntegrationRunCancelFreesWorktreeForNextStart` failed once during #562's full parallel suite (`cancellation-pending` with a `claim-busy` finding) and passed 3 of 3 alone. When it fires, it fails a build or finalize gate for no reason and costs an attempt.

The cause is in the test, not in run-cancel. The test cancels as soon as the suite reports "running", but the code that started the suite holds its launch claim a moment longer, until the run is recorded. A cancel in that gap correctly answers "not finished yet" (ADR-0132), and the parallel suite's disk load widens the gap.

## What changes

The test waits until the launch has finished, meaning the launcher has released its claim, before it cancels. It uses a deterministic signal, not a timer, and still asserts that one cancel reports `cancelled`. The fix is proven by making the race happen on demand with a temporary delay: the old test goes red, the fixed test stays green. The builder also sweeps the suite for other tests that cancel the same way. Test-only: no production code changes.

## Out of scope

Changing cancel, census, or launch-claim behavior. Any other flaky test (a later one gets its own change). Slow tests and budget breaches (the budget report owns those).
