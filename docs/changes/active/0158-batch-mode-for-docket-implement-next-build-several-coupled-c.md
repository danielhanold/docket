---
id: 158
slug: batch-mode-for-docket-implement-next-build-several-coupled-c
title: Batch mode for docket-implement-next — build several coupled changes on one branch
status: proposed
priority: low
type: feat
created: 2026-07-28
updated: 2026-08-09
depends_on: []
related: [8, 157]
discovered_from: [157]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0157 rolled seven build-ready changes onto one branch by hand: one spec authored to coordinate
them, seven kills, and a hand-derived build order. It worked, and the reason it was worth doing is
structural rather than incidental — `docket-implement-next` pays a full claim / reconcile / plan /
build / review / PR cycle per change, and when several build-ready changes are small edits to the same
scripts and the same test suite, that per-change overhead dominates the actual work. The seven of 0157
also clustered: two pairs collided on one file each and were order-sensitive, so building them
separately would have cost a rebase and a stale-numbers re-derivation on top.

Doing it by hand has three costs a batch mode would remove: the human picks the membership, the
originals must be killed (losing them as individually-tracked units), and the build order is derived
in prose that nothing enforces.

## What changes

To be designed. The shape to explore — a way for `docket-implement-next` to claim and build **several**
build-ready changes on one branch, one plan, one review, one PR, without the hand-rolled rollup dance.

Open questions the brainstorm must settle:

- **Membership.** Who selects the batch — a human naming ids, a `--batch <n>` cap over the existing
  deterministic selection order, or an inferred grouping (shared type, shared files, shared
  `related:` edges)? File-overlap inference is the interesting option and the riskiest.
- **Tracking.** Do the members stay separate changes that all move to `implemented` against one `pr:`,
  or does a synthetic rollup change get minted and the members killed (what 0157 did by hand)? The
  first preserves per-change history and needs the manifest to admit a shared PR; the second is what
  already works but loses the units.
- **Ordering.** 0157's ordering was derived by reading seven specs. Can file collision be detected
  mechanically, and is a declared order in the plan enough, or does it need to be a first-class field?
- **Failure containment.** A red suite is batch-wide. What happens when one member cannot go green —
  drop it from the branch and re-mint it (0157's stated fallback), or fail the whole batch?
- **Review.** One whole-branch review over N units, or N scoped reviews? A finding against one member
  currently has nowhere to land once the members are killed.
- **Interaction with `depends_on`.** Two members where one depends on the other is either the best case
  for batching or a reason to refuse it.
- **Blast radius.** Batching multiplies the cost of a bad merge and of a claim held too long. The
  reclaim lease, `claimed_at` refreshes, and the finalize gate all assume one change per branch.

## Out of scope

- Change 0157 itself, which lands its seven units by hand and is not blocked on this.
- Parallel drain (change 0008) — fanning out concurrent runs over *independent* changes is the
  opposite trade: this stub is about coalescing *coupled* ones onto a single branch.

## Open questions

All of the above; nothing is settled. Worth an explicit early decision on whether the token saving is
real and large enough to justify the machinery, measured against 0157's actual build — if the rollup
turns out cheap to repeat by hand a few times a year, this should be killed rather than built.
