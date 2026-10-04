---
id: 506
slug: 'drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi'
title: 'Drop the retired .kiro and .windsurf globs from the managed .gitignore block'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [464]
discovered_from: [464]
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

`internal/reposetup/gitignore.go` still writes `.kiro/agents/docket-*.md` and `.windsurf/agents/docket-*.md` into the managed `.gitignore` block, but docket no longer generates agent wrappers for those harnesses. Found while building change 0464.

## What changes

Stub, a hypothesis for grooming: remove the two globs from the managed block, and check that rewriting an existing repository's block drops them cleanly.

## Out of scope

Other harness support changes.
