---
id: 386
slug: 'discharge-adr-0100-s-deferred-disposition-for-the-surviving'
title: 'Discharge ADR-0100''s deferred disposition for the surviving product runners'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: []
discovered_from: [370]
adrs: [100, 67, 79, 80, 87, 88]
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
| ADRs | [ADR-0100](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0100-native-host-dispatch-is-authoritative-for-registered-docket.md), [ADR-0067](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0067-runner-bearing-agent-requires-a-user-configured-model.md), [ADR-0079](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0079-shim-wrapper-frontmatter-pin-governs-the-parent-side-agent.md), [ADR-0080](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0080-detached-delegation-execution-posture-launch-then-observe.md), [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0088](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0088-halt-exit-code-is-a-property-of-run-state-not-discovery-path.md) |
<!-- docket:artifacts:end -->

## Why

Change 0370 deleted the Bash facade but retained scripts/runners/{codex,cursor,opencode} as declared POSIX product surface. ADR-0100 deferred the disposition of the ADRs that govern those runners, so ADRs 0067/0079/0080/0087/0088 remain Accepted and undispositioned even though the surrounding facade was retired. The ADR ledger currently overstates what is still in force.

## What changes

Review ADR-0100's deferred disposition against the post-0370 reality (runners survive, facade gone) and update the status/consequences of ADRs 0067/0079/0080/0087/0088 accordingly — deprecating, keeping, or annotating each with a dated note — then regenerate and validate the ADR index.

## Out of scope

Deleting or rewriting the runner scripts themselves; any code change to runner behavior.
