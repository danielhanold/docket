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

### Recurrence evidence (this session) — the gate never once told the truth about a healthy run

The 0333 build was driven through **four** dispatches. In every one, the parent's prose and the
gate verdict read as "waiting / incomplete / stop," while the fan-out build workers were in fact
healthy and committed their tasks. The gate systematically **under-reported a run that was
succeeding task-by-task** — it never emitted a "this run is progressing, keep going" signal, only
`retry-once`, `stop`, or (on resume) `no-attributable-claim`.

| Dispatch | Parent prose read as | Gate verdict | What actually landed in git |
|---|---|---|---|
| 1 (initial) | waiting on plan-writer | `gate-retry-once` | plan-writer healthy; plan committed `606fca6c` |
| 2 (the retry) | waiting on Task 1 worker | `gate-stop` (retry spent, terminal) | Task 1 worker healthy; committed `36239f39` seconds later |
| 3 (human resume A) | "blocking on Task 5 worker" | attributed `gate-done no-attributable-claim`; observe `run-incomplete` | Tasks 2–5 all committed: `df74c284`, `b1bbdfe2`, `b5d9ffd8`, `48066711` |
| 4 (human resume B) | in progress | — | continuing Task 6→8 |

The point is not any single premature stop — it is that across a multi-hour, five-task build that
was **working the whole time**, the gate produced zero true-positive "healthy" readings and one
false-terminal `gate-stop`. A point-in-time durable snapshot taken against a worker that commits
seconds later is structurally a coin-flip: dispatch 2's `gate-stop` and dispatch 3's Task-1-already-
done state were the same run, read moments apart, giving opposite pictures.

### Second defect: the gate has no attributed verdict for a *resumed* in-progress run

`gate-stop` forbids re-dispatch and says "a human is needed." But when the human does exactly that —
arms a fresh gate and re-dispatches to resume the already-`in-progress` change — the attributed
`gate-verdict <key>` returns **`gate-done no-attributable-claim`**. Attribution binds only a claim
whose `claimed_at` is **at or after the dispatch epoch** (`rungate_verdict.go`
`attributeGateClaim`), and a resumed change was claimed long before, so there is *nothing to
attribute*. The gate's own prescribed recovery path therefore lands in a blind spot the gate cannot
verify: the CLAUDE.md bracket (`gate-before` → dispatch → `gate-verdict <key>`) is unusable for a
resume, and you must fall back to observe mode (`gate-verdict --unattributed <id>`), which has no
retry accounting and cannot authorize anything. So the gate both (a) forces a human handoff on a
healthy run and (b) cannot then track the handoff it forced. These two defects compound: the
premature `gate-stop` is what pushes you into the un-attributable resume path.

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
- Give the gate an **attributed verdict path for a resumed in-progress change** (e.g. accept the
  change id as an explicit attribution target when a fresh claim cannot be bound), so the human
  recovery `gate-stop` mandates is itself gate-trackable with retry accounting — instead of
  degrading to observe mode that can authorize nothing.

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
- Should the resume-attribution fix live here or be split into its own change? It is a distinct code
  path (`attributeGateClaim`) from the liveness/retry question, though both surfaced from the same
  0333 run and share the "gate can't track a healthy run" root.
- Is the durable-snapshot-vs-just-committed race (a verdict sampled seconds before a worker commits)
  better fixed by a short re-poll/grace window before a terminal verdict, or does the liveness signal
  subsume it?

## Reconcile log
