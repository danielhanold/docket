---
id: 470
slug: 'install-the-no-real-git-test-guard-in-internal-gatedrive'
title: 'Install the no-real-git test guard in internal/gatedrive'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: [466]
stacked_on:
related: [466]
discovered_from: [466]
adrs: []
spec:
plan:
results:
trivial: true
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

Change 0466 moved gatedrive's real-git and real-process tests (about 12 of them) behind the `integration` build tag to bring `test_go_race` back under its 60s budget row. It installed the shared no-real-git guard (`testsupport.InstallNoGitGuard`) in `internal/app`, `internal/repository/transaction` and `internal/workspace`, but not in `internal/gatedrive`. Today the only thing that would notice a real-git test creeping back into gatedrive's default (non-integration) corpus is its budget row — a slow, indirect, machine-dependent signal. The guard fails the test directly and names the offender.

## What changes

- Add a `TestMain` to `internal/gatedrive` (it has none today) that installs `testsupport.InstallNoGitGuard`, mirroring the `main_test.go` wiring 0466 added in `internal/repository/transaction` and `internal/workspace` (the helper returns an error and the `TestMain` owns the exit, per `TestProcessExitSitesAreAllowlisted`).
- Add a mutation test in gatedrive, mirroring 0466's per-package `nogit_guard_test.go`, proving the guard goes red when a default-corpus test runs git.
- Confirm gatedrive's default corpus is git-free under the guard (a quick grep of its non-integration `*_test.go` files found no git / `exec.Command` use); move any stragglers the guard catches behind the `integration` tag.
- Keep gatedrive's budget rows unchanged unless re-measurement shows the guard itself costs time.

## Out of scope

- Changing the guard helper itself or its behavior in the packages 0466 already covers.
- Installing the guard in any package other than `internal/gatedrive`.
- Re-tuning the `test_go_race` / `test_go_toolchain` budget rows or the race backstop.

## Open questions

None. Trivial: this reapplies 0466's established pattern (`TestMain` + `testsupport.InstallNoGitGuard` + a per-package mutation test) to one more package, with no design choices. The only unknown is whether the guard catches a default-corpus git straggler in gatedrive, and the fix for that is 0466's existing move behind the `integration` tag.
