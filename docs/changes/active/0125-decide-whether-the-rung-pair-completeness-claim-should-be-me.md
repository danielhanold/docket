---
id: 125
slug: decide-whether-the-rung-pair-completeness-claim-should-be-me
title: Decide whether the rung-pair completeness claim should be mechanically enforced
status: proposed
priority: medium
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: []
discovered_from: [112]
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
type: chore
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0112 completed the `finalize.test_command` cross-layer masking matrix: with three config
rungs (local `.docket.local.yml` > committed `.docket.yml` > global `config.yml`) there are six
ordered rung pairs, and section S of `tests/test_docket_config.sh` now pins all six as fixtures
`s4`–`s9`.

The whole-branch review found that the **completeness claim itself is unguarded prose**. The
six-pair enumeration lives only in the section header comment (`tests/test_docket_config.sh:1038-1045`);
nothing in the code derives the rung count. If a fourth config layer were ever added to the
resolver, the ordered-pair count goes 6 → 12, six cells silently go unpinned, and the header
comment becomes false with **zero test failures**.

This is the `correspondence-guard-runs-one-way` shape the learnings ledger already names: the
matrix is proven only in the direction "for each pair the author enumerated, a fixture exists."
Nothing proves the converse — that the enumeration still covers every pair the resolver actually
has.

The review judged this correctly out of scope for 0112 (which deliberately pins current behavior)
and worth a follow-up, which is what this stub is.

## What changes

Decide whether the rung-pair completeness claim should be mechanically enforced, and if so how.
The design tension to resolve at grooming:

- An enforcement guard would have to read the resolver's source shape to count rungs — exactly the
  brittle-anchor failure mode change 0114 is currently weighing (whether line-number and
  source-shape anchors are a supportable convention at all).
- A hand-maintained enumeration of the six pairs is an **enumerated floor** that ages directly into
  the gap it was written to close (the `enumerated-floor` learning, promoted to AGENTS.md).

So the honest options span: derive the rung count from the resolver's `lcl`/`yaml_get "$CFG"`/`gbl`
helper set and assert the fixture count matches the pair count; assert only that the three known
rung readers still exist (a weaker floor that at least reddens when a fourth is added); or accept
the prose claim and close this as a documented non-goal.

Coordinate with change 0114 — if the repo decides against source-shape anchors, that constrains the
viable designs here.

## Out of scope

- Re-opening 0112's fixtures or its mutation protocol; the six pairs that exist are pinned and proven.
- Adding a fourth config layer. This is about detecting one, not building one.

## Triage note (2026-07-26, change 0124)

**The change-0114 coordination this stub was waiting on has resolved — grooming is unblocked.**

0114 is `done` and landed **ADR-0054** (cross-reference anchor style). Its verdict was
**"convert, do not close"**: source-shape/line-number anchors are not abandoned, they are moved to a
guarded form. Re-measured at its reconcile, the guarded population had doubled 13 → 26 lines across
8 files, and it concluded the idiom "accretes faster than it rots — which is the case for the
guard."

That removes the constraint framed in `## What changes`. The design tension as written assumed
0114 might rule *against* source-shape anchors, which would have foreclosed deriving the rung count
from the resolver's `lcl`/`yaml_get`/`gbl` helper set. It did not. So that option is live, and it
should be weighed against ADR-0054's guarded-anchor conventions rather than treated as brittle by
default.

The genuine open question survives and is still a human call: whether the completeness claim is
worth enforcing at all, or is better closed as a documented non-goal. Left in the human queue for
that reason, not for the 0114 dependency.
