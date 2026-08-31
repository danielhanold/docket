---
id: 311
slug: installer-embedded-assets-and-four-harnesses
title: 'Installer, embedded assets, and four first-class harnesses'
status: done
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-14
depends_on: [304, 305]
stacked_on:
related: [77, 135, 192, 242]
discovered_from: [303]
adrs: [60, 78]
spec: docs/superpowers/specs/2026-08-13-installer-embedded-assets-and-four-harnesses-design.md
plan: docs/superpowers/plans/2026-08-13-installer-embedded-assets-and-four-harnesses.md
results: docs/results/2026-08-13-installer-embedded-assets-and-four-harnesses-results.md
trivial: false
auto_groomable:
branch: feat/installer-embedded-assets-and-four-harnesses
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/207
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-installer-embedded-assets-and-four-harnesses-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-installer-embedded-assets-and-four-harnesses-design.md) |
| Plan | [2026-08-13-installer-embedded-assets-and-four-harnesses.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-13-installer-embedded-assets-and-four-harnesses.md) |
| Results | [2026-08-13-installer-embedded-assets-and-four-harnesses-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-13-installer-embedded-assets-and-four-harnesses-results.md) |
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

- 2026-08-13 — Reconciled against origin/main. Dependencies 0304 (Go executable/protocol/test-build
  skeleton) and 0305 (configuration and capability envelope) are merged and archived; the spec's
  "landed foundation" section is accurate. One addition since the spec was written: 0306's
  loss-preserving document layer (`internal/document`, including validated marker-block handling)
  also merged today and is available for reuse in the managed-block (dispatch material) rendering
  and ownership checks — reuse it rather than re-implementing marker validation. No
  `internal/assets`, `internal/install`, or `internal/harness` packages exist yet; the authored
  asset roots (`skills/`, `agents/`, `agents/harness-defaults.yml`, templates, references,
  dispatch instructions) exist on origin/main as the spec assumes. Vendor agent-schema
  revalidation (Claude/Codex/Cursor/OpenCode native fields) is deferred to plan/build per the
  spec's own instruction. Scope unchanged; no drift; no auto-capture candidates surfaced.
