---
id: 426
slug: 'constrain-agent-enter-to-explicit-legacy-and-feature-worktre'
title: 'Constrain agent.enter to explicit legacy and feature-worktree use'
status: 'proposed'
priority: 'high'
type: 'refactor'
created: '2026-09-11'
updated: '2026-09-11'
depends_on: [425]
stacked_on:
related: [393, 412]
discovered_from: [423]
adrs: [114]
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
| ADRs | [ADR-0114](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0114-anchor-codex-feature-scoped-role-entry-to-the-owning-worktre.md) |
<!-- docket:artifacts:end -->

## Why

After native Multi-Agent V2 coordinator dispatch becomes the supported production route, leaving root-level agent.enter as an implicit equivalent path would preserve two competing launch architectures and could continue masking invalid V1 coordinator assignments. At the same time, app-server entry still has distinct value as an explicit compatibility and diagnostic mechanism, and its feature-worktree mode enforces a safety boundary that native child dispatch does not currently provide. Those purposes need to be separated and constrained instead of deleting the entire operation with the coordinator workaround.

## What changes

Remove root-coordinator agent.enter from every generated or automatic routing path and retain it only as an explicitly invoked legacy compatibility/diagnostic facility. Emit a clear deprecation and provenance notice when root-coordinator mode is selected, document that it cannot make a known V1 coordinator assignment valid, and ensure repository capability validation is applied independently of launch route. Keep agent.enter --worktree as a supported feature-scoped entry mode with its existing exact-worktree verification and foreground lifecycle. Split tests and documentation for root compatibility entry from feature-worktree safety so either capability can later evolve or be removed independently. Define observable retirement criteria for the legacy root mode—covering supported Codex releases, successful native certification, and absence of unresolved host regressions—and identify the remaining app-server surface, schema, guards, and documentation that can be deleted when those criteria are met. Reconcile terminology and architecture records so agent.enter is no longer presented as the default way to make Docket composition work.

## Out of scope

Removing feature-worktree entry before native Codex can enforce the same working-directory boundary, allowing V1 coordinators as a supported fallback, changing native coordinator routing delivered by the dependency, removing the run gate, completing change 0412, or immediately deleting the app-server client and agent.enter command before the stated retirement criteria have been observed.
