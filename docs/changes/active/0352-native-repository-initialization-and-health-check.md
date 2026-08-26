---
id: 352
slug: native-repository-initialization-and-health-check
title: 'Native repository initialization and health check'
status: proposed
priority: high
type: feat
created: 2026-08-26
updated: 2026-08-26
depends_on: []
stacked_on:
related: [351, 311, 318]
discovered_from: [303, 351]
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

The approved Go migration architecture names `repository init`, `repository migrate`, and
`repository check` as a required operation family, but no migration child implemented or clearly
retained ownership of it. The Go machine installer deliberately excluded complete repository setup,
leaving the hard-cutover product without a native way to establish or verify a fresh Docket
repository.

Change 0351 adds reusable, ownership-safe reconciliation for parent-facing repository surfaces, but
does not initialize configuration, metadata routing, worktree plumbing, or other repository-wide
state. The missing repository operation needs its own design and lifecycle.

## What changes

- Define and implement native `docket repository init` and `docket repository check` commands for
  the complete supported fresh-repository postcondition.
- Make initialization idempotent, ownership-safe, and explicit about every working-tree and Git
  effect; make `check` read-only with precise drift and remediation output.
- Reuse change 0351's repository-surface planner rather than creating a second parent-instruction
  writer.
- Establish the supported boundary between fresh initialization and migration of an existing
  Bash-era or partially configured repository.

## Out of scope

- Machine installation, uninstall, or version-tree collection.
- Reimplementing change 0351's global cleanup, fresh-binary handoff, or repository-surface renderer.
- Silently migrating an existing repository as a side effect of machine installation.
- Restoring or modifying legacy repository-migration scripts.

## Open questions

- What exact state constitutes a completely initialized repository: committed configuration,
  metadata branch and worktree, ignore rules, parent-facing harness surfaces, and any local-only
  state?
- Which initialized files remain uncommitted for human review, and which Git refs or commits—if
  any—may `repository init` create automatically?
- How should one operation recover across both reversible file writes and Git ref/worktree effects?
- Should `repository migrate` be designed as a separate change once the fresh-init postcondition is
  fixed, or share this change's implementation while remaining an explicit command?
- At grooming time, should this change depend on 0351's landed reconciler or consume a narrower
  shared planning interface extracted independently?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
