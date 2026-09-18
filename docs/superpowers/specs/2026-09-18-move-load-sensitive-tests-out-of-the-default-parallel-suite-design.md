<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0434 — Move load-sensitive tests out of the default parallel suite lane](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0434-move-load-sensitive-tests-out-of-the-default-parallel-suite.md)**
<!-- docket:backlink:end -->

# Change 0434 — Stabilize load-sensitive coverage using the existing integration partition

## Intent and settled scope

Use the existing integration build tag, feature shards, shared shard executor, and runner concurrency cap to address long-running tests. Every retained test remains mandatory in the full Docket suite. There is no new test lane, scheduler, quarantine, retry mechanism, or timeout policy. All wrappers remain parallel, preserving ADR-0108.

The title's reference to leaving the default parallel suite means separating eligible slow tests from the default Go corpus; it does not mean removing them from the full suite or scheduling them serially. Already-tagged tests are rebalanced within the existing integration partition. A separate gatedrive correctness investigation is required because moving a faulty concurrent test does not fix its failure.

## Evidence and current structure

At grooming, main is 3ccf9fac511f370c200315674a1edf97766b0a5b.

The change-0411 report records internal/app integration timeouts at Go's ten-minute package timeout under -j11, and a passing isolated finalize-rebase run. It also records TestConcurrentSameOwnerAdvanceRelaunchesOnce/terminal_relaunch_winner_fails returning "gatedrive: drive already terminal" in both whole-module wrappers. These are reported observations, not proof that either failure is purely environmental; the targeted isolated command changed multiple variables and cannot establish causality alone.

Change 0333 already moved slow real-repository and subprocess tests behind //go:build integration. Tests use TestIntegration or TestRaceIntegration feature prefixes, and tests/test_go_integration_*.sh wrappers declare SHARD_PKG, SHARD_PREFIX, and SHARD_MODE. tests/lib/go-integration-shard.sh runs the selected corpus uncached. The structural contract discovers packages and runners, proves exactly-one ownership and race-mode correspondence, rejects default-corpus leakage and empty selections, and vets the tagged corpus. Change 0362 generalized discovery; no package allowlist is needed for new integration coverage.

Change 0373 added DOCKET_GO_TEST_CONCURRENCY at the runner and shared real-process fixtures. The existing app finalize-rebase wrapper already runs tagged coverage. The gatedrive concurrency fixture uses a fake ProcessSeam and real store synchronization; it is not automatically a slow integration workload.

## Design

### 1. Measure and classify before moving tests

Inventory the current wrappers and their selected tests from their live declarations and the repository's test census. Focus measurement on the app integration shards implicated by the failure logs and default-corpus gatedrive tests; do not infer an offender list from filenames alone.

Capture a baseline whole-suite run at configured/default parallelism and isolated uncached measurements for the suspected shards/tests. Record source revision, platform/CPU count, job count, effective Go concurrency cap, cache state, commands, per-shard durations, full-suite wall time, failures, and budget classifications. Keep comparison settings consistent. Use existing runner budget confirmation and direct targeted diagnostics; do not add a benchmark service or modify gate attempt accounting.

Distinguish aggregate shard cost, contention amplification, a single dominant test, and a functional failure. A default-corpus test moves behind the integration tag only when its measured cost and workload justify the existing slow-test partition. Fast fake-backed regressions remain in the default corpus.

### 2. Reuse tagged feature shards

For justified migrations, use *_integration_test.go with the integration build constraint and established TestIntegration / TestRaceIntegration prefixes. Put concurrency-bearing integration coverage in a mode=race wrapper with its race rationale; sequential integration work uses mode=normal. Preserve all assertions, subcases, fixture isolation, and required race instrumentation.

For app tests already tagged, split or rebalance the measured oversized feature group into smaller cohesive groups using the current declaration/helper pattern. Adjust test prefixes when necessary to keep shard selections disjoint: never retain a broad parent prefix that also selects a new child shard. Reuse sibling capacity where practical and add a new topical wrapper only when measurements justify it.

