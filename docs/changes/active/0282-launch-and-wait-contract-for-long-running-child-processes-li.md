---
id: 282
slug: launch-and-wait-contract-for-long-running-child-processes-li
title: 'Launch-and-wait contract for long-running child processes — liveness-keyed, not marker-keyed'
status: proposed
priority: critical
type: fix
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: [251, 260, 273, 275, 277]
discovered_from: [276]
adrs: []
spec: docs/superpowers/specs/2026-08-09-launch-and-wait-contract-for-long-running-child-processes-li-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-launch-and-wait-contract-for-long-running-child-processes-li-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-launch-and-wait-contract-for-long-running-child-processes-li-design.md) |
<!-- docket:artifacts:end -->

## Why

Finalizing change 0276 took ~30 minutes, of which **~20 were spent polling a process that had
already been dead for 19 of them**. The mechanism, reconstructed from the run log:

1. The gate launched `scripts/run-tests.sh` in a way that tied the child to the Bash tool call's
   lifetime. When the call ended, the suite took a `TERM` at 18s: `run-tests: interrupted (TERM)
   after 18s — 0 of 102 test files had finished`.
2. The wait loop keyed on a **success marker** — `until grep -q "^EXIT=" <file>` — in a file the
   dead process was the only writer of. The marker could never appear (verified: that file has
   zero `EXIT=` lines and was last written at the moment of death).
3. The loop therefore ran to its bound **twice** (582s + 584s) before the agent tailed the actual
   suite log, which had been sitting at 131 bytes saying "interrupted (TERM)" the whole time.

The relaunch — `nohup` to survive the tool call, polling `kill -0 $pid` instead of a marker —
worked and took the honest 206s.

Two distinct defects, and the second is the expensive one:

- **Launch:** the suite was not detached, so it died with its tool call. This hazard is already
  known (the docket suite has outgrown the foreground Bash ceiling) but the rule evidently is not
  reaching the agents that need it at the moment they launch a suite.
- **Wait:** the guard keyed on *the success marker appearing* rather than on *the process being
  alive*. Those two conditions differ **exactly when the thing you are waiting for dies** — which
  is precisely when the guard needs to fire. A liveness check fails in seconds; a marker check
  waits out its full timeout and then reports nothing diagnostic.

This is the same defect shape as the finding that same run harvested
(`refusal-keyed-on-residue-not-condition.md`): keying on a downstream artifact instead of on the
condition itself. It is filed `critical` because it is silent in both directions — the run reported
success, and the 20 wasted minutes appear nowhere in its report. It was found only by reading the
log. Every autonomous docket run that shells out to the suite carries this exposure, and the cost
is paid in wall-clock on the longest, least-supervised runs.

## What changes

Settled design (2026-08-09 auto-groom; critic-gated, two rounds, 0 needs-human; full decision
trail in the linked spec's `## Assumptions`):

- A new shared helper, **`scripts/gate-run.sh`** (+ `gate-run.md` contract), with two verbs:
  `--launch` (detached new-session start, every stream to a durable per-run dir, pid/pgid plus an
  opaque identity token recorded, a separate atomic `EXIT=<code>` sentinel file written on child
  exit) and `--observe` (one short-lived observation; five states — `running` / `passed` /
  `failed` / `died` / `unavailable` — with liveness anchored on the identity-checked process
  group, never solely on the sentinel). A dead child is detected on the next observation, not at
  a wait-loop bound; callers key on the stdout report line.
- **Death is a distinct outcome from failure.** `died` (never finished) never collapses into
  `failed` (ran and went red) and never mints repair work.
- Call-site posture on `died`: kill the recorded group, then **one** bounded relaunch — scoped to
  idempotent children (the suite gate); non-idempotent sites keep their existing failure
  postures. Second death is abort-and-report.
- Site rewiring: `docket-build` § *Gate execution posture* + `references/gate-execution.md` name
  the helper and the liveness-keyed rule; finalize inherits by citation; the full executable-site
  scope is derived by whole-repo grep at plan time. `runner-dispatch.sh` is a conscious exclusion
  (0277 churn; its own sentinel-only observe gap is a named residual with a follow-up stub minted
  at reconcile).
- Mutation-tested per the repo's own rule: `tests/test_gate_run.sh` kills the child mid-wait and
  asserts a prompt `died` with the log tail, plus an identity-guard fixture so a recycled pgid
  cannot read alive; new `runtime-budgets.tsv` row.

## Out of scope

- Making the suite itself faster. The 206s run is honest cost; change 0280 already tracks the
  `OVER BUDGET` shards.
- Reworking the finalize gate's semantics (what it validates, when it skips).
- The early-yield behavior — the agent returning "I'll wait for the suite to finish" instead of
  blocking — which is a separate foreground-dispatch defect and deserves its own change if it is
  not already covered.

## Open questions

Resolved at grooming (spec assumptions 1–12): helper script owning both launch and observe
(convention prose rejected); site scope derived by grep at plan time, with `runner-dispatch.sh` a
conscious, residual-named exclusion; death ⇒ group-kill then one bounded relaunch for idempotent
children only, abort-and-report otherwise and on second death; live-slow children stay under the
existing `GATE_OBSERVATION_BUDGET` fail-closed posture — no new knob.
