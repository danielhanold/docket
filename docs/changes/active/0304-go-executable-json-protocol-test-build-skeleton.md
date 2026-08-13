---
id: 304
slug: go-executable-json-protocol-test-build-skeleton
title: 'Go executable, JSON protocol, and test/build skeleton'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: []
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

Every migration slice needs one buildable Go module, executable entry point, stable protocol
envelope, and test convention before independently developed packages can compose safely.

## What changes

Establish the Go module and `docket` executable, protocol-v1 result encoding, text-versus-JSON CLI
selection, build metadata, baseline test/static-check commands, and compatibility-fixture layout.

## Out of scope

Domain behavior, repository mutation, installation, harness emission, and any retained workflow.

## Open questions

Settle exact package names, Go version, CLI library choice, and protocol envelope fields during
grooming without pulling later subsystem behavior into this foundation.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
