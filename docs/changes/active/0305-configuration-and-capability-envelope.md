---
id: 305
slug: configuration-and-capability-envelope
title: 'Configuration and capability envelope'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [304]
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

Go must understand existing repositories and fail safely on deferred behavior before any later
mutation engine can rely on resolved policy.

## What changes

Implement real-YAML configuration layers, retained precedence and coordination fences, global
model/effort overrides, and explicit supported/obsolete/inert/deferred capability classification.

## Out of scope

Document mutation, domain lifecycle rules, per-repository model routing, and skill rebinding.

## Open questions

Settle exact Go configuration types, YAML library, diagnostic schema, and the full key-by-key
classification against the final `v0.9.2` surface during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
