---
id: 153
slug: 'a-hold-keeps-a-groomed-change-out-of-autonomous-build-select'
title: 'A hold keeps a groomed change out of autonomous build selection; naming it overrides it'
status: 'Accepted'
date: '2026-10-19'
supersedes: []
reverses: []
relates_to: [6]
change: 560
---

## Context

ADR-0006 equates build-ready with "the autonomous builder will build it". Some groomed changes need a person for a step, or the person does not want them built yet; a loop claiming one halts, ending the whole drain. Deferring was the only exclusion, and it also blocks named runs and grooming revisions.

## Decision

An optional `hold: true` frontmatter field. It is a selection-only exclusion:

- Readiness reads `on-hold` only where it would otherwise be `build-ready`; every earlier readiness outcome wins.
- Autonomous selection (status `ready`, the no-id context, and id-allowlist drains) skips a held change.
- Claim eligibility accepts it, so any single-id act (a named run, a claim, `change.complete`) overrides the hold.
- Grooming picks and finalize are untouched.
- `change.groom` (every exit except abstain) and `change.create` set and lift it, with an optional operation-owned `## Why on hold` reason section.
- The auto-groomer sets it for specs that name a manual step and never lifts it.
- A malformed value reads as held (fail safe).

ADR-0006's equivalence of build-ready and autonomously built no longer holds for a held change.

## Consequences

A loop drain no longer halts on changes that need a person: they stay groomed, visible as `on hold`, and buildable by naming them. Readiness gains one outcome and selection one skip; claim, grooming, and finalize paths are unchanged. A held change is never built unattended until a human lifts the hold or names it, and a malformed value errs toward not building.

## Alternatives considered

- Deferring: rejected; it blocks named runs and grooming revisions.
- A readiness gate on the claim: rejected; it would refuse the named run.
- Also gating grooming picks: rejected; duplicates `auto_groomable` and adds surface.
- Carrying on past halts instead: rejected here; a separate change that needs a typed halt reason.
