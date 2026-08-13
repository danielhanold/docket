---
id: 318
slug: config-contraction-self-hosting-and-hard-cutover
title: 'Configuration contraction, self-hosting, and hard cutover'
status: proposed
priority: critical
type: refactor
created: 2026-08-12
updated: 2026-08-12
depends_on: [317]
stacked_on:
related: []
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

Docket must explicitly enter the supported Go v1 capability envelope and prove it can manage its
own real lifecycle before the production Bash implementation is removed.

## What changes

Apply the pre-approved committed configuration contraction; rehearse and verify self-hosting;
capture migration learnings manually; remove production Bash and Bash-only tests; replace active
documentation; publish and verify the hard Go cutover.

## Out of scope

Reintroducing deferred capabilities, retaining a Bash fallback, Homebrew, and changing the existing
repository compatibility contract.

## Open questions

Settle the exact deletion and rollback rehearsal after release acceptance passes. The product-scope
authorization to disable `auto_capture`, terminal publishing, build checkpoints, and results-only
gate skipping is already granted and is not an open question.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
