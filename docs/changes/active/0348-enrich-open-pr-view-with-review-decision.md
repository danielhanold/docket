---
id: 348
slug: enrich-open-pr-view-with-review-decision
title: Enrich the exact-PR view with reviewDecision so open-PR snapshots populate Approved
status: proposed
priority: medium
type: fix
created: 2026-08-26
updated: 2026-08-26
depends_on: []
stacked_on:
related: [347]
discovered_from: [347]
adrs: [97]
spec: docs/superpowers/specs/2026-08-26-enrich-open-pr-view-with-review-decision-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-26-enrich-open-pr-view-with-review-decision-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-26-enrich-open-pr-view-with-review-decision-design.md) |
| ADRs | [ADR-0097](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0097-pr-identity-is-verified-by-parsed-pr-number.md) |
<!-- docket:artifacts:end -->

## Why

Change 347 (ADR-0097, exact-PR-number identity) made `githubcli.ViewPullRequest` read one PR by
its exact recorded number. That view does not request GitHub's `reviewDecision` field, so an
open-PR snapshot never populates its `Approved` flag — the flag is structurally always empty.

347's classification fix sidestepped the gap by guarding on an observed head branch, so identity
mismatches surface correctly regardless of approval state. That closed the bug 347 was scoped to,
but it left the underlying gap in place: nothing that wants to know "is this open PR approved?" can
learn it from the snapshot. The 347 PR flagged this as an independent, pre-existing follow-up.

## What changes

Enrich the exact-PR view (`githubcli.ViewPullRequest`) to request GitHub's `reviewDecision`, and
populate the snapshot's `Approved` field from it so an open-PR snapshot genuinely reflects approval
state. Only `APPROVED` maps to true; `REVIEW_REQUIRED`, `CHANGES_REQUESTED`, and null map to false,
while unknown non-null vocabulary fails closed. The existing approval gate consumes the populated
boolean unchanged. This is additive to the exact-PR-number identity path — no change to how a PR is
located.

## Out of scope

- Any change to the exact-PR-number identity mechanism itself (ADR-0097) or to how finalize locates
  a change's PR from `pr:`.
- The approval-gate policy in `docket-finalize-change` beyond consuming a now-populated `Approved`.

## Open questions

None remain.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
