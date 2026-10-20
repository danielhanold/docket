---
id: 569
slug: 'flaky-and-failing-tests-standing-bucket'
title: 'Fix the flaky run-cancel worktree test'
status: 'done'
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
plan: 'docs/superpowers/plans/2026-10-20-flaky-and-failing-tests-standing-bucket.md'
results: 'docs/results/2026-10-20-flaky-and-failing-tests-standing-bucket-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/flaky-and-failing-tests-standing-bucket'
pr: 'https://github.com/danielhanold/docket/pull/438'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-20-flaky-and-failing-tests-standing-bucket-design.md](../../superpowers/specs/2026-10-20-flaky-and-failing-tests-standing-bucket-design.md) |
| Spec (merged) | [2026-10-20-flaky-and-failing-tests-standing-bucket-design.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/specs/2026-10-20-flaky-and-failing-tests-standing-bucket-design.md) |
| Plan | [2026-10-20-flaky-and-failing-tests-standing-bucket.md](../../superpowers/plans/2026-10-20-flaky-and-failing-tests-standing-bucket.md) |
| Results | [2026-10-20-flaky-and-failing-tests-standing-bucket-results.md](../../results/2026-10-20-flaky-and-failing-tests-standing-bucket-results.md) |
| ADRs | [ADR-0132](../../adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md) |
<!-- docket:artifacts:end -->

## Why

`TestIntegrationRunCancelFreesWorktreeForNextStart` failed once during #562's full parallel suite (`cancellation-pending` with a `claim-busy` finding) and passed 3 of 3 alone. When it fires, it fails a build or finalize gate for no reason and costs an attempt.

The cause is in the test, not in run-cancel. The test cancels as soon as the suite reports "running", but the code that started the suite holds its launch claim a moment longer, until the run is recorded. A cancel in that gap correctly answers "not finished yet" (ADR-0132), and the parallel suite's disk load widens the gap.

## What changes

The test waits until the launch has finished, meaning the launcher has released its claim, before it cancels. It uses a deterministic signal, not a timer, and still asserts that one cancel reports `cancelled`. The fix is proven by making the race happen on demand with a temporary delay: the old test goes red, the fixed test stays green. The builder also sweeps the suite for other tests that cancel the same way. Test-only: no production code changes.

## Out of scope

Changing cancel, census, or launch-claim behavior. Any other flaky test (a later one gets its own change). Slow tests and budget breaches (the budget report owns those).

## Reconcile log

### 2026-10-20

2026-10-20: Re-traced against main 731b8edd9. The spec still holds: Driver.launchScopeless still runs Launch, attachLaunch, then claim.close(); the test still cancels right after awaitRunPhase("running"); the only other running-phase caller (the vanished case of TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled) still waits on awaitStart before cancelling. No scope change.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: b5e6d4883fb2370c460445d57f762c49a357a57f
ran_at:   2026-10-20T11:04:18Z
<!-- docket:build-evidence:end -->
