---
id: 465
slug: 'test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa'
title: 'test_go_race times out on internal/app in CI (Go''s 10m per-package limit)'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-28'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [308, 332, 333, 373]
discovered_from: []
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

The release-candidate workflow's source-gate job (macos-15 runner) intermittently fails on `test_go_race`: `go test -race -count=1 ./...` hits Go's default 10m per-package timeout in `internal/app` (`panic: test timed out after 10m0s`, `FAIL github.com/danielhanold/docket/internal/app 600.05s`). Seen on runs 36131363772 (2026-09-25, PR for change-unblock) and 36345759919 (2026-09-27, PR #342 / change 0382). It is not a hang: the goroutine dump shows an ordinary 3s-old test in flight each time (TestProductionUnverifiedWorkspaceRetryNeverSettles, TestCompletionParticipantDurableProof), and the package runs ~911 tests. The fast default corpus of internal/app under the race detector has simply grown to sit right at 600s on the CI runner. Green runs confirm the margin is gone: test_go_race took 581s, 621s, 831s, 908s on recent passing runs (budget row in tests/runtime-budgets.tsv is 60s), so pass/fail is a coin flip on runner load. Change 0333 moved the real-git tail behind the `integration` build tag, and its assumption that this gate stays sub-60s in the parallel lane no longer holds. A red source-gate blocks every PR.

## What changes

Hypothesis to validate while grooming (trace first; prefer existing machinery): make test_go_race reliably pass in CI without weakening the race gate. Candidate directions: (1) find what grew the default internal/app corpus under -race (profile per-test time with `go test -race -json`; look for tests that belong behind the `integration` tag per 0333's partition, or sleep/poll-heavy tests that dominate under the detector) and move/trim them; (2) set an explicit `-timeout` on the race run sized to the measured worst case, if the growth is legitimate; (3) revisit test_go_race's lane/budget row now that it is nowhere near 60s (0332's serial-lane reasoning), and why the budget report did not surface an authoritative breach. Acceptance: several consecutive CI source-gate runs green on test_go_race with clear headroom under the package timeout, and the runtime-budgets row reflects reality.

## Out of scope

PR #345 (change 0463)'s red run 36353406865 is a separate, branch-local failure: `internal/cli/gate_test.go:1061: undefined: gateTempDir` — change 0462 (PR #343) deleted `gateTempDir` in favor of `testsupport.TempDir`, and #345's new test still calls it. That branch needs a rebase onto main and the call switched to `testsupport.TempDir`; it is not part of this change. Also out of scope: weakening the race gate (dropping -race, narrowing ./..., or skipping internal/app).
