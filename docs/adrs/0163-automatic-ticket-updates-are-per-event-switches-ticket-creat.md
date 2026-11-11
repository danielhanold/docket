---
id: 163
slug: 'automatic-ticket-updates-are-per-event-switches-ticket-creat'
title: 'Automatic ticket updates are per-event switches; ticket creation is never automatic'
status: 'Accepted'
date: '2026-11-11'
supersedes: []
reverses: []
relates_to: [44, 162]
change: 587
---

## Context

Docket can post an update to a change's linked ticket when a person asks (docket-update-ticket, ADR-0162). Change 587 lets workflows post those updates by themselves after certain events. That raises two questions: how the automatic behaviour is switched and what happens when it fails, and whether docket should also create tickets by itself. Call-site directions to invoked skills follow ADR-0044.

## Decision

Automatic ticket updates are per-event switches: `spec_written`, `pr_opened`, `pr_merged`, and `results_completed`. A switch that is off means nothing happens for that event: no post, no question, no report line. There is no queue, ledger, retry, or prompt. When a switch is on and a ticket is linked, the workflow directs `docket-update-ticket` at that event's site to post the update; with no linked ticket nothing is posted or reported. A missing connector or a failed post is one line in the run's report and never a gate: it does not change the run's disposition or any later step.

Ticket creation is never automatic. Two reasons. First, creating a ticket is a person's action: it puts work into a team's queue and makes commitments the team owns. Second, there is no clear trigger point. A stub is too early: it may be killed, merged, or reshaped before anyone designs it. A groomed change has its own problems: grooming happens at three design sites (new-change, groom-next, auto-groom); unattended grooming has nobody to approve the ticket; some changes already have tickets; and some teams need the ticket earlier than grooming.

The update skill now runs inside dispatched and forked workflow agents, not only in the person's session.

## Consequences

Teams opt into exactly the automatic updates they want, one event at a time, and the default (all off) changes nothing. A failed post never blocks or alters a build, finalize, or grooming run; the cost is that a missed update is only reported, never retried, so a person re-posts it on request if it matters. Tickets still have to be created or linked by a person (docket-link-ticket), which keeps the ticket system's queue under human control at the cost of one manual step per change. Because the update skill now runs in dispatched and forked agents, whether those agents can reach the session's connector tools depends on the harness; where they cannot, the result is the one-line report.

## Alternatives considered

Create the ticket at change creation: rejected; a stub is too early and may never be built, and creating a ticket is a person's action. Create the ticket at grooming: rejected; grooming spans three design sites, unattended grooming has nobody to approve, some changes already have tickets, and some teams need the ticket earlier. Support both creation points behind settings: rejected; it carries both sets of problems and adds configuration with no clear trigger. A queue, ledger, or retry for failed updates: rejected as unneeded machinery; one report line is enough. A single on/off switch for all updates: rejected in favour of per-event switches so teams choose which events reach the ticket.
