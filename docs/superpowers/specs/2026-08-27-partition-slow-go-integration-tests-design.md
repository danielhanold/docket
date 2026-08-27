<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0333 — Partition slow Go integration tests and retire the race gate's 300s ceiling exemption](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0333-partition-internal-app-to-retire-the-race-gate-s-300s-ceilin.md)**
<!-- docket:backlink:end -->

# Partition Slow Go Integration Tests — Design

## Context

Change 0332 temporarily collapsed the repository's four Go race shards into one serial
`go test -race ./...` gate with a 300-second budget exemption. At the time,
`internal/app` dominated that invocation: roughly 190 seconds without race instrumentation and
200 seconds with it. The cost came from real-Git and subprocess integration tests, not from the
race detector, so the default Go gate and the race gate paid substantially the same integration
cost twice.

The package has continued to grow. On 2026-08-26, change 0357's build gate timed out
`internal/app` at Go's default ten-minute per-package deadline; an isolated race run also timed
out at 600 seconds. Change 0357 is high priority, its implementation is complete, and its full
gate is halted pending this partition.

The adapter packages have also outgrown the original boundary. Fresh standalone measurements on
2026-08-27, current tree and warm shared Go caches, were:

| Package | Command | Wall clock | Go-reported package time |
|---|---|---:|---:|
| `internal/githubcli` | `go test -race -count=1 -timeout 10m ./internal/githubcli` | 129.43s | 128.426s |
| `internal/gitcli` | `go test -race -count=1 -timeout 10m ./internal/gitcli` | 91.23s | 89.808s |

`githubcli` now exceeds the normal budget regime's 90-second standalone confirmation threshold,
and `gitcli` has no useful margin beneath it. Test-level profiling showed broad long tails rather
than one removable hotspot: `githubcli` has one 20-second merge test plus many 3–5-second command
protocol tests; `gitcli` has one 14-second malformed-batch test plus several 2–4-second process and
repository tests. Partitioning only `internal/app` therefore cannot honestly retire the special
race-gate handling.

## Goals

- Remove slow real-process integration tests from ordinary `go test ./...` in
  `internal/app`, `internal/githubcli`, and `internal/gitcli`.
- Keep every integration scenario mandatory in the normal full-suite gate.
- Run race instrumentation only where concurrent behavior makes it valuable.
- Give every integration shard a normal runtime-budget row no greater than 60 seconds, with
  meaningful measured headroom.
- Return the fast whole-module race gate to the parallel lane and delete the 300-second ceiling
  exemption.
- Make missing, duplicated, or misclassified shard coverage fail closed.
- Unblock change 0357 without weakening its focused or real-Git regression coverage.

## Non-goals

- Refactoring production package boundaries or production behavior.
- Changing `scripts/run-tests.sh` scheduling or the budget-confirmation state machine.
- Raising Go timeouts as a substitute for partitioning.
- Implementing host-relative budgets from change 0273 or shell-suite sharding from change 0280.
- Reworking the existing `e2e`-tagged `internal/app/finalize_e2e_test.go` matrix and its healthy
  `tests/test_go_finalize_e2e.sh` runner.
- Resuming or finalizing change 0357. That happens after this change lands.

## Decision

### One structural integration boundary

Slow tests move into files whose first line is:

```go
//go:build integration
```

A test belongs behind that tag when it runs real Git or GitHub subprocesses, constructs
multi-repository/worktree workflows, exercises process timeouts, or drives a complete operation
through those real boundaries. Tests that use fakes or exercise fast pure orchestration remain
untagged.

Homogeneous files move as a unit. Mixed files split so fast tests do not become integration tests
merely because a sibling is slow. Shared real-repository helpers move into integration-tagged
helper files; no untagged test may depend on a tagged helper.

The existing `e2e` build tag stays independent. It already removes the finalize end-to-end matrix
from the default Go package and gives it a dedicated, sub-60-second runner.

### Structural test names define feature shards

Normal integration tests use a feature-bearing top-level prefix:

```text
TestIntegration<Feature>...
```

