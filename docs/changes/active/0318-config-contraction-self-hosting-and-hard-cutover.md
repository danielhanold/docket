---
id: 318
slug: config-contraction-self-hosting-and-hard-cutover
title: 'Remaining configuration cleanup, self-hosting, and hard cutover'
status: proposed
priority: critical
type: refactor
created: 2026-08-12
updated: 2026-08-18
depends_on: [317]
stacked_on:
related: [322, 326]
discovered_from: [303]
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

Docket must prove that the installed Go product can manage its own complete real lifecycle before
the production Bash implementation is removed. The minimal capability contraction needed to start
that lifecycle now lands earlier in 0326.

## What changes

Rehearse and verify full self-hosting from the installed runtime; finish any configuration and
migration-ledger cleanup not owned by 0326; capture migration learnings manually; remove production
Bash and Bash-only tests; replace active documentation; publish and verify the hard Go cutover.

## Out of scope

Reintroducing deferred capabilities, repeating 0322's bootstrap/adoption or 0326's early
configuration contraction, retaining a Bash fallback, Homebrew, and changing the existing
repository compatibility contract.

## Open questions

Settle the exact deletion and rollback rehearsal after release acceptance passes. The early
authorization to disable `auto_capture`, terminal publishing, build checkpoints, and results-only
gate skipping is consumed by 0326 and is not reopened here.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
