---
id: 282
slug: launch-and-wait-contract-for-long-running-child-processes-li
title: 'Launch-and-wait contract for long-running child processes — liveness-keyed, not marker-keyed'
status: implemented
priority: critical
type: fix
created: 2026-08-09
updated: 2026-08-10
depends_on: []
related: [251, 260, 273, 275, 277]
discovered_from: [276]
adrs: [81]
spec: docs/superpowers/specs/2026-08-09-launch-and-wait-contract-for-long-running-child-processes-li-design.md
plan: docs/superpowers/plans/2026-08-10-launch-and-wait-contract-for-long-running-child-processes.md
results: docs/results/2026-08-10-launch-and-wait-contract-for-long-running-child-processes-li-results.md
trivial: false
auto_groomable: true
branch: feat/launch-and-wait-contract-for-long-running-child-processes-li
claimed_at: 2026-08-10T05:51:37Z
pr: https://github.com/danielhanold/docket/pull/191
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-launch-and-wait-contract-for-long-running-child-processes-li-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-launch-and-wait-contract-for-long-running-child-processes-li-design.md) |
| Plan | [2026-08-10-launch-and-wait-contract-for-long-running-child-processes.md](https://github.com/danielhanold/docket/blob/feat/launch-and-wait-contract-for-long-running-child-processes-li/docs/superpowers/plans/2026-08-10-launch-and-wait-contract-for-long-running-child-processes.md) |
| Results | [2026-08-10-launch-and-wait-contract-for-long-running-child-processes-li-results.md](https://github.com/danielhanold/docket/blob/feat/launch-and-wait-contract-for-long-running-child-processes-li/docs/results/2026-08-10-launch-and-wait-contract-for-long-running-child-processes-li-results.md) |
| PR | [#191](https://github.com/danielhanold/docket/pull/191) |
| ADRs | [ADR-0081](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0081-gate-run-contract-narrowed-per-platform-process-group-where-no-session-primitive-exists.md) |
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

- A new shared helper, **`scripts/gate-run.sh`** (+ `gate-run.md` contract), with three verbs:
  `--launch` (detached new-session start whose establishment is acknowledged before a handle is
  returned, `--root`-parented per-run dir at `umask 077` with every stream durable, pid/pgid plus an
  opaque identity token recorded, a separate atomic terminal-record file written on child exit),
  `--observe` (one short-lived observation; six states — `running` / `passed` / `failed` /
  `died` / `stopped` / `unavailable` — with liveness anchored on the identity-checked process
  group, never solely on the record, and only `running` retryable), and `--stop` (identity-checked
  before signaling, `TERM` →
  bounded grace → `KILL` on the recorded group, idempotent, recording a `stopped` marker and no
  terminal record). A dead child is detected on the next observation, not at a wait-loop bound.
  **stdout carries only the machine-readable report line; every diagnostic goes to stderr.**
- The terminal record encodes **termination kind** (`kind=exit code=<n>` / `kind=signal`), not a
  bare integer: a child killed by a signal never finished, so it is `died`, not `failed` — the
  distinction must not depend on whether the supervisor happened to outlive the child.
- **The terminal record outranks liveness and outranks a stop, everywhere.** `--observe` re-reads it
  after a dead liveness probe; `--stop` re-reads it immediately before signaling and again after the
  kill, and writes its marker only once termination is verified. Both are transplanted from
  `runner-dispatch`'s give-up path, which already solved this; without them a run that **passed**
  reports `died`, or a run that had already succeeded gets killed.
- **Death is a distinct outcome from failure.** `died` (never finished) never collapses into
  `failed` (ran and went red) and never mints repair work.
- Call-site posture on `died`: `--stop` the run, then **one** bounded relaunch **gated on `--stop`'s
  report** — `stopped` relaunches, `already-terminal` re-observes first (a record may have appeared),
  and `unavailable` aborts without relaunching. That last leg is load-bearing: `died` includes the
  leader-dead-orphans-alive state, and there the identity guard cannot prove the surviving group is
  ours, so it refuses to kill it — relaunching would race a live suite. Scoped to idempotent children
  (the suite gate); non-idempotent sites keep their existing failure postures; second death is
  abort-and-report. A caller that abandons a still-`running` child — budget exhaustion, halt, abort —
  calls `--stop` before it reports, so no suite outlives the run the human is about to inspect.
- Site rewiring: `docket-build` § *Gate execution posture* + `references/gate-execution.md` name
  the helper and the liveness-keyed rule; finalize inherits by citation; the full executable-site
  scope is derived by whole-repo grep at plan time. `runner-dispatch.sh` is a conscious exclusion
  (0277 churn; its own sentinel-only observe gap is a named residual with a follow-up stub minted
  at reconcile).
- Mutation-tested per the repo's own rule: `tests/test_gate_run.sh` kills the child mid-wait and
  asserts a prompt `died` with the log tail, plus an identity-guard fixture so a recycled pgid
  cannot read alive — and, on the signal path, cannot be signaled; `--stop`'s `KILL` escalation,
  idempotence, and no-record guarantee each carry a reddening mutation. Three **deterministic
  interleaving** fixtures (not sleeps) cover the races: an observer held between its record read and
  its liveness probe must report `passed`; a `--stop` held before its `TERM` must report
  `already-terminal` and signal nothing; a `--stop` killed before its marker write must leave no
  marker. New `runtime-budgets.tsv` row.

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
conscious, residual-named exclusion; death ⇒ `--stop` then one bounded relaunch for idempotent
children only, abort-and-report otherwise and on second death; live-slow children stay under the
existing `GATE_OBSERVATION_BUDGET` fail-closed posture — no new knob.

Reopened and resolved at human spec review (2026-08-09), two rounds. Round 1: the helper ships a
`--stop` verb (spec assumption 13), reversing assumption 5's parked residual that an abandoned live
child outlives a halted run. Round 2 reviewed that draft and corrected four soundness defects in it
— the leader-dead relaunch guarantee was false (22), `failed` was still marker-keyed rather than
termination-keyed (16), and both `--observe` and `--stop` carried completion-versus-liveness TOCTOU
races (19, 21). Post-groom design; assumptions 13–22 did not pass the auto-groom critic.

**One open implementation question, escalated rather than assumed** (spec assumption 15): capability
1 requires a new *session*, `runner-dispatch`'s `set -m` delivers only an own process group, and
`setsid` is absent on macOS with no `perl` dependency taken. The plan must establish which primitive
delivers a genuine new session on the supported platforms; finding none without a new dependency is
a design finding to escalate, not a gap to paper over.

## Reconcile log

### 2026-08-10 — reconcile at claim (docket-implement-next)

Re-read the change and its spec against `related: [251, 260, 273, 275, 277]`, the changes archived
since the spec was written, the ADRs accepted since, and current code. **Scope holds unchanged; no
work has been done elsewhere.** Four findings folded in.

1. **Nothing is built.** `scripts/gate-run.sh`, `scripts/gate-run.md`, and `tests/test_gate_run.sh`
   are all absent; `tests/runtime-budgets.tsv` exists and carries no `gate_run` row. The whole
   `## What changes` scope is still net-new.

2. **ADR-0080 was accepted on 2026-08-09 (change 0271) and is now the nearest prior art.** It
   records the *delegation*-side launch-then-observe posture and ships
   `skills/docket-build/references/delegation-execution.md`. Two consequences for this change, both
   strengthening rather than altering it:
   - ADR-0080 **measured** the `set -m` technique and states its limit in exactly the terms spec
     assumption 15 needs — *"the child gets its own process GROUP, not a new SESSION — it remains
     in the launcher's session, so session-scoped teardown was not tested and is not claimed."*
     Assumption 15's premise is therefore no longer an inference from `runner-dispatch.md` prose;
     it is a recorded, measured ADR consequence, and the probe ladder should cite it.
   - `delegation-execution.md` is a **new file in the same design family** that post-dates the
     spec. It is not an enumerated site — the plan-time whole-repo grep owns site derivation per
     the never-hand-list rule — but it is named here so the grep's output is read against a known
     candidate rather than reviewed blind.

3. **The platform question is answerable, and the ladder's rungs are as the spec predicted.** On
   this machine (darwin 25.6.0): `setsid` is **absent**, `/usr/bin/script` is **present**. The
   probe ladder in assumption 15 therefore reaches its `script(1)` rung on the primary supported
   platform, and the round-4 widening — a rung passes only on the *full* capability set (unmerged
   durable streams, no injected framing, no isatty flip, no new `SIGHUP` path), not the session bit
   alone — is the load-bearing part of that probe, not a caveat on it.

4. **The `runner-dispatch.sh` conscious exclusion is still correct, and its named gap is still
   real.** Verified in current source: the only `kill -0` in `runner-dispatch.sh` sits in the
   give-up path, not in `--observe`, and `runner-dispatch.md` still states *"The sentinel is the
   \*only\* source of liveness."* Change 0277, which is reworking that surface, remains `proposed`
   and unbuilt, so the churn argument for excluding it stands. The gap is carried as a named
   residual in the spec and will be repeated in this change's results file; a follow-up stub was
   minted at this pass per the spec's instruction.

**Couplings re-checked.** 251, 260, 275, 277 are all `proposed`/build-ready and 273 waits on 251 —
none has landed, so no rebase collision exists yet and the `tests/runtime-budgets.tsv` contention
noted in spec assumption 8 remains an ordinary whichever-lands-second rebase. No `depends_on` is
warranted.
