---
id: 591
slug: 'follow-the-team-s-own-ticket-skill-for-project-specific-tick'
title: 'Follow a companion skill when creating a ticket'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-10'
updated: '2026-11-11'
depends_on: [585]
stacked_on:
related: [581, 584, 585, 587]
discovered_from: [585]
adrs: [162]
spec: 'docs/superpowers/specs/2026-11-11-follow-the-team-s-own-ticket-skill-for-project-specific-tick-design.md'
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
| Spec | [2026-11-11-follow-the-team-s-own-ticket-skill-for-project-specific-tick-design.md](../../superpowers/specs/2026-11-11-follow-the-team-s-own-ticket-skill-for-project-specific-tick-design.md) |
| ADRs | [ADR-0162](../../adrs/0162-ticket-work-is-two-inline-skills-split-by-attended-linking-v.md) |
<!-- docket:artifacts:end -->

## Why

Many people who use a ticket system already have an agent skill for their ticket project, from their company, their team, or themselves: a companion skill. It knows the project's title format, which sprint new tickets go into, which custom fields must be set and their ids, the default labels, the ticket type, and limits such as making one ticket-system call at a time. Docket's create path knows none of this. A ticket docket creates lands with the wrong title format, no sprint, no custom fields, and no default labels, and someone fixes it by hand.

## What changes

When `docket-link-ticket` creates a ticket from a change, it finds the person's companion skill and follows it for project-specific details. A new optional config key, `tickets.companion_skill`, names it at any config layer. When the named skill isn't available, the agent warns in one line and falls back to finding one whose description covers the ticket project. Several matches mean asking the person. No match means creating the ticket as today.

The companion skill decides:
- the ticket type, only when it states one;
- the title format, filled with the change's title;
- the field values: sprint, custom fields, labels, assignee, and the like;
- anything it does after the ticket exists;
- how the calls are made.

Docket keeps the readiness check, the description body, the person's approval of a draft that shows everything the companion skill will do, and recording the link only after the connector confirms. The rule that a created ticket's description is never rewritten is removed: ticket skills may edit the ticket when the work needs it.

## Acceptance criteria

`create.md` says when and how to find a companion skill, what it decides, and what docket keeps, and repository tests pin that text. The never-rewritten wording and its test are gone. `tickets.companion_skill` resolves at every config layer and is documented. With the key unset and no matching skill, creating a ticket behaves as before. The results list an Important live check from another repository with a companion skill installed.

## Out of scope

Using the companion skill when linking, importing, or posting updates (`docket-update-ticket`); naming, shipping, or describing any particular companion skill or its values; syncing ticket fields after creation; provider API code or credentials in docket; any ticket-system call from this repository; unattended ticket creation (change 587 decides how a companion skill behaves there).
