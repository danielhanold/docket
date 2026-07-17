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
auto_groomable:
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
