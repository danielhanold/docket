---
id: 162
slug: 'ticket-work-is-two-inline-skills-split-by-attended-linking-v'
title: 'Ticket work is two inline skills, split by attended linking versus updating'
status: 'Accepted'
date: '2026-11-10'
supersedes: []
reverses: []
relates_to: [161, 44]
change: 584
---

## Context

Changes 581, 583, and 584 add ticket integration. Ticket work needs the session's own connector tools (MCP servers and plugins) and, for create and import, a person to answer refinement questions and approve a draft. A forked skill cannot reach the session's connector tools or ask the person.

## Decision

`docket-link-ticket` (link, create a ticket from a change, import a change from a ticket; always with the person present) and a later `docket-update-ticket` (posting updates; the person or nobody) are both inline skills, harness-neutral, posting through the agent's own connector tools. docket installs no connector, stores no credential, and has no provider API code. Autonomous workflows only ever load the update skill, so refinement and import instructions never take space in an implement or finalize run. docket-export stops claiming ticket work; docket-link-ticket calls it for file exports.

## Consequences

No provider code to maintain. Behavior lives in skill prose and can only be verified live in another repository against a sandbox project. Two skills to keep consistent.

## Alternatives considered

- One ticket skill for all operations: rejected, because autonomous runs would load the attended refinement and import text.
- A forked or agent-wrapped skill: rejected, because it has no connector access and cannot ask the person.
- Provider API code in the binary: rejected, because it needs credentials and per-provider code.
