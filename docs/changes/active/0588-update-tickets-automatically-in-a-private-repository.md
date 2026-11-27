---
id: 588
slug: 'update-tickets-automatically-in-a-private-repository'
title: 'Update tickets automatically in a private repository'
status: 'in-progress'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-27'
depends_on: [586, 587]
stacked_on:
related: [581]
discovered_from: [581]
adrs: [44, 144, 162, 163, 165]
spec: 'docs/superpowers/specs/2026-11-27-update-tickets-automatically-in-a-private-repository-design.md'
plan: 'docs/superpowers/plans/2026-11-26-update-tickets-automatically-in-a-private-repository.md'
results:
auto_groomable:
hold:
branch_prefix:
branch: 'feat/update-tickets-automatically-in-a-private-repository'
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-27-update-tickets-automatically-in-a-private-repository-design.md](../../superpowers/specs/2026-11-27-update-tickets-automatically-in-a-private-repository-design.md) |
| Plan | [2026-11-26-update-tickets-automatically-in-a-private-repository.md](../../superpowers/plans/2026-11-26-update-tickets-automatically-in-a-private-repository.md) |
| ADRs | [ADR-0044](../../adrs/0044-autonomy-precedence-call-site-pre-specification.md), [ADR-0144](../../adrs/0144-private-visibility-leak-check-blocks-outgoing-pushes-and-pr.md), [ADR-0162](../../adrs/0162-ticket-work-is-two-inline-skills-split-by-attended-linking-v.md), [ADR-0163](../../adrs/0163-automatic-ticket-updates-are-per-event-switches-ticket-creat.md), [ADR-0165](../../adrs/0165-unattended-private-ticket-updates-post-only-on-a-clean-final.md) |
<!-- docket:artifacts:end -->

## Why

A private repository's automatic updates must go out without a person reviewing the cleanup, so something else has to stop a leak.

## What changes

An automatic update in a private repository is posted instead of skipped. It attaches cleaned copies of the spec and results file the way an on-request update does, but nobody reviews the cleanup, so a final check runs the publish leak check's own patterns over the comment and every attachment before anything is uploaded. Any hit, a `docket-export` word-check hit, or a check that cannot run posts nothing and writes one report line; the run itself is untouched.

A new read-only operation, `docket leak check --file …`, exposes the leak matcher for this, with the same options the publish check uses. The rule that an automatic update in a private repository posts nothing is removed from `docket-update-ticket`, the tickets concepts page, the reference docs, and their contract tests, and `leak_check.match_word` gets its missing row in the config keys reference.

## Acceptance criteria

- `docket leak check` reports the same hits as the publish leak check for the same text, with tests; a file it cannot read is refused, never reported clean.
- `docket-update-ticket` posts an automatic private update only on a clean final check, and its text states the post-nothing rule for both checks (a `docket-export` word-check hit, and any final-check result other than clean).
- No doc, skill, or contract row still says an automatic update in a private repository posts nothing.
- The results list an Important live check from a private scratch repository.

## Out of scope

- Widening the leak patterns.
- Running the final check on on-request updates, or changing `docket-export`.

## Reconcile log

### 2026-11-27

Unchanged against main `fb655faed`: dependencies #586 and #587 are done, the spec's traced anchors (`leakOptions`, `runLeakCheck`, `leakCheckApplies`, automatic.md's private clause) still hold, and scope stands.
