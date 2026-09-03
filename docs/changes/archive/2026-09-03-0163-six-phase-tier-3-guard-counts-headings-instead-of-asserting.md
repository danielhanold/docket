---
id: 163
slug: six-phase-tier-3-guard-counts-headings-instead-of-asserting
title: Six-phase Tier 3 guard counts headings instead of asserting the phase set
status: 'killed'
priority: low
type: fix
created: 2026-07-28
updated: '2026-09-03'
depends_on: []
related: []
discovered_from: [135]
adrs: []
spec:
plan:
results:
trivial: true
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

`tests/test_cursor_contract_docs.sh`'s six-phase assert over `docs/cursor/validation.md` counts
occurrences of `^### Phase [1-6]` and compares the count to 6. That predicate is satisfied by any
six matching headings — a duplicated `Phase 3` alongside a missing `Phase 5` passes. The guard
therefore pins a population size, not the coverage it claims: that Tier 3's certifying checklist
still has all six distinct phases.

Recorded as a known weakness in change 0135's results file (`## Parked, deliberately not fixed`)
rather than fixed in that branch, because the fix wave was already closing. It is a live instance of
the harvested `assert-detects-removal-not-replacement` and
`marker-scoped-guard-needs-a-population-floor` findings.

## What changes

Assert the *identity* of the phases, not their count — extract the phase numbers, sort them, and
compare against the exact set `1 2 3 4 5 6`, failing loudly on a duplicate or a gap. Mutation-prove
it both ways: duplicate a phase heading and delete another, and confirm the guard goes red where it
is green today.

## Out of scope

Any change to the Tier 3 checklist's own content or phase count. If the checklist legitimately grows
a seventh phase, the guard's expected set is updated with it — that is the guard working.

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — the flawed assert lived in the deleted tests/test_cursor_contract_docs.sh; the Go port has no phase-count predicate.
