---
id: 108
slug: 'bound-total-go-test-load-at-the-runner-and-isolate-real-proc'
title: 'Bound total Go test load at the runner and isolate real-process test temp dirs behind a shared fixture'
status: 'Accepted'
date: '2026-09-02'
supersedes: []
reverses: []
relates_to: []
change: 373
---

## Context

Under the full-suite gate's parallel load, unrelated tests reddened non-deterministically across builds 0371 and 0397 — a process-group liveness ceiling, an internal/app per-package timeout, and t.TempDir() "directory not empty" teardown races in internal/gitcli and internal/repository/transaction. Root-causing them found two structural mechanisms, not per-test flakes. (A) Oversubscription: the suite runner launches every tests/test_*.sh target at -j=NumCPU, and 34 of those targets run their own `go test` at -p=NumCPU (a topology change 0333 produced when it partitioned internal/app behind the integration build tag), so nothing bounded the product — roughly -j x NumCPU concurrent Go test packages, each spawning real git and supervisor subprocesses. (B) Post-test writers into t.TempDir(): Go's cleanup does a single os.RemoveAll with no retry, which races a still-writing detached child — a Setsid supervisor, or git auto-gc/maintenance kicked off by the test's own repository operations.

## Decision

Test isolation under parallel load is handled structurally, in two places, and no wrapper is pinned to serial execution.

1. Total Go test load is bounded AT THE RUNNER. internal/suiterunner exports a DOCKET_-namespaced concurrency cap into every target's sandbox: DOCKET_GO_TEST_CONCURRENCY = clamp(M * cpus / jobs, 1, cpus), where M is a measured ratio constant (goLoadMultNum/goLoadMultDen, pinned at 2/1). The Go wrappers — tests/lib/go-integration-shard.sh plus the three whole-module wrappers — translate that value into `go test -p <n>` and GOMAXPROCS. Absent the variable (a solo `bash tests/test_X.sh`, or a bare `go test`), Go's own defaults apply unchanged. The cap is honored by the heavy Go test wrappers; the light list/compile wrappers intentionally stay at Go defaults.

2. Every internal package whose tests spawn real git or supervisor processes uses the shared fixture internal/testsupport instead of a bare t.TempDir(). The fixture drains the test's own detached children, then retries os.RemoveAll over a measured tolerance window (cleanupTolerance = 4s); it disables git background work (gc.auto, gc.autoDetach, maintenance.auto, core.fsmonitor) via a single-source constant; and it roots process registries under the fixture's own temp dir rather than a shared $TMPDIR. A fail-closed, mutation-tested repoguard proves no bare t.TempDir() survives in an internal/ real-process package, with the package set derived by grep at test time rather than hand-listed.

3. internal/testsupport is test-only and must NOT import a product package that its own tests then import — the testsupport-to-suiterunner import cycle. That is why the shared git-background-off constant lives in the neutral leaf package internal/gitbg, consumed by BOTH the runner sandbox and the fixture.

## Consequences

Gate flakiness under parallel load is addressed structurally rather than by retry or serialization: five consecutive green full-gate runs at one head, with zero unrelated reds. Future real-process test packages under internal/ MUST adopt testsupport.TempDir — the repoguard enforces it and fails closed. Future suite-runner work must respect the exported DOCKET_GO_TEST_CONCURRENCY cap rather than reintroducing an unbounded -p. The two tuning constants (the load multiplier M and cleanupTolerance) are machine-measured and carry their measurement in-comment, so a re-tune must re-measure rather than guess. Known limitation, tracked as a follow-up: cmd/ real-process test packages (e.g. cmd/docket/gate_cli_test.go) are NOT yet covered by the fixture or the guard.

## Alternatives considered

Pinning the offending wrappers to serial execution: rejected — it hides the oversubscription instead of bounding it, and costs wall-clock on every future run. Bumping the per-package Go test timeout: rejected — it treats a load symptom, and the teardown races are not timeouts. Retrying flaky tests at the runner: rejected — it converts a deterministic structural defect into tolerated noise. Putting the git-background-off constant in internal/suiterunner next to the sandbox: rejected — it creates the testsupport/suiterunner import cycle, hence the neutral leaf package internal/gitbg.