Examples include `TestIntegrationFinalizeMerge...`, `TestIntegrationPlanning...`,
`TestIntegrationGitHubMerge...`, and `TestIntegrationGitBatch...`. Feature names describe stable
behavioral areas, not arbitrary bucket numbers. If one feature area cannot retain headroom beneath
60 seconds, it splits into narrower named areas; numeric `A`/`B` buckets are a last resort after a
natural behavior boundary has been exhausted.

The initial cuts are derived from fresh standalone per-test timings. They start from the measured
areas already visible in the suite:

- `internal/app`: finalize cleanup/publish, finalize merge/rebase, planning/change operations,
  and remaining real-repository workflows, split further wherever a group exceeds the target.
- `internal/githubcli`: merge/retarget behavior, PR ensure/comment behavior, and discovery/probe
  protocols.
- `internal/gitcli`: batch/blob protocols, process timeout/wait behavior, and repository/worktree
  operations.

Exact runner counts and prefixes are build-time measurement outputs, not frozen estimates in this
spec. Every retained group must satisfy the acceptance thresholds below.

### Targeted race integration tests

Concurrency-sensitive integration tests use the distinct structural prefix:

```text
TestRaceIntegration<Feature>...
```

They live in clearly named `*_race_integration_test.go` files carrying the same `integration`
build tag. Each has a short adjacent comment naming the concurrent behavior that makes race
instrumentation valuable. The qualifying behavior is shared mutable state, concurrent adapter
calls, process lifecycle coordination, or a race/recovery protocol—not merely the fact that a test
uses Git or GitHub.

These tests run exactly once, in dedicated race shards. Sequential subprocess drivers run exactly
once, in normal integration shards. Fast untagged tests continue to run through both the ordinary
and whole-module race gates.

### Declarative shard runners

Each shard is an ordinary `tests/test_*.sh` file, discovered and scheduled by the existing suite
runner. Runner names encode package and feature. Every runner delegates to one shared helper with
three literal declarations:

- package (`./internal/app`, `./internal/githubcli`, or `./internal/gitcli`);
- top-level test prefix;
- mode (`normal` or `race`).

The helper configures the same stable Go module/build caches as the existing Go gates, checks that
the prefix selects at least one test, and runs:

```text
go test -tags integration -count=1 -run '^<prefix>' <package>
```

Race mode additionally passes `-race`. `-count=1` is mandatory: correctness, completeness, and
performance evidence must not be served from Go's test-result cache.

The helper also supports an inspection mode used by the contract test. Inspection prints the
same three declarations the helper would execute and performs no test run. This makes the live
runner invocation—not a duplicated registry—the source of truth for shard membership.

Shard scripts run in the suite's parallel lane. Within a shard, `t.Parallel()` is added only after
an isolation audit confirms the test uses its own temporary repositories, does not mutate process
environment or package globals, and does not share externally named resources. Runner-level
parallelism is the baseline; test-level parallelism is an evidenced optimization.

## Completeness and static analysis

A dedicated integration-contract test fails closed through these checks:

1. Discover `*_integration_test.go` and `*_race_integration_test.go` under the three target
   packages; require the `integration` build constraint at the first line.
2. Enumerate tagged top-level tests with `go test -tags integration -list` for each package. A
   listing or compile failure is fatal, and an empty discovered corpus is fatal.
3. Discover every integration shard runner by filename and invoke its inspection mode. A malformed
   declaration, unsupported package/mode, or empty prefix is fatal.
4. Match every discovered tagged test against every live declaration. Each test must match exactly
   one runner.
5. Require `TestRaceIntegration...` tests to match a `race` runner and forbid ordinary
   `TestIntegration...` tests from matching one. Both directions are checked.
6. Enumerate tests without build tags and require that no integration-prefixed test is visible to
   ordinary `go test ./...`.
7. Require every discovered runner to select at least one test, so a stale runner cannot pass as an
   empty no-op.
8. Run `go vet -tags integration` over all three target packages. Default `go vet ./...` cannot see
   the tagged corpus, so tagged static analysis needs an explicit owner.

