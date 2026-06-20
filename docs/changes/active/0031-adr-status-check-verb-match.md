---
id: 31
slug: adr-status-check-verb-match
title: ADR status-consistency check — match the supersede/reverse verb, not just the target id
status: proposed
priority: low
created: 2026-06-20
updated: 2026-06-20
depends_on: [30]
related: [30]
adrs: [12, 13]
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Why

`scripts/adr-checks.sh` (shipped by change 0030) has a deliberately faithful but
under-detecting arm in its `adr-status-inconsistent` check. Arm (b) verifies that
when ADR-X declares `supersedes: [Y]` or `reverses: [Y]`, the old ADR-Y's
`status:` line was flipped to point **back at X** — but it compares only the
*target id*, not the *verb*. So an ADR-Y whose status reads `Superseded by
ADR-X` when it should read `Reversed by ADR-X` (right target, wrong verb) passes
silently. The mismatch is a real ledger inconsistency the check morally should
catch; 0030 reproduced the original `docket-adr` prose exactly ("flipped to point
back"), which never required verb-matching, so closing the gap is an
*enrichment* deferred out of 0030's "faithful re-implementation, no redesign"
scope.

## What changes

Make `adr-checks.sh`'s `adr-status-inconsistent` arm (b) verb-aware: a
`supersedes:` edge requires the target's status to be `Superseded by ADR-X`; a
`reverses:` edge requires `Reversed by ADR-X`. A right-id/wrong-verb back-pointer
becomes a finding. Add a fixture per verb-mismatch case (and keep the existing
correct-flip controls green).

## Out of scope

- The other two checks (`adr-numbering-gap`, `adr-dangling-link`) and arm (a) —
  unchanged.
- Any change to how ADRs are authored or how statuses are written by `docket-adr`.

## Open questions

- Is right-target/wrong-verb worth a distinct finding message, or fold into the
  existing arm-(b) message with the expected verb named?
- Severity: warn-only like the rest (almost certainly yes — keep it consistent).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
