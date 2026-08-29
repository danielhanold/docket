---
id: 370
slug: 'delete-the-frozen-bash-facade-and-legacy-test-surface'
title: 'Delete the frozen Bash facade and legacy test surface'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-29'
updated: '2026-08-29'
depends_on: [369]
stacked_on:
related: [318, 322, 326, 361, 366, 367]
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

Once every maintained consumer uses the Go CLI directly, the legacy Bash facade and its helper/runtime tree become duplicate production machinery. Keeping them would preserve two control planes, two test architectures, and the configuration seams the v1 cutover is intended to retire.

## What changes

Re-prove that the frozen facade has no maintained executable callers, delete the facade and helper/runtime tree, remove mechanism-only tests and obsolete environment/configuration seams, migrate surviving behavioral invariants to mutation-sensitive Go or retained POSIX coverage, and contract the canonical test runner to the final Go-plus-bootstrap product set.

## Out of scope

Do not migrate maintained callers; that is change 369. Do not rewrite historical records, archived specs/results, accepted ADR history, or frozen v0.9.2 artifacts. Do not publish v1.0.0-rc1 or run the human fresh-host and rollback protocol.
