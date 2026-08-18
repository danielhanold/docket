---
id: 326
slug: pre-go-mutation-configuration-contraction
title: 'Pre-Go mutation configuration contraction'
status: proposed
priority: critical
type: chore
created: 2026-08-18
updated: 2026-08-18
depends_on: [315]
stacked_on:
related: [316, 318, 322]
discovered_from: [316]
adrs: []
spec: docs/superpowers/specs/2026-08-18-pre-go-mutation-configuration-contraction-design.md
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
| Spec | [2026-08-18-pre-go-mutation-configuration-contraction-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-18-pre-go-mutation-configuration-contraction-design.md) |
<!-- docket:artifacts:end -->

## Why

Docket's repository currently opts into deferred capabilities that Go v1 correctly refuses before
mutation. Leaving their contraction until the final self-hosting change makes the first Go-managed
implementation run impossible and creates a circular dependency at change 0316.

## What changes

Turn off the three active deferred capabilities in committed `.docket.yml`, remove the
repository-local agent-routing request, and turn off repository-local automatic capture on the
migration host. Verify the full four-layer resolved configuration permits Go mutation while
preserving global model/effort overrides and the fail-closed capability policy.

## Out of scope

Changing Go's configuration schema or capability classifier, modifying global agent pins,
installing the Go binary, adopting legacy harness artifacts, implementing any lifecycle operation,
removing Bash, publishing a release, or claiming that this configuration check alone proves
self-hosting.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
