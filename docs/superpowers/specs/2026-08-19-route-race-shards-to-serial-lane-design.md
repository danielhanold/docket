<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0332 — Route the -race test shards out of the parallel test pool](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0332-route-race-shards-to-serial-lane.md)**
<!-- docket:backlink:end -->

# Route the `-race` shards out of the parallel test pool

**Change:** 0332 · **Date:** 2026-08-19 · **Type:** refactor · **Priority:** high

## Problem

`scripts/run-tests.sh` runs the suite as a parallel `-j` fan-out, and all four instrumented Go
data-race shards ride in that pool:

- `tests/test_go_race.sh` (budget 60s)
- `tests/test_go_race_process.sh` (25s)
- `tests/test_go_race_transaction.sh` (45s)
- `tests/test_go_race_workspace.sh` (45s)

Each shard runs `go test -race`, whose instrumented build carries a documented ~4–5× slowdown
(`internal/gitcli` alone: 12s → ~49s instrumented, per the shards' own header comments) and which
internally spawns `GOMAXPROCS`-wide race workers. Under the full parallel fan-out, those workers
oversubscribe the cores the shell test jobs need, inflating the shell jobs' wall-clock past their
`tests/runtime-budgets.tsv` ceilings. The result is a **load-dependent build gate**: the same commit
passes or fails depending only on what the machine was doing at that second.

This was proven by change 0329's build-gate halt — a fully-built change blocked purely because the
gate could not go green on a loaded machine, while every failing file passed in isolation, and the
untouched merge-base failed the identical file set. It is corroborated structurally: the `-race`
shards are recent infrastructure (changes 0308–0314), and `tests/runtime-budgets.tsv` documents
20–25 % wall-clock swings on an *identical* commit between quiet and loaded passes.

The two existing budget-regime changes do not address this. 0251 (hardware-pinned 5/2 slack) makes
the budget *check* tolerant of a slow run but does not remove the contention; 0273 re-seeds a
pre-migration set of *shell* budget rows and never measured the Go shards. The contention itself —
the `-race` shards co-scheduled with the shell fan-out — is unowned.

## Decision

`scripts/run-tests.sh` already has an unused **serial lane**. `tests/runtime-budgets.tsv`'s third
column is `parallel|serial`; the runner splits on it at `scripts/run-tests.sh` (the `mode_of` split
into the `PAR`/`SER` arrays) and runs the `serial` lane one file at a time (the `SER` loop's
`launch; wait` sequence). Every one of the current rows is `parallel` — the lane has zero users.

Flip the four `-race` shard rows from `parallel` to `serial`. Nothing in `run-tests.sh` changes —
the mechanism is already wired. The shards then run one at a time, after the parallel pool drains,
each with the machine effectively to itself.

**Why not also pin `GOMAXPROCS`/`-p`.** In the serial lane a shard runs essentially alone, so it
*should* use all cores — that is what makes it finish fast. Capping its internal parallelism would
make an already-isolated shard slower for no contention benefit. `GOMAXPROCS`/`-p` earns its keep
only for a job running concurrently with others, which serialization has removed. Pinning is
therefore out of scope, not merely deferred.

## What changes

- `tests/runtime-budgets.tsv`: change the lane column to `serial` on the four `-race` shard rows.
  No budget seconds change unless re-measurement (below) shows a shard's serial-lane cost genuinely
  differs from its current row; a row moves only on measured re-shape, never to paper over the host
  slowing down (the standing `runtime-budgets.tsv` / `scripts/run-tests.md` rule).
- Re-measure each shard in the serial lane and record the method and readings in the results
  artifact, the same way the shards' existing header comments record their `-j 1 --timings`
  measurements.

## Validation

- The four shards each pass in the serial lane (`scripts/run-tests.sh` full run), and their budget
  rows hold under re-measurement.
- A **full-suite run under load** no longer flakes the shell tests: the failure mode that halted
  0329 (shell files breaching their rows while the `-race` shards run) does not reproduce.
- Any test that guards the budget table's shape (row count, column values, partition asserts in the
  shards themselves) still passes — a lane-column edit must not falsify a table-shape guard.

## The one residual, and the follow-up lever

The `SER` loop's first `launch; wait` starts the first serial shard while the parallel tail is still
draining, so exactly one race shard can briefly overlap the tail. This is expected to be
immaterial (one shard, a few seconds), and the change measures rather than assumes it. **If** that
overlap still flakes under load, the correct next lever is **ordering** — schedule the race shards
strictly last so nothing overlaps them — not `GOMAXPROCS` pinning. Ordering is called out here as
the sanctioned follow-up so a later reader does not reach for the parallelism cap this spec
deliberately rejected; it is out of scope for this change unless measurement demands it.

## Out of scope

- Any change to `scripts/run-tests.sh` scheduling code (the serial lane is already wired).
- `GOMAXPROCS`/`-p` pinning of the shards.
- 0251's budget-check regime and 0273's shell-row re-seed — orthogonal, separately tracked.
- Re-shaping or re-sharding the `-race` gate's coverage (it is already sharded to a `go list ./...`
  partition, and repo-wide `-race` scope is deliberate).

## Open questions

- None blocking. The serial-lane cost of each shard and the reality of the first-shard/tail overlap
  are resolved by the re-measurement step, not by up-front decision.
