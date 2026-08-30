---
id: 370
slug: 'delete-the-frozen-bash-facade-and-legacy-test-surface'
title: 'Delete the frozen Bash facade and legacy test surface'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-29'
updated: '2026-08-30'
depends_on: [372]
stacked_on:
related: [318, 322, 326, 361, 366, 367, 369, 371, 372]
discovered_from: [318]
adrs: [14, 29, 30, 33, 36, 74, 99]
spec: 'docs/superpowers/specs/2026-08-29-delete-the-frozen-bash-facade-and-legacy-test-surface-design.md'
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
| Spec | [2026-08-29-delete-the-frozen-bash-facade-and-legacy-test-surface-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-delete-the-frozen-bash-facade-and-legacy-test-surface-design.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Once retained lifecycle consumers use Go, registered-agent invocation uses native host dispatch, and
deferred feature paths are sealed, the legacy Bash facade and its helper/runtime tree become
duplicate production machinery. Keeping them would preserve two control planes, two test
architectures, and the configuration seams the v1 cutover is intended to retire.

## What changes

- Reconcile against the merged 0369 → 0371 → 0372 cutover and fail closed unless every
  shape-derived facade/runtime candidate
  is classified and no maintained executable caller remains.
- Classify each substantive legacy assertion before deletion; move surviving behavior to
  mutation-sensitive Go coverage or the retained POSIX owner for repository-root `install.sh` or
  the release downloader.
- Delete the facade, production helpers/runtime, legacy runner, compatibility launchers,
  environment/configuration seams, and mechanism-only tests.
- Contract `docket development test` to Go plus exactly the two retained POSIX product suites while
  preserving source fidelity, isolation, completeness, interruption, aggregation, budgets, and
  ADR-0074 semantics.
- Add shape-derived, mutation-tested final absence guards and complete the facade-era ADR/index
  consequences through the ADR workflow.

## Out of scope

Retained consumer migration (0369), native dispatch migration (0371), deferred-feature retirement
and the final consumer seal (0372); a replacement shim or shell control plane; unrelated configuration
redesign; rewrites of historical records, archived specs/results, Accepted ADR history, or frozen
v0.9.2 artifacts; release/tag/assets; and human fresh-host or rollback work (0366).

## Design decisions

Deletion follows coverage replacement, never precedes it. Unknown consumers and uncertain test
assertions block removal. Final guards classify executable shape and ownership rather than pinning
current spellings, counts, or filenames. A small missed caller consistent with the merged cutover
may be reconciled; a material migration redesign halts for regrooming.
