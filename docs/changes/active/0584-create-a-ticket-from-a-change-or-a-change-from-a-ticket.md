---
id: 584
slug: 'create-a-ticket-from-a-change-or-a-change-from-a-ticket'
title: 'Create a ticket from a change, or a change from a ticket'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-09'
depends_on: [582, 583]
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

An engineer should be able to hand work to a ticket system as a clear request, or pull a ticket into docket, without retyping it, while keeping requests separate from proposed solutions.

## What changes

`docket-link-ticket`, always attended: link an existing ticket by URL; create a ticket from a change after a readiness check and targeted refinement written back through `change.groom` (only while `proposed`), with a draft the human approves and the ticket type mapped from the change's type through the connector; import a ticket into a new proposed change with mapped type and ticket fields set. Records the fields only after the connector confirms. Fixed by the spec: the readiness criteria, never inventing scope, the type mapping rule, description written once. Left to grooming: reference file layout and trigger wording.

**Acceptance criteria:** The skill's text covers each path and every refusal the spec names; repository tests check its structure; results list an Important live check against a sandbox project from another repository.

**Covers (581 spec):** decision 4 (link, import, create, ticket type), decision 10's `docket-link-ticket` half

**Likely touches:** new inline skill `docket-link-ticket` with a create reference (readiness check, refinement, draft) and an import reference; uses docket-export's ticket kind and change 583's operation; install/skill registration and generated surfaces

**Design source:** this change is position 3 of the ticket-integration split planned by change 581. The settled design lives in 581's spec, `docs/superpowers/specs/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-design.md`, and its plan in `docs/results/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-results.md` (the entry for position 3, *Coverage check*, *ADRs*, and *Trace notes*). Binding sections: Decisions 4 (Establishing the link: link, import, create) and 10 (Layering — the `docket-link-ticket` half), plus *Vocabulary*, *Editorial contract* (decision 11, for the ticket draft), *Out of scope*, and *What the code says today*. Grooming must read those sections and **restate the settled decisions this change carries in its own spec** — do not merely link to them — so the build needs nothing from change 581. Anything 581 marks as fixed is not reopened; only the points it leaves to grooming are decided here.

## Out of scope

Updates (changes 585–588); automatic ticket creation at change creation (change 587); any ticket call from this repository.
