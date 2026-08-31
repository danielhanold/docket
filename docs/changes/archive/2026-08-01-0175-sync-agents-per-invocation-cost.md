---
id: 175
slug: sync-agents-per-invocation-cost
title: sync-agents.sh costs ~5.5s per invocation and dominates the test suite
status: done
priority: medium
type: perf
created: 2026-07-31
updated: 2026-08-01
depends_on: [173]
related: [150, 173, 174, 176]
discovered_from: [168]
adrs: []
spec: docs/superpowers/specs/2026-07-31-sync-agents-per-invocation-cost-design.md
plan: docs/superpowers/plans/2026-07-31-sync-agents-per-invocation-cost.md
results: docs/results/2026-07-31-sync-agents-per-invocation-cost-results.md
trivial: false
auto_groomable: false
branch: feat/sync-agents-per-invocation-cost
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/144
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-31-sync-agents-per-invocation-cost-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-31-sync-agents-per-invocation-cost-design.md) |
| Plan | [2026-07-31-sync-agents-per-invocation-cost.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-31-sync-agents-per-invocation-cost.md) |
| Results | [2026-07-31-sync-agents-per-invocation-cost-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-31-sync-agents-per-invocation-cost-results.md) |
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

### 2026-07-31 — reconciled against landed 0173 and current `origin/main`

Re-read the change and spec against `related` (0150, 0173, 0174, 0176), recent ADRs 0062–0065,
and current `origin/main` (`07818ef4`). Dependency 0173 is `done`; its widened `field_of`, new
`field_of_raw`, and two-leg bare-scalar validator are now the baseline this optimization must
preserve. The old narrow-class text in the spec was refreshed accordingly.

One implementation constraint was made explicit: `harness_agent_line` is consumed through command
substitution, so a cache filled lazily inside that helper would be filled in a subshell and lost.
The cache must therefore be populated synchronously by its caller before command-substituted reads;
the standing fork-count guard proves that priming remains effective.

The existing suite already has tab-indented agent-layer cases, so this change preserves those
assertions rather than adding a duplicate. Change 0169 is concurrently editing `sync-agents.sh`
and its tests on a separate in-progress feature worktree; none of its uncommitted bytes were read or
adopted. Its committed scope adds Codex sidecar defaults and does not invalidate this change, but
normal PR conflict reconciliation may be needed if it merges first.

No scope change and no adjacent follow-up met the auto-capture bar. The shared-extractor follow-up
is already change 0179, and the separate resolver optimization is already change 0176.

### 2026-07-31 — build halted before Task 1

The configured `skills.build: docket-build` role routed Task 1 to its economy profile, but the
named worker dispatch was rejected before execution: `claude-opus-5` is not supported when using
Codex with a ChatGPT account. No task code or task commit was produced. The profile-routed build
contract is Tier C and does not authorize inline substitution; the feature worktree is preserved
at plan commit `38ee3624` for inspection or resume after the installed profile mapping is made
compatible with this harness.

### 2026-07-31 — build resumed after profile repair

Preflight re-verified the `in-progress` claim and clean feature worktree. `origin/main` remains
`07818ef4`, the exact integration SHA used by the prior reconcile, so the reconciled design is
still fresh and no repeated scope edit is needed. Resume continues at Task 1 of the linked plan.

### 2026-07-31 — build halted at Task 3 performance guard

The repaired profile workers completed Task 1 (`f9b84b97`) and Task 2 (`655f3cdf`) with their
focused suites green. Task 3's real PATH-shim measurement found 644 retained parser-tool calls
(`sed` 288, `head` 0, `awk` 185, `grep` 171), versus 788 on `origin/main`. That is an improvement
but remains above the plan's required standing ceiling of 400, so the new guard is correctly red.

Task 3 owns the test guard, not a second source refactor, and returned `BLOCKED` with no commit.
The configured build contract makes that an unsafe-continuation halt: it cannot silently widen the
accepted budget or reopen completed Task 2. Its 39-line uncommitted test change remains in the
feature worktree for inspection; review, full-suite gate, results close-out, and PR creation were
not reached.

### 2026-07-31 — optimization resumed with target retained

The human explicitly kept the `<400` parser-call ceiling as the goal and authorized extending the
Task 2 source optimization until Task 3's retained guard turns green. Preflight re-verified the
claim and preserved worktree; `origin/main` is still `07818ef4`, so the reconcile remains fresh.
Resume keeps commits `f9b84b97` and `655f3cdf` plus the uncommitted 39-line guard, and routes the
unresolved performance repair through the configured build profile rather than weakening the
target.
