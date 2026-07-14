---
id: 75
slug: cwd-independent-repo-root-anchor
title: Anchor the repo root to the main worktree — CWD-independent scripts, a fail-closed cleanup guard, and a durable finalize posture
status: in-progress
priority: high
created: 2026-07-14
updated: 2026-07-14
depends_on: []
related: [76]
adrs: []
spec: docs/superpowers/specs/2026-07-14-cwd-independent-repo-root-anchor-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: feat/cwd-independent-repo-root-anchor
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-14-cwd-independent-repo-root-anchor-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-14-cwd-independent-repo-root-anchor-design.md) |
<!-- docket:artifacts:end -->

## Why

docket runs a three-worktree layout — the main checkout, the persistent `.docket/` metadata
worktree, and ephemeral `.worktrees/<slug>` feature worktrees — and every skill's Step 0 runs
preflight. But the scripts derive the repo root from **where the caller happens to be standing**:
`git rev-parse --show-toplevel` and `cd "$REPO_DIR" && pwd -P` both return the *linked worktree*,
not the repo. A stray `cd` — from an agent, a hook, or a human — is enough to misdirect them, and
today it does, in three distinct ways (all verified against the code on throwaway fixtures,
2026-07-14; full detail in the spec):

- **D1 — `cleanup-feature-branch.sh` deletes the REMOTE branch, then fails.** Unreported partial
  data loss. From any linked-worktree CWD its relative `target` never resolves, so the worktree
  removal is skipped and `git branch -D` fails harmlessly — but execution still reaches
  `git push --delete`, **which succeeds**, and only then dies at the postcondition. Every existing
  cleanup test invokes from the main root, so the entire class is untested.
- **D2 — `preflight` from a linked worktree mints a nested metadata worktree.** Observed live during
  0073's finalize: a real `<repo>/.docket/.docket` worktree, detached at a stale commit, into which
  a later render wrote an **empty** `BOARD.md`. Every command exited 0; the debris was visible only
  in `git worktree list`.
- **D3 — cleanup deletes the agent's own CWD.** `git worktree remove --force` succeeds, but the
  agent's *next* Bash call cannot start. No script can fix this — a child cannot change its parent's
  CWD — so this half is irreducibly skill-side.

Two scripts in this repo already solve the root cause (`sync-integration-branch.sh:40-51`,
`disable-worktree-hooks.sh:34`) with the main-worktree idiom, one of them carrying a comment saying
exactly why. The scripts already know how to ask git where they are; they just don't, everywhere
else.

## What changes

Anchor the repo root to the **main worktree**, everywhere, and make the two destructive paths
fail-closed rather than half-destructive:

- **Resolver** (`docket-config.sh`) — resolve the repo from the main worktree rather than CWD, so a
  script invoked from `.docket/`, a `.worktrees/<slug>` tree, or any subdirectory targets the same
  primary root. Emit a `REPO_ROOT` literal in the **`plain` format only**.
- **Preflight** (`scripts/lib/docket-preflight.sh`) — build the metadata-worktree path absolute,
  pass `-C`, and guard: refuse when the computed target would land *inside* an existing worktree.
  The ensure step becomes idempotent with respect to CWD.
- **Cleanup** (`cleanup-feature-branch.sh`) — absolute target, plus a fail-closed refusal when the
  caller's CWD is at or inside the target, placed before **both** the worktree removal and the
  remote delete.
- **Finalize posture** (`docket-finalize-change`) — run the merge, metadata updates, and cleanup
  from the durable root (the `REPO_ROOT` literal), so removing `.worktrees/<slug>` cannot yank the
  agent's CWD out from under the run. The gate's suite run stays in the feature worktree.
- **The `docket-status.sh:363` landmine** — that artifacts-refresh commit block is *currently dead*
  (its pathspec matches nothing under a relative `$mw`). Anchoring brings it alive for the first
  time, including an early-`return` that would abandon `terminal-publish` and `cleanup`. Budgeted
  and tested here, not discovered in production.
- **Regression tests** for each defect — including cleanup invoked from all three CWD classes, which
  is the assertion that would have caught D1.

## Out of scope

- Changing whether cleanup is part of finalize (it already is).
- Broadening the `.worktrees/<slug>` provenance guard.
- Redesigning the finalize consent / merge-authorization model.
- Any change to what the metadata worktree *is*, where it lives, or the branch model.
- Automatic removal of an already-stray nested worktree — destructive; left to the human.

## Open questions

- Should a script invoked from a non-primary worktree silently retarget the primary root, or warn?
  Silent retargeting is friendlier but hides genuine caller bugs like the one that produced D2.
  (Cleanup is settled either way: it *refuses*, per the spec.)

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- 2026-07-14 (scope merge) — Change **0076** (`cwd-independent-repo-root-resolution`) was minted
  concurrently with this stub's auto-groom and claimed the script-level resolver fix, which made the
  boundary between the two a live human decision; `docket-auto-groom` therefore abstained at
  emission rather than re-scope autonomously. The human resolved it by **merging**: 0075 carries the
  whole root-anchor fix and 0076 was killed as folded-in. The change was renamed from
  `finalize-safe-cwd-before-cleanup` (that slug described only the skill-posture half), its priority
  raised to `high` (D1 is live partial data loss), and the critic-verified design emitted as the
  linked spec.
- 2026-07-14 (auto-groom) — Two of the original stub's premises were wrong and are corrected above:
  (1) "step 4 correctly removes that worktree" — from a linked-worktree CWD, cleanup does **not**
  remove it; it deletes the *remote* branch and exits 1 (D1). (2) "the primary integration checkout
  **or** the `.docket/` metadata worktree" — `.docket/` is **not** a safe durable root (it is itself
  a linked worktree, and running preflight from inside it is what triggers D2); the primary checkout
  is the only one.
