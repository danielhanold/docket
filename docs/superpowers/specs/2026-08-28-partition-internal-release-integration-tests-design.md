<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0362 — Partition internal/release integration tests to clear the race-gate per-package timeout](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-28-0362-partition-internal-release-to-clear-the-race-gate-timeout.md)**
<!-- docket:backlink:end -->

# Partition `internal/release` integration tests — design

- **Change:** 0362 — Partition `internal/release` integration tests to clear the race-gate
  per-package timeout
- **Date:** 2026-08-28 (UTC)
- **Status:** build-ready design
- **Related:** #0333 (done; establishes the integration-shard pattern), #0361 (in progress;
  halted on the race-gate timeout this change removes)
- **ADRs:** none

## Problem

The default whole-module race gate runs `go test -race -count=1 ./...`. Slow release-packaging
tests remain in that default corpus even though they exercise sequential cross-build, subprocess,
archive, and filesystem behavior rather than concurrent state. Under the full suite's parallel
load, `internal/release` has exceeded Go's 600-second per-package timeout and blocked unrelated
changes.

The package is healthy in isolation. Change 0361 recorded a warm standalone race run of 139.311
seconds. Interactive grooming for this change measured the package with fresh, otherwise-empty Go
build caches on an idle supported macOS host:

- ordinary `go test -count=1 ./internal/release/`: 28.62 seconds wall clock;
- `go test -race -count=1 -timeout 30m ./internal/release/`: 152.27 seconds wall clock.

The normal cold run fits an ordinary integration shard with headroom. Race instrumentation alone
pushes the same sequential corpus far beyond Docket's 60-second per-test-file ceiling, before suite
contention is added. The correct boundary is therefore behavioral: keep fast unit coverage in the
default corpus and execute the slow sequential release integration coverage once, without `-race`,
through the integration-shard system introduced by change 0333.

The same grooming pass measured the current `tests/test_go_toolchain.sh` with a fresh build cache at
68.99 seconds. That run also found an unrelated existing worktree-sensitive assertion, so its
correctness verdict is not evidence about this change; its wall-clock result is evidence that the
post-partition toolchain gate must be remeasured rather than assumed safe.

## Goals

1. Remove slow release packaging and archive integration tests from ordinary `go test ./...` and
   therefore from the default race gate.
2. Keep every existing release test scenario mandatory and executed exactly once in its intended
   mode.
3. Extend the integration partition structurally so future packages do not require another
   hand-maintained package allowlist.
4. Size every affected runtime row against a defined worst case: a fresh Go build cache on an
   otherwise idle supported macOS machine, followed by verification under the complete parallel
   suite.
5. Keep each affected runner within a 45–50-second target and an absolute 60-second ceiling without
   raising Go timeouts or hiding cost in a larger budget.

## Non-goals

- No production changes under `internal/release`; release output, checksums, downloader rendering,
  workflows, and release-smoke behavior remain unchanged.
- No release race shard unless implementation-time inspection finds a concrete concurrency
  protocol absent during grooming. Sequential subprocess and filesystem work does not qualify.
- No re-partition of the already-tagged `internal/app`, `internal/githubcli`, or `internal/gitcli`
  test scenarios.
- No general suite-scheduler or parallelism policy change.
- No resumption, finalization, or mutation of change 0361's branch or metadata.
- No network-cold benchmark. The module-download cache remains available; only the Go build cache
  is fresh. Network latency is not a finite or reproducible test-runtime budget.

## Test partition

Move `internal/release/package_test.go` and `internal/release/archive_test.go` wholesale to
`*_integration_test.go` files whose first line is `//go:build integration`, followed by the required
blank line. These two files contain the real cross-build, subprocess, tar/gzip, and filesystem-heavy
corpus. Keep `render_test.go`, `version_test.go`, and `checksums_test.go` in the default corpus; their
tests are fast unit or lightweight filesystem checks and continue to receive ordinary whole-module
race coverage.

Rename moved top-level tests into one structural family:

- package tests: `TestIntegrationReleasePackage...`;
- archive tests: `TestIntegrationReleaseArchive...`.

Create one initial normal-mode runner, `tests/test_go_integration_release.sh`, with package
`./internal/release` and prefix `TestIntegrationRelease`. It delegates to the existing shared shard
helper. It executes with `-tags integration -count=1` and without `-race`.

No current release test contains goroutines, synchronization primitives, `t.Parallel()`, concurrent
API calls, or process-lifecycle coordination. Consequently, no current test earns the
`TestRaceIntegration...` prefix or a race-mode runner. If reconcile discovers new concurrent release
coverage before implementation, the implementer must record the concrete concurrent protocol and
route only that coverage through a race shard; the sequential corpus remains normal-mode.

Before moving tests, capture a sorted inventory of release test names visible across default and
integration tag states. Maintain an old-name to new-name map for renamed tests. Final verification
must prove that every original test maps to exactly one surviving test and that scenario logic and
assertions are unchanged apart from the build constraint, file relocation, and top-level name. Keep
the inventory and map as committed point-in-time evidence, and wire a fidelity check to compare that
evidence with the live corpus; this is the population floor that catches deletion of the tagged
files and runner together.

