---
id: 298
slug: stacked-changes-build-a-new-change-on-top-of-a-parent-change
title: "Stacked changes — build a new change on top of a parent change's branch"
status: done
priority: medium
type: feat
created: 2026-08-11
updated: 2026-08-12
depends_on: []
related: [158]
discovered_from: []
adrs: [92]
spec: docs/superpowers/specs/2026-08-11-stacked-changes-design.md
plan: docs/superpowers/plans/2026-08-12-stacked-changes.md
results: docs/results/2026-08-12-stacked-changes-build-a-new-change-on-top-of-a-parent-change-results.md
trivial: false
auto_groomable: false
branch: feat/stacked-changes-build-a-new-change-on-top-of-a-parent-change
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/203
issue:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-11-stacked-changes-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-11-stacked-changes-design.md) |
| Plan | [2026-08-12-stacked-changes.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-08-12-stacked-changes.md) |
| Results | [2026-08-12-stacked-changes-build-a-new-change-on-top-of-a-parent-change-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-08-12-stacked-changes-build-a-new-change-on-top-of-a-parent-change-results.md) |
| PR | [#203](https://github.com/danielhanold/docket/pull/203) |
| ADRs | [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
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

## Reconcile log

### 2026-08-12

Reconciled at claim. The spec is one day old, so the pass found no drift to fold in:

- **Nothing of the subsystem exists yet.** A whole-repo search for `stacked_on` / `stacked-merged`
  returns no hit outside the spec itself — no partial implementation to reconcile against, and no
  field name collision in the manifest, the renderers, or the board.
- **Related change 0158 (batch mode) is still `proposed`** and auto-groom-blocked, so the
  out-of-scope boundary the spec draws against it still holds: nothing has been built on either
  side of that line.
- **Recent ADRs (0085–0091) do not bear on stacking.** They cover the critic's return channel,
  in-context gating dispatch, worktree contention, halt exit codes, liveness probes, publish-deferred
  coverage, and backtick hygiene — none of which constrains the lifecycle state, the base resolver,
  or the merge-site close-out this change adds.
- **The touch-point surfaces named in the spec all still exist under the names it uses**:
  `render-change-links.sh`, `render-board.sh`, `docket-status.sh`, `terminal-publish.sh`,
  `archive-change.sh`, and `verify-run.sh`.

Scope is unchanged. No auto-capture candidates surfaced at this pass: everything the design implies
is inside this change's own boundary.
