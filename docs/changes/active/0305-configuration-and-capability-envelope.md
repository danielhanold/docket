---
id: 305
slug: configuration-and-capability-envelope
title: 'Configuration and capability envelope'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-13
depends_on: [304]
stacked_on:
related: []
discovered_from: [303]
adrs: [15, 16, 19, 20, 52]
spec: docs/superpowers/specs/2026-08-13-configuration-and-capability-envelope-design.md
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
| Spec | [2026-08-13-configuration-and-capability-envelope-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-configuration-and-capability-envelope-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Go must understand existing repositories and fail safely on deferred behavior before any later
mutation engine can rely on resolved policy.

## What changes

Implement a typed real-YAML configuration resolver with retained four-layer precedence,
coordination fences, winning-layer provenance, built-in defaults, global-only model/effort
overrides, and an exhaustive capability classifier. Add read-only configuration inspection and a
typed mutation preflight that refuses active deferred or dropped behavior before transaction entry.

## Out of scope

Document parsing or mutation, domain lifecycle rules, authoritative Git reads, metadata
transactions, status assembly, harness rendering, workflow execution, per-repository model routing,
skill rebinding, configuration contraction, and all behavior owned by changes 0306–0318.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
