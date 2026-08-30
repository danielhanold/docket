---
id: 371
slug: 'cut-generated-agent-invocation-over-to-native-host-dispatch'
title: 'Cut generated agent invocation over to native host dispatch'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-30'
updated: '2026-08-30'
depends_on: [369]
stacked_on:
related: [311, 317, 318, 370, 366]
discovered_from: [369]
adrs: [36, 74]
spec: 'docs/superpowers/specs/2026-08-30-cut-generated-agent-invocation-over-to-native-host-dispatch-design.md'
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
| Spec | [2026-08-30-cut-generated-agent-invocation-over-to-native-host-dispatch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-cut-generated-agent-invocation-over-to-native-host-dispatch-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md) |
<!-- docket:artifacts:end -->

## Why

The Go v1 architecture deliberately relies on each supported harness's native named-agent dispatch rather than a Docket runner-dispatch verb. Maintained generated agents and dispatch instructions still contain Bash-era delegation assumptions, blocking the consumer cutover and facade deletion.

## What changes

- Define one canonical native-dispatch policy and render it through the existing Claude, Codex,
  Cursor, and OpenCode adapters.
- Preserve exact same-name agent resolution, unchanged request forwarding, and caller-side
  implement-next gate ownership.
- Regenerate all owned dispatch artifacts deterministically and prove fresh isolated installation,
  immediate/detached completion, missing-agent failure, and mutation coverage.
- Remove maintained `runner-dispatch` calls from the native-dispatch surface without adding a Go
  delegation operation or another compatibility path.

## Out of scope

Lifecycle-operation migration owned by change 369; deferred auto-capture, learning-index, and terminal-publish retirement; physical Bash facade deletion; release and self-host acceptance; any new cross-harness runner or delegation verb.

## Design decisions

Host-native dispatch owns child creation, Docket's caller-side gate owns attribution and retry
authority, and the registered agent owns its workflow contract. Host status and child prose never
supersede the gate verdict. Missing native registration fails visibly with no shell, cross-harness,
generic-agent, or silent-inline fallback. Reconciliation halts if the existing shared generator seam
cannot support the four adapters as one bounded cutover.
