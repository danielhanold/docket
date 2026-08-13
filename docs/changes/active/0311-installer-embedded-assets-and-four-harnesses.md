---
id: 311
slug: installer-embedded-assets-and-four-harnesses
title: 'Installer, embedded assets, and four first-class harnesses'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [304, 305]
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

Go v1 is not usable until released and source-linked assets install safely and natively into every
directly supported host.

## What changes

Implement embedded asset manifests, atomic extraction, ownership and drift checks, source-linked
development mode, global model/effort rendering, and installation plans for Claude, Codex, Cursor,
and OpenCode.

## Out of scope

Cross-harness runner delegation, per-repository routing, skill rebinding, and release download
packaging.

## Open questions

Settle each harness's exact native paths, wrapper schema, and dispatch material from current
first-class behavior during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
