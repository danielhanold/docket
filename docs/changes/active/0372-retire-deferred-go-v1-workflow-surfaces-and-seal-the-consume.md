---
id: 372
slug: 'retire-deferred-go-v1-workflow-surfaces-and-seal-the-consume'
title: 'Retire deferred Go v1 workflow surfaces and seal the consumer cutover'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-30'
updated: '2026-08-30'
depends_on: [371]
stacked_on:
related: [312, 316, 326, 369, 370]
discovered_from: [369]
adrs: [14, 29, 30, 33, 36, 74, 99]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Several Bash-era workflow legs remain active even though their Go v1 capabilities were explicitly deferred: automatic stub capture, automated learnings-index maintenance, and terminal publishing. Treating them as missing Go verbs would silently expand v1 and keep the Bash facade alive indefinitely.

## What changes

In one reviewable PR after native dispatch lands, remove active callers, hooks, generated instructions, and tests for the explicitly deferred workflow legs while preserving configuration compatibility, historical records, and fail-closed diagnostics. Replace enumerated caller checks with shape-derived, mutation-tested guards that seal the maintained consumer cutover. Record the final architectural disposition of the superseded facade-era decisions.

## Out of scope

Retained lifecycle-operation migration owned by change 369; native host dispatch cutover owned by change 371; implementation of the deferred capabilities; physical Bash facade deletion; release, rollback, or four-host self-host acceptance.
