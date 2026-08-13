---
id: 303
slug: go-migration-program-record-and-bash-backlog-disposition
title: 'Go migration program record and Bash-backlog disposition'
status: proposed
priority: critical
type: docs
created: 2026-08-12
updated: 2026-08-12
depends_on: []
stacked_on:
related: [285]
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-12-go-migration-architecture-design.md
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
| Spec | [2026-08-12-go-migration-architecture-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-12-go-migration-architecture-design.md) |
<!-- docket:artifacts:end -->

## Why

Docket's Go architecture and fifteen-change migration sprint are approved, but the active backlog
still mixes the new program with Bash-only work that must no longer compete for implementation.
The architecture, sprint topology, and disposition boundary need one durable Docket record before
Claude Code begins the migration.

## What changes

- Publish the approved [migration program map](../../superpowers/specs/2026-08-12-go-migration-program-map.md)
  as the sprint's topology and scope index.
- Audit every active Bash-era change and classify it as absorbed, retained, deferred, or killed.
- Name the owning Go successor before killing any absorbed product requirement.
- Confirm the fifteen child changes and their dependency edges match the approved program map.
- Record Claude Code as the implementation host and the pre-approved configuration contraction for
  the final self-hosting change.

## Out of scope

- Implementing any Go subsystem.
- Grooming all fifteen child changes ahead of their landed predecessor interfaces.
- Changing Docket's active configuration before the final cutover gate.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