## Structural integration contract

Generalize `tests/test_go_integration_contract.sh` instead of appending `internal/release` to its
current package enumerations. Discover integration packages from the repository's actual
`*_integration_test.go` files and discover shard membership from each runner's live inspection
output. Validate runner package declarations by shape and by existence in the module, not by a
four-package allowlist.

The contract must continue to prove, fail-closed:

1. every integration test file has the honored build constraint in the required first-line and
   blank-line shape;
2. tagged package listing succeeds and the tagged-only corpus is non-empty;
3. every tagged-only test has a `TestIntegration...` or `TestRaceIntegration...` prefix;
4. every tagged test matches exactly one runner by package and prefix;
5. every runner selects at least one tagged test;
6. normal tests map only to normal runners and race-prefixed tests map only to race runners;
7. the runner's executed race flag agrees with its declared mode;
8. no integration-prefixed test leaks into the default-tag corpus;
9. tagged static analysis succeeds for every discovered integration package; and
10. the discovered tagged-package and runner-package sets correspond in both directions.

Derive package coverage from repository syntax and live declarations. Do not maintain a second
registry of package names or runner assignments. Any discovered package-listing, runner-inspection,
or vet failure is a red result, never clean absence.

Mutation-prove the contract and fidelity floor by making one isolated break at a time and observing
the intended assert redden: remove the release build tag, remove the release runner, remove both the
release tagged files and runner, create a duplicate matching prefix, misdeclare normal release
coverage as race mode, drop the executed race flag from an existing race runner, and expose a tagged
release test to the default corpus. Each mutation must prove it landed, defeat Go's result cache
where execution occurs, and restore from a backup copy.

## Worst-case measurement and adaptive sharding

The acceptance environment is a supported macOS host with no competing workload, an available
module-download cache, and a newly created empty `GOCACHE` for each measurement. This is the defined
worst case: compilation starts cold, while unbounded external CPU starvation and network latency are
excluded because neither can support a finite budget.

After the partition, measure each affected runner three times from a fresh build cache:

- `tests/test_go_integration_release.sh`;
- `tests/test_go_race.sh`;
- `tests/test_go_toolchain.sh`, or its replacement shards if split.

Use the slowest valid reading. A failed run is not a timing sample: diagnose it, and for an unrelated
environmental failure compare the identical command on clean `origin/main` before classifying it.
Set each runtime row to the next five-second boundary above the worst reading plus five seconds of
headroom, with the existing minimum, and never above 60 seconds. Re-derive `EXPECTED_SERIAL` and
`EXPECTED_TOTAL` from the final runner set and measurements.

The target is 45–50 seconds. A 51–60-second result has no practical headroom and triggers a split;
anything above 60 seconds is an immediate design failure, not grounds for a larger row or timeout.

### Adaptive split order

1. **Release shard:** start with the single `TestIntegrationRelease` runner. If it exceeds 50
   seconds cold, split by the existing package/archive structural prefixes. Do not add
   `t.Parallel()` without an explicit isolation audit covering temporary directories, process
   environment, package globals, and named resources.
2. **Toolchain gate:** first separate independent static checks from executable tests if the
   post-partition cold runner exceeds 50 seconds. If the `go test` leg alone still exceeds 50
   seconds, partition packages into measured runners derived from `go list ./...`.
3. **Default race gate:** if the post-partition cold whole-module race runner exceeds 50 seconds,
   partition its package set into measured runners derived from `go list ./...`.

Any package split must carry a bidirectional exact-cover guard: the shard package sets must be
disjoint and their union must equal the live `go list ./...` package set. A package added later must
be assigned automatically or redden the contract; it may never fall through an exclusion list.
Runner-level parallel execution remains the baseline, but the final cuts are decided by measured
cold standalone wall clock and then validated under the complete suite's real contention.

## Verification and completion criteria

The change is complete only when all of the following hold:

- the before/after inventory proves every release scenario survives exactly once;
- ordinary `go test -count=1 ./...` cannot see the moved release tests;
- the tagged release runner sees and executes every moved test exactly once;
- no release race runner exists unless reconcile identified and documented a concrete concurrent
  protocol;
- the generalized integration contract passes and each required mutation makes its intended guard
  fail;
- all affected runtime rows are derived from three fresh-build-cache measurements and retain the
  required headroom under the 60-second ceiling;
- the default race gate no longer approaches the `internal/release` 600-second package timeout;
- the complete configured suite passes; and
- `EXPECTED_SERIAL` and `EXPECTED_TOTAL` match the final runner topology.

Record the commands, all three readings per affected runner, the selected worst reading, the row
calculation, and the parallel-suite result in build evidence. Green correctness tests alone cannot
prove this performance change; measured wall clock is part of acceptance.

## Change relationships

Change 0333 remains related as the pattern and machinery being extended. Change 0361 remains
related because its halted run exposed the blocker and can resume after this change lands. Neither
is a dependency: 0333 is already done, and 0361 does not need to finish before this partition can be
built. No ADR is required because the change extends an existing test architecture without changing
a durable product or workflow decision.
