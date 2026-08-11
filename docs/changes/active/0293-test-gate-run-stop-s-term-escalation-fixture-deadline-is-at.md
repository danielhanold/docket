---
id: 293
slug: test-gate-run-stop-s-term-escalation-fixture-deadline-is-at
title: 'test_gate_run_stop''s TERM-escalation fixture deadline is at exact parity with stop_run''s own TERM budget'
status: proposed
priority: medium
type: fix
created: 2026-08-11
updated: 2026-08-11
depends_on: []
related: []
discovered_from: [284]
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

**Trigger** — surfaced at change 0284's build gate. The full 106-file suite came back RED on
`tests/test_gate_run_stop.sh`, at the single assert *"the stop is held where the completed marker
would be written"*. The same file passed standalone, passed under 4x self-contention, and passed on
a clean `origin/main` worktree under the same load; a second full-suite run on the same branch was
green. It is a flake, and reading the two sides shows it is a flake with **exactly zero margin**.

**Opportunity** — the fixture's deadline is at *exact parity* with the thing it waits on, and
nothing in the repo relates the two numbers. That leg's child does `trap "" TERM`, so
`gate-run.sh`'s `stop_run` must exhaust its whole TERM wait before it escalates to KILL and reaches
the `post-kill-pre-annotate` barrier:

- `stop_run`'s TERM wait is `while kill -0 … && [ "$waited" -lt 20 ]; do sleep 0.5; …` — **10.0s**.
- `tests/lib/gate_run_common.sh`'s `wait_for_file` defaults to `ticks=100` with `sleep 0.1` —
  **10.0s**.

The fixture therefore gives up at the same instant the production path is expected to act. Under
full-suite parallel contention any scheduling jitter tips it, and the failure reads as a real
regression in the file every refactor of `gate-run.sh` is gated on — which is exactly how it
presented at 0284's gate, costing a full diagnostic cycle to clear.

This is the runtime-cost sibling of `budget-headroom-is-spent-before-it-is-breached`, one level
down: a **fixture** deadline rather than a budget row, and it reports binary (waited / timed out)
while hiding that the headroom is gone.

**Independent value** — stands with 0284 fully reverted: both numbers, and the parity between them,
are on `origin/main` today and were verified there. 0284 touches the identity predicate, never
`stop_run`'s escalation timing.

**Boundary** — in scope: give the TERM-escalation legs of `tests/test_gate_run_stop.sh` a deadline
with real headroom over `stop_run`'s own escalation budget, ideally *derived* from it rather than
restated, so the two cannot drift apart again; and sweep `tests/lib/gate_run_common.sh`'s other
`wait_for_file` call sites for the same parity (derive the site list from a whole-repo grep, never
hand-list it). Deliberately out of scope: changing `stop_run`'s 20x0.5s TERM budget or its KILL
escalation, `gate-run.sh`'s semantics, the `barrier`/`wedge` hooks themselves, and the pre-existing
`tests/test_sync_agents_runners.sh` budget breach (change 0280).

**Reason for deferral** — 0284's spec makes `tests/test_gate_run.sh` and
`tests/test_gate_run_stop.sh` passing **unchanged** its own safety net: an edit to either file is
the declared tell that its `gate-run.sh` refactor was not behaviour-preserving. Fixing this on that
branch would remove the evidence that the refactor was safe, so it cannot ride 0284 at any price.
