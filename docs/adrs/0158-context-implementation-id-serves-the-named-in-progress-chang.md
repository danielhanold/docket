---
id: 158
slug: 'context-implementation-id-serves-the-named-in-progress-chang'
title: 'context.implementation --id serves the named in-progress change'
status: 'Accepted'
date: '2026-10-20'
supersedes: []
reverses: []
relates_to: [126, 106]
change: 360
---

## Context

`context.implementation --id` required the explicit change to be claimable exactly as a claim requires, so it refused any non-proposed change. In 14 of 14 recent implement-next runs, the run hit that refusal when re-reading its own claimed change. Skill re-reads after claim, and the documented halted-resume entry, both go through this operation, so the refusal broke them. Spec: docs/superpowers/specs/2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess-design.md.

## Decision

`context.implementation --id` serves an explicitly named `in-progress` change. It returns the full bundle with `claim_eligible: false`, `claim_refusal: not-ready-not-proposed`, `workflow.feature_branch` set to the recorded `branch:`, and a readable `halt.run_halted`. A run can therefore re-read its own claimed change, and the documented halted-resume entry works. Every other non-proposed status still refuses. Selection without `--id` still picks only build-ready proposed changes. This amends the earlier contract that the explicit change must be claimable exactly as a claim requires.

## Consequences

Runs and resume flows re-read their claimed change through the one existing bundle instead of failing. Callers must read `claim_eligible`/`claim_refusal` rather than treating a served bundle as permission to claim. The run gets the record revision it needs for later writes by running `git rev-parse HEAD:<record path>` in the local metadata checkout before each write; the receipt carries no revision field.

## Alternatives considered

A separate read operation for claimed changes: rejected because it duplicates the bundle. A new receipt field carrying the record revision: rejected at grooming; the run reads the revision from the local metadata checkout before each write instead.
