---
id: 311
slug: installer-embedded-assets-and-four-harnesses
title: 'Installer, embedded assets, and four first-class harnesses'
status: in-progress
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-13
depends_on: [304, 305]
stacked_on:
related: [77, 135, 192, 242]
discovered_from: [303]
adrs: [60, 78]
spec: docs/superpowers/specs/2026-08-13-installer-embedded-assets-and-four-harnesses-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/installer-embedded-assets-and-four-harnesses
claimed_at: 2026-08-13T20:20:01Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-installer-embedded-assets-and-four-harnesses-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-installer-embedded-assets-and-four-harnesses-design.md) |
| ADRs | [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0078](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md) |
<!-- docket:artifacts:end -->

## Why

Go v1 is not usable until released and source-linked assets install safely and natively into every
directly supported host.

## What changes

Design and implement a deterministic embedded asset bundle, versioned extraction, journaled
ownership-safe installation, source-linked development mode, global model/effort rendering, and
native user-level plans for Claude, Codex, Cursor, and OpenCode.

Release and development installs share one manifest, compatibility guard, transaction engine, and
four pure harness renderers. `install check` diagnoses drift without mutation. Repository-local
shadow cleanup is left as an ownership-proof seam for a later explicit-repository operation.

## Out of scope

Behavior owned by changes 0305–0310 and 0312–0318, including documents, repository/domain/Git
operations, workflow semantics, repository init/migration, durable process supervision, ADR/finalize
workflows, release archives/downloads, and Bash retirement. Also excluded are cross-harness runner
delegation, per-repository routing or configuration, skill rebinding, and Homebrew packaging.

## Design decisions

The focused design is approved in the linked spec. It installs verified release assets beneath a
versioned user data root or links contributor skills to a validated source checkout; renders direct
native agents and global dispatch material at each harness's user-level paths; consumes only
built-in and global model/effort settings; proves ownership before replacement or pruning; and uses
a rollback journal because multiple harness directories cannot share one atomic rename.

Later workflow changes may update the opaque authored inventory and regenerate the bundle without
expanding this installer's behavioral scope.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
