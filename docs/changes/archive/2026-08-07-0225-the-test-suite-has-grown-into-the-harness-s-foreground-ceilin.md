---
id: 225
slug: the-test-suite-has-grown-into-the-harness-s-foreground-ceilin
title: The test suite has grown into the harness's foreground ceiling — cut its wall-clock runtime
status: killed
priority: medium
type: perf
created: 2026-08-06
updated: 2026-08-07
depends_on: []
related: [150, 175, 185, 223, 224]
discovered_from: [203]
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

The suite is 79 test files and takes **~10 minutes** wall-clock — right at the maximum foreground
timeout the Claude Code harness allows. Measured on 2026-08-06: an independent foreground run
reached `test_sync_agents_opencode.sh` (74th of 79) at exactly the 10-minute kill, and an identical
gate run inside the 0203 implementer returned `Exit code 143` where another completed.

Change 0223 makes the gate **survive** that ceiling by changing execution posture. This change
addresses why the gate is pressed against a ceiling at all. The two are independent: 0223 is
correct even on a fast suite, and this one is worth doing even with 0223 landed.

Cost compounds per run, not per change. A single 0203-shaped change ran the full suite **three
times** — initial gate, post-review-fix re-gate, and a pre-push re-verify after the results commit
— so ~30 minutes of a ~100-minute run was suite execution. `finalize.gate: local` then re-runs it
again at merge. Every future change pays this.

Two predecessors already worked this line and both landed: change 0175 cut `sync-agents.sh`'s
~5.5s-per-invocation cost, which dominated the suite, and change 0185 added per-assertion and
per-command timing profilers. **Those profilers are the natural starting instrument here** — the
measurement tooling already exists, so this change should begin by using it rather than by guessing.

Change 0150 (pin or report the resolved shell toolchain across the suite) is adjacent and still
`proposed`; whatever it decides about toolchain resolution may interact with per-file process
startup cost.

## What changes

Reduce full-suite wall-clock materially. Scope to be settled at grooming, but the candidate levers:

- **Profile first** — use the 0185 profilers to rank the actual cost centres rather than assume.
  Expect a long tail of per-file process startup on top of a few dominant files.
- **Parallel or sharded execution** — the suite is a loop over independent per-file scripts, which is
  the shape that parallelizes most cheaply. Determinism and interleaved output are the risks.
- **Fixture reuse** — repeated tmpdir/repo-fixture construction across files is a likely shared cost.

## Out of scope

- The gate's execution posture — that is change 0223.
- Exit-code keying — that is change 0224.
- Deleting or weakening tests to buy time. Coverage is not the budget being spent here.

## Open questions

- What is the actual distribution — a few dominant files, or a flat long tail? Decides whether
  parallelism or targeted fixes pays more.
- Does parallel execution break any test's assumptions (shared tmp paths, fixed ports, git fixtures,
  the metadata worktree)?
- Is there a target worth naming (e.g. comfortably under half the ceiling), or is "materially faster"
  the honest goal?
- Does change 0150's toolchain resolution change per-file startup cost, and should these be sequenced?

## Reconcile log

## Why killed

Superseded by change 0227 (parallel test-suite runner). 0227 is the groomed, build-ready realization of this problem statement: it profiled the suite (629s, 5954 assertions, a four-file tail at 66% of wall time), and specs a parallel runner plus tail sharding to a <157s target, answering all four of this change's open questions. 0223 (execution posture) and 0224 (exit-code keying) remain independent and now point at 0227. The one lever 0227 leaves out of scope — per-invocation cost of sync-agents/resolver calls — is self-surfacing via 0227's per-file runtime-budget guard and can be re-proposed with data if the tail regrows.
