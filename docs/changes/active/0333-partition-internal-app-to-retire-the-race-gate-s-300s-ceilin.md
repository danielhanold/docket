---
id: 333
slug: partition-internal-app-to-retire-the-race-gate-s-300s-ceilin
title: 'Partition internal/app to retire the -race gate''s 300s ceiling exemption'
status: proposed
priority: medium
type: refactor
created: 2026-08-20
updated: 2026-08-20
depends_on: []
stacked_on:
related: [251, 273, 280]
discovered_from: [332]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while re-scoping change 0332 (route the `-race` shards out of the parallel
pool). 0332 collapsed the four `-race` shards into one serial `go test -race ./...` gate and gave it
a **temporary 300s exemption** to `tests/test_runtime_budgets.sh`'s hard 60s ceiling, because
measurement showed the main shard cannot meet 60s by any lane or `go list` shard. That exemption is a
known hole in a deliberate relief counter; this change closes it properly.

**What measurement found (2026-08-20, one machine, normal load):**

- The `-race` main shard is **~206s standalone serial**, and it is **essentially one package**:
  per-package `-race` was `internal/app` ~200s, `internal/githubcli` 83s, `internal/gitcli` 62s, all
  17 other packages ≤5s. In a single shared-build `./...` invocation the cheap packages overlap under
  `internal/app`'s long pole, so `internal/app` sets the shard's wall clock.
- **The race detector is not the cost.** `internal/app` is **190s uninstrumented**, ~200s under
  `-race` — a ~1.05× multiplier. The 190s is the package's own suite: 47 test files, ~316 tests,
  finalize/planning integration tests that shell out to real `git` (slowest single test 14s — a long
  tail, no hot spot). Per-file buckets: finalize cluster ~115s (cleanup 26, git 22, merge 17, rebase
  16, closeout 15, block 11, publish 8), change/planning/evidence ~70s, everything else ~15s.
- **`internal/app` is billed twice per suite.** `tests/test_go_toolchain.sh`'s plain `go test ./...`
  pays the same ~190s as *its* long pole, and the `-race` gate pays it again. ~380s of suite
  wall-clock rides on this one package.

**The work.** Bring `internal/app`'s race/integration cost under the normal 60s regime so the 300s
exemption can be removed, and cut the double-payment. The candidate mechanism is the one change 0316
already used for `internal/app/finalize_e2e_test.go`: partition the heavy integration/e2e tests
behind a **build tag** into dedicated shard(s), which excludes them from `go test ./...` *and* lets
them carry `t.Parallel()`. A `-run` partition is the fallback. Whichever is chosen: multiple shards
each under 60s, `go test ./...` no longer paying the e2e cost, the 300s exemption in
`tests/test_runtime_budgets.sh` deleted, and `EXPECTED_SERIAL`/`EXPECTED_TOTAL` re-derived.

**Also in scope to decide, not assume:** whether these sequential subprocess integration tests earn
their place under `-race` at all (the detector's value is on concurrent code — the adapter surfaces
in `gitcli`/`githubcli`, not the finalize drivers), and whether some belong in a slower lane run
outside the per-file-budgeted suite entirely.

**Boundary** — this change owns the `internal/app` partition and the exemption removal. It does not
re-open 0332's collapse (that gate stays as the single serial `./...` entry point until the partition
lands). Related to 0251 (budget-check regime) and 0273 (host-relative budgets — the principled home
the 300s exemption should ultimately be subsumed by) and 0280 (shard/re-budget OVER-BUDGET files);
none of those measured the Go shards, so this is distinct work.

**Reason for deferral from 0332** — 0332 is blocking other tickets and the partition is a materially
larger change (identifying the tag boundary across ~316 tests, wiring new shard files + budget rows +
guards, changing what `go test ./...` covers). 0332 takes the exemption now; this change does it right.
