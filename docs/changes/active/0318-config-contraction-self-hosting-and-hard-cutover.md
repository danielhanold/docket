---
id: 318
slug: config-contraction-self-hosting-and-hard-cutover
title: 'Go-only source cutover'
status: 'in-progress'
priority: critical
type: refactor
created: 2026-08-12
updated: '2026-08-29'
depends_on: [317, 352, 363]
stacked_on:
related: [322, 326, 361, 366]
discovered_from: [303]
adrs: []
spec: docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'refactor/config-contraction-self-hosting-and-hard-cutover'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-29T16:48:15Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md) |
<!-- docket:artifacts:end -->

## Why

Docket's Go lifecycle is complete, but maintained workflows, configuration, tests, and operator
documentation still route through the production Bash facade and its helper/runtime tree. That
dual implementation prevents the reviewed Go source from becoming the one authoritative product
and keeps the repository's canonical suite tied to the implementation being retired.

## What changes

- Derive a whole-repository inventory of legacy executable paths, retained product invariants,
  dependent guards, immutable history, and the two approved POSIX product surfaces.
- Rewrite maintained skills, agents, generated dispatch material, workflows, setup checks,
  configuration, and operator instructions to use the PATH-resolved Go CLI and JSON contracts.
- Remove the production Bash facade, helper/runtime tree, compatibility paths, environment
  bridges, legacy-runtime lifecycle dependencies, and mechanism-only tests in the same PR.
- Preserve every surviving product invariant with mutation-sensitive Go coverage or true `/bin/sh`
  coverage owned by repository-root `install.sh` and the release downloader.
- Add `docket development test` as the Go-native whole-suite implementation, entered from source
  through a branch-faithful Go bootstrap and used by contributors, finalization, and the
  release-candidate source gate.
- Replace active documentation with the Go-only model and identify `v1.0.0-rc1` as the upcoming
  first public Go candidate without asserting that publication already exists.

## Out of scope

Whole-backlog migration-ledger dispositions, manual migration learning records, post-merge
candidate packaging, native target smokes, genuinely fresh Claude/Codex/Cursor/OpenCode lifecycle
proofs, `v0.9.2` rollback rehearsal, tag or GitHub Release creation, public-install verification,
and final release evidence or metadata closeout. Those human-attended gates belong to change 0366,
which depends on this source cutover. Also excluded: stable `v1.0.0`, Homebrew, Windows,
signing/notarization, SBOM or provenance signing, uninstall, version-tree garbage collection,
unrelated capability changes, and rewrites of historical or frozen records.

## Design decisions

Change 0318 remains one reviewable code PR and keeps its existing id, slug, recorded branch, and
claim continuity. The source gate tests the exact checkout under review rather than a stale
installed binary. Historical records and frozen `v0.9.2` fixtures are not rewritten; an active
baseline change creates a new versioned fixture with provenance. Generator behavior may be proved
inside this PR, while external fresh-process and public-release truth is reserved for 0366.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

