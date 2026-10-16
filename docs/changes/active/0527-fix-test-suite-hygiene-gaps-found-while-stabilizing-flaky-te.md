---
id: 527
slug: 'fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te'
title: 'Fix test-suite hygiene gaps found while stabilizing flaky tests'
status: 'implemented'
priority: 'low'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-16'
depends_on: [554]
stacked_on:
related: [273, 528, 536]
discovered_from: [507, 531, 554]
adrs: []
spec: 'docs/superpowers/specs/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-design.md'
plan: 'docs/superpowers/plans/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te.md'
results: 'docs/results/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te'
pr: 'https://github.com/danielhanold/docket/pull/421'
blocked_by:
reconciled: true
claimed_at: '2026-10-16T16:23:16Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-design.md](../../superpowers/specs/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-design.md) |
| Plan | [2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te.md](../../superpowers/plans/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te.md) |
| Results | [2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-results.md](../../results/2026-10-16-fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te-results.md) |
<!-- docket:artifacts:end -->

## Why

Change 507 (flaky-test stabilization) surfaced several unrelated test-suite defects that no existing change covers. Each lets a test silently stop running, fail for environmental reasons, or fail falsely.

Change 531's build surfaced two more: a gate-driver test that fails under disk load, and runtime budgets that were raised where `tests/README.md` says to split the test instead.

Change 554's build fixed the flaky `TestObserveRunningThenTerminal` (item 7) and found the same race in a sibling test (item 8).

## What changes

Eight small test-suite items. The first six were re-traced against `main` on 2026-10-16; the seventh is already fixed by #554; the eighth applies #554's fix to a sibling test (the linked spec has the detail):

- **Unrunnable test functions.** A repo-wide guard in `internal/repoguard` flags any function shaped like a test (`func(*testing.T)`, `*testing.B`, `*testing.F`) that `go test` will never run because of its name and that nothing calls. This catches the e2e case (renaming `TestE2EFoo` to `testE2EFoo` leaves `tests/test_go_finalize_e2e.sh` green) and the same mistake in every other test file.
- **No-git guard path.** `InstallNoGitGuard` stores the temp dir uncleaned, so `TestNoGitGuardShadowsGitOnPath` fails when `TMPDIR` is not in clean form (for example `/tmp//`; a single trailing slash is fine). Store the cleaned path.
- **Bare `EOF` from the codex client.** The `TestAgentEnterCLIPreservesRequestAndReceipt` EOF did not reproduce (0 of 230 runs). The real gap: the codex client reads the app-server's stderr before its copy finishes, so a crashing server's message can be lost, and the test's fake server exits silently on every failure branch. Fix the client to capture stderr before reporting a read error (bounded, never hangs), and make the fake server say why it exits.
- **Exit-sites guard walks worktrees.** `TestProcessExitSitesAreAllowlisted` walks into `.worktrees/` and fails from the main checkout. Switch it to the shared `repoguard.MaintainedFiles` walk.
- **Gate-driver tests that race a timer.** `child-signal-death` (failing 34 of 40 under disk load) and three other gate-driver tests depend on a child staying alive for a fixed number of milliseconds while several fsync'd writes happen. Make the child wait for a go-signal file the test creates instead of a timer.
- **Budget rows.** Lower the concurrency shard's row back to its measured value (about 35). Move reposetup's 23 private-mode tests into a new shard under a new prefix, `TestIntegrationPrivateRepoSetup`, and set both rows from fresh serial measurements.
- **Flaky `TestObserveRunningThenTerminal` — fixed by #554.** One group-wide SIGKILL is not atomic on macOS, so the supervisor could write a terminal record before dying and the test never saw `vanished`. #554 (commit `83d80f3`) kills the supervisor first, then the group. Nothing to build; confirm the fix is on `main` (by content; a rebase merge changes the hash).
- **Same one-step kill in `TestRecoverMarksCleanlyAbandonedOwnedRun`.** It builds its abandoned run with the same single group SIGKILL, and an older 3-attempt setup retry hides the race. Kill the supervisor first as #554 did, drop the retry loop, and keep the setup-shape check as a single failing check. Sweep `internal/process` for any other test building that shape the same way.

## Out of scope

The runner's solo re-check not detecting another worktree's concurrent suite (#528). Putting budgets on a host-relative basis and re-seeding the budget table (#273); this change only re-shapes the concurrency and reposetup shards. The workflowlifecycle shard split (#536). Changing what any existing test asserts.

## Reconcile log

### 2026-10-16

2026-10-16 — Re-checked against origin/main c084cd93c. Scope unchanged. #554 merged: item 7's supervisor-first kill is on main (95d6b56c0), so item 7 is confirm-only as planned. #536 merged and added the `TestIntegrationPrivateMode` shard prefix; the planned `TestIntegrationPrivateRepoSetup` prefix must still be re-checked against the live `SHARD_PREFIX` set (per-package) at build time. `noGitGuardDir` is still stored raw; `TestRecoverMarksCleanlyAbandonedOwnedRun` still uses the one-step group kill; 23 private reposetup tests still carry the old names (21 + 2).

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: cb025f23bfbb5d7b71ce5ac5a399feba8abee568
ran_at:   2026-10-16T16:58:22Z
<!-- docket:build-evidence:end -->
