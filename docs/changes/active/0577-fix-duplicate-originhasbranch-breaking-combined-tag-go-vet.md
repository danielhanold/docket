---
id: 577
slug: 'fix-duplicate-originhasbranch-breaking-combined-tag-go-vet'
title: 'Fix duplicate originHasBranch breaking combined-tag go vet'
status: 'proposed'
priority: 'low'
type: 'fix'
deliverable: 'pr'
created: '2026-10-22'
updated: '2026-10-22'
depends_on: []
stacked_on:
related: []
discovered_from: [575]
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

`go vet -tags integration,e2e ./...` does not compile: `originHasBranch` is declared in both `internal/app/finalize_e2e_test.go` (e2e tag, added 2026-08-19) and `internal/app/leak_check_private_integration_test.go` (integration tag, added 2026-10-06). Each tag vets cleanly alone, so the clash is invisible until both tags are vetted together. It predates change 575 and was found during that change's review.

## What changes

Remove the duplicate declaration, for example by sharing one helper between the two tagged test files, so `go vet -tags integration,e2e ./...` compiles.

## Out of scope

Adding a CI step that vets the combined tags, unless grooming decides it belongs here.
