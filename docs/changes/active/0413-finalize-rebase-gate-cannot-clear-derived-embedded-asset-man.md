---
id: 413
slug: 'finalize-rebase-gate-cannot-clear-derived-embedded-asset-man'
title: 'Finalize rebase gate cannot clear derived embedded-asset manifest collisions'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-08'
updated: '2026-09-08'
depends_on: []
stacked_on:
related: []
discovered_from: [327]
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

Finalize's rebase-onto-main gate halts on rebase-conflicts-unresolved when a feature branch carries multiple commits that each regenerate internal/assets/embedded/manifest.json (a derived embedded-asset manifest that changes whenever any authored SKILL.md or agent text changes) and main has independently regenerated the same manifest and the same authored files. Replaying the branch commit-by-commit re-collides on manifest.json at every manifest-touching commit, and the fixed 2-dispatch rebase-resolver budget cannot clear the repeated collision, so finalize aborts and reports a human-required halt. Observed on change 327 / PR #291 (2026-09-08): the rebase halted, and a human had to rebase locally and regenerate the manifest once from the merged authored roots before finalize could complete. This will recur for any change whose branch touches authored asset text across more than a couple of commits.

## What changes

Explore and implement a fix so finalize's rebase gate can clear collisions on derived artifacts without exhausting the resolver budget. Candidate directions to evaluate during brainstorm: (a) regenerate the embedded bundle once from the merged authored roots instead of merging manifest.json hunk-by-hunk; (b) squash or collapse the feature branch's manifest-touching commits before the rebase gate so the manifest regenerates a single time; (c) teach docket-rebase-resolver to recognize manifest.json as a derived artifact and re-derive rather than three-way-merge it, so a derived-artifact conflict does not consume a resolver dispatch.

## Out of scope

Changing how the embedded-asset bundle itself is generated (cmd/genassets) or how SKILL.md word/line ceilings are enforced; broadening the rebase-resolver dispatch budget as a blanket fix independent of the derived-artifact distinction.
