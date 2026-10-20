---
id: 157
slug: 'everyday-repository-commands-recognize-the-metadata-branch-b'
title: 'Everyday repository commands recognize the metadata branch by name and shape'
status: 'Accepted'
date: '2026-10-20'
supersedes: []
reverses: []
relates_to: []
change: 574
---

## Context

The content proof's search through the integration history (`verifyLegacyEquivalence`) made `repository prepare` take about 70s on Bash-seeded repositories, and nearly every workflow runs prepare first. The proof answers a question only the commands that create or delete planning data need answered, yet every everyday command paid for it.

## Decision

`repository prepare`, `repository check`, `repository repair`, `repository configure-tests`, and `repository configure-harnesses` recognize the metadata branch by its name plus two structural checks (`verifyMetadataShape`): the branch has exactly one parentless root, and it shares no history with the integration branch. A probe error stays unknown and refuses; it never reads as owned.

The content proof in `verifyMetadataOwnership` (seed receipts and legacy equivalence) is reserved for the commands that create or delete planning data: `repository init` and `repository migrate`.

This partly reverses change 0378's out-of-scope note on bare single-root acceptance, for the everyday commands only. A later change must not put the content proof back into prepare.

## Consequences

Prepare no longer walks the integration history, so the roughly 70s cost on Bash-seeded repositories disappears from every workflow's startup.

A receiptless or invalid-receipt root now reads as owned under the everyday commands, while `repository migrate` and `repository init` still refuse it. The half-migrated remedies point at migrate, which refuses such a root, so that path needs a human.

The content proof survives only where data is created or deleted; reintroducing it into prepare would bring the cost back and is ruled out by this decision.

## Alternatives considered

- Batching the history walk: fast, but keeps a full walk on every command for no everyday benefit.
- Caching the proof: adds new state that must be invalidated correctly.
- Name only: would adopt an ordinary branch someone happened to name `docket`.
- Retiring migrate: unrelated to the ownership question.
