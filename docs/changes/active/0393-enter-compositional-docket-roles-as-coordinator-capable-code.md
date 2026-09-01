---
id: 393
slug: 'enter-compositional-docket-roles-as-coordinator-capable-code'
title: 'Enter compositional Docket roles as coordinator-capable Codex root threads'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-01'
updated: '2026-09-01'
depends_on: []
stacked_on:
related: [364, 365, 384]
discovered_from: [384]
adrs: [36, 59, 60, 94]
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
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md) |
<!-- docket:artifacts:end -->

## Why

Change 0384 was intended to restore nested Docket composition in Codex, but its generated-wrapper change did not alter the launch topology. The actual registered docket-implement-next path still starts as a depth-1 agent without top-level collaboration controls, so it cannot dispatch docket-plan-writer and change 0364 has halted at Step 4 for the second time. Sequential live spikes isolated the boundary: a generic child can spawn a grandchild, the exact registered docket-implement-next child still cannot, and the same role contract succeeds when entered as a coordinator-capable root thread. This follow-up applies that empirically selected root-entry design to the production Codex path.

## What changes

Add a Codex harness-native coordinator root-entry path for Docket roles that own nested dispatch. Seed the new root thread from the same installed role definition used by registered dispatch, preserving developer instructions, model, reasoning effort, skill preload, recursion guard, request payload, working directory, permissions, foreground completion, and return contract without hand-duplicating wrapper prose. Route the VS Code-backed docket-implement-next entry through that path and prove the real docket-implement-next to docket-plan-writer edge in a fresh process, alongside focused adapter tests, a mutation that restores the old depth-1 launch and fails, and the repository's full build gate.

## Out of scope

A generic process requirement for verification that could not have run before merge; granting broad collaboration controls to every spawned agent; a typed parent-relay protocol or approved-relay fallback; generic-agent, shell-runner, subprocess-session, or cross-harness substitutes; changing Docket's role topology, child payloads or receipts, Tier-C authorization, model or effort pins, skill bindings, worktree scopes, or the separate run-gate continuation behavior tracked by change 0359.
