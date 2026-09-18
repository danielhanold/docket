<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0434 — Move load-sensitive tests out of the default parallel suite lane](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-18-0434-move-load-sensitive-tests-out-of-the-default-parallel-suite.md)**
<!-- docket:backlink:end -->
# Move load-sensitive tests out of the default parallel suite lane — Results

## Outcome

Stabilized the load-sensitive coverage change 0411 flagged, using only the existing
integration-partition machinery. Three independent strands landed, each measurement-gated:

- **Gatedrive terminal-relaunch correctness (fix).** `driveSlice`'s `reserveRelaunch` error branch
  (the `process.StateSignaled, process.StateVanished` path) now treats `errAlreadyTerminal` the same
  way it already treats `errRelaunchRaceLost`: the same-owner reservation loser sets
  `relaunchRaceLost` and returns, routing through `driveAndPersistClaim`'s reload-and-return path so
  it reports the winner's authoritative recorded state instead of surfacing the raw
  `gatedrive: drive already terminal` sentinel as an `Advance` error (0411's failure text). Exactly
  one backend launch, `RelaunchCount`/`Attempt` accounting and the original deadline preserved, no
  orphan; genuine `verifyOwner`/`ErrIO`/store-I/O failures still propagate. A deterministic
  regression (`TestLoserAfterTerminalSettleReturnsRecordedState`, with `terminalSettleProc` /
  `gatedLoserSeam` fixtures) stays in the fast default corpus and revert-reddens.

- **Shard rebalance (measured migrations).** The three app integration shards whose isolated serial
  time exceeded their ceilings and broke the sub-60s regime — `app_rebase` (77.0s), `app_change`
  (61.0s), `app_workflow` (56.1s) — were each split into two disjoint-prefix children by renaming
  the `TestIntegration*` functions (rename-only; bodies, subtests, and assertions byte-identical):
  `FinalizeRebaseGate`/`FinalizeRebaseRecovery`, `ChangeAuthoring`/`ChangeRuntime`,
  `WorkflowRepo`/`WorkflowLifecycle`. Three new declaration-only wrappers were added over the
  existing `tests/lib/go-integration-shard.sh` executor. Test totals are preserved exactly
  (15/104/32), every wrapper stays `parallel`, and every scenario stays mandatory in the full suite.
  No default-corpus test met the migrate bar (the gatedrive corpus is fake-backed and fast, so it
  stays in the default lane — its failure was a correctness problem, not a placement one).

- **Budget ceilings (re-derived).** The six touched/created rows in `tests/runtime-budgets.tsv` were
  re-derived from fresh worst-of-two isolated serial readings by the table's own rule (round up to
  the next multiple of 5, +5s margin): Gate 45, Recovery 40, Authoring 35, Runtime 40, Repo 35,
  Lifecycle 35 — all comfortably under the 60s hard ceiling, none inflated to absorb growth, all
  `parallel`. ADR-0108 is cited unchanged and not superseded.

No new lanes, scheduler, quarantine, retries, timeout increases, attempt-limit changes, weakened
assertions, or skipped coverage were introduced.

## Verification performed

- **Governed build gate: green.** The full configured suite (`go run ./cmd/docket development test`)
  was driven through the native gate driver at the final head `34cf438c` and returned `PASSED`; a
  durable build-evidence record was minted and re-verified against that head (`result: green`).
- **Reliability evidence: three consecutive green whole-suite runs** at the final head and default
  parallelism (292s / 279s / 274s wall), versus a red-on-both baseline (479s / 297s). The measured
  win is reliability (0/2 → 3/3 green); the full-suite wall-clock delta is modest and host-relative
  and is reported without any unmeasured speedup claim.
- **Partition contract + budget guard: green.** `tests/test_go_integration_contract.sh` (exactly-one
  ownership, no default-corpus leakage, race-direction correspondence, no empty selection) and
  `TestRuntimeBudgetsCorrespondence` both pass; each split/new wrapper runs green with its expected
  declared test count.
- **Mutation checks: all six coverage boundaries redden and restore.** Missing runner, missing budget
  row, overlapping prefix, missing build tag, dropped race instrumentation, and empty selection were
  each mutated (backup-copy restores, uncached runs, landed-mutation proofs); every mutation reddened
  its guard and restored green — no guard left vacuous.
- **Gatedrive fix proof:** deterministic RED before the fix (propagated `gatedrive: drive already
  terminal`), GREEN after; revert-reddens confirmed; full gatedrive package green plain and `-race`
  (existing `TestConcurrentSameOwnerAdvanceRelaunchesOnce` subtests preserved); `-count=25` stress
  supplement green.
- **Independent deep review: clean, zero findings** (build evidence verified, gatedrive correctness,
  rename disjointness/completeness, budget derivation, and out-of-scope adherence all confirmed).

Measurement raw logs and per-shard/per-test breakdowns are recorded in the plan file's committed
`## Measurement record` and `## Final evidence` sections.

## Findings and limitations

### Isolated-serial ceilings are host-relative

All ceilings were derived on this host (11-core arm64) from worst-of-two isolated serial readings,
which is the runtime-budget table's own derivation basis but is inherently host-relative. The split
kept every touched shard well under ceiling with headroom, so modest host variance should not breach.

### The observed baseline load flake and the fixed interleaving were different surface symptoms

Task 1's baseline observed the load-sensitive gatedrive flake as
`TestIntegrationProcessDeathPermitsAtMostOneRelaunch` HALTED (`relaunch-exhausted`) under whole-module
`-race`, which passed on re-run and in isolation. The deterministic regression nonetheless reproduced
the spec's hypothesized `errAlreadyTerminal`-propagation interleaving exactly on the first RED run, so
the narrow planned fix was applied (hypothesis-confirmed path, not the actual-defect-found path). At
the final head all three baseline red causes (gofmt drift, `test_go_race` gatedrive flake, app-load
contention) are resolved and the suite is 3/3 green, but the fix is proven against the deterministic
interleaving rather than against the exact non-deterministic baseline symptom.

## Follow-ups

### Pre-existing whole-module wrappers sitting on the budget-watch list

Every final green run emitted five `BUDGET WATCH:` screening lines (streak-of-5), all for large
pre-existing whole-module/heavyweight wrappers this change did not touch: `test_go_finalize_e2e`,
`test_go_integration_app_closeout`, `test_go_integration_app_merge`, `test_go_race`, and
`test_go_toolchain`. None are `SERIAL CONFIRMED OVER BUDGET`, and none of the six rebalanced split
shards appears in any budget line. These are candidates for the same measure-and-rebalance treatment
in a future change; they are out of 0434's measured scope. A human can capture this with
`docket change create` if it warrants tracking.

### Branch built on a base that main has since advanced past

The branch was cut from main `3ccf9fac`; `main` has since advanced (e.g. `internal/app/finalize_rebase*`
gained untagged default-corpus `TestFinalizeRebaseContinue*` tests, and the go1.27.1 gofmt realignment
of `internal/githubcli/comment_integration_test.go` landed upstream). The gofmt drift was unblocked on
this branch byte-identically to main (so the PR diff for that file vs main is empty); the remaining
main advance will be reconciled by finalize's rebase-and-retest gate at merge. The new upstream
`TestFinalizeRebaseContinue*` tests use a different prefix from the rebalanced `TestIntegrationFinalizeRebase*`
shards, so they do not collide with this change's split.
