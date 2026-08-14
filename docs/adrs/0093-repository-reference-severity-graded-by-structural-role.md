---
id: 93
slug: repository-reference-severity-graded-by-structural-role
title: "Repository reference severity is graded by structural role, not uniformly"
status: Accepted
date: 2026-08-14
supersedes: []
reverses: []
relates_to: []
change: 307
---

## Context

Change 0307's Go read-model (`internal/repository` `BuildSnapshot` validation plus `internal/domain`
`ValidateADRGraph`) validates every cross-reference in a decoded repository snapshot.
`ValidationReport.HasErrors()` is the single gate future mutating operations call, and read-only
consumers receive the full report.

A uniform error severity for every dangling reference makes it impossible for a composer to supply a
legitimate SUBSET of the repository — only ADRs for an index pass, or a corpus sample. Docket's real
change graph is one weakly-connected component, so any subset dangles somewhere, and the mutation
gate then trips on data that is not damaged.

## Decision

Severity is graded by the reference's **structural role**.

**Gating/structural references** dangle as `SeverityError` — they encode two-sided invariants or gate
lifecycle policy, so an unresolved one genuinely blocks mutation:

- a change's `depends_on` and `stacked_on`
- an ADR's `supersedes` / `reverses` targets
- status back-pointer verb matches
- self-references

**Associative cross-links** dangle as `SeverityWarning` — they gate nothing, and a composer supplying
part of the ledger legitimately leaves them unresolved:

- a change's `related` / `discovered_from` / `adrs`
- learning changes
- an ADR's `relates_to` and its producing change back-link

Implemented as `referenceSeverity` in `internal/repository/validate.go` and `adrReferenceSeverity` in
`internal/domain/adr.go` (change 0307; deep-review finding #3 aligned the ADR side with the change
side).

## Consequences

- Enables partial-repository composition — the 0310 status/health slice, index passes, frozen test
  corpora — without false mutation-gate trips.
- Read-only reports still surface every dangling associative link as a warning.
- Cost: a genuinely broken associative link no longer blocks mutation; repairing those relies on
  humans reading warnings.
- The two severity functions must stay aligned; each has tests pinning the split.
