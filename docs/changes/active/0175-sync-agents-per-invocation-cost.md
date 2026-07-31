---
id: 175
slug: sync-agents-per-invocation-cost
title: sync-agents.sh costs ~5.5s per invocation and dominates the test suite
status: proposed
priority: medium
type: perf
created: 2026-07-31
updated: 2026-07-31
depends_on: []
related: [150, 174]
discovered_from: [168]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

A single `bash sync-agents.sh --help` takes **5.5 seconds**. `--help` is not a recognized flag, so
it falls through into a full generation pass — the script has no cheap path at all.

That cost is multiplied by the tests that exercise it. Measured 2026-07-31:
`test_sync_agents.sh` 197.8s, `test_sync_agents_codex.sh` 66.8s, `test_sync_agents_cursor.sh`
14.3s — **279s of a 530s suite, 53% of total wall clock**, spent re-running full wrapper
generation per assertion group.

Change 0174 makes git fixtures cheap by reuse, which does nothing here: these files are
invocation-bound, not fixture-bound. `test_render_board.sh` (17.8s over ~163 invocations of
`render-board.sh` at ~0.15s each) is the same class at a smaller scale and may belong in the same
design.

This is worth designing rather than patching, because the two candidate fixes point in different
directions and have different value: a `--help`/no-op fast path is cheap and narrow, while making
generation itself faster would pay out for every real `sync-agents.sh` run a human or an install
triggers — not just for the suite. Which of those is the actual goal is exactly the question a
brainstorm should settle.

## What changes

To be designed. At minimum, establish where the 5.5s actually goes before choosing between a
fast-path flag, a cheaper generation pass, or test fixtures that stop invoking the real generator
per assertion.

## Out of scope

- Git fixture reuse — change 0174.
- A parallel suite runner — change 0150 records the missing-runner gap.

## Open questions

- Where does the 5.5s go? No profile has been taken; the number is wall clock on a full pass.
- Is the goal suite speed, or real-run speed? They select different fixes.
- Does `test_render_board.sh` belong in this change or its own?

## Reconcile log
