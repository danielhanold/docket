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
spec: 'docs/superpowers/specs/2026-10-22-fix-duplicate-originhasbranch-breaking-combined-tag-go-vet-design.md'
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
| Spec | [2026-10-22-fix-duplicate-originhasbranch-breaking-combined-tag-go-vet-design.md](../../superpowers/specs/2026-10-22-fix-duplicate-originhasbranch-breaking-combined-tag-go-vet-design.md) |
<!-- docket:artifacts:end -->

## Why

`go vet -tags integration,e2e ./...` does not compile: `originHasBranch` is declared in both `internal/app/finalize_e2e_test.go` (e2e tag, added 2026-08-19) and `internal/app/leak_check_private_integration_test.go` (integration tag, added 2026-10-06). Each tag vets cleanly alone, so the clash is invisible until both tags are vetted together. It predates change 575 and was found during that change's review.

## What changes

Rename the integration-tag test helper `originHasBranch`, which checks a full ref, to `originHasRef` at its declaration and at every caller, so `go vet -tags integration,e2e ./...` compiles. The e2e helper keeps its name and behavior. The build proves the fix once by running the combined vet.

## Out of scope

- A suite check or CI step that vets the combined tags. Nothing runs both tags together, so a guard would turn a harmless future clash into a failed build (decided at grooming).
- A shared helper across the two tags, or any change to the e2e helper's probe.
