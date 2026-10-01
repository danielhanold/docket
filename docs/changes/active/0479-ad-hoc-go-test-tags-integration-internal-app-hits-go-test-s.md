---
id: 479
slug: 'ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s'
title: 'Refuse an unfiltered integration-tagged run of internal/app before go test''s 10-minute timeout'
status: 'in-progress'
priority: 'low'
type: 'chore'
created: '2026-09-30'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [333, 434, 465, 466]
discovered_from: [472]
adrs: [108]
spec: 'docs/superpowers/specs/2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s-design.md'
plan: 'docs/superpowers/plans/2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-01T12:17:26Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s-design.md) |
| Plan | [2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s.md](https://github.com/danielhanold/docket/blob/chore/ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s/docs/superpowers/plans/2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s.md) |
| ADRs | [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

`go test -tags integration ./internal/app/` with no `-run` filter runs the whole integration corpus of `internal/app` in one process. That takes about 19 minutes, longer than go test's default 10-minute per-package timeout, so the run dies at 10 minutes with a goroutine-dump panic. Change 0472's build lost a verification step this way, and the command keeps appearing in plans: five so far, three of them dated 2026-09-28 to 2026-09-30.

The trace at grooming found that neither the suite runner nor the gate drive is affected. The suite runs this corpus only through prefix-filtered shard runners, each well under a minute. Only hand-typed and plan-prescribed runs hit the limit, and nothing tells the caller the supported forms until the 10 minutes are gone.

## What changes

`internal/app`'s integration-tagged test binary refuses an unfiltered run at go test's default timeout, right after compile, and prints the supported forms: run a shard runner, filter with `-run '^<Prefix>'`, or pass `-timeout 30m` for a deliberate whole run. The check lives in `internal/testsupport` next to the existing no-real-git guard and follows its build-tag split; only `internal/app` calls it.

`tests/README.md` gains a short section on running integration-tagged Go tests by hand. A cheap end-to-end test proves the refusal and turns red if the guard or its call is removed.

## Out of scope

Timeout changes to the suite, the shard runners, or the race gate (0465, ADR-0108); other packages' integration corpora; AGENTS.md and the learnings ledger; editing merged plans; weakening or skipping integration tests; and the closeout budget breach (0475).

## Reconcile log

### 2026-10-01

Re-traced against origin/main 85bace7de: `internal/testsupport` still holds only the no-real-git guard pair (`nogit_install.go` / `nogit_install_off.go`), `internal/app/gate_test.go` `TestMain` still passes `nogitPkg`/`nogitShardGlob` to `InstallNoGitGuard`, `tests/test_go_integration_contract.sh` keeps its 15s row, and the `internal/app` shard ceilings still sum to 1125s. No related change has landed this work; scope and spec stand unchanged.

## Run halted

### 2026-10-01

The build role halted on Task 1 (`internal/testsupport` unfiltered-run guard). The `docket-build-standard` worker returned `BLOCKED`, which is a halting condition in docket-build.

- **Cause:** the plan runs Task 1's mutation checks inside a gate drive, and those checks edit `internal/testsupport/unfiltered.go` in place while the drive is running. The driver halted that drive with `worktree-changed` (drive `17a7ffb4cdc6d9c4670f910ec7b3e442`). After that, scope `7e140da6585286d1681a5c04234a2827` refused any further start: `predecessor-not-reusable` when chained to the halted drive, and `scope-second-live-drive` without a predecessor. The scope was never acknowledged.
- **Code state:** RED (drive `5691b297…`, the intended build failure) and GREEN (drive `0dc01a08…`, PASSED) both completed. The worker reports that all three clause mutations reddened the unit tests and that each was restored byte-identical. The four Task 1 files are **uncommitted** in the feature worktree: `internal/testsupport/unfiltered.go`, `unfiltered_guard.go`, `unfiltered_guard_off.go`, `unfiltered_test.go`. Nothing was committed beyond the plan (`edb7def52`).
- **To resume:** check the four files, prepare a fresh scope, run one clean GREEN drive, and commit them as Task 1. Then continue to Task 2 through `change.resume-halted`. Mutation steps in this plan (Task 1 and Task 2's wiring mutation) should run outside a gate drive, or in a copy of the tree, because the driver halts on any edit made during a run.
