---
id: 332
slug: route-race-shards-to-serial-lane
title: Route the -race test shards out of the parallel test pool
status: proposed
priority: high
type: refactor
created: 2026-08-19
updated: 2026-08-19
depends_on: []
stacked_on:
related: [251, 273]
discovered_from: [329]
adrs: []
spec: docs/superpowers/specs/2026-08-19-route-race-shards-to-serial-lane-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-19-route-race-shards-to-serial-lane-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-19-route-race-shards-to-serial-lane-design.md) |
<!-- docket:artifacts:end -->

## Why

The build gate is load-dependent: the same commit passes or fails only according to what else the
machine is doing. Change 0329 was built end-to-end and then halted at the gate for exactly this
reason — every failing file passed in isolation, and the untouched merge-base failed the identical
set. The cause is the four instrumented `go test -race` shards riding the parallel `-j` fan-out:
each spawns `GOMAXPROCS`-wide race workers that oversubscribe the cores the shell test jobs need,
pushing those jobs past their `runtime-budgets.tsv` ceilings. This is recent — the `-race` shards
landed in changes 0308–0314 — and it makes the local gate unusable under load regardless of the
change under test.

Neither existing budget change covers it: 0251 makes the budget *check* tolerant of a slow run but
leaves the contention in place, and 0273 re-seeds a pre-migration set of shell rows and never
measured the Go shards.

## What changes

Flip the lane column from `parallel` to `serial` on the four `-race` shard rows in
`tests/runtime-budgets.tsv`. The runner's serial lane is already wired (`scripts/run-tests.sh`) and
has zero current users, so no runner code changes — the shards simply run one at a time, after the
parallel pool drains, each with the machine to itself. Re-measure the shards in the serial lane and
record the readings. Design detail, the deliberate rejection of `GOMAXPROCS` pinning, and the
ordering follow-up lever are in the linked spec.

## Out of scope

- Any change to `scripts/run-tests.sh` scheduling code — the serial lane already exists.
- `GOMAXPROCS`/`-p` pinning of the shards (would slow an already-isolated shard for no benefit).
- 0251's budget-check regime and 0273's shell-row re-seed — orthogonal, separately tracked.
- Re-shaping the `-race` gate's coverage — its `go list ./...` partition and repo-wide scope are
  deliberate.

## Open questions

None blocking. The serial-lane cost of each shard and the first-shard/tail overlap are settled by
the re-measurement step, not by up-front decision.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
