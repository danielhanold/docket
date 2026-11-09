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
related: [581, 583]
discovered_from: [581]
adrs: []
spec: 'docs/superpowers/specs/2026-11-09-add-acceptance-criteria-to-changes-and-rename-export-s-issue-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-11-09-add-acceptance-criteria-to-changes-and-rename-export-s-issue-design.md](../../superpowers/specs/2026-11-09-add-acceptance-criteria-to-changes-and-rename-export-s-issue-design.md) |
<!-- docket:artifacts:end -->

## Why

A ticket needs acceptance criteria that belong to the change, and the export that will become a ticket body is named after GitHub issues. Both are small foundations every later ticket change uses.

## What changes

Two small foundations for the ticket integration:

- **Acceptance criteria.** An optional `## Acceptance criteria` change-body section. `change.create` writes it between What changes and Out of scope when given; `change.groom` (outcomes `spec` and `revise`) can add or edit it as an owned section, and adding it later appends it at end of file. It is never a build gate. The convention documents it.
- **Export kind rename.** Export's `issue` kind becomes `ticket`, with its internal identifiers renamed alongside it and no compatibility alias. Its section allowlist gains Acceptance criteria, and a change exports as `<date>-ticket-<slug>.md`.

**Acceptance criteria:** A change created with acceptance criteria shows the section between What changes and Out of scope; groom adds it at end of file or replaces it in place; `docket open change <id> --export` writes `<date>-ticket-<slug>.md` and keeps the section; a change without the section builds as before and exports the same sections as before, under the `ticket` filename.

The linked spec carries the full design, including the decisions settled by change 581.

## Out of scope

Reviewers checking a build against acceptance criteria; the legacy `issue:` frontmatter field (change 583); ordered insertion of sections added after creation; renaming export's "another issue" reference wording.
