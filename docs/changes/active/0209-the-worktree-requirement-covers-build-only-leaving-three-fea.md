---
id: 209
slug: the-worktree-requirement-covers-build-only-leaving-three-fea
title: The --worktree requirement covers build-* only, leaving three feature-scoped agent families ungated
status: proposed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: [208]
discovered_from: [206]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0206 required `--worktree` for `build-*` agents, but its spec states the decision generally:
*"Feature-scoped agents must name their tree, and the facade refuses to run one that did not."* The
implementation covers one family. Its whole-branch review found three more delegatable families that
are equally feature-scoped, two of which **commit**:

- `docket-rebase-resolver` — runs `git add` + `git rebase --continue` on an in-progress rebase,
  which by git's own rules is in the feature worktree (the main tree cannot have `feat/<slug>`
  checked out).
- `docket-integration-repair` — "writes a minimal fix", commits.
- The `docket-review-*` trio — read-only, but a wrong tree means findings about the wrong diff,
  silently.

None matches `build-*`, so a `runner:` on any of them still yields the silent
main-tree-on-the-integration-branch anchor 0206 exists to eliminate, under whatever auto-approve
grant the runner carries. This is the "check the twin it did not touch" case
(learnings: `fix-reintroduces-its-own-defect-class`).

## What changes

- Widen the facade's gate from the `build-*` shape to the feature-scoped set, or better, key it on a
  **declared agent scope** rather than a name list — a name list is an enumerated floor that ages
  into the gap it was written to close.
- Add the corresponding `sync-agents.sh` `emit_shim` required slot for the same set.
- Keep the generation guard bidirectional, as 0206 established.

## Out of scope

- The containment-vs-membership weakness in gate 3 (its own change).
