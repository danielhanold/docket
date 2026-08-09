---
id: 268
slug: de-flake-the-reclaim-leg-of-test-docket-status-under-paralle
title: 'De-flake the reclaim leg of test_docket_status under parallel contention'
status: proposed
priority: medium
type: fix
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [245]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while running change 0245's full-suite build gate. The first gate run came back `EXIT=1` on a single assert in `tests/test_docket_status.sh`: `NOT OK - reclaim(auto off): prints the state-valid remedy naming docket.sh reclaim-claims`. Run standalone the suite passes; re-run in parallel it passed. Change 0245 touches none of `tests/test_docket_status.sh`, `scripts/docket-status.sh`, or `scripts/reclaim-claims.sh`, so this is a pre-existing flake under parallel contention, not a regression.

**Opportunity** — the reclaim leg of `test_docket_status.sh` is not hermetic under `scripts/run-tests.sh`'s parallel phase. Whatever shared state or timing it depends on (a lease-TTL comparison against wall-clock, or a fixture path another suite also writes) makes its verdict a function of machine load rather than of the code under test. A flaky assert in the suite that gates every merge is worse than a missing one: it trains readers to re-run until green, which is exactly how a real red gets waved through.

**Independent value** — stands entirely with change 0245 reverted; the flake predates that branch and will keep firing on unrelated branches until the shared state or timing dependency is removed.

**Boundary** — diagnose and de-flake the `reclaim(auto off)` leg of `tests/test_docket_status.sh`: isolate its fixture state and remove any wall-clock or load-sensitive dependency, then demonstrate stability under `scripts/run-tests.sh` at high `-j`. It stops there. It does not re-audit the other suites for hermeticity, and it does not touch `scripts/reclaim-claims.sh`'s production behavior unless the diagnosis proves the flake lives there.

**Reason for deferral** — change 0245's branch is scoped to `sync-agents.sh` wrapper generation and its own new suites. Debugging an unrelated suite's parallel-contention flake would expand that branch into a second, independently reviewable concern, and its fix belongs with someone reading the reclaim leg's own design rather than riding a refactor branch.
