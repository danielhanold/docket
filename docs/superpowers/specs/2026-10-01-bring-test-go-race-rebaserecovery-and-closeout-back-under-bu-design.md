<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0487 — Bring test_go_race, rebaserecovery, and closeout back under budget, fix the repoguard concurrent-gate timeout, and gofmt comment_integration_test.go](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-02-0487-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu.md)**
<!-- docket:backlink:end -->

# Bring the race gate and two integration shards back under budget — design

Consolidates #0475, #0476, #0478, #0484 and #0485 (combined 2026-10-01).

## Problem

The runner's budget report has serial-confirmed three breaches, and one gate timeout has gone red under concurrent load:

| File | Row | Solo threshold (row × 3/2) | Serial-confirmed solo |
|---|---|---|---|
| `tests/test_go_race.sh` | 60 parallel | 90s | 94s (0482) |
| `tests/test_go_integration_app_closeout.sh` | 40 parallel | 60s | 67–69s (0470, 0472) |
| `tests/test_go_integration_app_rebaserecovery.sh` | 40 parallel | 60s | 61–63s (0468, 0469) |

During 0482's gate, `test_go_race.sh` also hit its 8-minute `-race` backstop in `internal/repoguard` at load 45–69, while 0481's gate hit the same backstop in the same package. A re-run passed.

Separately, `internal/githubcli/comment_integration_test.go` is the only file `gofmt -l internal/ cmd/` flags (0478).

## Grooming measurements (2026-10-01, origin/main 97cc46c3e, 11 cores)

**The race gate.** Run as `go test -race -count=1 -json ./internal/repoguard` with the shared docket-go-cache:

| | Unpatched (load ~5–14) | Prototype pre-filter (load ~5) |
|---|---|---|
| `internal/repoguard` package | 87.6s | 25.8s |
| `TestRetiredVocabularySeal` | 65.5s | 1.4s |
| `…/maintained_surfaces` | 55.7s | 1.25s |

All tests passed with the prototype. Nothing else in the package is close: the next slowest are `TestRealProcessPackagesUseFixtureTempDir` at 11s and everything else at ≤2.6s.

A patched whole-module `go test -race ./...` happened to run while another session's suite was running its own `test_go_race`, which is the #0485 shape. It finished in 75s wall at load ~27. The slowest packages were `repoguard` 67s, `gatedrive` 61s, `app` 59s and `cli` 54s, so `repoguard` is no longer an outlier.

