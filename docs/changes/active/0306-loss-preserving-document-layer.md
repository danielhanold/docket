---
id: 306
slug: loss-preserving-document-layer
title: 'Loss-preserving document layer'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-13
depends_on: [304]
stacked_on:
related: [266]
discovered_from: [303]
adrs: []
spec: docs/superpowers/specs/2026-08-13-loss-preserving-document-layer-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-loss-preserving-document-layer-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-loss-preserving-document-layer-design.md) |
<!-- docket:artifacts:end -->

## Why

Go must read existing Markdown records as typed YAML without allowing the first intentional field
or managed-block write to normalize or destroy unrelated source bytes.

## What changes

Implement an immutable source-byte document model, typed semantic YAML decoding, top-level field
location mapping, validate-all loss-preserving field and managed-block patches, canonical
new-document syntax, frozen `v0.9.2` compatibility goldens, and parser/patch fuzz targets.

## Out of scope

Configuration behavior, repository-wide domain policy, Git reads, metadata transactions,
record-specific renderer content, and individual planning or implementation workflow operations.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
