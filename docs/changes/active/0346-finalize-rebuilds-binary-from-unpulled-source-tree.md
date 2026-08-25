---
id: 346
slug: finalize-rebuilds-binary-from-unpulled-source-tree
title: "Finalize's post-merge binary rebuild runs against an unpulled source tree"
status: proposed
priority: medium
type: fix
created: 2026-08-25
updated: 2026-08-25
depends_on: []
related: []
discovered_from: [342]
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

The repo policy (AGENTS.md, "Rebuild the binary after a merge to main") requires that after a PR
merges into `main`, the installed `docket` binary be rebuilt so the installed tool matches source:
`docket development install --source /Users/homer/dev/docket`.

During the close-out of change 342, `docket-finalize-change` ran that rebuild — but the merge lands
on `origin/main` while the **primary source worktree's local `main` was never pulled**. It sat at the
pre-merge head (`63a202e8`), behind the merge commit (`c323e266`). So the rebuild compiled
**pre-merge source**: the binary's timestamp updated, but its contents did not include the just-merged
change. The rule's intent — installed tool == merged source — was silently unmet, and nothing in the
run surfaced it. A human had to pull `main` and rebuild by hand to actually satisfy the policy.

This is a real close-out defect, not a one-off: finalize merges via the GitHub PR (rebase method), so
the merge exists on the remote but not in the local primary tree unless finalize explicitly pulls it.
Rebuilding `--source /Users/homer/dev/docket` against that stale tree will *always* miss the merge.
The failure is invisible because the install command succeeds and the binary mtime advances — the
staleness only shows up if someone checks `git merge-base --is-ancestor <merge> HEAD` on the source
tree, which nobody does by default.

## What changes

Make finalize's post-merge rebuild build from a source tree that provably contains the merge.
Candidate directions to weigh at brainstorm time (not decided):

- Before the rebuild step, fast-forward the primary source tree: `git -C <source> pull --ff-only
  origin <integration_branch>` (or fetch + ff-only merge), then assert the merge commit is an
  ancestor of the source `HEAD` before invoking `docket development install`.
- Rebuild from a tree finalize already knows contains the merge, rather than assuming the primary
  worktree is current.
- At minimum, **verify and fail loud**: after the rebuild, assert the merge commit is reachable from
  the built source, and surface a clear error if not — never let a stale rebuild report success.

## Out of scope

- The AGENTS.md policy itself (rebuild-after-merge stays required) — this is about finalize
  satisfying it correctly, not changing the requirement.
- The `docket.sh` facade vs `docket` binary verb-surface divergence noted during the same run — that
  is a separate observation; file it independently if it warrants tracking.

## Open questions

- Is a `pull --ff-only` on the primary worktree always safe at finalize time, or can that tree be
  dirty / on a non-integration branch (in which case finalize must detect and skip-with-warning
  rather than fail the whole close-out)?
- Does this apply only in docket-mode (separate `.docket` metadata worktree, primary tree on the
  integration branch), or also in main-mode where the single tree already carries the merge locally?
- Should the merge-reachable assertion be a hard failure of the close-out, or a loud warning that
  still lets `done` stand (the merge itself already landed correctly)?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
