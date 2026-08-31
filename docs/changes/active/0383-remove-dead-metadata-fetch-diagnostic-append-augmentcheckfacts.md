---
id: 383
slug: 'remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts'
title: 'Remove or plumb the dead metadata-fetch diagnostic append in augmentCheckFacts'
status: proposed
priority: low
type: fix
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: [377, 378]
discovered_from: [377]
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

`repository_check.go`'s `augmentCheckFacts` builds a metadata-fetch diagnostic that is appended to a
value nothing subsequently reads — a dead diagnostic append. It was flagged as review finding F2
during change [[0377]]'s cutover review but is **pre-existing on `main`** (it came in with change
[[0378]]), so it sat outside 0377's branch scope and was reported as follow-up rather than fixed
there. Either the diagnostic is meant to reach a caller and is currently mis-plumbed, or it is truly
dead and should be removed; either way the current state is a latent bug.

## What changes

Decide whether the metadata-fetch diagnostic in `augmentCheckFacts` should surface to a caller or be
deleted, then either plumb it to where it belongs or remove the dead append. Keep the fix minimal —
this is a single-site correctness/cleanup change, not a rework of the check facts pipeline.

## Out of scope

- Any broader refactor of `repository_check.go`'s facts augmentation beyond this one diagnostic.
- Re-litigating 0378's ownership-verifier design.

## Open questions

- Is the diagnostic intended to be user-visible (plumb it) or genuinely unused (delete it)? Resolve
  by tracing what, if anything, was meant to consume it when 0378 introduced it.

## Reconcile log