Keep shared helper APIs, the existing e2e partition, whole-module fast/race wrappers, and concurrency cap intact unless a narrow wiring correction is demonstrated necessary. Do not move the entire gatedrive package or its fast tests merely because one concurrency case failed.

Each wrapper has a parallel row in tests/runtime-budgets.tsv. Derive affected ceilings from uncached isolated measurements using the existing table convention, targeting useful headroom and the established sub-60-second shard regime. Do not inflate ceilings to conceal growth; split or reduce fixture cost. If one indivisible case or host contention remains unsolved, report the evidence and unmet acceptance condition rather than inventing a serial lane, raising timeouts, or claiming that tagging solved it.

### 3. Resolve the gatedrive terminal interleaving

Source inspection identifies a concrete hypothesis: Store.reserveRelaunch returns errAlreadyTerminal if another same-owner caller settled the drive before the reservation CAS. The reserveRelaunch error branch in Driver.driveSlice recognizes errRelaunchRaceLost but currently propagates errAlreadyTerminal. Other concurrent persistence/attachment paths already reload the authoritative record for these terminal races.

Prove or disprove this interleaving with deterministic synchronization, not sleep-based timing or stress passes alone. Arrange for both callers to observe the original dead run, then for the winner to relaunch and persist its terminal outcome before the delayed loser attempts reservation. Preserve coverage of both a running replacement and an immediately terminal replacement.

If reproduced, make the narrow product correction consistent with existing same-owner concurrency semantics: return authoritative recorded state, perform exactly one replacement launch, preserve attempt/relaunch accounting and the original deadline, and create or stop no spurious orphan. Genuine ownership, I/O, and reservation failures must remain failures. Assert the returned documents as well as durable state and launch counts. Reverting the correction must make the deterministic regression fail.

If the hypothesis is false, record the causal trace and repair the actual fixture or product defect within this same boundary. Do not accept errAlreadyTerminal as an arbitrary allowed error just to make the existing assertion pass. Keep the small deterministic regression in the fast corpus; only separately measured expensive coverage is eligible for tagging.

## Coverage and validation

- Run the existing integration partition contract and budget correspondence guard after migration/splitting. Prove each moved test is absent from the default corpus, present exactly once in its mandatory tagged shard, and instrumented in the intended race mode.
- Mutation-check the affected coverage boundaries: missing runner, overlapping prefix, missing build tag, and dropped required race instrumentation must fail the relevant existing checks. Use uncached runs and verify that each mutation actually landed.
- Run the deterministic gatedrive regression in plain and race-instrumented modes. Demonstrate a failing pre-fix interleaving and a passing corrected interleaving; repeated stress runs supplement that proof.
- Run the entire configured build gate from source through internal/suiterunner. All tagged shards remain part of that gate. Obtain three consecutive passing whole-suite runs at the final code head and default/configured parallelism as reliability evidence; every governed gate attempt obeys the existing durable limit. Do not bypass or reset accounting to reach the repetition target.
- Compare final isolated shard timings and full-suite wall time with the baseline on the same host/settings. Report the measured delta without promising an unmeasured speedup. Investigate any material slowdown; tagging/splitting without evidence of reduced bottleneck or improved reliability is not acceptance.
- Read all budget findings even on green runs. Serial-confirm screening candidates with the existing machinery; no new SERIAL CONFIRMED OVER BUDGET findings may remain unexplained or be hidden by a ceiling increase. Distinguish pre-existing unrelated findings from change-induced ones using comparable baseline evidence.
- Record failures, non-reproduction, measurement limits, coverage results, and final-head evidence in the implementation results. An isolated green test is not a substitute for the final whole-suite gate.

## Boundaries and relationships

No serial wrapper pins, scheduler redesign, automatic retries, build/finalize attempt-limit changes, relaxed assertions, skipped tests, blanket timeout increases, host-relative budget project, or unrelated formatting cleanup. No implementation is part of grooming.

Related changes: 0333 (partition), 0362 (structural discovery), 0373 (load cap and fixtures), 0273 (deferred host-relative budgets), and 0411 (discovery). No dependency or stacking relationship is required: the reused machinery has landed and 0411's documentation change is independent. Cite ADR-0108 unchanged; this design does not supersede it.
