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
related: [150, 173, 174, 176]
discovered_from: [168]
adrs: []
spec: docs/superpowers/specs/2026-07-31-sync-agents-per-invocation-cost-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-07-31-sync-agents-per-invocation-cost-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-31-sync-agents-per-invocation-cost-design.md) |
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

**Measured 2026-07-31** (the profile the stub asked for): the 5.5s is ~2,430 subprocess forks —
976 `sed`, 770 `head`, 477 `awk`, 204 `grep` — spent re-parsing three small YAML layer files. Not
git, not I/O, not the config resolver. `harness_agent_line` re-parses a whole layer file (five
forks) on every (harness, agent, layer) triple: 192 calls over ~6 distinct parses.

The goal is **real-run speed**, not just suite speed — a cheaper generation pass pays out for every
run a human, an `install.sh`, or a skill triggers, and the suite improvement follows for free.

## What changes

Parse each layer file once per run and cache it; extract fields with bash builtins instead of
forking. The layer-precedence logic is deliberately untouched, so the existing test suite stays the
correctness oracle. Also add real argument validation — today an unrecognized flag (including
`--help`) silently falls through into a full generation pass that writes wrapper files.

Because a perf change has no oracle in the suite, acceptance is measured wall clock recorded in the
results file, plus a standing fork-count assert that goes red if the cache is ever silently inert.

Design: [`docs/superpowers/specs/2026-07-31-sync-agents-per-invocation-cost-design.md`](../../superpowers/specs/2026-07-31-sync-agents-per-invocation-cost-design.md).

## Out of scope

- `test_render_board.sh` — a different script an order of magnitude smaller, not yet shown to share
  the cause. File a stub only if the idiom transfers.
- `docket-config.sh` per-invocation cost — change 0176, kept independent and cross-linked.
- Git fixture reuse — change 0174.
- A parallel suite runner, and toolchain pinning — change 0150.

## Reconcile log
