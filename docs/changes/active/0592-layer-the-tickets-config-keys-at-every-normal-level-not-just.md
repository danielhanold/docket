---
id: 592
slug: 'layer-the-tickets-config-keys-at-every-normal-level-not-just'
title: 'Layer the tickets.* config keys at every normal level, not just the repo'
status: 'proposed'
priority: 'medium'
type: 'refactor'
deliverable: 'pr'
created: '2026-11-11'
updated: '2026-11-11'
depends_on: []
stacked_on:
related: []
discovered_from: []
adrs: [19, 161, 162]
spec:
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
ticket_provider:
ticket_id:
ticket_url:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0019](../../adrs/0019-global-config-fence-classification.md), [ADR-0161](../../adrs/0161-ticket-fields-are-three-flat-stored-scalars-and-issue-is-ret.md), [ADR-0162](../../adrs/0162-ticket-work-is-two-inline-skills-split-by-attended-linking-v.md) |
<!-- docket:artifacts:end -->

## Why

The ticket settings (tickets.provider, tickets.site, tickets.project and the five tickets.auto.* switches) are declared repo-only, so a value in the global config or .docket.local.yml is warned and ignored. Provider and site are usually the same for every repo a person works in, and a person may want to try the auto switches on one machine without committing them. Other keys get normal repo-local > repo-committed > global > built-in layering. This change makes the argument for treating the tickets keys the same way, and if it holds, makes the change.

## What changes

Trace why the tickets.* keys were made repo-only and whether that reason still holds. If the argument for normal layering holds, change all eight keys in the config schema to the same scope as ordinary keys, update the config docs and the docket-link-ticket / docket-update-ticket skill prose that says where the values come from, and add tests for each layer. If it does not hold, record the reasons in the spec and leave the keys repo-only.

## Acceptance criteria

A value for each tickets.* key set in the global config or .docket.local.yml is honored with the usual per-field precedence, and diagnostic.config reports the layer it came from. Private-repository behavior for these keys is stated in the spec and covered by a test.

## Out of scope

Changing the repository-identity keys (integration_branch, changes_dir, adrs_dir, results_dir), which stay repo-only. Any new ticket setting or provider support. Reading ticket settings from anywhere other than the existing config layers.
