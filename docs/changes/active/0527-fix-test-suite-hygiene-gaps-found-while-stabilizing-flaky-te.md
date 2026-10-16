---
id: 527
slug: 'fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te'
title: 'Fix test-suite hygiene gaps found while stabilizing flaky tests'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-16'
depends_on: []
stacked_on:
related: [273, 528, 536]
discovered_from: [507, 531]
adrs: []
spec: 'docs/superpowers/specs/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-design.md'
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
| Spec | [2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-design.md](../../superpowers/specs/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 507 (flaky-test stabilization) surfaced several unrelated test-suite defects that no existing change covers. Each lets a test silently stop running, fail for environmental reasons, or fail falsely.

Change 531's build surfaced two more: a gate-driver test that fails under disk load, and runtime budgets that were raised where `tests/README.md` says to split the test instead.

## What changes

Seven small test-suite fixes. The first six were re-traced against `main` on 2026-10-16; the seventh is diagnose-first (the linked spec has the detail):

- **Unrunnable test functions.** A repo-wide guard in `internal/repoguard` flags any function shaped like a test (`func(*testing.T)`, `*testing.B`, `*testing.F`) that `go test` will never run because of its name and that nothing calls. This catches the e2e case (renaming `TestE2EFoo` to `testE2EFoo` leaves `tests/test_go_finalize_e2e.sh` green) and the same mistake in every other test file.
- **No-git guard path.** `InstallNoGitGuard` stores the temp dir uncleaned, so `TestNoGitGuardShadowsGitOnPath` fails when `TMPDIR` is not in clean form (for example `/tmp//`; a single trailing slash is fine). Store the cleaned path.
- **Bare `EOF` from the codex client.** The `TestAgentEnterCLIPreservesRequestAndReceipt` EOF did not reproduce (0 of 230 runs). The real gap: the codex client reads the app-server's stderr before its copy finishes, so a crashing server's message can be lost, and the test's fake server exits silently on every failure branch. Fix the client to capture stderr before reporting a read error (bounded, never hangs), and make the fake server say why it exits.
- **Exit-sites guard walks worktrees.** `TestProcessExitSitesAreAllowlisted` walks into `.worktrees/` and fails from the main checkout. Switch it to the shared `repoguard.MaintainedFiles` walk.
- **Gate-driver tests that race a timer.** `child-signal-death` (failing 34 of 40 under disk load) and three other gate-driver tests depend on a child staying alive for a fixed number of milliseconds while several fsync'd writes happen. Make the child wait for a go-signal file the test creates instead of a timer.
- **Budget rows.** Lower the concurrency shard's row back to its measured value (about 35). Move reposetup's 23 private-mode tests into a new shard under a new prefix, `TestIntegrationPrivateRepoSetup`, and set both rows from fresh serial measurements.
- **Flaky `TestObserveRunningThenTerminal`.** It timed out once waiting for `vanished` after its helper's process group was SIGKILLed, on a heavily loaded machine (found while finalizing change 539); it passed on rerun. Diagnose first: reproduce under load and read the stuck-state line the test already prints, then fix the cause in `internal/process` or in the test. If it will not reproduce, change nothing and record the run count; never just raise the timeout.

## Out of scope

The runner's solo re-check not detecting another worktree's concurrent suite (#528). Putting budgets on a host-relative basis and re-seeding the budget table (#273); this change only re-shapes the concurrency and reposetup shards. The workflowlifecycle shard split (#536). Changing what any existing test asserts.
