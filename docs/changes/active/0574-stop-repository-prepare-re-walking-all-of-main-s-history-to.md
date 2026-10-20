---
id: 574
slug: 'stop-repository-prepare-re-walking-all-of-main-s-history-to'
title: 'Recognize the docket branch by its shape in the everyday repository commands'
status: 'in-progress'
priority: 'medium'
type: 'fix'
deliverable: 'pr'
created: '2026-10-20'
updated: '2026-10-20'
depends_on: []
stacked_on:
related: [360, 378]
discovered_from: [561]
adrs: []
spec: 'docs/superpowers/specs/2026-10-20-stop-repository-prepare-re-walking-all-of-main-s-history-to-design.md'
plan: 'docs/superpowers/plans/2026-10-20-stop-repository-prepare-re-walking-all-of-main-s-history-to.md'
results:
auto_groomable:
hold:
branch_prefix:
branch: 'fix/stop-repository-prepare-re-walking-all-of-main-s-history-to'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-20T17:58:17Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-20-stop-repository-prepare-re-walking-all-of-main-s-history-to-design.md](../../superpowers/specs/2026-10-20-stop-repository-prepare-re-walking-all-of-main-s-history-to-design.md) |
| Plan | [2026-10-20-stop-repository-prepare-re-walking-all-of-main-s-history-to.md](../../superpowers/plans/2026-10-20-stop-repository-prepare-re-walking-all-of-main-s-history-to.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository prepare` takes about 70 seconds on this repository, and nearly every docket workflow runs it first. `repository check`, `repair`, `configure-tests`, and `configure-harnesses` pay the same cost. Observed 2026-10-20 while building change 561.

The cause is the metadata-ownership check. When the metadata branch's first commit has no docket seed receipt and is not empty, which is true of every repository seeded by the old Bash migration script, the check searches all of main's history for the snapshot the seed was copied from. It spawns thousands of separate git processes and keeps nothing, so every run repeats the search.

That content proof only matters for the two commands that create or delete planning data: `repository init` and `repository migrate`. The everyday commands only write to the metadata branch. For them, two cheap structural checks catch the realistic accident, an ordinary branch someone named `docket`.

## What changes

The everyday repository commands (`prepare`, `check`, `repair`, `configure-tests`, `configure-harnesses`) recognize the metadata branch by its name plus two deterministic checks in the binary: it has exactly one starting commit, and it shares no history with the integration branch. A check that fails to run reads as unknown and refuses, as today. `repository init` and `repository migrate` keep the full content proof, unchanged.

The foreign-branch finding that prepare and check show is reworded to name the structural reason in plain words. A new ADR records the split, so the content proof does not drift back into the everyday commands.

Acceptance: `docket repository prepare` on this repository takes seconds, not a minute.

## Out of scope

Retiring `repository migrate` or the old single-branch layout support; it is unrelated to the ownership check. Any change to `repository init` or `repository migrate`. Speeding up the full content proof, for example by batching its git calls. Caching or storing a proof. The seed receipt format. The coordination cost inside implement-next runs, which change 360 covers.

## Reconcile log

### 2026-10-20

Re-traced against main c58321f27: `verifyMetadataOwnership` still has the five call sites the spec names (prepare, check augment, migrate x2, init); related #360 still proposed, #378 done. No drift; scope unchanged.
