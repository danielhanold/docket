---
id: 369
slug: 'migrate-maintained-consumers-to-the-direct-go-cli'
title: 'Migrate retained lifecycle consumers to typed Go operations'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-29'
updated: '2026-08-30'
depends_on: [318]
stacked_on:
related: [317, 322, 326, 361, 366, 367, 370, 371, 372]
discovered_from: [318]
adrs: [36, 74, 99]
spec: 'docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md'
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
| Spec | [2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

The Go v1 cutover should migrate only behavior already supported by its public typed operations.
Treating intentionally deferred Bash capabilities as missing Go verbs would expand v1 and keep this
change too large to implement autonomously.

## What changes

- Derive and classify the maintained lifecycle consumer inventory by behavioral shape.
- Migrate planning, maintenance, implementation, and metadata-only finalize consumers only where
  an exact public Go operation already exists.
- Remove caller-owned follow-up work already performed atomically by those operations, including
  proven redundant ADR-index rendering.
- Update canonical executable instructions, regenerate maintained copies deterministically, and
  add a stage-local mutation-tested guard for the migrated surface.
- Leave native agent dispatch, deferred features, the final global seal, and Bash deletion to their
  explicit dependent changes.

## Out of scope

New Go verbs or shims; native dispatch (0371); automatic capture, learning automation, terminal
publication, and the final consumer seal (0372); physical Bash deletion (0370); release and
self-host acceptance (0366); post-cutover board work (0367); and historical-record rewrites.

## Design decisions

Consumer behavior is classified before editing: existing typed Go operation, native host dispatch,
intentionally deferred capability, transaction-absorbed behavior, historical/non-executable, or
unresolved. Only the first and fourth classes move here. Reconciliation halts if an in-scope caller
needs a new operation or substantial bespoke adapter. This is a sequential merged-main dependency,
not a stacked branch.
