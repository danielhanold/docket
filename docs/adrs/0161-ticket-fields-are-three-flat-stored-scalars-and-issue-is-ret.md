---
id: 161
slug: 'ticket-fields-are-three-flat-stored-scalars-and-issue-is-ret'
title: 'Ticket fields are three flat stored scalars, and `issue:` is retired'
status: 'Accepted'
date: '2026-11-09'
supersedes: []
reverses: []
relates_to: [71, 19]
change: 583
---

## Context

A change needs a durable, typed link to its ticket in the team's ticket system. Frontmatter's value model has no mapping kind (internal/document/value.go), and the patcher's classifyShape treats a mapping as ShapeUnsupported, so a nested field would need a new frontmatter value kind. The legacy `issue:` field is an unused leftover of the removed GitHub board mirror. Spec: docs/superpowers/specs/2026-11-09-link-a-change-to-a-ticket-design.md.

## Decision

A change's ticket link is three flat scalars, stored as given rather than derived:

- `ticket_provider`: a free lowercase token matching `^[a-z][a-z0-9-]*$`.
- `ticket_id`: a non-empty single line.
- `ticket_url`: an absolute http or https URL with a host.

The three are written together, and only by `change.link-ticket` (at any status; link, replace only on an explicit request, or unlink) and by `change.create`. A partial or invalid set reads as "not linked" everywhere and surfaces a `ticket-link-invalid` warning; it never blocks. `issue:` is removed from every read path; the four archived records holding an empty `issue:` value keep it as inert history.

## Consequences

Later ticket capabilities key on one typed link. Provider and id stay correct even if configuration changes later. One word, ticket, runs through docket. The cost: a hand edit can produce a broken set, and that is only warned on, never blocked.

## Alternatives considered

- A nested `ticket:` mapping: rejected because it needs a new frontmatter value kind.
- Deriving provider and id from the URL: rejected because it breaks when configuration changes and needs provider-specific knowledge.
- Keeping `issue:`: rejected because it leaves two words for one thing.
