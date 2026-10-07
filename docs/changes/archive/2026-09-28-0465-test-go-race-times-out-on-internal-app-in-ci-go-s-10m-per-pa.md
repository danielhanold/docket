---
id: 465
slug: 'test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa'
title: 'test_go_race times out on internal/app in CI (Go''s 10m per-package limit)'
status: 'done'
priority: 'high'
type: 'fix'
created: '2026-09-28'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [308, 332, 333, 362, 373]
discovered_from: []
adrs: [108]
spec: 'docs/superpowers/specs/2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa-design.md'
plan: 'docs/superpowers/plans/2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa.md'
results: 'docs/results/2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa'
pr: 'https://github.com/danielhanold/docket/pull/346'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa-design.md](../../superpowers/specs/2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa-design.md) |
| Plan | [2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa.md) |
| Results | [2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-09-28-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa-results.md) |
| ADRs | [ADR-0108](../../adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

The release-candidate workflow's source-gate job (macos-15 runner) intermittently fails on `test_go_race`: `go test -race -count=1 ./...` hits Go's default 10m per-package timeout in `internal/app` (`panic: test timed out after 10m0s`, `FAIL github.com/danielhanold/docket/internal/app 600.05s`). Seen on runs 36131363772 (2026-09-25, PR for change-unblock) and 36345759919 (2026-09-27, PR #342 / change 0382). It is not a hang: the goroutine dump shows an ordinary 3s-old test in flight each time (TestProductionUnverifiedWorkspaceRetryNeverSettles, TestCompletionParticipantDurableProof), and the package runs ~911 tests. The fast default corpus of internal/app under the race detector has simply grown to sit right at 600s on the CI runner. Green runs confirm the margin is gone: test_go_race took 581s, 621s, 831s, 908s on recent passing runs (budget row in tests/runtime-budgets.tsv is 60s), so pass/fail is a coin flip on runner load. Change 0333 moved the real-git tail behind the `integration` build tag, and its assumption that this gate stays sub-60s in the parallel lane no longer holds. A red source-gate blocks every PR.

## What changes

Make `test_go_race` reliably pass in CI, without weakening the race gate, by making change 0333's partition an enforced invariant: **the default-tag `internal/app` test corpus never starts a real `git` process.**

Grooming measured the cause. Under `-race` at 3 CPUs, the 337 default tests that run real git account for 225s of `internal/app`'s 237s, and the other 574 tests take 11s. The default corpus grew from 256 to about 920 tests after 0333 because new real-git tests landed outside the `integration` tag.

- **Re-partition:** move the real-git, subprocess and process-lifecycle default tests behind `//go:build integration`, into plain (non-race) `internal/app` shard runners with measured budget rows. Genuinely concurrent scenarios go to a race shard.
- **Runtime guard:** a default-build-only test hook makes any real `git` exec fail loudly, whatever code path reaches it, and is proven by a mutation test.
- **Backstop timeout:** an explicit `-timeout` in `tests/test_go_race.sh`, so an overrun fails with a readable message instead of Go's 10m panic.
- **Honest budget:** re-measure and correct the `tests/runtime-budgets.tsv` rows.
- **Budget-state key fix:** key the suite runner's budget state on the repo-relative target path, so overruns accumulate across worktrees and a serial confirmation can actually fire. Today every `.worktrees/<slug>` starts its own streak, which is why nobody was alerted.

Acceptance: several consecutive CI source-gate runs green on `test_go_race` with clear headroom, the guard proven by mutation, the integration contract green, and the budget rows reflecting reality.

## Out of scope

- PR #345 (change 0463)'s red run 36353406865 is a separate, branch-local failure: `internal/cli/gate_test.go:1061: undefined: gateTempDir`. Change 0462 (PR #343) deleted `gateTempDir` in favor of `testsupport.TempDir`, and #345's new test still calls it. That branch needs a rebase onto main with the call switched to `testsupport.TempDir`.
- Weakening the race gate: dropping `-race`, narrowing `./...`, or skipping `internal/app`.
- Guards or re-partitioning for `internal/gitcli`, `internal/githubcli`, or other packages.
- Persisting budget state across CI runs, or changing how CI classifies screening findings.
- Broad `t.Parallel()` adoption in `internal/app`.

## Reconcile log

### 2026-09-28

2026-09-28 — Reconciled against current main (3f9813fbc). Premise holds: tests/test_go_race.sh still runs a single `go test -race -count=1 ./...` with no -timeout and a 60s parallel row; internal/app TestMain (gate_test.go) still routes only supervisor/guardian re-exec; suiterunner run.go still passes the absolute o.Target.Path to ContextKey; 27 internal/app integration shard runners exist to extend. No related or recent change already addresses this; scope unchanged.
