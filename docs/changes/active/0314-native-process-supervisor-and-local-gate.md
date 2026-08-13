---
id: 314
slug: native-process-supervisor-and-local-gate
title: 'Native process supervisor and local gate'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-12
depends_on: [304]
stacked_on:
related: [264, 285]
discovered_from: [303]
adrs: [81]
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

Change 0285 exposed that Bash cannot establish the desired macOS session and distinguish a real
exit code from signal death. Go can provide both without another discovered runtime.

## What changes

Implement Darwin/Linux session and process-group launch, private durable run directories, separate
logs, exact exit-versus-signal records, observe/stop, identity-safe signalling, and owned recovery.

## Out of scope

Python or Perl discovery, a global daemon, CI gate polling, and the claim-to-implemented workflow.

## Open questions

Reconcile the native supervisor contract against change 0285, ADR-0081, and landed protocol types;
record the ADR action that supersedes the Python-era decision.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
