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
related: [322, 326, 361, 366, 369, 370]
discovered_from: [303]
adrs: [74]
spec: docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'refactor/config-contraction-self-hosting-and-hard-cutover'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-29T17:34:05Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md) |
| ADRs | [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md) |
<!-- docket:artifacts:end -->

## Why

Docket's Go lifecycle is complete, but the repository's canonical whole-suite orchestration still
lives in the Bash runner that the cutover will eventually retire. Replacing the runner first gives
the later consumer migration and deletion changes a Go-owned, branch-faithful build gate while
leaving the frozen prior workflow available as a parity oracle and migration host.

## What changes

- Add `docket development test` as the Go-native whole-suite implementation, entered through a
  branch-faithful source bootstrap rather than an installed binary.
- Cut `finalize.test_command`, the contributor whole-suite command, and release-candidate source
  validation over to that one canonical runner.
- Continue executing the complete existing Go and Bash test corpus while the facade, helpers,
  runtime, callers, and old runner remain present and green.
- Preserve isolation, safe parallelism, exact durable-result accounting, deterministic aggregation,
  interruption behavior, screen-then-serial-confirm budgets, and ADR-0074 gate semantics.
- Prove the new orchestration contract through differential parity, deterministic synthetic
  fixtures, and mutation-sensitive tests.

## Out of scope

Maintained-consumer migration (0369); facade/runtime/configuration/test deletion (0370); broad
documentation or generated-asset cutover; a replacement forwarding shim; whole-backlog ledger
disposition; release packaging or publication; native target smokes; fresh Claude/Codex/Cursor/
OpenCode lifecycle proof; v0.9.2 rollback; and post-cutover board configuration (0367).

## Design decisions

Change 0318 keeps its id, slug, recorded branch, and claim continuity but becomes only the runner
stage of a sequential merged-main dependency chain. The exact checkout under review is tested; the
existing Bash implementation remains frozen and green as the parity oracle. Each intermediate
merge is independently usable. Changes 0369 and 0370 own consumer migration and physical deletion,
and 0366 owns external human and release truth.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

