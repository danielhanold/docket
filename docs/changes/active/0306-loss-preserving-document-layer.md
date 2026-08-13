---
id: 306
slug: loss-preserving-document-layer
title: 'Loss-preserving document layer'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [304]
stacked_on:
related: [266]
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

The compatibility contract requires typed validation without allowing the first Go write to
normalize or destroy unrelated bytes in existing Markdown records.

## What changes

Implement frontmatter location mapping, typed semantic decoding, loss-preserving field and managed
block patches, marker validation, canonical new-document rendering, compatibility goldens, and fuzz
targets.

## Out of scope

Repository-wide domain policy, Git reads, transactions, and individual workflow operations.

## Open questions

Settle the source-preservation representation and YAML-library boundary against frozen Bash-era
fixtures during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
