---
id: 359
slug: run-gate-gives-up-too-soon
title: 'the run gate gives up too soon'
status: proposed
priority: high
type: fix
created: 2026-08-27
updated: 2026-08-27
depends_on: []
stacked_on:
related: [237, 334]
discovered_from: [333]
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

`gate-verdict` can escalate a healthy, still-running implement-next run to a terminal
`gate-stop` — forbidding any re-dispatch — while the build worker is legitimately still
working and simply has not committed its effects yet.

The gate verdict is a **durable-postcondition snapshot with no liveness or in-progress-activity
awareness**. `RunVerify` (`internal/app/run_verify.go`), the sole verdict authority, derives every
verdict (`run-complete` / `run-incomplete` / `run-halted` / `run-waiting` / `run-unclaimed`) purely
from committed state: git head, remote head, recorded PR, evidence receipts, `## Run halted`
markers, and fingerprinted gate-drive receipts on disk. There is deliberately **no** PID / process
/ signal check anywhere — `run_verify.go` even states it: `run-waiting` means "a safe local
continuation *exists*," not "a process might still be running." So the instant it is sampled during
a long **inline** measurement or build phase that has not yet committed, the run reads
`run-incomplete / not-implemented`, and once the single retry permit is spent
(`rungate_verdict.go`, `ConsumeGateRetry`), the next `run-incomplete` becomes a terminal
`gate-stop`. The verdict is correct *per its own contract* but **premature relative to a genuinely
progressing run**, and it fires exactly when the run is doing legitimate slow work (which is often
the whole point of the change being built).

This surfaced live implementing change 0333. Observed:

- Run 1 stopped early (parent yielded awaiting the plan-writer) → `gate-retry-once`.
- Run 2 (the retry) advanced further — the plan committed — but was sampled while the build worker
  was mid-flight running `go test -race ./internal/app` **inline and blocking** (a ~227s default /
  ~264s race package, which is precisely the >600s-ceiling problem 0333 exists to fix) → the durable
  snapshot read `run-incomplete / not-implemented`, and with the retry already spent →
  **`gate-stop`**.
- Process trace at that moment confirmed the worker was alive, un-orphaned, and progressing;
  moments later Task 1 committed (`36239f39`) and the log showed the measurement completing normally.

Net effect: the contract shut the door on a run that was fine, and — because `gate-stop` forbids
re-dispatch — a human is now required to recover a run that needed no intervention. The retry budget
is also spent on non-signal (a slow-but-healthy phase), so it is unavailable for a genuinely failed
retry later.

## What changes

Design a fix so the gate does not terminally give up on a run that is merely slow rather than
stuck. Candidate directions to weigh during brainstorm (not yet decided):

- Give the verdict a notion of **in-progress liveness / recent activity** so an actively-progressing
  run is distinguished from a stalled one — e.g. a `run-working` (or reuse of `run-waiting`)
  disposition when a fresh claim lease / activity receipt is advancing, mapped to a non-terminal
  gate decision rather than `gate-stop`.
- Reconsider **retry accounting** so a slow-but-healthy phase does not consume the single retry
  permit — spend retries on genuine failure signal, not on "hasn't committed yet."
- Reconsider whether `gate-stop` on `run-incomplete` should be terminal at all when the run is still
  live, vs. a "keep waiting / re-poll" decision.

## Out of scope

- The orchestrator **yield-wedge** behaviour (a parent backgrounding a child and yielding to await a
  notification) is a *separate* known bug and is not what this change fixes — though it interacts
  with this one, since a yielding parent is what lets the gate be sampled mid-phase.
- Broad redesign of `RunVerify`'s durable-postcondition model; the fix should be about not giving up
  too soon, not about replacing the postcondition authority.

## Open questions

- What is the cheapest reliable **liveness/activity signal** available to the gate without turning it
  into a process probe (claim-lease freshness? a gate-drive activity receipt timestamp?)?
- Should the answer be a new verdict/decision token, or a change to how `run-incomplete` +
  spent-retry maps to `gate-stop`?
- How should a genuinely stuck run still converge to `gate-stop` in bounded time under the new
  behaviour (avoid trading premature-stop for never-stop)?

## Reconcile log
