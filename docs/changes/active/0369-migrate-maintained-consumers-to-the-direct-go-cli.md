---
id: 369
slug: 'migrate-maintained-consumers-to-the-direct-go-cli'
title: 'Migrate maintained consumers to the direct Go CLI'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-29'
updated: '2026-08-29'
depends_on: [318]
stacked_on:
related: [317, 322, 326, 361, 366, 367]
discovered_from: [318]
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

The Go command surface cannot become the only supported control plane while skills, agents, generators, workflows, setup checks, and operator instructions still route through the legacy Bash facade. Migrating these maintained consumers is a repo-wide but reviewable boundary that must land before deletion.

## What changes

Rewrite every maintained executable consumer and its generator to invoke the PATH-resolved docket Go CLI directly. Leave the existing Bash facade frozen and operational for compatibility during this intermediate state, prove no maintained caller remains, and prevent new facade callers from being introduced.

## Out of scope

Do not delete the Bash facade, helper/runtime tree, or legacy mechanism tests. Do not publish v1.0.0-rc1 or perform fresh-host self-host acceptance. Do not introduce a replacement forwarding shim.
