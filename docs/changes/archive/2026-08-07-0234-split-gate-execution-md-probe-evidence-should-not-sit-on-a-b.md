---
id: 234
slug: split-gate-execution-md-probe-evidence-should-not-sit-on-a-b
title: 'Split gate-execution.md: probe evidence should not sit on a blocking-read surface'
status: done
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: [231]
discovered_from: [223]
adrs: []
spec: docs/superpowers/specs/2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-design.md
plan: docs/superpowers/plans/2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-plan.md
results: docs/results/2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-results.md
trivial: false
auto_groomable: true
branch: feat/split-gate-execution-md-probe-evidence-should-not-sit-on-a-b
pr: https://github.com/danielhanold/docket/pull/169
blocked_by:
claimed_at: 
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-design.md) |
| Plan | [2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-plan.md) |
| Results | [2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b-results.md) |
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
  non-obvious precondition, the *Reading a verdict* rules, and one **compact `### <harness>`
  section** per shipped harness carrying its version, its verdict token, and any scope qualifier.
- **Move to a new non-blocking sibling**, `skills/docket-build/references/gate-execution-evidence.md`:
  § *Method*, the one-variable-per-run ladder, the measured launch durations, and the per-harness
  evidence narratives — with a pointer from the kept surface and a back-pointer to 0223's results
  file.
- **Ratchet** the kept file's size-budget row down to its new measured actual, and add a row for the
  new file. The ratchet is the enforcement; without it the evidence drifts back.

The stub's "compact verdict table" was rejected during grooming on guard structure:
`tests/test_gate_execution_posture.sh` requires `### <harness>` headings whose set equals
`HD_SHIPPED_HARNESSES`, a verdict line inside each section slice, and per-harness *prose* about the
measured mode. A markdown table satisfies none of those, and rewriting a guard file that is
mutation-tested clause by clause is a larger, riskier diff than the one this change is for. The
short-section shape is that compact row expressed in the structure the guards already enforce.

Expected result: the kept file drops from 168 lines to roughly 93.

## Out of scope

- Re-probing any harness verdict, or measuring the `claude` forked mode. Those are 0223's open
  verification items in its results file and stay there.
- Changing `docket-build` § *Gate execution posture* itself, or the `GATE_OBSERVATION_BUDGET`
  contract. The posture is correct; only where its supporting evidence lives is in question.
- Relaxing the blocking-read requirement. The kept surface is still read before the gate.
- Editing `docs/results/` in any way, or de-duplicating the version scoping it shares with this
  file. The evidence file points at it instead.
- Rewriting any existing assert in `tests/test_gate_execution_posture.sh`. New guards are additive.

## Open questions

Both resolved during autonomous grooming; the reasoning and the rejected alternatives are in the
spec's `## Assumptions` block.

- **Where the evidence lands** → a new non-blocking sibling reference. The results file is a
  published close-out record of a completed change, and an ADR's lifecycle is wrong for a
  measurement that must be rewritten whenever a harness version moves.
- **Budget ratchet** → yes, down to the new measured actual, with an in-diff justification.

## Reconcile log

### 2026-08-07 — implementer reconcile

Verified against `origin/main` @ `035e8eba`. Everything the spec asserts about current reality still
holds, so the design carries forward unchanged:

- `skills/docket-build/references/gate-execution.md` is **168 lines / 1612 words**, exactly the
  figure the spec measured; its `BUDGETS` row in `tests/test_skill_size_budgets.sh:632` is still
  `175 1650`, and `references/` holds only `gate-execution.md` + `task-routing.md` — no evidence
  sibling exists yet.
- The guard shapes the spec plans around are unchanged in `tests/test_gate_execution_posture.sh`
  (387 lines): the `>= 40` non-blank floor, the (10b) `verdict[^.]{0,80}only[^.]{0,80}measur`
  window, and the (10c) per-harness `forked|dispatched` slice with its `[^-]interactive` and
  co-occurrence asserts. The line citations in the spec are accurate.
- The three § *Method* citations on the kept surface (`:34`, `:43`, `:49-50`) and the fourth inside
  `### cursor` (`:136`) are all present as described.

**A8 rechecked — the 0231 coupling is live, not hypothetical.** Change 0231 went `in-progress` at
`2026-08-07T15:35:48Z`, roughly one minute after this claim, and its branch
`feat/a-presumed-dead-build-worker-can-wake-and-race-its-own-repla` exists locally. Both branches
are cut from the same `origin/main` tip, so if 0231 does add or raise a `BUDGETS` row it will land
an adjacent row plus a justification-comment entry in the same block this change edits. Handling is
unchanged from A8: keep this change's budgets edit minimal and scoped to the two rows it owns
(`gate-execution.md` ratcheted down, `gate-execution-evidence.md` added), append the justification
entry at the end of the comment block rather than inserting mid-block, and let whichever PR merges
second rebase. No scope change follows from it.

Scope, out-of-scope, and all eight assumptions stand as written. No obsolescence, no invalidation.

