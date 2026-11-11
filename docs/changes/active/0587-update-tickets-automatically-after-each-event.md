---
id: 587
slug: 'update-tickets-automatically-after-each-event'
title: 'Update tickets automatically after each event'
status: 'in-progress'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-11'
depends_on: [585]
stacked_on:
related: [581, 588, 592]
discovered_from: [581]
adrs: [44, 162]
spec: 'docs/superpowers/specs/2026-11-11-update-tickets-automatically-after-each-event-design.md'
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch: 'feat/update-tickets-automatically-after-each-event'
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-11-update-tickets-automatically-after-each-event-design.md](../../superpowers/specs/2026-11-11-update-tickets-automatically-after-each-event-design.md) |
| ADRs | [ADR-0044](../../adrs/0044-autonomy-precedence-call-site-pre-specification.md), [ADR-0162](../../adrs/0162-ticket-work-is-two-inline-skills-split-by-attended-linking-v.md) |
<!-- docket:artifacts:end -->

## Why

Teams that want it should get ticket updates without the engineer remembering to ask, and nothing at all when they do not. Today a change's ticket hears about a spec, an opened PR, a merge, or finished results only when someone asks for that update.

## What changes

Four config switches, each off by default, get their effect: `tickets.auto.spec_written`, `tickets.auto.pr_opened`, `tickets.auto.pr_merged`, and `tickets.auto.results_completed`. When one is on and the change already has a ticket, the workflow that just finished that step posts the update as a comment, without asking:

- docket-groom-next and docket-auto-groom, after a change's first spec lands;
- docket-implement-next, after the PR opens, or after a results-only change completes;
- docket-finalize-change, after a merged change is closed out.

When the switch is off, or the change has no ticket, nothing happens: no prompt and no report line. A missing connector or a failed post costs one report line and never stops or fails the run. In a private repository an automatic update posts nothing and reports one line; change 588 lifts that. The maintenance sweep never posts, and when a stack root lands only the root's ticket is updated.

Ticket creation stays on request. The `tickets.auto.change_created` switch planned by change 581 is removed. This was decided at grooming and departs from 581's plan, for two reasons:

- Creating a ticket is an action for a person, not for an agent, so automating it is usually the wrong approach.
- There is no clear place for the trigger: a captured stub is not ready to be a ticket, and a groomed change with a full spec has its own problems. Supporting both was judged too complex for what it saves.

A new concepts page, `docs/concepts/tickets.md`, explains how ticket work fits together: the link, the three ways a change gets a ticket, on-request and automatic updates, the four events, what goes with an update, the connector, and the settings. It also documents the two reasons above in detail. Today the user docs cover ticket work only in the reference tier. The config-key reference and the glossary each point to the page.

No new operation is needed; the existing ticket read already returns the switch and the ticket state.

## Acceptance criteria

- With a switch on and a ticket linked, each of the five wiring sites directs the update skill to post that event's update; a test pins those five sites and no others.
- With every switch off, no workflow posts, asks, or reports anything about tickets.
- With a switch on and no ticket, nothing is posted and nothing is reported.
- In a private repository an automatic update posts nothing and reports one line.
- A missing connector or failed post is one report line; the run's outcome is unchanged.
- `tickets.auto.change_created` no longer exists in the schema, the sample config, or the docs.
- The docs have a concepts page on tickets, listed in both docs indexes. It explains how ticket work fits together, and every statement on it matches the skills and the binary as built.
- That page explains in detail why docket never creates a ticket by itself: creating a ticket is a person's action, and there is no one right moment to create it.
- The results list an Important live check, done by a person from a shared scratch repository, never from this one.

## Out of scope

Automatic ticket creation, at change creation, at grooming, or anywhere else; automatic updates in a private repository (change 588); confirmation prompts, queues, and retries; automatic updates for revisions, blockers, kills, or changes the sweep closed out; updates to stacked descendants' tickets.

## Reconcile log

### 2026-11-11

Unchanged against `main` 4874f44ea (the spec was traced at that same commit); dependency #585 is done, and `change_created` still appears at the sites the spec names.
