---
id: 584
slug: 'create-a-ticket-from-a-change-or-a-change-from-a-ticket'
title: 'Create a ticket from a change, or a change from a ticket'
status: 'in-progress'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-10'
depends_on: [582, 583]
stacked_on:
related: [581, 578, 585]
discovered_from: [581]
adrs: [44, 161]
spec: 'docs/superpowers/specs/2026-11-10-create-a-ticket-from-a-change-or-a-change-from-a-ticket-design.md'
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch: 'feat/create-a-ticket-from-a-change-or-a-change-from-a-ticket'
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-10-create-a-ticket-from-a-change-or-a-change-from-a-ticket-design.md](../../superpowers/specs/2026-11-10-create-a-ticket-from-a-change-or-a-change-from-a-ticket-design.md) |
| ADRs | [ADR-0044](../../adrs/0044-autonomy-precedence-call-site-pre-specification.md), [ADR-0161](../../adrs/0161-ticket-fields-are-three-flat-stored-scalars-and-issue-is-ret.md) |
<!-- docket:artifacts:end -->

## Why

An engineer should be able to hand work to a ticket system as a clear request, or pull a ticket into docket, without retyping it, while keeping requests separate from proposed solutions.

## What changes

A new inline skill, `docket-link-ticket`, always run with the person present, with three paths:

- **Push (create):** turn a change into a ticket. A readiness check asks whether another engineer could own it as a user story; anything missing is asked about and written back to the change while it is still proposed, never invented. The draft — title, project, ticket type mapped from the change's type, the exported body, and the linked spec when there is one (a GitHub link in a shared repository, a cleaned attachment in a private one) — is shown once, and the ticket is created on the person's go. The link is recorded only after the ticket system confirms it.
- **Link:** record an existing ticket on a change; replacing or removing a link only on request.
- **Import:** turn a ticket into a new proposed change with its type mapped and the ticket fields set.

One ticket may sit on several changes; the skill names the others and goes ahead. `status --json` gains each linked change's ticket (active and archived) so the skill can see existing links. `docket-export` stops claiming ticket work in its trigger description.

## Acceptance criteria

- `status --json` reports a linked change's ticket for active and archived changes, and none for an unlinked or broken link.
- The skill and its two references cover every path and refusal the spec names; repository tests guard its inline shape and key rules.
- `docket-export` no longer triggers on a Jira ticket body or comment.
- Results list an Important live check, from another repository against a sandbox ticket project, covering push (shared and private, with a spec), link, and import.

## Out of scope

Posting ticket updates (changes 585–588); automatic ticket creation at change creation (change 587); rewriting a ticket's description after creation; any ticket-system call from this repository.
