---
id: 574
slug: 'stop-repository-prepare-re-walking-all-of-main-s-history-to'
title: 'Stop repository prepare re-walking all of main''s history to prove metadata ownership'
status: 'proposed'
priority: 'medium'
type: 'fix'
deliverable: 'pr'
created: '2026-10-20'
updated: '2026-10-20'
depends_on: []
stacked_on:
related: [360]
discovered_from: []
adrs: []
spec:
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`docket repository prepare` takes about 68 seconds on the docket repo with nothing else running (about 52s of CPU, nearly half of it system time), and almost every command runs it first, so `docket open` and similar commands each take about 70 seconds. Observed 2026-10-20 while building change 561; 561 did not cause it.

Cause (from reading the code, not yet profiled): the `.docket` branch's root commit ("seed metadata branch from main (migrate-to-docket dogfood)") has no seed receipt trailers, so ownership falls to the receiptless legacy path. `verifyLegacyEquivalence` in `internal/app/metadata_ownership.go` lists every commit reachable from main (4,462 here) and, newest first, reads each snapshot's `.docket.yml` and tree entries and sometimes builds a tree, each as its own git call, until one matches the seed. The matching snapshot is the old migration commit, so the walk goes nearly all the way back. The result is not kept, so every prepare repeats it. It is slower still while the test suite runs, because the git subprocesses compete for CPU.

## What changes

Make a repeat prepare on an already-proven repo cheap. Groom to choose among:

1. Record or cache the proof once the walk matches (a receipt, or a cached matching commit), so later prepares take the fast path. The code comment calls this path content-read-only, so writing a receipt is a design question.
2. Re-prove only when the metadata root tree or main's tip changed since the last successful proof.
3. Batch the per-commit git calls into one pass (`git log --raw` or `cat-file --batch`). A smaller win that does not remove the root cause.

The acceptance bar is a `docket repository prepare` on this repo that takes seconds, not a minute, with the same healthy/foreign/unknown outcomes.

## Out of scope

Weakening the ownership check or treating a truncated search as foreign (the existing fail-closed behavior stays). Changing the receipt format for newly seeded repos. The coordination cost inside implement-next runs, which change 360 covers.
