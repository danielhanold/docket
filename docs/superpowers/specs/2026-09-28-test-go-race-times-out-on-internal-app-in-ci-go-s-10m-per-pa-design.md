<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0465 — test_go_race times out on internal/app in CI (Go's 10m per-package limit)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0465-test-go-race-times-out-on-internal-app-in-ci-go-s-10m-per-pa.md)**
<!-- docket:backlink:end -->

# Default internal/app corpus never runs real git — design

**Change:** 0465 · **Date:** 2026-09-28 · **Status:** groomed

## Problem

The release-candidate source-gate (macos-15, 3 CPUs) intermittently fails `tests/test_go_race.sh`: `go test -race -count=1 ./...` hits Go's default 10m per-package timeout in `internal/app`. Green runs take 581–908s against a 60s row in `tests/runtime-budgets.tsv`. A red source-gate blocks every PR.

Measured at grooming (2026-09-28, `GOMAXPROCS=3 go test -race -count=1 -p 3 -json ./internal/app/` on the developer machine):

- The package took **238s**, and the per-test elapsed times sum to **237s**. The package is effectively serial (14 `t.Parallel()` calls across the default corpus; about 87% of one core used), so wall-clock equals the sum of the tests.
- The default-tag corpus has grown from **256 tests** at change 0333's merge (2026-08-27) to **911–924** now. The growth is concentrated in the run-gate, gate-drive, finalize-rebase, change-groom and "unrelated invalid record" families.
- Each default test was run alone with a logging `git` shim on `PATH`. **337 tests start real git and account for 225.5s. The other 574 tests account for 11.4s.** 91 of the 93 tests taking ≥1s use real git. The slowest test, `TestGateLaunchSecondRefusedWhileFirstLives` at 10s, is a process-lifecycle test.

So the race gate has picked up exactly the real-git, subprocess and process-lifecycle corpus that change 0333 moved behind `//go:build integration`. The partition is opt-in, and nothing prevents new real-git tests from landing in the default corpus. Under CI's derived cap (jobs = NumCPU, so `GoTestConcurrency` gives `-p 2`/`GOMAXPROCS=2` on a shared 3-core runner), 238s becomes 600–900s.

**Why the budget machinery never flagged it.** `suiterunner.ContextKey` puts the target's **absolute path** first in the budget-state key. Every `.worktrees/<slug>` checkout therefore gets its own record. Its streak stays between 1 and 8, so the 5-overrun serial confirmation seldom fires. The local store shows `test_go_race` at 200–558s against a 60s ceiling, and every record is still `watching`. CI starts each run with an empty store, so screening can never escalate there either.

## Decisions

1. **Enforced invariant: the default-tag `internal/app` test corpus never starts a real `git` process.** Real-git, subprocess and process-lifecycle scenarios live behind `//go:build integration`. This turns 0333's partition from a convention into an enforced rule. It extends existing machinery: the tag, the shard runners, 0362's structural discovery and the `test_go_integration_contract.sh` totality proof.
2. **Moved tests run in plain (non-race) integration shards**, following 0333's rule that race instrumentation is only for tests exercising real concurrency. A moved test that genuinely exercises concurrent behavior (e.g. `TestGateLaunchSecondRefusedWhileFirstLives`, or any test racing two goroutines or processes on shared state) goes to a `TestRaceIntegration…` race shard, with a nearby rationale comment as 0333 requires.
3. **The guard is keyed on runtime behavior, not on spellings.** It does not scan for `exec.Command("git"` or fixture-helper names. A default-build-only test hook makes any real `git` exec fail, whatever path reaches it (the `gitcli` client's PATH fallback, a fixture helper, a bare `exec.Command`). This follows AGENTS.md: "Key a guard on syntactic shape, never an enumerated list of spellings" — here the shape is the exec itself.
4. **The budget-state key uses the repo-relative target path**, so observations accumulate across worktrees of the same repo and the existing screen-then-confirm machinery can produce a `SERIAL CONFIRMED OVER BUDGET` line. Existing records are orphaned once, which is acceptable because the store is advisory and fail-open. No schema bump is required unless the implementer finds one necessary.
5. **Backstop timeout, not a cure.** `test_go_race.sh` passes an explicit `-timeout`, set well above the post-fix measured worst case and below Go's 10m default. If it is ever exceeded, the failure is readable instead of a goroutine-dump panic. It is not raised to hide growth: the budget row and the guard are the growth detectors.
6. **Scope is `internal/app` only.** `internal/gitcli` and `internal/githubcli` are git/gh adapters by nature. Their slow corpus was already partitioned by 0333, and they are not what grew. No per-package generalization (YAGNI).

## Design

### 1. Re-partition the real-git default tests (`internal/app/*_test.go`)

- Identify every default-tag `internal/app` test that starts real git. The authoritative census is the guard in §2: turn it on and every offender goes red. Don't use a hand-maintained list (AGENTS.md: derive sites, never hand-list them).
- Move each one into an `internal/app/*_integration_test.go` file (line 1 `//go:build integration`, blank line 2), renamed with the `TestIntegration…` prefix. Use `TestRaceIntegration…` only for genuinely concurrent scenarios (Decision 2). Shared fixtures that only moved tests use move with them. Fixtures still used by default tests stay in default files.
- Add or extend shard runners (`tests/test_go_integration_app_*.sh` over `tests/lib/go-integration-shard.sh`) so every moved test matches exactly one runner by name prefix. Size the shards so each gets an ordinary parallel-lane budget row: about 225s of race-instrumented work, likely much less without `-race`, split across several prefix-scoped shards. Register every new runner in `tests/runtime-budgets.tsv` with a measured row.
- `tests/test_go_integration_contract.sh` must stay green unchanged. Its checks (1)–(10) already prove the partition is total, the prefixes are right, race modes match and there are no leaks into the default corpus.
- Renames change test names. Any skill/doc/test reference to a moved test by name must be updated. Derive these references with a whole-repo grep.

### 2. Runtime guard: default build cannot reach real git

- Add a default-build-only file in `internal/app`, e.g. `nogit_guard_test.go` with `//go:build !integration`. Before `m.Run()` it puts a directory at the front of `PATH` containing a `git` executable that exits non-zero and prints a diagnostic to stderr. The diagnostic names the rule (default `internal/app` tests must not run real git; move the test behind `//go:build integration` per change 0465/0333).
- Wire it into the existing `TestMain` in `internal/app/gate_test.go` without disturbing its supervisor (`process.SupervisorRequested`) and guardian (`GuardianRequested`) re-exec routing. Those re-exec roles must behave exactly as today. The integration build must not install the shim. Split the hook so the `!integration` file supplies the install and an `integration` counterpart supplies a no-op, or use an equivalent that keeps `TestMain` single-sourced.
- Verify how the code under test resolves git before relying on `PATH`. `gitcli`'s client falls back to `"git"` on PATH when its path is empty, and `internal/app/rungate_store.go` has a bare `exec.Command("git", …)`. If any path resolves git absolutely (a configured path, `DOCKET_*` override, cached `LookPath` from before `TestMain`), the guard must cover that path too, or the gap must be documented and closed.
- **Mutation test (required, AGENTS.md "A guard is code").** A test proves the guard fires. For example, a default test runs `exec.Command("git", "--version")` (or goes through the `gitcli` client) and asserts it fails with the guard's diagnostic. Stripping the shim install must turn that test red. Record the mutation run in the results file.

### 3. Backstop `-timeout` (`tests/test_go_race.sh`)

Pass an explicit `-timeout` on the `go test -race` invocation. Size it at several times the measured post-fix worst case of the slowest package under the CI-equivalent cap, and below 10m. Update the header's PARTITION AND LANE note to describe the invariant and the timeout, anchoring any cross-reference on symbol names (ADR-0054).

### 4. Honest budget rows (`tests/runtime-budgets.tsv`)

After §1–§2, re-measure `tests/test_go_race.sh` and every new/changed shard under the suite runner and set each row by the file's own rule: measured serial, rounded up to the next multiple of 5, plus a 5s margin. If `test_go_race.sh` still cannot fit a parallel-lane row (another package now dominates), treat that as a finding to report in the results file. Do not silently grant a large ceiling.

### 5. Budget-state key on the repo-relative path (`internal/suiterunner`)

Change the path component `ContextKey` receives (at its call site in the run entrypoint, or inside `ContextKey`) from the absolute target path to the path relative to `Config.RepoRoot`. The other dimensions stay the same (jobs, cpus, OS, arch, ceiling, mode, schema). The store's trailing absolute-path column can stay for human readability. Add a test proving that two checkouts of one repo at different absolute paths produce the same key for the same target.

## Testing

- Guard mutation test (§2). With the shim install removed, the proving test goes red. Restored, it goes green.
- The whole default `internal/app` corpus passes with the guard on. That is the proof that no default test reaches git.
- `tests/test_go_integration_contract.sh` is green, with every moved test in exactly one shard and in the right race mode.
- `ContextKey` worktree-independence test (§5).
- Measurement: `GOMAXPROCS=3 go test -race -count=1 -p 3 ./internal/app/` wall-clock before and after, recorded in the results file. Expect roughly 15–30s after, down from ~238s.
- Full suite at the build gate (`build.test_command`), reading the budget report, per AGENTS.md.

## Acceptance

- The `internal/app` share of `test_go_race` stays well under its row at a 3-CPU cap, and `test_go_race.sh` has an honest parallel-lane row (or a reported finding per §4).
- Several consecutive CI source-gate runs are green on `test_go_race`, with clear headroom under the explicit timeout. Confirming this is a human action after merge; list it in the results file.
- The guard is proven by mutation, and the integration contract is green.
- Budget-state records for one target converge across worktrees.

## Out of scope

- PR #345 / change 0463's branch-local `gateTempDir` compile failure. That branch needs its own rebase.
- Weakening the race gate: dropping `-race`, narrowing `./...`, or skipping `internal/app`.
- Guards or re-partitioning for `internal/gitcli`, `internal/githubcli`, or other packages.
- Persisting budget state across CI runs, or changing how CI classifies screening findings.
- Broad `t.Parallel()` adoption in `internal/app`. That might also speed things up, but it has a large shared-state blast radius, and the partition alone closes the gap.
