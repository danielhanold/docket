---
id: 234
slug: split-gate-execution-md-probe-evidence-should-not-sit-on-a-b
title: Split gate-execution.md: probe evidence should not sit on a blocking-read surface
status: proposed
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [223]
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

`skills/docket-build/references/gate-execution.md` (created by change 0223) is read **blocking
before every gate run** — `skills/docket-build/SKILL.md` § *Gate execution posture* ends with
"read it now (blocking) before starting the gate." It is 168 lines against a 175/1650 budget that
was already raised from 150/1350 during 0223's own build.

Most of that content is not instruction. The file quarantines two different things under one roof:

1. **Harness-specific instruction** — the six required capabilities, and the mitigation that
   satisfies them ("detach into a new session and redirect every stream to a durable location; the
   parent must not return until the child has finished detaching"). Roughly 15 lines. This belongs
   on a runtime-read surface.
2. **The evidence for the verdicts** — the § *Method* probe design, the one-variable-per-run
   ladder, the four launch durations (0s / 19s / 11s / 5s), the 180s stand-in gate, the inherited
   blocking claim that failed to reproduce on cursor, the permission-classifier denial that left
   the `claude` forked mode unmeasured, and four external version strings. This is a measurement
   report.

A build agent about to start a suite needs (1) and does not need (2). Two things make (2) actively
costly where it sits rather than merely redundant:

- **It is already duplicated.** `docs/results/2026-08-07-the-build-gate-contract-never-states-an-execution-posture-for-results.md`
  carries the same version scoping and the same re-probe caveats as human verification items. The
  evidence exists in both places; only one of them is loaded into an agent's context on every
  build.
- **It rots on an external schedule.** The file's own rule is that verdicts are version-scoped and
  must be re-probed when the version moves. Four third-party version strings sit in a file the
  build path reads unconditionally, and — as the results file states — nothing in the suite can
  detect that staleness.

The quarantine boundary 0223 drew was the right one for its stated purpose (keep product names out
of the capability contract) but it was drawn around *product-specificity* when the load-bearing
distinction for a blocking-read file is *instruction vs. evidence*.

## What changes

Split the file along that second axis:

- **Keep on the runtime-read surface**: the six required capabilities, the mitigation and its
  non-obvious precondition, the *Reading a verdict* rules, and a compact verdict table
  (harness → token → version → scope qualifier).
- **Move to the record**: § *Method* and the per-harness evidence narratives — to the results file,
  an ADR, or a non-blocking sibling reference, with a pointer from the kept surface.

`tests/test_gate_execution_posture.sh` asserts a verdict for every harness in
`HD_SHIPPED_HARNESSES`; that assertion must keep holding against the compact table, so the table is
the structure to design first.

## Out of scope

- Re-probing any harness verdict, or measuring the `claude` forked mode. Those are 0223's open
  verification items in its results file and stay there.
- Changing `docket-build` § *Gate execution posture* itself, or the `GATE_OBSERVATION_BUDGET`
  contract. The posture is correct; only where its supporting evidence lives is in question.
- Relaxing the blocking-read requirement. The kept surface is still read before the gate.

## Open questions

- Where does the evidence land — appended to 0223's results file (its natural home, but that file
  is archived and terminal), a new ADR (durable and indexed, but ADRs record decisions rather than
  measurements), or a sibling reference the skill body does *not* blocking-read?
- Does the size-budget row for the kept file get ratcheted back down as part of this change, or
  left with headroom? A ratchet is the only thing that prevents the evidence drifting back in.
