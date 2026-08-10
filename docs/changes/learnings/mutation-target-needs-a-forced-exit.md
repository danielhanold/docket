---
slug: mutation-target-needs-a-forced-exit
hook: "When the code under test is a loop whose only exit is the thing you are guarding, the mutation hangs instead of reddening — give the stub an independent hard stop that resolves to a comparable value."
topics: [testing, guards, mutation]
changes: [286]
created: 2026-08-10
updated: 2026-08-10
promotion_state: candidate
promoted_to:
---

## Apply
Mutation testing assumes the mutated run **terminates** and reports. That assumption breaks whenever
the guarded thing *is* the termination condition — a poll loop's deadline check, a retry cap, a
budget comparison. Delete it and the run does not fail, it spins. A hanging job reads as a slow
suite, not a broken guard, and a runner that reports wall-clock advisorily (docket's own
`scripts/run-tests.sh` prints `OVER BUDGET` but never kills a job) will never convert it into a
finding. The usual outcome is that the mutation test gets abandoned or mis-scored rather than
corrected — the same failure shape as
[[stacked-gap-regex-hangs-instead-of-failing]], reached from the opposite direction.

**Build the independent stop into the stub, not into the code under test.** The stub that feeds the
loop counts its own invocations and, past a cap well above any legitimate run, hard-stops by
emitting a distinguishable sentinel value. Two properties make it work:

1. **Independent of the guarded condition** — otherwise removing the guard removes the stop too.
2. **Resolves to a comparable outcome, not an error** — the sentinel must flow through the code's
   own fail-closed arm so the runaway ends as a *comparable assertion value* (`unavailable|201`)
   that the assert can name. A stop that aborts the shell gives the assert nothing to say.

Then mutation-test the stop itself: with the guard deleted, the test must go **red with that
sentinel in its diagnostic**, quickly.

## War story
- 2026-08-10 (#286, PR #192) — `run_loop` in `tests/test_gate_run.sh` drove the canonical
  `gate-run --observe` poll loop against a stubbed helper. Its only exit was the fence's own deadline
  check, so the mutation that deleted that check hot-spun rather than failing. The stub now hard-stops
  past 200 observations and emits `state=LOOPCAP`, which the fence's fail-closed `*)` arm disposes as
  `unavailable` — the runaway resolves to `unavailable|201` and the assert reddens instantly. A guard
  whose failure mode is a hang is not a guard.
