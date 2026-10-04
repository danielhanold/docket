---
id: 516
slug: 'remove-stale-auto-groom-comments-and-fix-testskillhandoffsit'
title: 'Remove stale auto_groom comments and fix TestSkillHandoffSites'' ''cannot be invoked'' match'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [502]
discovered_from: [502]
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

Two small leftovers found while building change 0502. (1) Code comments still say an unset `auto_groomable` inherits a repository `auto_groom` setting, which no longer exists: `internal/domain/entities.go` (the `AutoGroomable` field comment and the comment above the optional-bool reader) and `internal/app/change_create.go` (the `AutoGroomable` request field comment). (2) `TestSkillHandoffSites` treats the phrase "cannot be invoked" as a skill invocation, so 0502 had to reword skill text to dodge the guard instead of the guard reading the sentence correctly.

## What changes

Rewrite the stale comments to state what an unset `auto_groomable` means today. Make `TestSkillHandoffSites` stop counting negated phrasing such as "cannot be invoked" as an invocation, keyed on syntactic shape rather than a list of spellings, and mutation-test the guard both ways (a real invocation still reddens it; a negated mention does not).

## Out of scope

Any behavior change to auto-groom selection. The frozen fixture `internal/render/testdata/records/PROVENANCE.md`, which names deleted templates and stays as written.
