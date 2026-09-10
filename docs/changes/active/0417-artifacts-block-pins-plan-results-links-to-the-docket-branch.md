---
id: 417
slug: 'artifacts-block-pins-plan-results-links-to-the-docket-branch'
title: 'Artifacts block pins plan/results links to the docket branch, where those files never live'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-10'
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [410, 341, 136]
discovered_from: [416]
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

The `## Artifacts` block renders every row's GitHub blob URL against the fixed metadata branch (`.../blob/docket/<path>`), because `linkContextOf` hardcodes `MetadataBranch` and `pathRow` consults only that `LinkContext`. But the plan and results FILES never live on the `docket` branch: they live on the change's feature branch while the change is `implemented`, and reach the integration branch (`main`) only through the PR merge. So the Plan and Results links 404 in both lifecycle states — pre-merge (file is on the feature branch, link says `docket`) and post-merge (file is on `main`, link still says `docket`). Observed on change 0416: its results file was pushed to the feature branch and is reachable there, but the change file's Results link points at `blob/docket/...` and cannot be clicked. Spec and ADR links are correct — those records genuinely live on `docket` — so only the two feature-branch/integration-branch artifacts are affected. The renderer's own doc comment already flags this as a known gap ('the Bash renderer's lifecycle-pinned Plan/Results branch is a later concern').

## What changes

Lifecycle-pin the blob ref for the Plan and Results rows so it points where the file actually is: the change's feature branch (`branch:`) while the change has not yet merged (implemented/in-progress), and the integration branch once the change is `done`. Leave Spec and ADR rows on the metadata branch unchanged. Plumb the integration-branch name and the change's feature branch into the render path (`LinkContext`/`ArtifactBlockContent`), select the ref per-row by change status, re-render the block wherever a frontmatter write already re-renders it (create, implemented, closeout), and cover the new behavior with renderer unit tests plus regenerated golden files and a mutation-tested guard.

## Out of scope

Adding a PR row to the Go renderer (still deferred). Terminal publication / copying results onto the metadata branch (deferred from Go v1; approach C was declined). Changing where plan/results files physically live. The reciprocal `docket:backlink` blocks. Change 0405's separate gate-handshake investigation.
