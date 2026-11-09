---
id: 586
slug: 'post-a-ticket-update-on-request-in-a-private-repository'
title: 'Post a ticket update on request in a private repository'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-09'
depends_on: [585]
stacked_on:
related: [581]
discovered_from: [581]
adrs: []
spec:
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

In a private repository the artifact cannot be linked; it must be cleaned and attached without leaking docket.

## What changes

The update exports and cleans the artifact through docket-export with its normal before → after review, uploads the attachment first (in two steps where the connector needs it) and posts the comment last, naming the attachment. A failed required upload posts nothing.

**Acceptance criteria:** The skill text covers the attachment order and failure path; results list an Important live check from a private scratch repository.

**Covers (581 spec):** decision 3's attachment order, decision 9 (private delivery, attended; the two-step attachment)

**Likely touches:** `docket-update-ticket`'s private-delivery reference; uses docket-export unchanged (attended)

**Design source:** this change is position 5 of the ticket-integration split planned by change 581. The settled design lives in 581's spec, `docs/superpowers/specs/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-design.md`, and its plan in `docs/results/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-results.md` (the entry for position 5, *Coverage check*, *ADRs*, and *Trace notes*). Binding sections: Decisions 3 (attachments first, comment last) and 9 (Delivery by visibility — private, attended), plus *Vocabulary*, *Editorial contract*, and *Out of scope*. Grooming must read those sections and **restate the settled decisions this change carries in its own spec** — do not merely link to them — so the build needs nothing from change 581. Anything 581 marks as fixed is not reopened; only the points it leaves to grooming are decided here.

## Out of scope

Unattended private posting and the leak-pattern check (change 588).
