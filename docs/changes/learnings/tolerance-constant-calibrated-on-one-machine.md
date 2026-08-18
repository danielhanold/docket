---
slug: tolerance-constant-calibrated-on-one-machine
hook: "A tolerance constant measured on one machine's contention profile is wrong in both directions elsewhere — too tight it flakes, too loose enforcement goes vacuous; record the measurement, not just the number."
topics: [thresholds, performance, portability]
changes: [227, 312, 325]
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
