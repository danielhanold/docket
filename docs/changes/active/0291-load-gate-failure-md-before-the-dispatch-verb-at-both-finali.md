---
id: 291
slug: load-gate-failure-md-before-the-dispatch-verb-at-both-finali
title: 'Load gate-failure.md before the dispatch verb at both finalize gate steps'
status: proposed
priority: medium
type: refactor
created: 2026-08-11
updated: 2026-08-11
depends_on: []
related: []
discovered_from: [260]
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

**Trigger** — surfaced by change 0260's whole-branch review (docket-review-deep), while wiring the
dispatch-capability `carve-out` posture into `skills/docket-finalize-change/references/gate-failure.md`.

**Opportunity** — at both merge-gate dispatch moments, `skills/docket-finalize-change/SKILL.md`
orders its instructions dispatch-verb-first: gate step 2 reads "On conflict, dispatch the
`docket-rebase-resolver` subagent … On any conflict, **read `references/gate-failure.md` now
(blocking)**", and gate step 5 carries the parallel clause for `docket-integration-repair`. The
blocking read that loads the failure posture therefore sits *after* the instruction to dispatch, so
a top-to-bottom reader attempts the dispatch and only then loads the posture governing what to do
when that dispatch is unavailable. Moving the blocking-read clause ahead of the dispatch verb in
both steps closes the ordering gap — no new prose, no copy-pinning of the marker into SKILL.md.

**Independent value** — stands with 0260 reverted: the ordering is pre-existing and governs every
abort-and-report reason `gate-failure.md` owns, not only the carve-out 0260 added. It is a delivery
property of the reference-loading idiom, and the same clause-ordering question applies wherever a
skill pairs an action verb with a blocking reference read.

**Boundary** — re-order the existing clauses inside the two numbered gate steps of
`skills/docket-finalize-change/SKILL.md`, and adjust any guard whose predicate keys on their
relative position (`tests/test_finalize_gate.sh` already asserts one line-order property on that
file — "local validation precedes the push" — so the ordering asserts need re-checking). It stops
there: no change to what is dispatched, when finalize dispatches it, or to `gate-failure.md`.

**Reason for deferral** — 0260's spec settled the opposite placement as an audited assumption
(Assumption 3: the marker lives in `gate-failure.md`, explicitly *not* in SKILL.md's step 2 / step 5
dispatch sentences, to avoid two marker sites to keep in agreement). Editing those exact sentences
reopens a decision a human audited, and an autonomous run must not reverse an audited assumption
inside a branch scoped to honour it. It also touches a file 0260 otherwise never modifies.
