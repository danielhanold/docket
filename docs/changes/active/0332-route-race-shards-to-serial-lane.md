---
id: 332
slug: route-race-shards-to-serial-lane
title: Route the -race test shards out of the parallel test pool
status: 'in-progress'
priority: high
type: refactor
created: 2026-08-19
updated: '2026-08-19'
depends_on: []
stacked_on:
related: [251, 273]
discovered_from: [329]
adrs: []
spec: docs/superpowers/specs/2026-08-19-route-race-shards-to-serial-lane-design.md
plan: 'docs/superpowers/plans/2026-08-19-route-race-shards-to-serial-lane.md'
results:
trivial: false
auto_groomable:
branch: 'feat/route-race-shards-to-serial-lane'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-19T21:50:14Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-19-route-race-shards-to-serial-lane-design.md` |
| Plan | `docs/superpowers/plans/2026-08-19-route-race-shards-to-serial-lane.md` |
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

**Scope expanded 2026-08-20** (see the "Scope expanded" reconcile entry and the rewritten spec).
Measurement showed the main shard cannot meet a 60s ceiling by any lane — it is ~206s, dominated by
`internal/app`'s ~190s integration suite, a cost independent of the race detector (~1.05× multiplier)
and unshardable by `go list` (one package). So this change now **collapses the four shards into one**:

- `tests/test_go_race.sh` runs `go test -race ./...` (whole module), **serial** lane, **300s**
  ceiling; its package-exclusion logic and four-shard completeness guard are removed.
- Delete `tests/test_go_race_process.sh`, `tests/test_go_race_transaction.sh`,
  `tests/test_go_race_workspace.sh`.
- `tests/runtime-budgets.tsv`: four `-race` rows → one `tests/test_go_race.sh<TAB>300<TAB>serial`;
  reconcile the header prose that describes the deleted 0309/0313/0314 split.
- `tests/test_runtime_budgets.sh`: `EXPECTED_SERIAL` 4 → 1, `EXPECTED_TOTAL` re-derived from the
  edited table, and one documented, shape-keyed, self-guarding **exemption** to the 60s ceiling for
  the race gate (its own 300s sub-ceiling; bound to the serial lane; mutation-tested).

The 300s exemption is a **known temporary hole**; the durable fix (a build-tag/`-run` partition of
`internal/app`) is tracked as the follow-up change below.

## Out of scope

- Any change to `scripts/run-tests.sh` scheduling code — the serial lane already exists.
- `GOMAXPROCS`/`-p` pinning of the gate.
- Partitioning `internal/app`, re-examining `-race` on sequential integration tests, the toolchain
  double-payment — the **follow-up change** owns all of this.
- 0251's budget-check regime and 0273's host-relative re-seed — orthogonal, separately tracked.

## Open questions

None blocking. The exemption is explicitly temporary; the follow-up change owns the principled fix.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-19

### 2026-08-19 — reconciled against current reality

Verified the spec's premises against the current tree before planning:

- The four instrumented `-race` shard rows in `tests/runtime-budgets.tsv` (`test_go_race.sh`, `test_go_race_process.sh`, `test_go_race_transaction.sh`, `test_go_race_workspace.sh`) are all still lane `parallel` — the exact rows this change flips.
- The serial lane in `scripts/run-tests.sh` is still wired and unused: `mode_of`, the `PAR`/`SER` split, and the `SER` loop's `launch; wait` sequence are present, and no current budget row is `serial` — so the flip needs no runner-code change, as the spec states.
- Related changes 251 and 273 remain `proposed` and orthogonal (budget-check regime and shell-row re-seed); discovered_from 329 remains halted at its gate. None alters this change's scope.

No scope adjustment required; proceeding to plan and build as specified.

## Scope expanded — halt cleared (2026-08-20)

The 2026-08-19 halt below was correct: the plan's re-measurement could not make the main shard's row
hold, and the repo rules forbid papering over it. A human-driven investigation (Daniel + interactive
session) then measured the root cause and re-scoped the change rather than chasing an idle machine:

- The main shard is **~206s standalone serial**, dominated by **`internal/app` (~200s)**; the other
  three fit their rows in the same loaded run (18/36/39 vs 25/45/45). So it is not host contention —
  the main shard is genuinely oversized.
- `internal/app` is **190s uninstrumented** (~1.05× under `-race`): the race detector is not the
  cost; the cost is the package's own ~316-test integration/e2e suite. It is a single Go package, so
  no lane or `go list` shard can bring it under 60s.

**Decision (approved by Daniel):** collapse the four shards into one serial `go test -race ./...`
gate with a documented, temporary **300s ceiling exemption** — see the rewritten spec's *Decision*
section for the full design (exemption must be shape-keyed, serial-bound, and mutation-tested). The
durable fix (`internal/app` build-tag partition) is deferred to a new follow-up change. The halt is
**cleared**: the re-measurement judgment that blocked the autonomous run is removed by the exemption,
so the build no longer needs a trustworthy sub-60s reading for the main shard. Note the earlier
lane-flip commit `d602ef1e` is superseded by the collapse and its edits will be largely rewritten.

## Run halted

Halted 2026-08-19 by an autonomous docket-implement-next run at the Step-5 build gate.

**What the run completed.** Reconcile is done (`reconciled: true`, above). The plan is written and
attached (`plan:` set; commit on the feature branch). Task 1 of the plan — the actual fix — is built
and committed on branch `feat/route-race-shards-to-serial-lane` (commit `d602ef1e`): the four
instrumented `-race` shard rows in `tests/runtime-budgets.tsv` are flipped `parallel` -> `serial`,
`tests/test_runtime_budgets.sh`'s relief counter `EXPECTED_SERIAL` moved `0 -> 4` with the forcing
shared state named at the counter, `EXPECTED_TOTAL` unmoved at 2140, the tsv header note added.
`bash tests/test_runtime_budgets.sh` passes and the runner schedules the four files into the serial
lane (verified against `tests/test_go_race_process.sh` + `tests/test_runtime_budgets.sh`).

**Why halted (needs a human).** Plan Task 2 — the serial-lane re-measurement that must record
trustworthy readings and confirm the rows hold — could not complete on this host, and the same
contamination defeats the Step-5 full-suite build gate, so no trustworthy green build-evidence can be
minted here:

- The measurement worker took three interleaved standalone-serial readings per shard
  (`scripts/run-tests.sh -j 1 --timings`). `test_go_race.sh` read 218/220/222s against its 60s row
  (its always-measured standalone cost is 53–60s); `test_go_race_workspace.sh` read 41/42/41s
  against its 45s row (history 37/37). `test_go_race_process.sh` (17s→row 25) and
  `test_go_race_transaction.sh` (39s→row 45) held.
- This is **host contention, not a genuine re-shape**: Task 1 changed only the tsv lane column — the
  shards' test code is byte-identical to when the header last measured them — and a shard's cost
  cannot re-shape without a code change. `ANECompilerService`, XprotectService, and Spotlight
  `mds`/`mds_stores` were saturating the machine during the run (the gitcli-dominated main shard
  re-execs dozens of freshly-built instrumented git binaries, each of which Xprotect/Gatekeeper
  scans), and the three rounds were stably slow (218/220/222), ruling out cold-cache warmup.
- The repo's standing rule forbids the alternatives: recording these numbers as the shards' serial
  cost, marking the rows "holds" off a 222s worst, or raising the workspace row to 50, would be
  "papering over a slow host" (Global Constraints; learnings `budget-headroom-is-spent-before-it-is-breached`,
  `tolerance-constant-calibrated-on-one-machine`). Plan Step 2 explicitly routes a main-shard reading
  over its 60s ceiling to "STOP and report the readings as a blocker" — a shard split is out of scope.
- This is not an escalation: a stronger model faces the identical macOS host with the same
  Xprotect/Spotlight/ANE daemons, which cannot be quiesced from inside the run. At the time of the
  halt the load average was still ~2.1 with Spotlight/WindowServer active.

**What a human must decide.** Re-measure the four shards on a genuinely idle machine to obtain
trustworthy standalone-serial readings, then finish plan Tasks 2–3 (write the results artifact, run
the full-suite gate that IS the experiment, record the overlap residual) and open the PR — or, if the
existing rows are accepted on the strength of the design argument (a lane change moves no code, and
the rows were originally sized from exactly this standalone-serial reading), decide that explicitly.
The lane-flip fix itself (commit `d602ef1e`) is sound and its guard is green; only the re-measurement
validation and the trustworthy full-suite green gate are blocked. The interleaved timing TSVs and the
measurement log are on this machine for inspection (see the run report).
