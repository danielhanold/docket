---
id: 271
slug: runner-delegation-has-no-execution-posture-for-a-child-that
title: 'Runner delegation has no execution posture for a child that outlives its foreground call'
status: proposed
priority: medium
type: fix
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [258]
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

`runner-dispatch.sh` calls the runner adapter **in the foreground** and returns its verbatim exit
code (script header; change 0237 replaced the original `exec` to regain that seam). On the claude
harness the call is made by a generated shim wrapper, so the whole delegated child is bounded by the
harness's foreground call — 600000 ms, the maximum the Bash tool allows. Any delegated task that
outlasts that window is killed.

Observed 2026-08-08 on the `docket-implement-next 258` run, after change 0269's fix made opencode
delegation actually work: `docket-build` routed plan Task 1 to `docket-build-standard`, the wrapper
made its single facade call, and the opencode child ran real work inside the feature worktree — a
green baseline plus four completed mutation proofs. Then `runner-dispatch` exited **143**, killed at
the ceiling, before the task's own verification and commit. The wrapper contract forbids a silent
retry and forbids an inline fallback, so the worker returned `BLOCKED` and the run halted with the
work stranded uncommitted in the worktree. The task was not pathological: one run of
`tests/test_docket_config.sh` costs ~60 s and the task serializes seven of them.

This is the same defect docket already diagnosed and fixed one layer up. Change 0223 established a
**gate execution posture** for `docket-build`'s full-suite gate — the run must survive the
initiating call's teardown, redirect every stream to a durable location, record an unambiguous
terminal result, be established from that artifact rather than from any caller-visible completion
signal, observe within a finite budget (`gate_observation_budget`), and fail closed. Six required
capabilities, quarantined in `skills/docket-build/references/gate-execution.md` with per-harness
verdicts. Change 0249 propagated the posture to the `docket-build-task` workers.

Both of those bound a **test command**. Neither binds the **delegation boundary**, which has the
identical shape — a long-lived child observed by its owner across a foreground boundary — and is
the one place the mitigation cannot be re-derived per-run, because the shim wrapper is generated and
makes exactly one call.

The crash-detection half is already built and should be reused rather than reinvented. The facade's
**run gate** (change 0237) does not trust the child's exit code: it re-syncs metadata from fresh
origin, diffs the in-progress claim set across the hand-off, and reads a disposition from durable
state via `verify-run.sh` — `run-complete` / `run-halted` / `run-incomplete <unmet…>` /
`run-unclaimed`. That is exactly capability 5's four-state distinction, already implemented against
git state instead of against a process. Today it is gated to `--agent implement-next` only
(`GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1`).

## What changes

Give the runner-delegation boundary an execution posture, and extend the existing state-based
disposition reader to serve as its completion detector.

- State the posture for `runner-dispatch` by **capability**, citing
  `skills/docket-build/references/gate-execution.md` as the single source rather than restating the
  six capabilities. The delegation call must not assume the child fits inside one foreground call.
- Detach the adapter into its own session with every stream redirected to a durable per-dispatch
  location, and observe it within a finite budget — the same one mitigation that satisfied all six
  capabilities on all four harnesses. The per-harness verdicts in that reference were measured for a
  *gate* launch; re-probe them for an *adapter* launch rather than inheriting them.
- Extend the run gate beyond `--agent implement-next` so a `build-*` delegation also gets a durable
  disposition. This needs a `build-*` postcondition — a build task leaves a commit on the feature
  branch, not a PR — so `verify-run.sh` grows a second verdict family rather than having the
  implement-next conjuncts (`status` / `pr` / `branch`) stretched to fit.
- Reuse the observation budget knob if it fits (`gate_observation_budget`), or state why a
  delegation needs its own.

## Out of scope

- Reducing suite runtime, and any edit to change 0258's plan. Narrowing 0258's mutation proofs to
  run only the relevant sections is a worthwhile plan edit on its own merits, but it treats the
  symptom; this stub is the general fix and 0258 must not be blocked on it.
- Change 0269's shim model/effort pin. Disjoint defect on the same path, already in progress.
- ADR-0024's never-yield rule for dispatched subagents. As change 0223 established, observing an
  external process from its owner is a different thing; this change must not be written as touching
  that rule.

## Open questions

- **`verify-run.sh` has no time floor**, and its header says that is sound *only* because of where
  it is called — "at a seam where the child process has already returned, so 'stopped' and 'still
  working' are not ambiguous." Detaching the adapter destroys that precondition: the facade would
  observe while the child is still alive. Does the reader grow a floor (which its own header calls
  out as the thing `board-checks.sh` needs and it deliberately avoids), or does the durable result
  artifact carry the terminal sentinel so liveness is never inferred from state at all? The second
  is more consistent with capability 3, and probably the answer.
- Signal handling. The facade installs no traps by deliberate decision, and the header already
  records that a pid-directed TERM kills the facade and **orphans** the still-running adapter — a
  path it names as a design decision owed to the run gate's error posture. Detaching makes that
  orphan the normal case rather than an edge; the posture must say what a re-observation finds and
  who reaps it.
- Whether a delegated child that outlives its dispatch should be resumable, or whether the honest
  contract is that the caller re-observes and the child is never re-entered.
