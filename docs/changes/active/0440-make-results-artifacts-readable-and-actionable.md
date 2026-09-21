---
id: 440
slug: 'make-results-artifacts-readable-and-actionable'
title: 'Make results artifacts readable and actionable'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-21'
updated: '2026-09-21'
depends_on: []
stacked_on:
related: [1, 190, 330, 374, 410]
discovered_from: []
adrs: [102]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0102](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0102-build-and-finalize-own-independent-gate-and-test-command-con.md) |
<!-- docket:artifacts:end -->

## Why

Results artifacts currently emphasize Docket implementation details and provide too little practical help to a mid-level engineer reviewing a delivered change. Humans need a clear account of the behavior, required actions, useful functional checks, and unresolved problems.

## What changes

Adopt the approved human-readable results specification: a short action statement at the top; behavior-focused outcomes; Important and Optional human checks with complete setup, steps, expected results, and cleanup; concise verification; and one plain-language known-issues and follow-ups section. Allow optional walkthroughs of automated behavior and link deeper technical detail. Strengthen shared authoring guidance and structural validation while preserving existing artifact and evidence contracts.

## Out of scope

Implementation during this capture; retrospective rewriting of historical results; changes to checkpoint ownership, exact-head evidence, merge policy, or post-merge behavior; readability scoring; new review rounds, lifecycle states, or configuration; automatic follow-up creation.
