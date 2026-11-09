---
id: 582
slug: 'add-acceptance-criteria-to-changes-and-rename-export-s-issue'
title: 'Add acceptance criteria to changes and rename export''s issue kind to ticket'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-09'
depends_on: []
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

A ticket needs acceptance criteria that belong to the change, and the export that will become a ticket body is named after GitHub issues. Both are small foundations every later ticket change uses.

## What changes

An optional `## Acceptance criteria` body section, after What changes and before Out of scope, settable by `change.create` and editable through `change.groom`'s owned sections (outcomes `spec` and `revise`), documented in the convention. Export's `issue` kind becomes `ticket`, internal identifiers renamed with it, its section allowlist gains Acceptance criteria, and a change exports as `<date>-ticket-<slug>.md`. Fixed by the spec: placement, optional and never a gate, no compatibility alias. Left to grooming: exact placement wording in the convention, test shape.

**Acceptance criteria:** A change created or groomed with acceptance criteria shows the section in place; `docket open change <id> --export` writes `<date>-ticket-<slug>.md` and keeps the section; a change without the section builds as before and exports the same sections as before, under the `ticket` filename.

**Covers (581 spec):** decisions 6 and 7

**Likely touches:** `change.create` and `change.groom` owned sections; the convention's body-section list (and its embedded twin); `internal/export` (`KindIssue` → `KindTicket`, `issueSections` → `ticketSections`, `stepIssueSections`, `keepIssueSections`, tests `issueH2`, `issue-x.md`); `internal/app/open_export.go` `exportKinds`

**Design source:** this change is position 1 of the ticket-integration split planned by change 581. The settled design lives in 581's spec, `docs/superpowers/specs/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-design.md`, and its plan in `docs/results/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-results.md` (the entry for position 1, *Coverage check*, *ADRs*, and *Trace notes*). Binding sections: Decisions 6 (Acceptance criteria) and 7 (Export's `issue` kind becomes `ticket`), plus *Vocabulary*, *Out of scope*, and *What the code says today*. Grooming must read those sections and **restate the settled decisions this change carries in its own spec** — do not merely link to them — so the build needs nothing from change 581. Anything 581 marks as fixed is not reopened; only the points it leaves to grooming are decided here.

## Out of scope

Reviewers checking a build against acceptance criteria; the legacy `issue:` frontmatter field (change 583).
