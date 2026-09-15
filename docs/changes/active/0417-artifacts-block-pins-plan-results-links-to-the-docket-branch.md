---
id: 417
slug: 'artifacts-block-pins-plan-results-links-to-the-docket-branch'
title: 'Artifacts block pins plan/results links to the docket branch, where those files never live'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-10'
updated: '2026-09-15'
depends_on: []
stacked_on:
related: [410, 341, 136]
discovered_from: [416]
adrs: []
spec: 'docs/superpowers/specs/2026-09-10-artifacts-block-pins-plan-results-links-to-the-docket-branch-design.md'
plan: 'docs/superpowers/plans/2026-09-14-artifacts-block-pins-plan-results-links-to-the-docket-branch.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/artifacts-block-pins-plan-results-links-to-the-docket-branch'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-15T00:30:18Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-10-artifacts-block-pins-plan-results-links-to-the-docket-branch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-10-artifacts-block-pins-plan-results-links-to-the-docket-branch-design.md) |
| Plan | [2026-09-14-artifacts-block-pins-plan-results-links-to-the-docket-branch.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-14-artifacts-block-pins-plan-results-links-to-the-docket-branch.md) |
<!-- docket:artifacts:end -->

## Why

The `## Artifacts` block renders every row's GitHub blob URL against the fixed metadata branch (`.../blob/docket/<path>`), because `linkContextOf` hardcodes `MetadataBranch` and `pathRow` consults only that `LinkContext`. But the plan and results FILES never live on the `docket` branch: they live on the change's feature branch while the change is `implemented`, and reach the integration branch (`main`) only through the PR merge. So the Plan and Results links 404 in both lifecycle states — pre-merge (file is on the feature branch, link says `docket`) and post-merge (file is on `main`, link still says `docket`). Observed on change 0416: its results file was pushed to the feature branch and is reachable there, but the change file's Results link points at `blob/docket/...` and cannot be clicked. Spec and ADR links are correct — those records genuinely live on `docket` — so only the two feature-branch/integration-branch artifacts are affected. The renderer's own doc comment already flags this as a known gap ('the Bash renderer's lifecycle-pinned Plan/Results branch is a later concern').

## What changes

Lifecycle-pin the blob ref for the Plan and Results rows so it points where the file actually is: the change's feature branch (`branch:`) while the change has not yet merged (implemented/in-progress), and the integration branch once the change is `done`. Leave Spec and ADR rows on the metadata branch unchanged. Plumb the integration-branch name and the change's feature branch into the render path (`LinkContext`/`ArtifactBlockContent`), select the ref per-row by change status, re-render the block wherever a frontmatter write already re-renders it (create, implemented, closeout), and cover the new behavior with renderer unit tests plus regenerated golden files and a mutation-tested guard.

## Out of scope

Adding a PR row to the Go renderer (still deferred). Terminal publication / copying results onto the metadata branch (deferred from Go v1; approach C was declined). Changing where plan/results files physically live. The reciprocal `docket:backlink` blocks. Change 0405's separate gate-handshake investigation.

## Reconcile log

### 2026-09-14

2026-09-14: Reconciled against current source. Confirmed the defect and design still hold verbatim: internal/render/link.go's LinkContext carries only MetadataBranch and BlobURL pins every row to blob/docket/<path>; internal/app/link_context.go's linkContextOf is the sole LinkContext constructor (hardcoding reposetup.MetadataBranchName) guarded by link_context_guard_test.go; internal/render/artifacts.go's pathRow/adrCell resolve every row via link.BlobURL, and its doc comment still names the lifecycle-pinned Plan/Results branch as deferred. Scope unchanged: lifecycle-pin Plan/Results rows to the change's feature branch (branch:) while not done, integration branch (main) once done, leaving Spec/ADR on the metadata branch; plumb integration + feature branch into the render path and select per-row by status; re-render at the existing create/implemented/closeout write sites (~18 linkContextOf call sites and 15 ArtifactBlockContent sites confirmed); cover with renderer unit tests, regenerated goldens, a mutation-tested repoguard, and a budget re-baseline if counts shift. Relations (related [410,341,136], discovered_from [416]) remain accurate; no obsolescence, no fundamental invalidation.
