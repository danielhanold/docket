---
id: 365
slug: codex-nested-dispatch-capability-boundary
title: 'Make nested Docket dispatch reliable for every Codex agent invocation'
status: 'in-progress'
priority: critical
type: fix
created: '2026-08-29'
updated: '2026-08-29'
depends_on: []
stacked_on:
related: [353, 359]
discovered_from: [361]
adrs: [36, 59, 60, 94]
spec: docs/superpowers/specs/2026-08-29-codex-nested-dispatch-capability-boundary-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'fix/codex-nested-dispatch-capability-boundary'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-29T13:48:09Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-29-codex-nested-dispatch-capability-boundary-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-codex-nested-dispatch-capability-boundary-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0094](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0094-plan-authoring-is-a-pinned-internal-composition-agent.md) |
<!-- docket:artifacts:end -->

## Why

The Go installer emits and registers `docket-plan-writer`, but a live Codex implement-next run
halted before planning because the parent inspected a nested JavaScript tool inventory, did not see
Codex's top-level collaboration controls there, and falsely declared native dispatch unavailable.
No plan-writer dispatch was attempted or rejected. The same false capability inference can affect
every Docket skill that composes another agent.

This blocks both user-facing entry paths Docket currently supports: prose routed through the
managed repository dispatch block and direct `@docket-…` agent invocation. The earlier change 0353
treated raw named-agent invocation as operator error, but current Docket routing deliberately uses
that path, so all registered Codex agents must carry a correct nested-dispatch contract.

## What changes

Teach every generated Codex agent to use direct named-agent dispatch from its active top-level tool
surface and to reject nested orchestration inventories as evidence of unavailability. Strengthen
the shared capability-resolution rule by shape, preserve the existing tiered postures for genuine
dispatch rejection, and cover every current composition family through inventory-derived tests and
live fresh-process validation of both prose and direct `@agent` invocation. Update Codex setup and
validation documentation, including the requirement to restart the Codex process after wrapper
installation.

## Out of scope

Runner/subprocess fallbacks for clients that genuinely reject nested dispatch; changing any
model/effort pin, agent topology, return protocol, tier assignment, or worktree scope; authorizing
implicit inline execution; adding skill wrappers for agent-only workers; and the separate run-gate
continuation work tracked by change 0359.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