The implementation evidence mutation-tests at least four independent failure shapes: remove one
build tag, delete one runner, duplicate one runner prefix, and flip a race runner to normal mode.
Each mutation must redden the contract for its intended reason with Go's test cache defeated.

## Budget and race-gate transition

Fresh standalone measurement determines each shard's cut and row. The target on the profiling host
is 45–50 seconds, leaving operational room beneath the table's hard 60-second row ceiling. A shard
that merely measures 59–60 seconds is already out of budget headroom and must be split again.

After the tagged corpus is excluded:

- `tests/test_go_toolchain.sh` continues to run `go test ./...`, now over the fast default corpus.
- `tests/test_go_race.sh` continues to run `go test -race ./...`, now over the fast default corpus,
  and moves from `serial` back to `parallel`.
- Targeted race integration tests run only in their dedicated shard runners.
- Normal integration tests run only in their dedicated normal shard runners.

`tests/test_runtime_budgets.sh` and `tests/runtime-budgets.tsv` then remove all temporary change-0332
machinery:

- delete `RACE_GATE`, `RACE_CEILING`, the path-specific 300-second exception, and its special
  assertion wording;
- delete the assertion coupling the sole serial row to the race gate;
- change the race row to a normal ceiling no greater than 60 seconds in the parallel lane;
- set `EXPECTED_SERIAL` from the resulting table (expected to return from one to zero unless fresh
  reality contains another independently justified serial row);
- re-derive every new/changed row and `EXPECTED_TOTAL` from post-partition standalone measurements;
- update the table header and source comments that describe the monolithic package cost or the
  temporary exemption.

No budget is raised to absorb a slow shard. The available remedies are a better structural cut,
safe parallelism, or removal of duplicated execution.

## Migration integrity

Before moving tests, capture the top-level test inventory for all three packages. Maintain a
before/after mapping from every original test to its retained or structurally renamed destination.
Every original scenario must survive exactly once. Aside from build constraints, top-level naming,
helper relocation, and isolation-approved `t.Parallel()` calls, test assertions and scenario logic
remain unchanged.

After the move, derive helper consumers across the whole three-package corpus. No untagged file may
refer to a helper that exists only under `integration`. A missing helper must fail compilation in
the default-tag contract rather than being patched by moving unrelated fast tests behind the tag.

The integration contract is a coverage guard and is treated as code. In addition to the retained
mutation cases above, the build records the pre/post package and shard timings. Green correctness
with unchanged or worse wall clock is a failed performance outcome.

## Error handling

- Missing Go tooling, test-list failures, tagged compile failures, vet failures, malformed shard
  declarations, and empty selections fail the owning test with package/shard diagnostics.
- Runner stdout/stderr is retained and replayed through the existing suite runner; the shared
  helper never converts a failed `go test` into an empty success.
- A shard exceeding its target is reshaped before acceptance. The old exemption and timeout raises
  are not fallback paths.
- A test whose race value is unclear remains a normal integration test until concurrent behavior is
  identified; broad race coverage of sequential drivers is not the default.

## Acceptance

1. Ordinary `go test ./...` passes and exposes no integration-prefixed test.
2. Whole-module `go test -race ./...` passes over the fast corpus in the parallel lane with a
   normal row no greater than 60 seconds.
3. Every integration shard passes standalone with test caching disabled, targets 45–50 seconds,
   and has a row no greater than 60 seconds.
4. Every tagged test is assigned exactly once and to the correct race mode; every retained
   completeness mutation fails for the intended reason.
5. `go vet -tags integration` passes for all three packages.
6. The before/after test inventory has no missing or duplicated scenario.
7. The 300-second exemption, race-specific sub-ceiling, and serial coupling are absent from
   maintained source and documentation.
8. `EXPECTED_SERIAL`, all affected rows, and `EXPECTED_TOTAL` match the measured post-partition
   table.
9. The full configured suite (`scripts/run-tests.sh`) passes, and its budget report contains no
   authoritative breach for the changed Go gates or new shards.
10. Results record current package baselines, post-partition default/race package times, every shard
    time and row, suite parallelism, and the four required mutation outcomes.
