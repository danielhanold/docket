---
id: 581
slug: 'split-the-ticket-integration-design-into-buildable-changes'
title: 'Split the ticket integration design into buildable changes'
status: 'proposed'
priority: 'medium'
type: 'chore'
deliverable: 'results-artifact'
created: '2026-11-09'
updated: '2026-11-09'
depends_on: []
stacked_on:
related: [561, 578]
discovered_from: []
adrs: [7, 12, 19, 44, 71, 141, 144]
spec:
plan:
results:
auto_groomable:
hold:
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
| ADRs | [ADR-0007](../../adrs/0007-github-board-mirror-boundary.md), [ADR-0012](../../adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0019](../../adrs/0019-global-config-fence-classification.md), [ADR-0044](../../adrs/0044-autonomy-precedence-call-site-pre-specification.md), [ADR-0071](../../adrs/0071-writer-guarantees-yaml-validity-by-construction.md), [ADR-0141](../../adrs/0141-build-artifacts-plan-results-evidence-live-on-the-metadata-b.md), [ADR-0144](../../adrs/0144-private-visibility-leak-check-blocks-outgoing-pushes-and-pr.md) |
<!-- docket:artifacts:end -->

## Why

Docket should let an engineer share progress with people who work in a ticket system such as Jira, without those people learning docket or reading an agent's working history: a clear request when a ticket is created, and short updates at the points that matter, with the detail left in docket's artifacts.

An earlier draft of this integration (written in another tool on 2026-11-07) left five groups of questions open, had not been checked against the code, and did not say how to break a feature this size into changes that can be built one at a time. The linked spec is the re-brainstormed final design: every open question answered, the code facts re-traced, and the vocabulary settled (ticket, event, update).

The feature is too large for one PR, and each piece will be groomed by hand just in time, after the pieces it depends on are built. That needs a plan that cuts it into the right changes, in the right order, before any of them is created.

## What changes

A results artifact holding the project plan for the ticket integration described in the linked spec:

- an ordered list of proposed changes, each roughly one PR, each with its title, type, the spec decisions it covers, its dependencies, what it touches, a rough size, and ready-to-use stub text (Why, What changes, Acceptance criteria, Out of scope);
- a coverage check mapping every spec decision and exclusion to exactly one change;
- the build order, and which changes are independent;
- which decisions warrant an ADR, and which change records each one;
- trace notes on anything in the current code that contradicts or refines the spec.

The run re-traces the code the spec names before splitting, and resolves the three trace items the spec lists (what produces `stacked-merged`, how docket-export's private review stop is skipped for automatic updates, and the shape of the read operation).

## Out of scope

Writing any code, committing to any branch, or creating the planned changes: the human creates them from the plan and grooms each one by hand. Re-opening decisions the spec settles. Specs for the planned changes. The two unrelated findings the spec lists at its end (a stale convention sentence and the disagreement about what produces `stacked-merged`), apart from the trace item the plan itself needs.
