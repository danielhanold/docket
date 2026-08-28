---
id: 318
slug: config-contraction-self-hosting-and-hard-cutover
title: 'Remaining configuration cleanup, self-hosting, and hard cutover'
status: proposed
priority: critical
type: refactor
created: 2026-08-12
updated: 2026-08-28
depends_on: [317, 352, 363]
stacked_on:
related: [322, 326, 361]
discovered_from: [303]
adrs: []
spec: docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md
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
| Spec | [2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md) |
<!-- docket:artifacts:end -->

## Why

Docket must prove that the installed Go product can manage its own complete real lifecycle before
the production Bash implementation is removed. The final cutover must preserve settled Go v1
compatibility and rollback boundaries while publishing the first public Go release candidate.

## What changes

- Require 0352's native repository operation family and 0363's removal of unused main-mode
  compatibility before cutover.
- Rehearse and verify full self-hosting from the exact installed candidate through Claude, Codex,
  Cursor, and OpenCode.
- Finish remaining active configuration and migration-ledger cleanup, and capture migration
  learnings manually through Go.
- Remove production Bash and implementation-only Bash tests while preserving retained product
  coverage and the two approved POSIX bootstrap/downloader surfaces.
- Replace active documentation and publish the exact accepted artifacts as `v1.0.0-rc1`.

## Out of scope

Implementing 0352 or 0363, reintroducing deferred capabilities, repeating 0322's
bootstrap/adoption or 0326's early configuration contraction, retaining a Bash fallback, changing
the existing-repository compatibility contract inside 0318, Homebrew, or stable `v1.0.0`
promotion.

## Design decisions

The public tag is `v1.0.0-rc1`. The exact post-merge candidate is packaged once, accepted through
the four target tuples and four fresh host sessions, rehearsed against an isolated `v0.9.2`
rollback copy, then published without rebuilding. Historical records and the frozen rollback corpus
are not rewritten during active-surface cleanup.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
