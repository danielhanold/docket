---
id: 480
slug: 'check-finalize-cleanup-s-not-final-message-for-killed-change'
title: 'Report killed changes truthfully in finalize cleanup'
status: 'in-progress'
priority: 'low'
type: 'fix'
created: '2026-09-30'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [483, 474, 316]
discovered_from: [474]
adrs: []
spec: 'docs/superpowers/specs/2026-10-01-check-finalize-cleanup-s-not-final-message-for-killed-change-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/check-finalize-cleanup-s-not-final-message-for-killed-change'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-01T11:58:11Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-01-check-finalize-cleanup-s-not-final-message-for-killed-change-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-01-check-finalize-cleanup-s-not-final-message-for-killed-change-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0474 renamed "terminal" to "final" across the lifecycle vocabulary and left `finalize cleanup`'s default-branch refusal reading "change is not final". Tracing confirmed a killed change reaches that branch — `FinalizeCleanup` handles only `done` and `stacked-merged`, and nothing upstream filters by status — so `docket finalize cleanup --id <killed>` returns `invalid-state` / `not-final`, which is false: `killed` is final.

The close-out docs compound it: `close-out.md` step 4 and the implementer's reconcile-kill notes tell kill callers to run cleanup and imply it prunes the killed change's worktree and branch. In practice the call fails under abort-and-report after the kill has already archived, and the resources are silently left behind.

## What changes

Make `finalize cleanup` report a killed change truthfully as a deliberate retention — `no-op`, disposition `retained`, new reason `killed-retained`, with an accurate message — mirroring the existing `stacked-merged` case, and correct the close-out kill-path docs (and their embedded copies) so they describe that outcome instead of promising pruning. Actually cleaning up a killed change's resources is follow-up change 0483.

## Out of scope

Implementing killed-change cleanup (change 0483). Any wider rewording of lifecycle messages beyond this case. Changes to the `not-final` default branch or the `stacked-merged` path.

## Reconcile log

### 2026-10-01

Re-traced against main 85bace7de: `FinalizeCleanup` still handles only `done` and `stacked-merged` (killed falls to the `default` `not-final` refusal), and the two doc sites (close-out.md step 4, edge-paths.md reconcile-kill) plus their embedded copies still carry the misleading text. Spec holds as written; no scope change. Change 0483 (killed-change cleanup) remains unbuilt follow-up.
