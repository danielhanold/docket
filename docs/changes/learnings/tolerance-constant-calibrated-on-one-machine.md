---
slug: tolerance-constant-calibrated-on-one-machine
hook: "A tolerance constant measured on one machine's contention profile is wrong in both directions elsewhere — too tight it flakes, too loose enforcement goes vacuous; record the measurement, not just the number."
topics: [thresholds, performance, portability]
changes: [227, 312, 325, 328]
created: 2026-08-07
updated: 2026-08-18
promotion_state: retained
promoted_to:
---

## Apply
When a guard compares a **measured** quantity (wall clock, memory, throughput) against a budget, the
slack factor you pick is calibrated against the machine you measured on — its core count, its
scheduler, how much the parallel run contends with itself. That number does not travel. Both failure
modes are real and they are not symmetric in how they present:

- **Too tight** — the guard reddens on healthy work. Loud, annoying, and self-correcting, because
  someone investigates.
- **Too loose** — the guard passes everything. Silent, and indistinguishable from a guard that is
  working. This is the one that kills the check without anyone noticing, especially where most rows
  sit at a floor value well under the ceiling: the slack swallows the whole distance and enforcement
  becomes decoration.

So: **record the measurement next to the constant**, not just the value. What hardware, what
contention factor was observed, and which direction the error goes if it is wrong. A future reader
adjusting it needs to know whether they are re-tuning for a faster machine or papering over a real
regression — the number alone cannot tell them, and the cheapest path back to green is always to
loosen it ([[guard-remedy-must-not-teach-the-evasion]]).

Better than tuning is removing the machine dependence: measure the quantity in a way that does not
vary with contention, so the threshold can be tightened back into a real gate
([[optimization-needs-a-measured-oracle]]).

## War story
- 2026-08-07 (#227, PR #165 — merged) — The parallel runner's per-file runtime budget compares
  measured wall clock against a table row, with a slack factor for contention. It shipped at **3/2**
  and rejected **11 healthy files**: under an 8-worker parallel run, measured contention inflation
  reached **2.22x**, so a file's parallel wall clock routinely exceeded 1.5x its serial budget. The
  constant was raised to **5/2**.

  That number is hardware-dependent in both directions, and the loose direction matters more here
  than usual: **69 of the table's rows sit at the 10s floor**, so a 2.5x slack means a file can
  triple in cost and stay green. The change shipped the constant with its calibration recorded in a
  comment at the definition site rather than as a bare literal, and opened **#0229** to make the
  check contention-independent — which is the actual fix, since only then can the budget breach go
  back to being a hard gate ([[exit-code-encodes-a-non-failure]]).
- 2026-08-17 (#312, PR #214) — The same constant read the other way round, from the *runner's* side.
  At the runner's default parallelism this machine saturated and about **24 files** printed
  `OVER BUDGET`, blowing their rows by roughly **10x**; the identical suite at `-j 3` ran 121/121
  clean and inside budget, and the one genuinely timing-sensitive test (`test_gate_run_stop`, a
  `--stop` barrier test of code this change never touched) was green in isolation and in the final
  run. Nothing about the files under test changed between those runs — only how hard the box was
  contending with itself. So an `OVER BUDGET` line is only evidence about a file once the
  **parallelism it was measured at** is recorded next to it; read without that, a saturated run
  invites either a pointless optimisation of an innocent file or a budget raise that makes the row
  vacuous. Record the `-j` level with the measurement, and treat a whole-suite cliff (dozens of rows
  over at once, all by a similar factor) as a statement about the machine, not the suite.
- 2026-08-18 (#325, PR #218 — merged) — The `test_gate_run_stop` flake #0312 flagged was root-caused
  and fixed. Its **8 barrier-rendezvous waits** used `wait_for_file`'s **10s** default — an
  isolation-calibrated deadline: under full-parallel-suite CPU contention the backgrounded
  `gate_run --stop` needs longer than 10s just to be *scheduled* to its injected rendezvous, so the
  `.reached` wait times out on a perfectly healthy stop (the too-tight, self-correcting direction).
  Fix: a `BARRIER_TICKS` (60s) ceiling applied at all 8 sites. This does not risk the loose-direction
  vacuity above, because `wait_for_file` returns the *instant* the marker appears — the larger ceiling
  only ever costs wall-time on a genuine hang (a real regression, which should fail), never on a
  slow-to-schedule stop. Verified with **8 concurrent copies under self-contention, all green**. The
  same finalize run surfaced a **second** flake in the same gate-run/supervisor family —
  `TestRecoverMarksCleanlyAbandonedOwnedRun` (`internal/process`, passes 5/5 in isolation), captured
  as **#0328** — confirming the pattern is family-wide: a supervisor test whose timing assumption
  holds in isolation breaks under load. Sizing a *test barrier* for load rather than isolation is the
  same record-and-widen-for-contention move as the runtime-budget constants above.
- 2026-08-18 (#328, PR #219 — merged) — **The predicted sibling closed, and it did not behave like
  0325.** The entry above named #0328 as the family-wide next case; this is its outcome, and it adds
  a limit to the family's method. The same widen-for-contention move was applied — the two `waitFor`
  deadlines went 30s → 60s, safe in the loose direction for the same reason as 0325's barrier
  (`waitFor` returns the instant its predicate holds, so the wider ceiling costs wall-time only on a
  genuine hang). But unlike 0325, **the flake would not reproduce**: 8 copies × `-count=5` of the
  target test, then 16 copies × `-count=2` of the whole `internal/process` package (~50s per copy,
  32 executions under real self-contention) — all green. 0325's 8-copy technique is therefore not a
  general oracle for this family: it reproduced a *barrier-wait* flake, where contention delays a
  scheduled rendezvous, and failed to reproduce a *state-race* flake, where the losing interleaving
  needs a specific ordering rather than merely a slow machine.

  So the fix landed as **diagnostics rather than a demonstrated repair**, and was labelled that way
  in the results file and PR body: a precondition helper asserting every durable verdict absent
  before `Recover` runs, plus a bounded setup retry, which converts a future mystery `Marked:0` into
  a named setup failure. Mutation-tested both directions (forced permanently-unmet → retries then
  fatals at the cap; forced unmet once → re-drives and passes). What this adds to the family: when
  the stress technique that worked on the previous member comes back green, that is evidence about
  *the technique's fit*, not about the flake's absence — widen the diagnostics so the next
  occurrence carries its own cause, and say plainly which of the two you shipped.

  Runtime footnote, per [[budget-headroom-is-spent-before-it-is-breached]]: the change cost nothing
  measurable — 16-copy stress ran **47.9–50.8s → 45.8–48.3s** before vs after, with zero retry
  events on healthy runs. `test_go_toolchain` measured **153s against its 55s ceiling** versus 150s
  at 0325's gate, i.e. unchanged; ten files were over at once at roughly 3x, which is the
  whole-suite cliff this finding's #312 entry says to read as a statement about the machine.
