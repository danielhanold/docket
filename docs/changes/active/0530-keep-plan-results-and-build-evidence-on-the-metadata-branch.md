---
id: 530
slug: 'keep-plan-results-and-build-evidence-on-the-metadata-branch'
title: 'Keep plan, results, and build evidence on the metadata branch, and ship the spec with the PR'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-10-05'
updated: '2026-10-06'
depends_on: []
stacked_on:
related: [417, 415, 410, 391, 337, 330, 529, 531, 532, 533]
discovered_from: []
adrs: [1, 12, 66, 99]
spec: 'docs/superpowers/specs/2026-10-05-keep-plan-results-and-build-evidence-on-the-metadata-branch-design.md'
plan: 'docs/superpowers/plans/2026-10-05-keep-plan-results-and-build-evidence-on-the-metadata-branch.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/keep-plan-results-and-build-evidence-on-the-metadata-branch'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-06T03:05:06Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-keep-plan-results-and-build-evidence-on-the-metadata-branch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-05-keep-plan-results-and-build-evidence-on-the-metadata-branch-design.md) |
| Plan | [2026-10-05-keep-plan-results-and-build-evidence-on-the-metadata-branch.md](https://github.com/danielhanold/docket/blob/feat/keep-plan-results-and-build-evidence-on-the-metadata-branch/docs/superpowers/plans/2026-10-05-keep-plan-results-and-build-evidence-on-the-metadata-branch.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

A change's build artifacts are split across two branches today. The spec lives on the `docket` branch. The plan and the results file are committed on the feature branch and merge into `main`. The test evidence that gates finalize lives in a marker block inside the PR description. That split has several costs:

- Every PR diff carries planning documents next to the code: the plan plus three or more results commits.
- After the merge, close-out pushes a commit straight to `main` ("final backlinks retargeted to archive") outside any PR. Branch protection can block that commit.
- A results commit made after the test gate moves the feature head, so the evidence goes stale and can force another full suite run just because a markdown file changed.
- The spec, which is the key file for spec-driven design, never reaches `main`, while the plan and results do.
- Links are absolute GitHub URLs, so archiving and branch moves break them.

This change is also the foundation for a private-visibility mode in which nothing docket-specific may reach the host repository. That mode depends on build artifacts no longer riding the PR.

## What changes

- Plan and results files are written to the metadata branch at their existing paths (`docs/superpowers/plans/…`, `<results_dir>/…`) through typed operations. They are no longer committed on the feature branch. Each results checkpoint becomes a small metadata commit.
- Build evidence moves from the PR-description marker block into a `## Build evidence` section of the change record. Every reader (the finalize merge gate, the finalize rebase skip, `run.verify`) and every writer reads or writes it there.
- The feature branch's first commit is a copy of the spec at `docs/superpowers/specs/<date>-<slug>-design.md`, so the spec merges into `main` with the code. The metadata branch keeps the source copy.
- Links between artifacts that live on the same branch become relative, so they survive archiving. A one-time pass converts existing records.
- Retired: the post-merge backlink commit to `main`, the `Docket-Plan-Path` trailer, and the deferred `finalize.skip_results_only_delta` key along with its guard.
- Living docs that say plan and results ride the PR are corrected.

## Out of scope

- Private visibility itself: remote, naming, footprint, and switching (separate changes in this series).
- Moving plan and results files already merged into `main`. They stay where they are, and their absolute links keep working.
- Changing the content contract of the plan or results file.
- A compatibility read of PR-description evidence for changes in flight at cutover. Those finish on the previous version first.

## Reconcile log

### 2026-10-06

Reconciled 2026-10-06 against origin/main 4e823a301 (0529 merged). Spec facts re-verified in current code: plansPlanningRoot and the Docket-Results-Path constant in internal/app/change_attach.go, the finalize.skip_results_only_delta key, and the PR-body evidence readers. #529 landed the PR-body backlink repoint leg the spec says stays. No scope change; spec stands as groomed. Cutover note: this run itself uses the pre-change binary, per the spec Cutover section.
