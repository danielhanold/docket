<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0373 — Harden integration/race test isolation under parallel load](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0373-harden-integration-race-test-isolation-under-parallel-load.md)**
<!-- docket:backlink:end -->

# Harden integration/race test isolation under parallel load — design

Change 0373. Groomed 2026-09-02 (interactive). Priority high: unrelated reds at the full-suite gate are halting otherwise-green implement-next and finalize runs, so other changes are blocked on this one.

## Problem

Four full-suite gate runs across two builds (0371, 0397) each reddened on a test the change under review did not touch, and every one was green when re-run serially:

1. a process-group liveness check in `internal/process` (`TestObserveRunningThenTerminal`, 30 s ceiling under `-race`; also stub 381),
2. the `internal/app` package hitting Go's per-package timeout,
3. `t.TempDir()` teardown failing with `directory not empty` in `internal/gitcli`,
4. the same teardown failure in `internal/repository/transaction` (`TestKeyedCommitCarriesFiveTrailers/keyed`, assertions passed, only cleanup failed, under `-j11`).

A fifth sighting (results file for change 0396, 2026-09-02) records concurrent gate-drive supervisors under `$TMPDIR` polluting `internal/process`'s recover-scan.

## Root cause — two structural mechanisms, not four gaps

The sightings span three packages, so a per-test fix is whack-a-mole. Two shared mechanisms explain all of them:

**A. Oversubscription.** `internal/suiterunner` launches every `tests/test_*.sh` target in parallel at `-j = NumCPU` (11 on the reference machine). Thirty-four of those targets are Go wrappers, each running its own `go test`, and three of them (`test_go_toolchain.sh`, `test_go_race.sh`, `test_go_finalize_e2e.sh`) run `go test ./...` over the whole module, which itself runs packages in parallel at `-p = NumCPU` with `-parallel = GOMAXPROCS` inside each package. Nothing bounds the product: the gate can schedule on the order of `-j × NumCPU` concurrent Go test packages, each with real git and supervisor subprocesses. That is the mechanism behind the wall-clock sightings (1, 2, and the `-race` ceiling in 381).

**B. Post-test writers into `t.TempDir()`.** Go's `t.TempDir()` cleanup is one `os.RemoveAll` with no retry; a process that is still writing into the directory when cleanup runs (a `Setsid` supervisor from `internal/process`, or a git child such as detached auto-gc / auto-maintenance) produces `directory not empty`. Under load (A) the window between the last assertion and the detached writer's exit widens. `internal/process` already carries a one-test workaround (`quiesceRun` in `launch_test.go`), which is evidence the gap is structural. This is the mechanism behind sightings 3, 4, and 5.

Both causes are hypotheses inferred from the code and the sightings; the build's reconcile pass verifies each against the tree before scoping the fix (learning: *groomed-root-cause-is-a-hypothesis*).

## Design

### 1. Bound total load at the runner

- The runner exports one concurrency cap into every target's sandbox environment (`internal/suiterunner/sandbox.go`, alongside the existing `HOME`/`TMPDIR`/git overrides) as a `DOCKET_`-namespaced variable derived from its own `-j` and `runtime.NumCPU()`. Name and derivation are settled at plan time; the contract is that a target can read *how much of the machine it may use*.
- The Go wrappers honor it: the shared `tests/lib/go-integration-shard.sh` helper and the three whole-module wrappers translate the cap into `go test -p <n>` and `GOMAXPROCS=<n>` for their child. When the variable is absent (a solo `bash tests/test_X.sh` or a bare `go test` run outside the runner) Go's defaults apply unchanged.
- Invariant the spec pins: the total number of concurrently running Go test packages across the whole gate stays at or under a small multiple of the CPU count, instead of `-j × NumCPU`. The multiplier is **measured during the build**, not chosen here: the plan sweeps candidate values on the reference machine, records wall clock per candidate, and picks the smallest multiplier that keeps every Go budget row under its ceiling.
- Every affected row in `tests/runtime-budgets.tsv` is re-seeded from post-fix measurements per the table's own rounding rule; no ceiling is raised without a written reason, and no row exceeds 60 s.

### 2. Shared real-process test fixture

A new internal test-support package (name settled at plan time; it lives under `internal/` and is imported only from `_test.go` files) replaces bare `t.TempDir()` in every package whose tests spawn real git or supervisor processes. The twelve packages today: `internal/app`, `internal/gitcli`, `internal/repository/transaction`, `internal/process`, `internal/workspace`, `internal/install`, `internal/gatedrive`, `internal/cli`, `internal/suiterunner`, `internal/repository`, `internal/repoguard`, `internal/release` (derived from a whole-repo grep for `exec.Command` / `"git"` in test files; the plan re-derives the list, never hand-copies it).

The fixture provides:

- **A temp dir with a draining cleanup.** Cleanup first drains the test's own detached children (the existing `quiesceRun` logic from `internal/process/launch_test.go` becomes the fixture's drain primitive rather than a one-test workaround), then retries `os.RemoveAll` over a bounded window (order of seconds, constant recorded with its measurement per learning *tolerance-constant-calibrated-on-one-machine*) before failing. A writer that outlives the window fails the test with a diagnostic naming the surviving paths, so a genuine leak surfaces as a finding rather than an opaque teardown crash.
- **Git background work off.** The fixture writes a per-fixture git config (via `GIT_CONFIG_GLOBAL` for the processes it spawns, or directly into each repository it initializes) that disables `gc.auto`, `gc.autoDetach`, `maintenance.auto`, and `core.fsmonitor`, so no git child survives the command that spawned it. The same four knobs are added to the runner sandbox's synthetic global gitconfig (`gitIdentityConfig` in `sandbox.go`) so gate runs and solo runs agree.
- **Process-registry isolation.** Tests that launch supervisors get a registry root under the fixture's own temp dir, never the shared `$TMPDIR`, closing sighting 5.

### 3. Fail-closed guard; no serial pins

- A `repoguard` test proves that no `_test.go` file in a real-process package calls bare `t.TempDir()` outside the fixture. The package list is derived by the same grep as section 2 at test time, not hand-listed. Mutation-tested: restoring one bare call reddens it.
- No wrapper receives a `serial` pin in `tests/runtime-budgets.tsv`. Serial pinning lengthens every gate and hides the isolation gap; the fix is structural or it is not done.
- Stub **381** is folded in: its test is covered by sections 1 and 2. The implementer's reconcile pass kills 381 as a duplicate once 373 is claimed.

### 4. Evidence and acceptance

The results file records:

- five consecutive full gate runs (`go run ./cmd/docket development test`) at the same head with zero unrelated reds;
- the measured multiplier from section 1 and the per-candidate wall-clock table;
- the re-seeded budget rows with before/after seconds;
- the four named offenders plus sighting 5 as explicit regression cases, each run serially and under the gate.

## Out of scope

- Any product behavior change; this is test infrastructure only.
- The budget registry mechanism itself (`tests/runtime-budgets.tsv` format, screen-then-confirm regime) beyond re-seeding the rows this change affects.
- Sharding over-budget files (changes 280, 296).
- Changing what any test asserts. A fixture adoption is a substitution, not a rewrite.
- The 0371 and 0397 changes themselves (merged).

## Related

- 381 — same class, one test; folded in.
- 333 — partitioned `internal/app` behind the `integration` tag; produced today's 34-wrapper topology.
- 280, 296 — budget-driven sharding of other files; independent.
- Learnings: *groomed-root-cause-is-a-hypothesis*, *tolerance-constant-calibrated-on-one-machine*, *transient-resource-lifecycle*, *shared-resource-keeps-first-owner-assumptions*.
