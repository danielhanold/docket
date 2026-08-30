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
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md) |
<!-- docket:artifacts:end -->

## Why

The Go v1 architecture deliberately relies on each supported harness's native named-agent dispatch rather than a Docket runner-dispatch verb. Maintained generated agents and dispatch instructions still contain Bash-era delegation assumptions, blocking the consumer cutover and facade deletion.

## What changes

In one reviewable PR, migrate the canonical Claude, Codex, Cursor, and OpenCode agent-generation surfaces and their generated dispatch blocks to native host dispatch. Preserve the caller-side Docket run-gate protocol, make regeneration deterministic, and cover fresh hermetic installation and invocation behavior. Remove maintained runner-dispatch callers without implementing cross-harness delegation.

## Out of scope

Lifecycle-operation migration owned by change 369; deferred auto-capture, learning-index, and terminal-publish retirement; physical Bash facade deletion; release and self-host acceptance; any new cross-harness runner or delegation verb.
