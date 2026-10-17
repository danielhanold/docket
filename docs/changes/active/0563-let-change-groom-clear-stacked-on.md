---
id: 563
slug: 'let-change-groom-clear-stacked-on'
title: 'Let change.groom clear stacked_on'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: []
discovered_from: [559]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

change.groom can set stacked_on on a change but cannot clear it, so unstacking a change (for example when its parent branch is abandoned or the work is rebased onto main) needs a hand edit of the record. Found during change 559.

## What changes

Let change.groom clear stacked_on through the typed request, with the same validation and board/artifacts re-render as setting it, so no hand edit is needed.

## Out of scope

Changing how stacks resolve their base or how killed parents are handled.
