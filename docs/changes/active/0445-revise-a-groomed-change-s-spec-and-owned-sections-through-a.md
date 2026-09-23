---
id: 445
slug: 'revise-a-groomed-change-s-spec-and-owned-sections-through-a'
title: 'Revise a groomed change''s spec and owned sections through a typed operation'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-09-23'
updated: '2026-09-23'
depends_on: []
stacked_on:
related: [382, 444]
discovered_from: [444]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
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

A common workflow is: groom a change to a spec, review the spec, then adjust it. Docket has no typed operation for the adjust step. `change.groom` refuses any change that already has a spec (its gate requires proposed, needs-design, no spec), and no other catalog operation revises a groomed spec or the change record's owned body sections. The only path today is a hand edit plus plain git commit in the `.docket` worktree, which bypasses the transaction, the exact-version CAS, and the `updated:` stamp that every other metadata write gets. Observed on change 0444 (2026-09-23): after a YAGNI review narrowed its spec, both the spec and the change's What changes / Out of scope had to be edited by hand.

## What changes

Add a typed, catalog-listed way to revise an already-groomed change: replace its linked spec's authored body and/or rewrite its owned proposal sections (Why / What changes / Out of scope / Open questions) in one metadata transaction, pinned to the record's exact version, stamping `updated:`, preserving the generated backlink and Artifacts blocks, and re-rendering derived views as other writes do. Interactive skills (docket-new-change, docket-groom-next) and their docs point to it for the review-then-adjust step. Whether this is a new operation or a relaxed `change.groom` gate, and which statuses permit revision, is for the brainstorm.

## Out of scope

Editing frozen build records (merged plans and results), Accepted ADRs, or terminal (done/killed) changes. Changing the spec path or relinking a different spec file. Revising trivial changes into spec'd ones or vice versa. Any automatic or autonomous spec revision. Review tooling or approval workflow for specs.
