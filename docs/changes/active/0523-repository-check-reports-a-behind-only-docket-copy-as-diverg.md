---
id: 523
slug: 'repository-check-reports-a-behind-only-docket-copy-as-diverg'
title: 'repository check flags a behind-only .docket copy as a conflict and an ahead primary as behind'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [366, 511]
discovered_from: [511]
adrs: []
spec: 'docs/superpowers/specs/2026-10-05-repository-check-reports-a-behind-only-docket-copy-as-diverg-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-repository-check-reports-a-behind-only-docket-copy-as-diverg-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-05-repository-check-reports-a-behind-only-docket-copy-as-diverg-design.md) |
<!-- docket:artifacts:end -->

## Why

After any typed metadata write, the local `.docket` copy is clean and only behind `origin/docket`, with no commits of its own. Typed operations (groom, finalize, `repository repair`, and the rest) push from a throwaway copy and never move the local one. Only `repository prepare` fast-forwards it, and every docket workflow runs `prepare` first. `docket repository check` still reports that normal state as a conflict, `metadata-worktree-dirty` plus `local-metadata-diverged`, and tells the user to reconcile with a human. That's a false alarm on both counts: nothing is dirty and nothing has diverged. Change 511 hit it after `repair`, and the upgrade guide works around it by running `prepare` again. Change 0366 hit it twice after finalize.

The cause: `check` asks only whether the local and remote tips are equal. `prepare` already tells same, behind, ahead, and diverged apart. The same verdict also makes `repository configure-tests` refuse ("not in a healthy state") after any docket action until `prepare` runs.

A sibling with the same shape: when the primary checkout has unpushed commits on the integration branch, `check` labels it `primary-behind-remote-tip` and says to fast-forward, which can't work in that state.

## What changes

- `repository check` uses the relationship `prepare` already computes (same, behind, ahead, diverged) instead of tip equality. A clean, behind-only local `.docket` copy is **healthy**: exit 0, no finding. The next `prepare` fast-forwards it.
- "Dirty" fires only for real uncommitted or untracked files. A copy with local-only commits reports `local-metadata-ahead`. "Diverged" is kept for true divergence.
- `repository configure-tests` stops refusing a repository whose only issue is a behind-only `.docket` copy. It shares the same verdict, so no extra code is needed.
- Bundled sibling: `check`'s primary-checkout finding is split by relationship. Behind keeps `primary-behind-remote-tip` and its fast-forward remedy. Ahead and diverged get their own findings, with remedies that work in those states. All three stay non-healthy.
- The Bash upgrade guide drops its extra `prepare` step after `repair`, and its executable test follows the new text.

## Out of scope

Changing how a truly diverged or ahead `.docket` copy is handled (it stays a conflict with a human remedy). Making typed operations advance the local `.docket` copy after they push. `prepare`'s routing, `repair`'s output, and `init`'s refusal text. Other `repository check` findings.
