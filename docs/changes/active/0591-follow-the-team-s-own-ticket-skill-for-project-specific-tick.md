---
id: 591
slug: 'follow-the-team-s-own-ticket-skill-for-project-specific-tick'
title: 'Follow the team''s own ticket skill for project-specific ticket details'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-10'
updated: '2026-11-10'
depends_on: [585]
stacked_on:
related: [581, 584, 585]
discovered_from: [585]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

Teams that use a ticket system often already have their own agent skill for their ticket project: the fields their tickets need and those fields' ids, defaults like the current sprint or a team field, a title format, and limits such as making one ticket-system call at a time. Docket's ticket skills know none of this. A ticket docket creates lands with no sprint, no team fields, and not in the team's title format, so someone fixes it by hand; and docket's connector calls can trip a rate limit the team already wrote down.

## What changes

When `docket-link-ticket` or `docket-update-ticket` is about to call the ticket system, it checks whether the session has a skill the team wrote for that ticket project. If it does, the docket skill loads it and follows it for project-specific details: required and default field values (such as the current sprint or a team field), the title format, the default ticket type, and how to pace calls. Docket's own rules still decide what gets posted and when: the events, the editorial contract, the readiness check, the draft the person approves, and recording a link only after the connector confirms. When a team skill and a docket rule disagree, docket's rule stands and the agent says so. With no team skill, nothing changes. Docket names no specific team skill and ships no provider- or team-specific metadata.

Left to grooming: how the team skill is found (the agent recognizing an installed skill by its description, or a per-repository `tickets.*` key naming it); which side wins where they overlap, notably the ticket type (docket maps it from the change's type, a team skill may default to one type) and the title (docket uses the change's title, a team skill may prescribe a format); and whether anything beyond call pacing applies when posting a comment.

## Acceptance criteria

Both ticket skills say when to look for a team skill, what to take from it, and which docket rules always win; repository tests check that text; with no team skill both skills behave as before; results list an Important live check from another repository that has its own ticket skill.

## Out of scope

Naming, shipping, or describing any particular team's skill or its values; syncing sprint, assignee, status, or other ticket fields after creation; provider API code or credentials in docket; any ticket-system call from this repository.
