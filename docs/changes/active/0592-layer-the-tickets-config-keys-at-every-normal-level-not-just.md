---
id: 592
slug: 'layer-the-tickets-config-keys-at-every-normal-level-not-just'
title: 'Layer the tickets.* config keys at every normal level, not just the repo'
status: 'implemented'
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
spec: 'docs/superpowers/specs/2026-11-11-layer-the-tickets-config-keys-at-every-normal-level-not-just-design.md'
plan: 'docs/superpowers/plans/2026-11-10-layer-the-tickets-config-keys-at-every-normal-level-not-just.md'
results: 'docs/results/2026-11-10-layer-the-tickets-config-keys-at-every-normal-level-not-just-results.md'
auto_groomable:
hold:
branch_prefix:
branch: 'refactor/layer-the-tickets-config-keys-at-every-normal-level-not-just'
pr: 'https://github.com/danielhanold/docket/pull/456'
ticket_provider:
ticket_id:
ticket_url:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-11-layer-the-tickets-config-keys-at-every-normal-level-not-just-design.md](../../superpowers/specs/2026-11-11-layer-the-tickets-config-keys-at-every-normal-level-not-just-design.md) |
| Plan | [2026-11-10-layer-the-tickets-config-keys-at-every-normal-level-not-just.md](../../superpowers/plans/2026-11-10-layer-the-tickets-config-keys-at-every-normal-level-not-just.md) |
| Results | [2026-11-10-layer-the-tickets-config-keys-at-every-normal-level-not-just-results.md](../../results/2026-11-10-layer-the-tickets-config-keys-at-every-normal-level-not-just-results.md) |
| ADRs | [ADR-0019](../../adrs/0019-global-config-fence-classification.md), [ADR-0161](../../adrs/0161-ticket-fields-are-three-flat-stored-scalars-and-issue-is-ret.md), [ADR-0162](../../adrs/0162-ticket-work-is-two-inline-skills-split-by-attended-linking-v.md) |
<!-- docket:artifacts:end -->

## Why

The ticket settings (tickets.provider, tickets.site, tickets.project and the five tickets.auto.* switches) are declared repo-only, so a value in the global config or .docket.local.yml is warned and ignored. Provider and site are usually the same for every repo a person works in, and a person may want to try the auto switches on one machine without committing them. Other keys get normal repo-local > repo-committed > global > built-in layering. This change makes the argument for treating the tickets keys the same way, and if it holds, makes the change.

## What changes

Move the eight repo-only ticket settings (tickets.provider, tickets.site, tickets.project and the five tickets.auto.* switches) to the same any-layer scope as ordinary keys and tickets.companion_skill, so a person can set them once in the global config or try them on one machine in .docket.local.yml. Document how a repository cancels a global value: write the default explicitly (`""` for a string, `false` for a switch) in .docket.yml or .docket.local.yml. When a bare empty value (a YAML null) is rejected on a string key that accepts empty, the error names the `""` fix. Update the config reference, the .docket.example.yml comment above the tickets block (which still says no docket command reads these settings), and any ticket-skill prose that says the values come only from the repository config, with tests for each layer.

## Acceptance criteria

A value for each of the eight keys set in the global config or .docket.local.yml is honored with the usual per-field precedence, and diagnostic.config reports the layer it came from. Writing `""` or `false` for a key in .docket.yml or .docket.local.yml cancels a global value, and that is documented in the config reference and .docket.example.yml. A bare empty value on a string key that accepts empty is still rejected, and the message says to write `""`. Private-repository behavior for these keys is stated in the spec and covered by a test. Neither the config reference nor .docket.example.yml claims any longer that no docket command reads these settings.

## Out of scope

Changing the repository-identity keys (integration_branch, changes_dir, adrs_dir, results_dir), which stay repo-only. Any new ticket setting or provider support. Reading ticket settings from anywhere other than the existing config layers.

## Reconcile log

### 2026-11-11

Unchanged against main b3bc2ae69. Trace step 1 re-run: readers of eff.Tickets are still only internal/app/config.go (diagnostic.config) and internal/app/ticket_context.go (auto switches); none needs cross-clone agreement.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: f4b62758b214f21eb9de4476dba27af91cff9b6c
ran_at:   2026-11-11T02:42:41Z
<!-- docket:build-evidence:end -->
