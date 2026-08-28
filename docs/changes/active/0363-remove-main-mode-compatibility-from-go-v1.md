---
id: 363
slug: 'remove-main-mode-compatibility-from-go-v1'
title: 'Remove main-mode compatibility from Go v1'
status: 'in-progress'
priority: high
type: refactor
created: 2026-08-28
updated: '2026-08-28'
depends_on: [352]
stacked_on:
related: [318, 326, 305, 309, 310, 312, 315, 316]
discovered_from: [352]
adrs: [1, 2, 52, 69]
spec: docs/superpowers/specs/2026-08-28-remove-main-mode-compatibility-from-go-v1-design.md
plan: 'docs/superpowers/plans/2026-08-28-remove-main-mode-compatibility-from-go-v1.md'
results:
trivial: false
auto_groomable:
branch: 'refactor/remove-main-mode-compatibility-from-go-v1'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-28T21:26:14Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-remove-main-mode-compatibility-from-go-v1-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-remove-main-mode-compatibility-from-go-v1-design.md) |
| Plan | [2026-08-28-remove-main-mode-compatibility-from-go-v1.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-28-remove-main-mode-compatibility-from-go-v1.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0002](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0002-docket-mode-default-and-bootstrap.md), [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md), [ADR-0069](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0069-mode-conditioned-clause-discriminates-on-provenance.md) |
<!-- docket:artifacts:end -->

## Why

Change 0352's repository-setup design exposed that the supported `main` metadata mode has no known
users yet imposes a second steady-state topology across configuration, status, metadata
transactions, finalization, backlink rendering, command output, and test matrices. Keeping it
through the Go cutover would complicate every repository operation and preserve a compatibility
promise Docket does not need.

The release candidate should have one repository model: planning metadata lives on the orphan
`docket` branch and code lands on the independently configurable integration branch. Legacy
single-branch repositories still need an explicit way out, so change 0352's native migration must
land before this compatibility path is removed. Change 0318 must wait for both changes before the
Go-only cutover.

## What changes

- Remove `main` as a supported metadata topology and remove the now-redundant `metadata_branch`
  setting from the active Go configuration schema, capability presentation, documentation, and
  examples.
- Collapse mode-conditioned status, source selection, metadata transactions, planning lifecycle,
  finalization, backlink, result, and CLI/JSON behavior onto the docket-branch model.
- Preserve `integration_branch` and GitFlow/trunk flexibility; removing main mode must not hard-code
  the integration branch to `main`.
- Keep change 0352's `repository migrate` as the explicit legacy-input exception: normal commands
  reject an obsolete single-branch configuration with a precise migrate remedy, while migration
  can recognize and remove the old key.
- Replace mode matrices with docket-mode coverage without dropping mode-independent invariants, and
  update current program documentation plus the accepted decision record that promised the opt-out.

## Out of scope

- Implementing or redesigning change 0352's repository initialization, migration, health, repair,
  or recovery operations.
- Removing or constraining `integration_branch`, changing the orphan metadata-branch model, or
  restoring terminal publishing.
- Rewriting historical specs, results, archived changes, accepted ADR text, or the frozen `v0.9.2`
  compatibility corpus to erase the old mode.
- General configuration contraction unrelated to metadata topology, machine installation, Bash
  production removal, self-host acceptance, or release publication owned by change 0318.

## Design decisions

- Make a clean protocol-v1 break: remove the mode-shaped configuration and status JSON fields
  rather than retaining constant compatibility values.
- Reuse change 0352's repository classifier and exact `legacy-repository` finding through a shared
  operational-context loader extracted from the existing status pin path. Normal repository-aware
  commands require migration; only repository check and migrate diagnose or exit legacy state.
- Keep `metadata_branch` solely as a decode-only obsolete tombstone and migration input. It never
  contributes an effective value or selects a branch.
- Preserve mode-independent tests by moving generic fixtures to docket topology and collapsing true
  matrices to their docket row; delete only assertions whose subject is the retired topology, and
  keep frozen compatibility inputs byte-identical.
- Supersede ADR-0002 with a new one-topology decision while retaining ADR-0001 and appending a dated
  cross-reference update because its orphan-branch architecture still stands.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-28

### 2026-08-28 — reconcile at claim

Verified against current `docket`/`main` reality before planning:

- **Prerequisite 0352 is `done`** (archived `2026-08-28-0352-native-repository-initialization-and-health-check.md`). Its repository classifier (`reposetup.StateLegacy`), health findings, and native `repository init`/`repository migrate` operations are landed in `internal/app` — the shared operational-repository gate this change extracts has a real classifier to reuse.
- **Main mode is still fully present** in production Go code, so the change is not obsolete and no other change has absorbed it. Confirmed live executable mode sites include `config.Effective.MetadataBranch` (`internal/config/config.go`, `defaults.go`, `resolve.go`), the `metadataModeMain`/`metadataModeDocket` selector and `StatusPin.Mode` assignment (`internal/app/status_git.go`), the `metadataBranchOf(pin)` helper (`internal/app/change_create.go` and its ~12 call sites across lifecycle/finalize/attach/reclaim/kill/halt), `status_result.go`'s `metadata_branch` JSON field, and `repository_facts.go`'s `metadataBranch`. Roughly 34 non-test executable sites carry the topology.
- **Related changes 0305, 0309, 0310, 0312, 0315, 0316, 0326 are all `done`** — Go-migration groundwork — and none removed main mode. **0318 remains `proposed`** and is the later Go-only cutover; it is `waiting-on-363-unbuilt` and does not absorb this contraction. `integration_branch` flexibility and the orphan metadata-branch model are untouched, per the spec's exclusions.
- **ADR posture unchanged:** this change supersedes ADR-0002 with a new one-topology decision and appends a dated cross-reference update to ADR-0001 (still Accepted); the new ADR id is allocated by the normal docket ADR workflow at review time and attached then.

Scope, boundary, and acceptance criteria in the spec hold as written. No sections, spec sections, or relations changed.