**Why the seal is slow.** `scanTextLine` and `scanGoLiteral` run every kindToken/kindWord row's regexp (about 110 rows) against every scanned line and every Go string literal. Each regexp begins `(^|[^A-Za-z0-9_])`, so Go's regexp engine gets no literal prefix to skip ahead with and steps through every byte. Separately, `scanBoundFlags` runs the catalog operation-reference alternation (about 150 alternatives) over every non-blank block of every scanned file, even though only blocks containing `--version` can produce a hit. The cost has grown with every ADR-0129 family that appended rows (32s → 94s solo, per 0482's results).

**The shards.** Both shards are serial (no `t.Parallel`), so wall time is the sum of their tests. Measured under load ~17–22 (solo is about 0.6× these):

- closeout: 21 tests, 108s. `RootCarry` 23.2s, `UnrelatedShapesArchive` 8.8s, `Refusals` 8.8s, `StackedPreservation` 8.2s, and the rest 1.7–4.8s each. It grew with 0327 and 0449.
- rebaserecovery: 17 tests, 73s. `CheckpointInvalidation` 17.7s, `ForwardRefreshRefusals` 9.7s, `ForwardRefreshInterruptions` 9.7s, `CarryPreservation` 8.0s, and the rest 1.2–3.1s. It grew with 0442.

## Decision

### 1. Make the retired-vocabulary seal's scan cheap (fixes #0484 and #0485)

In `internal/repoguard/retired_vocabulary_test.go`:

- **Text rows.** In `scanTextLine`, skip a kindToken/kindWord row unless `strings.Contains(line, r.Old)`, before running its regexp.
- **Go literals.** In `scanGoLiteral`, apply the same check against the unquoted value for kindToken/kindWord rows. kindGoFlag, kindGoPrefix and kindJSONKey are already cheap and stay unchanged.
- **Bound flags.** In `scanBoundFlags`, skip a block before computing `refs` (the catalog alternation) unless the block contains some kindBoundFlag row's `Old`.

Each check is an exact necessary condition of the regexp it guards: every matcher contains `regexp.QuoteMeta(r.Old)` literally. Detection is therefore unchanged by construction. No table row, kind, boundary rule, population floor or failure message changes.

**Proof the guard still guards** (AGENTS.md "a guard is code"):

- `non_vacuity`, `negative_controls`, `maintained_surfaces`, `generator_output` and `schema_walk` must pass unmodified.
- Mutation probe: break a pre-filter (for example, test `r.New` instead of `r.Old`, or invert the block skip) and confirm `non_vacuity` reddens. Record each probe's red output in the results file.

### 2. Prove the concurrent-gate fix, add no new machinery (#0485)

No cross-gate load bound and no ADR-0108 extension. The build reproduces #0485's shape: two full-suite gates (or two `tests/test_go_race.sh` runs alongside a full suite) started together on one machine.

- **Before:** an untouched merge-base checkout.
- **After:** the branch head.
- Record per-package `-race` elapsed times for `internal/repoguard` and the load average at each run. Before should show `repoguard` as the outlier; after should show it in line with the other slow packages and nowhere near the 8m backstop.

If the after-run still hits the backstop, report it as a confirmed follow-up in the results file and do not widen scope. The 8-minute backstop is not raised.

### 3. Split each over-budget integration shard in two (fixes #0475 and #0476)

Follow change 0434's precedent: the tests are unchanged, only names and wrappers move.

- Rename tests so each shard divides into two **disjoint** name prefixes. Neither new prefix may be a prefix of the other, and neither may overlap any other shard's `SHARD_PREFIX`. The existing integration contract (`tests/test_go_integration_contract.sh`) proves totality and must stay green unmodified.
- The old single prefix (`TestIntegrationFinalizeCloseout`, `TestIntegrationFinalizeRebaseRecovery`) would select both halves, so each old wrapper is re-pointed to one half and a sibling wrapper is added for the other. Every `tests/test_*.sh` needs a `tests/runtime-budgets.tsv` row (`TestRuntimeBudgetsCorrespondence`).
- Balance the halves by serial measurement. Illustrative groupings from the grooming numbers, with the final grouping chosen by the plan from fresh serial readings:
  - closeout: root/stacked/lifecycle tests (`RootCarry`, `Stacked*`, `Ordinary`, `Idempotent`, `Refusals`, `NeverEditsAuthoredBytes`, about 56s loaded) vs notes/backlink/unrelated-record tests (`Notes*`, `Backlink*`, `Unrelated*`, `NoNotesEmitsNoSection`, about 52s loaded).
  - rebaserecovery: forward-refresh tests (`ForwardRefresh*`, about 28s loaded) vs the rest (checkpoint, carry, resume and attempt, about 45s loaded). For example, the first half could be renamed to a fresh `TestIntegrationFinalizeRebaseRefresh*` prefix, which no shard uses today.
- Size each new row from serial solo measurements using the ledger's house rule: round up to the next multiple of 5, plus a 5s margin, minimum 10s. Each half's serial solo must clear its row's ×3/2 threshold with headroom. A fresh copy of a shard file must keep `SHARD_MODE="normal"`.

### 4. Budget rows and ledger narration

- `tests/test_go_race.sh` keeps its `60 parallel` row. The build serial-confirms it under that row, and grooming projects comfortably under 60s once `repoguard` drops about 60s. Re-size it only if serial readings genuinely require it, and record why (0466 precedent).
- Keep every narrated number bound to its measurement (change 0289). Where `tests/test_go_race.sh`'s header narrates the measured worst default-corpus package, update the number to the new reading if it changed.

### 5. gofmt (#0478)

Run `gofmt -w internal/githubcli/comment_integration_test.go` using the toolchain-declared gofmt (`gofmt_toolchain_test.go` describes the declared-toolchain rule). This is formatting only, and `gofmt -l internal/ cmd/` must print nothing afterwards.

## Acceptance

1. The full suite passes at the build gate (`build.test_command`).
2. The budget report shows **no** `SERIAL CONFIRMED OVER BUDGET` line for `tests/test_go_race.sh`, nor for any closeout or rebaserecovery shard file.
3. Serial solo readings for the race gate and each new shard file are recorded in the results file with their thresholds.
4. Under the same `-race` invocation as grooming, `TestRetiredVocabularySeal` measures in single-digit seconds and `internal/repoguard` well under its 87.6s baseline. Before/after numbers are recorded.
5. The pre-filter mutation probes redden `non_vacuity`. Their output is recorded.
6. The before/after two-gate run from Decision 2 is recorded with load averages.
7. `gofmt -l internal/ cmd/` prints nothing.

## Out of scope

- Raising the 8-minute `-race` backstop.
- A machine-wide or cross-gate Go test-load bound (an ADR-0108 extension). It was considered and declined at grooming; the measured cause is the seal.
- Changing the runner's budget regime, slack factor, report semantics, or how budgets are measured or enforced.
- Converting the shard tests to `t.Parallel()` (also rejected at 0466's grooming), or speeding up the slow integration tests themselves.
- Other budget rows and screening lines (`BUDGET WATCH`, `PARALLEL-SENSITIVE`), unless the investigation shows a shared cause.
- `-race` failures in packages other than `internal/repoguard`.
- Any other gofmt drift, or changing how gofmt is enforced.
