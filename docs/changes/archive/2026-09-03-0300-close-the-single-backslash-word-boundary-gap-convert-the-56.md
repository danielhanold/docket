---
id: 300
slug: close-the-single-backslash-word-boundary-gap-convert-the-56
title: 'Close the single-backslash word-boundary gap: convert the 56 sites and make the census gating'
status: 'killed'
priority: medium
type: fix
created: 2026-08-12
updated: '2026-09-03'
depends_on: []
related: []
discovered_from: [298]
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

**Trigger** — change 0298's fix-loop suite gate went red on `tests/test_grep_portability.sh`, which
rejected a `\\b` word-boundary spelling a fix commit had just introduced. That guard passed, but its
own output carries a standing note: `unguarded single-backslash \b sites in scope: 56 (computed,
not gating)`. The two-backslash form is gated; the far more common single-backslash form is counted
and waved through.

**Opportunity** — close the gap the guard already measures. `\b` is a GNU extension: BSD `grep` and
`git-grep` ERE do not implement it and return **zero matches silently** rather than erroring. A
guard, sentinel, or validator written with `\b` therefore does not fail loudly on a BSD box — it
goes permanently, invisibly green, which is this repo's defining defect class ("a mutation that
leaves an assert green is a defect until proven otherwise"). Converting the 56 sites to an explicit
`[^[:alnum:]_]` class and then flipping the guard from computed to gating removes a whole family of
potentially vacuous asserts and prevents the next one from being written.

**Independent value** — wholly independent of stacked changes. The 56 sites predate change 0298 and
survive its revert. The value is the same one the existing two-backslash gate already delivers,
extended to the spelling that is actually common in the tree.

**Boundary** — convert the enumerated single-backslash `\b` / `\<` / `\>` sites in maintained source
to explicit character classes, verify each converted site still matches what it was written to match
(and still reddens under its own mutation), then flip `tests/test_grep_portability.sh`'s
single-backslash census from computed-only to gating so the population cannot grow again. It
deliberately leaves alone: point-in-time records (specs, plans, archived changes, Accepted ADRs),
the already-gated two-backslash rule, and the ERE repetition-bound checks in the same file.

**Reason for deferral** — a repo-wide regex rewrite touching dozens of guard files has nothing to do
with stacking, and each converted site needs its own mutation re-check to prove the conversion did
not silently widen or narrow it. Riding that on change 0298's branch would swamp a feature diff with
unrelated churn and put its green suite at risk for no gain to the feature.

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — tests/test_grep_portability.sh and the 56 shell sites are deleted; Go regexp has no BSD word-boundary divergence.
