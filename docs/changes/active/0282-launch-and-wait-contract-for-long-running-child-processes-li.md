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
related: []
discovered_from: [276]
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

Establish a single **launch-and-wait contract** for any long-running child process a docket skill
shells out to (the suite is the motivating case; the rebase/build/publish long-poll sites are the
same shape), and route the existing launch sites through it. The contract must fix both halves:

- **Detached launch** — the child survives the tool call that started it, writing to a stable log.
- **Liveness-keyed wait** — the wait predicate is anchored on the process being alive
  (`kill -0`, or equivalent), never solely on a success marker that only a live process can emit.
  A dead child ends the wait **promptly**, with the child's own last output as the diagnostic.
- **Death is a distinct outcome from failure.** "The suite ran and went red" and "the suite never
  finished" must not collapse into one report line — 0276's run had no vocabulary for the latter.

Where this contract lives is the design question: a shared helper script (the ADR-0012
script-vs-model boundary argues for it — one CAS-correct implementation instead of N hand-rolled
poll loops, exactly the argument that produced `mint-stub.sh`), versus normative prose in
`docket-convention` that each skill re-implements. Prefer the script if the wait shape is uniform
enough to factor.

Whichever shape wins, the guard must be mutation-tested per the repo's own rule: kill the child
mid-wait and watch the wait return promptly with a death report. A wait loop that cannot be shown
to notice a dead child is decoration.

## Out of scope

- Making the suite itself faster. The 206s run is honest cost; change 0280 already tracks the
  `OVER BUDGET` shards.
- Reworking the finalize gate's semantics (what it validates, when it skips).
- The early-yield behavior — the agent returning "I'll wait for the suite to finish" instead of
  blocking — which is a separate foreground-dispatch defect and deserves its own change if it is
  not already covered.

## Open questions

- Shared helper script vs. normative convention prose — which, and if a script, does it own the
  launch too or only the wait?
- Which existing sites are in scope? A whole-repo sweep for launch/poll shapes is required
  (per the repo's own rule against hand-listing sites), sorted into prose vs. executable.
- Should a death mid-wait be `abort-and-report`, or is one bounded relaunch justified — given
  0276's relaunch did succeed on the first retry?
- Is there a bound past which a *live* child should still be abandoned, and does that interact
  with `GATE_OBSERVATION_BUDGET`?
