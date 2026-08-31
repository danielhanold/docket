---
id: 384
slug: 'launch-compositional-docket-agents-in-coordinator-capable-ha'
title: 'Launch compositional Docket agents in coordinator-capable harness contexts'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: [359, 364]
discovered_from: [365]
adrs: [36, 59, 60, 94]
spec: 'docs/superpowers/specs/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/launch-compositional-docket-agents-in-coordinator-capable-ha'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-31T16:14:19Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-31-launch-compositional-docket-agents-in-coordinator-capable-ha-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md) |
<!-- docket:artifacts:end -->

## Why

Change 0365 taught generated Codex wrappers to ask for nested dispatch, but its mandatory fresh-process certification was left unchecked. The first real test was the implement-next run for change 0364: the root launched the registered docket-implement-next agent, but that session had no collaboration control with which to launch docket-plan-writer, so it halted before planning. Codex documents nested agents and the root exposes the registered Docket roles, making Docket's launch method or context the defect to resolve.

## What changes

Separate workflow semantics from harness launch mechanics. Prototype the supported Codex entry paths, identify a native coordinator-capable launch that preserves nested named-agent controls, encode entry and child invocation through the harness adapter and generated surfaces, and certify root to Docket coordinator to registered child in a fresh process. Keep caller payload and return contracts and child role bodies harness-neutral; use generated per-agent metadata only where a harness must distinguish coordinator and leaf launches.

## Out of scope

Changing existing agent topology, payload or return protocols, model or effort pins, worktree scopes, Tier-C authorization, or skill bindings; adding a parent-relay, generic-agent, shell-runner, or cross-harness fallback unless every accessible native coordinator launch is conclusively unavailable; and the separate run-gate continuation work tracked by change 0359.
