<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0332 — Route the -race test shards out of the parallel test pool](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-20-0332-route-race-shards-to-serial-lane.md)**
<!-- docket:backlink:end -->

# Route the `-race` shards out of the parallel test pool

**Change:** 0332 · **Date:** 2026-08-19 · **Type:** refactor · **Priority:** high

> **Scope expanded 2026-08-20** after measurement (see *What measurement showed* below). The
> original decision — flip the four `-race` shard rows `parallel` → `serial` and keep their existing
> 60/25/45/45 ceilings — is superseded: the main shard cannot meet a 60s ceiling by any lane, so
> this change now **collapses the four shards into one serial `go test -race ./...` gate with a
> documented 300s ceiling exemption**. The lane diagnosis below still stands; the four-way shard and
> the "rows hold under re-measurement" plan do not.

## Problem

`scripts/run-tests.sh` runs the suite as a parallel `-j` fan-out, and all four instrumented Go
data-race shards ride in that pool (`tests/test_go_race.sh` 60s, `_process` 25s, `_transaction` 45s,
`_workspace` 45s). Each shard runs `go test -race`, which internally spawns `GOMAXPROCS`-wide race
workers. Under the full parallel fan-out those workers oversubscribe the cores the shell test jobs
need, inflating the shell jobs' wall-clock past their `tests/runtime-budgets.tsv` ceilings. The
result is a **load-dependent build gate**: the same commit passes or fails depending only on what the
machine was doing at that second. This was proven by change 0329's build-gate halt — a fully-built
change blocked purely because the gate could not go green on a loaded machine, while every failing
file passed in isolation and the untouched merge-base failed the identical set.

## What measurement showed (2026-08-20)

The original plan flipped all four shards to `serial` and expected their existing rows to hold under
re-measurement. Three do. The main shard does not, and the reason invalidates the four-way shard as a
structure:

- **Standalone serial readings** (one shard at a time, this machine, normal load):
  `test_go_race.sh` **206s** (row 60), `_process` **18s** (row 25), `_transaction` **36s** (row 45),
  `_workspace` **39s** (row 45). Three of four fit comfortably *in the same loaded run* — so this is
  not host contention crushing the machine; only the main shard is over, and by 3.4×.
- **The 206s is essentially one package.** Per-package `-race`: `internal/app` **~200s**,
  `internal/githubcli` 83s, `internal/gitcli` 62s, all 17 others ≤5s. In a single shared-build
  invocation the cheap packages overlap under `internal/app`'s long pole, so `internal/app` sets the
  whole shard's wall clock.
- **The race detector is not the cost.** `internal/app` is **190s uninstrumented**, ~200s under
  `-race` — a ~1.05× multiplier. The 190s is `internal/app`'s own integration/e2e suite (47 test
  files, ~316 tests, finalize/planning tests that shell out to real `git`; the slowest single test is
  14s — a long tail, no hot spot).
- **No lane or `go list` shard fixes it.** `internal/app` is one Go package; `go list` cannot split
  it. The only lever is a test-level (`-run` / build-tag) partition inside the package — real, but a
  much larger change than 0332, and deferred (see follow-up).

## Decision

Two independent facts drive it: (1) the serial lane removes the oversubscription for every shard; and
(2) in the serial lane the four shards run **sequentially** anyway — four separate `launch; wait`
invocations sum to ~299s, whereas a single `go test -race ./...` invocation is ~206s because go test
overlaps packages internally. So once we serialize, the four-way shard is not just unnecessary
scaffolding — it is *slower* than not sharding. Collapse it.

1. **`tests/test_go_race.sh` runs `go test -race ./...`** over the whole module, in the **serial**
   lane, with a **300s** ceiling. Drop its package-exclusion logic and its four-shard completeness
   guard (a single `./...` run covers the module by construction — nothing to partition, nothing to
   drift).
2. **Delete** `tests/test_go_race_process.sh`, `tests/test_go_race_transaction.sh`,
   `tests/test_go_race_workspace.sh`.
