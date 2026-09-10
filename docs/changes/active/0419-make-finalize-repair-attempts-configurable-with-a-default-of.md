---
id: 419
slug: 'make-finalize-repair-attempts-configurable-with-a-default-of'
title: 'Make finalize repair attempts configurable with a default of six'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-09-10'
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [349]
discovered_from: [349]
adrs: [10, 19]
spec:
plan:
results:
trivial: true
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
| ADRs | [ADR-0010](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0010-finalize-merge-gate-split-agents.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md) |
<!-- docket:artifacts:end -->

## Why

Finalize's integration-repair agent and finalize workflow currently impose a fixed two-attempt repair limit. This stops repair too soon when a failing post-rebase suite needs more iterations. Change 0349 made the separate rebase-resolver budget configurable but explicitly left this repair limit unchanged.

## What changes

- Add `finalize.repair_max_attempts` as a positive integer with a built-in default of 6, following the existing `finalize.resolver_max_attempts` configuration pattern and normal global, repository, and machine-local precedence.
- Expose the resolved value through typed finalize context and effective-configuration diagnostics, and pass it to the integration-repair agent so the actual repair workflow uses the configured cap instead of the hardcoded two-attempt limit.
- Count the initial repair attempt toward the maximum; stop early on success and use the existing stuck/halted path when the configured repair budget is exhausted. Keep repair attempts separate from conflict-resolver dispatches.
- Update maintained finalize skill, failure-reference, repair-agent, configuration documentation, and generated assets consistently. Record the replacement of ADR-0010's fixed repair cap without rewriting its accepted historical text.
- Verify the default of 6, custom limits including 1 and values greater than 2, invalid values, configuration precedence, context/dispatch propagation, and exhaustion behavior. Run the configured whole-suite build gate.

Trivial rationale: this is a bounded extension of the existing finalize configuration pattern. The user has settled the behavior and default; retain the existing integration-repair lifecycle and substitute the resolved positive-integer cap for its fixed two-attempt bound. No new retry architecture is required.

## Out of scope

Changing `finalize.resolver_max_attempts`, unlimited retries, broader retry or persistence infrastructure, changes to repair sign-off or merge policy, and weakening tests to reach green.
