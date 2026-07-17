---
id: 88
slug: implement-next-loop-continuation
title: Loop continuation — implement-next chains into the next ready change instead of stopping
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [8, 87]
adrs: [1]
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md) |
<!-- docket:artifacts:end -->

## Why

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads' loop
continuation primitive is `bd close --claim-next`: closing an issue atomically chains into
claiming the next ready one, so an agent drains a queue without an orchestrator re-dispatching it
per item. Its claim is atomic ("when multiple agents pull from the same ready queue, the first
claim wins") — the same guarantee docket's compare-and-swap claim push already provides.

docket has the claim safety but not the continuation. `docket-implement-next` is documented as
"runs solo per change": after opening a PR it stops, and on a *lost claim race* it aborts entirely
rather than selecting the next eligible change. A backlog of N independent build-ready changes
needs N separate invocations, and a race loser wastes its whole run. This is exactly the loop
primitive an autonomous serial drain (see #0087's headless finalize driver, and #0008's parallel
fan-out) is missing.

## What changes

- **Loser-picks-next**: when the post-race re-read shows another agent claimed the selected
  change, re-run selection over the remaining build-ready set and claim the next one, instead of
  aborting. (#0008 proposes the same semantics as part of parallel drain; brainstorm decides
  whether this change subsumes that piece of #0008 or lands independently first.)
- **Continue-after-PR (opt-in)**: an explicit continuation mode — after the human merge gate is
  reached (PR open, `implemented`), loop back to selection and claim the next ready change, with
  a bounded stop condition (no eligible work, N changes drained, or a budget/iteration cap).
  Default behavior stays one-change-per-run; continuation must be asked for.
- Stop conditions and their reporting: the run's final report enumerates what was drained, what
  was skipped and why, and which stop condition ended the loop.

## Out of scope

- Concurrent/parallel fan-out of multiple implementers (#0008 owns that).
- Merging PRs mid-loop (finalize stays a separate skill; the merge gate is untouched).
- Any orchestrator that chains groom → implement → finalize across skills — this change only
  makes the implement stage self-continuing.

## Open questions

- Does continuation run in one agent context (context growth over N builds) or re-dispatch a
  fresh implement-next per iteration with the loop living in a thin driver?
- Interaction with dependency chains: a drained change can unblock nothing until its PR merges —
  when does the loop stop versus skip?
- Relationship to #0008: subsume its loser-picks-next bullet, or depend on / be absorbed by it?

## Reconcile log

## Auto-groom blocked

**2026-07-17 — docket-auto-groom abstained (adversarial critic verdict: sound).**

The change's titular, primary value is **continue-after-PR** (bullet 2 — "chains into the next
ready change instead of stopping"); loser-picks-next (bullet 1) is a race-loser corner case, not the
headline. A build-ready spec cannot be emitted without settling the continuation **architecture**,
and that decision cannot be safely defaulted by an autonomous groomer.

**The undecidable decision — continuation architecture (stub Open question 1).** Both horns are
blocked:

- **In-context loop** (the running fork loops Step 7 → Step 1 in one context). This is what the
  stub's own `## Out of scope` mandates ("this change only makes the implement stage
  self-continuing"; no cross-skill orchestrator). But it accumulates N heavyweight builds
  (reconcile + plan + SDD + review each) in a single forked-subagent context — an **unprobed
  harness/context-limit assumption**. Per learning `harness-behavior-is-mode-and-version-scoped`
  (2026-07-17), a design must not rest on unprobed harness behavior; it must be spiked in the exact
  mode first. The groomer writes markdown only and cannot spike it. A bounded cap (e.g. N=3) does
  not remove the assumption — even the second in-context build is unprobed — and "gracefully abort
  on context exhaustion" is not implementable (context overflow kills a fork; it is not a catchable
  clean stop). (Note: loser-picks-next is in-context-safe because the race loser aborts *before*
  building, so nothing accumulates — the two loops are not the same.)
- **Thin external driver** (re-dispatch a fresh implement-next per iteration). This is the very
  loop/driver/orchestrator primitive that two human-reserved siblings already charter:
  **#0008** (parallel-backlog-drain — the "drain entry point ... new docket-drain skill or a mode",
  effective `auto_groomable: false`) and **#0087** (headless-finalize-driver — HIGH priority,
  `auto_groomable: false`, whose lead open question is "Trigger surface — the real design question;
  everything else follows from it"). #8, #87, #88 all circle one primitive; choosing the
  implement-side driver shape in isolation front-runs a cross-change, high-priority decision. Per
  `moving-base`, nothing has merged, so there is no settled base to reconcile the partition against.

Stub Open question 3 (trigger/entry surface) collapses into this decision — it is only a separate
question on the external-driver horn.

**What a human should supply.** (1) The **partition of the shared loop/driver/continuation
primitive across #0008 / #0087 / #0088** — a backlog-composition call reserved to a human (which
change owns the loop, whether the three share one driver pattern). (2) For the in-context horn, an
explicit **authorization plus a harness spike** of an N-build fork-context loop in the mode
implement-next actually runs (`context: fork`), before any design rests on it.

**Recommendations (a human decides — kill/defer/subsume are never autonomous):**

- The **loser-picks-next** slice (bullet 1) is small, in-context-safe, and independently
  designable. It is identical to a bullet #0008 owns; per `moving-base` #0008 would fold its copy to
  residual (it still owns concurrent fan-out, the independence guard, and the concurrency cap — not
  "genuinely covered"). Consider grooming this slice on its own, or folding it into #0008, rather
  than carrying it inside the blocked continuation change.
- Groom #0087 (HIGH) first: it is the same driver question one level up. Its resolution likely
  dictates whether #0088's continuation is a shared driver pattern or a distinct mechanism, and may
  subsume or reshape #0088 entirely.
- Do **not** narrow #0088's spec to only loser-picks-next and ship it as-is: that would drive the
  change to `done` reading "loop continuation shipped" while the actual continuation never landed.
