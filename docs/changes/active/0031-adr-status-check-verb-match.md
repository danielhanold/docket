---
id: 31
slug: adr-status-check-verb-match
title: ADR status-consistency check — match the supersede/reverse verb, not just the target id
status: in-progress
priority: low
created: 2026-06-20
updated: 2026-06-20
depends_on: [30]
related: [30]
adrs: [12, 13]
spec: docs/superpowers/specs/2026-06-20-adr-status-check-verb-match-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/adr-status-check-verb-match
pr:
blocked_by:
reconciled: true
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

_Resolved at grooming (see spec)._

- **Distinct message vs fold?** → Fold into the existing `adr-status-inconsistent`
  check-id; no new check-id. The message names the expected back-pointer
  (`expected 'Reversed by ADR-X'`) so the verb-mismatch case is diagnosable.
- **Severity?** → Warn-only, like every other finding; `--strict` already escalates
  any finding to exit 1.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- **2026-06-20** — Spec authored earlier this same session; reconcile confirms it
  still holds. `origin/main` unchanged (`ad799b1`) since authoring; `adr-checks.sh`
  arm (b) and `status_target` on `origin/main` match the spec's target verbatim
  (merged `${SUPS[$id]} ${REVS[$id]}` loop, id-only `back != id` check). No new ADRs
  (ledger still 1–13), no related-change drift. **No scope change** — proceed to
  plan as written.
