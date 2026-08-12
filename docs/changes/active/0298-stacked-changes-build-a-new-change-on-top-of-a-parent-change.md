---
id: 298
slug: stacked-changes-build-a-new-change-on-top-of-a-parent-change
title: "Stacked changes — build a new change on top of a parent change's branch"
status: in-progress
priority: medium
type: feat
created: 2026-08-11
updated: 2026-08-12
depends_on: []
related: [158]
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-11-stacked-changes-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: feat/stacked-changes-build-a-new-change-on-top-of-a-parent-change
claimed_at: 2026-08-12T01:30:33Z
pr:
issue:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-11-stacked-changes-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-11-stacked-changes-design.md) |
<!-- docket:artifacts:end -->

## Why

Testing an implemented change routinely surfaces follow-up work that is its own change — own
spec, own PR-sized unit — but must be built on top of the still-unmerged parent branch.
Today the branch model forbids that (feature branches always cut from the integration branch)
and `depends_on` only unblocks at `done`, so the human's real choices are merging half-finished
work into main or hand-rolling branches outside docket. Stacking makes the discovered-during-
testing workflow first-class.

## What changes

A new optional `stacked_on: <parent change id>` manifest field declares that a change builds on
its parent's feature branch. A shared effective-base resolver drives readiness, branch cut, PR
base, and the finalize gate: the child's branch is cut from the parent's pushed branch, its PR
targets it, and it merges into the parent — the stack root's single PR carries everything into
main. A child merged into its parent enters a new non-terminal `stacked-merged` state and is
promoted to `done` only when the root's code reaches the integration branch, preserving the
invariant that `done` means reachable-from-integration (dependency satisfaction, artifact
links, and terminal publishing all keep their meaning). One idempotent stack close-out runs
from both finalize and the status sweep. Autonomous finalize of a parent with open children
hard-blocks; interactive finalize warns and lets the human override, with open child PRs
explicitly retargeted before the parent branch is deleted. A killed parent never triggers the
merge fallback: descendants flip to `blocked` for a human decision. The board renders the
relationship plus the waiting / awaiting-root / rebase-pending states. Skill bodies gain only
trigger lines; the mechanics live in a shared reference read on trigger (progressive
disclosure).

Full design: see the linked spec.

## Out of scope

- Batch mode — several changes on one branch / one PR (change 0158).
- Continuous restacking when the parent branch moves; the child picks up parent commits at its
  own rebase gate.
- Reliance on GitHub's automatic PR base retargeting.

## Open questions

None — settled in the linked spec's brainstorm (2026-08-11).