3. **`tests/runtime-budgets.tsv`**: four `-race` rows → one row `tests/test_go_race.sh<TAB>300<TAB>serial`.
4. **`tests/test_runtime_budgets.sh`**: `EXPECTED_SERIAL` 4 → 1; `EXPECTED_TOTAL` re-derived (drop
   25+45+45, main row 60→300 → net +125); and the hard-60s-ceiling relief counter gains **one
   documented, shape-keyed exemption** for the race gate (below). Build-verify the exact
   `EXPECTED_TOTAL` from the edited table (`awk` sum), do not trust the arithmetic here.

### The 300s exemption is a known temporary hole

The 60s ceiling in `tests/test_runtime_budgets.sh` is a deliberate relief counter whose whole job is
to resist "just raise it for this one file." Exempting the race gate is exactly what it guards
against, so the exemption must be **narrow, explicit, and self-guarding**, not a blanket raise:

- Key it on the race-gate path with **its own explicit 300s ceiling**, so a future creep past 300
  still reddens, and the 60s ceiling still binds every other row.
- Bind it to the serial lane: the single `serial` row and the single over-60 row must be the *same*
  file (the `EXPECTED_SERIAL=1` counter plus an assert that the serial row is the race gate). An
  exempt row that were `parallel` would reintroduce the oversubscription this change removes.
- Document at the counter *why* the exemption exists (serial-isolated whole-module gate dominated by
  `internal/app`'s race-independent 190s), that it is temporary, and the follow-up change id that
  owns the principled fix.
- Mutation-test it: raising the race gate to e.g. 999 must redden; the exemption must not silently
  cover any other path.

**Why not fix `internal/app` now.** Partitioning `internal/app` behind build tags (the pattern change
0316 already used for `finalize_e2e_test.go`) is the correct durable fix and would speed both this
gate *and* `test_go_toolchain.sh`'s `go test ./...` (which pays the same ~190s as its long pole — the
package is effectively billed twice per suite). That is a separate, larger change; 0332 is blocking
other tickets, so it takes the exemption now and hands the structural fix to the follow-up.

**Why not pin `GOMAXPROCS`/`-p`.** A serial-lane job runs essentially alone and *should* use all
cores; capping its internal parallelism only makes an isolated gate slower for no contention benefit.
Out of scope, not deferred.

## Validation

- The build gate is self-validating: it runs `scripts/run-tests.sh` against *this branch's* table,
  which already carries the single serial 300s row, so the race gate is scheduled serial by
  construction. A green full-suite gate under load — the failure mode that halted 0329 not
  reproducing — is direct evidence the fix works.
- `tests/test_go_race.sh` passes running `go test -race ./...` (whole module; no package dropped).
- `tests/test_runtime_budgets.sh` passes: one serial row, `EXPECTED_SERIAL=1`, the re-derived
  `EXPECTED_TOTAL`, and the ceiling exemption green — with the mutation checks above confirmed by
  hand during build.
- No stale references to the three deleted shard files remain anywhere in the tree (derive the site
  list from a whole-repo grep; the `runtime-budgets.tsv` header prose describing the 0309/0313/0314
  split is maintained source and must be reconciled, not left dangling).

## Out of scope

- Any change to `scripts/run-tests.sh` scheduling code — the serial lane is already wired.
- `GOMAXPROCS`/`-p` pinning.
- Partitioning `internal/app` / re-examining whether `-race` earns its place on sequential
  integration tests / the toolchain double-payment — **the follow-up change** owns all of this.
- 0251's budget-check regime and 0273's host-relative re-seed — orthogonal; 0273 is the principled
  home the 300s exemption should eventually be subsumed by.

## Follow-up (a new change)

One proposed change captures everything measurement surfaced: `internal/app` is a race-independent
190s package billed twice per suite; the 300s exemption is a temporary hole in the 60s ceiling; the
durable fix is a build-tag/`-run` partition of `internal/app` (per the 0316 `finalize_e2e`
precedent). Related to 251/273/280; discovered_from 332.

## Open questions

None blocking. The exemption's temporariness and the follow-up ownership are stated above.
