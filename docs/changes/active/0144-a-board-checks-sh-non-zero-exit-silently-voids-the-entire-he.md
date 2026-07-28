---
id: 144
slug: a-board-checks-sh-non-zero-exit-silently-voids-the-entire-he
title: A board-checks.sh non-zero exit silently voids the entire health pass
status: proposed
priority: medium
type: chore
created: 2026-07-27
updated: 2026-07-27
depends_on: []
related: []
discovered_from: [117]
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

`docket-status.sh`'s `health_checks()` pipes `board-checks.sh` into a `while read` loop, so a
non-zero exit from the checker produces ZERO `check` lines and the health pass reports a clean
tree. Change 0117's final review found a live instance of this (a missing `adrs_dir` made
`board-checks.sh` exit 2 and silently dropped every health check) and fixed the trigger — but the
*swallowing* remains, and the regression test written for it cannot see the failure it was written
about: the mock `board-checks.sh` exits 0 regardless of its arguments, so the "still emits check
lines" assert passes against both the fixed and the unfixed code.

The general shape is worth closing deliberately: any future condition that makes `board-checks.sh`
exit non-zero becomes an invisible loss of the entire health pass, and the existing test scaffolding
structurally cannot detect it.

## What changes

Add a `board-checks.sh` mock that exits 2 (and one that exits 1) and assert what `health_checks`
does with it — at minimum that the failure is SURFACED rather than read as a clean tree. Decide
whether the current best-effort posture ("a board-checks failure produces no extra output and never
aborts the pass", per `scripts/docket-status.md`) is still the right contract now that a whole-pass
loss can hide behind it, or whether the pass should emit a distinguishable diagnostic line.

## Out of scope

- Re-litigating `board-checks.sh`'s own exit-2-on-bad-argument rule, which is correct for a hand-run
  caller.
- Change 0117's specific trigger, already fixed.
