---
id: 76
slug: cwd-independent-repo-root-resolution
title: Resolve the repo root independently of CWD — preflight run inside `.docket` mints a nested metadata worktree
status: proposed
priority: medium
created: 2026-07-14
updated: 2026-07-14
depends_on: []
related: [75]
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

Observed live during change 0073's finalize (2026-07-14). The skill's harvest step was run with the
Bash CWD set to the metadata worktree (`cd <repo>/.docket && docket.sh preflight`). Preflight's
"ensure the persistent `.docket/` metadata worktree exists" logic resolves the repo root from CWD,
so it resolved the root as `<repo>/.docket` — already a worktree — and, finding no `.docket/`
beneath it, **created one**: a nested `<repo>/.docket/.docket` worktree, registered in
`.git/worktrees/` as `-docket1`, detached at a stale commit, into which a later render wrote an
**empty** `BOARD.md` (`0 changes`).

Nothing was corrupted (the real board on `origin/docket` was verified intact and the stray tree held
no unique commits), but only because the empty board was rendered into the stray tree rather than the
real one. The failure is silent: every command exits 0, and the debris is only visible in
`git worktree list`.

The root cause is not finalize-specific. **Any** docket script that derives the repo root from CWD
misbehaves when invoked from a non-primary worktree, and every skill's Step-0 runs preflight. A
stray `cd` — from an agent, a hook, or a human — is enough. The scripts already know how to ask git
where they are; they just don't.

This is the script-level counterpart to #75. That change hardens *finalize's operational posture*
(run from a durable checkout so cleanup can't delete its own CWD); this one hardens *the scripts* so
no caller's CWD can mislead them. They are complementary, and this one supplies a fact #75's open
questions need: `.docket/` is **not** a safe "durable root" today, because running preflight from
inside it is precisely what triggers this bug.

## What changes

- Resolve the repo root from git rather than from CWD (e.g. the common dir / main worktree, not
  `pwd`), so a script invoked from `.docket/`, a `.worktrees/<slug>` tree, or any subdirectory
  targets the same primary root.
- Make the metadata-worktree ensure step recognize that it is **already inside** a worktree of this
  repo and resolve to the existing `.docket/` rather than minting a nested one — the operation
  should be idempotent with respect to CWD.
- Consider a guard: refuse (or self-correct) when the computed metadata-worktree path would land
  *inside* an existing worktree, since `<repo>/.docket/.docket` is never a legitimate target.
- Add a regression test that runs the Step-0 preflight with CWD set to the metadata worktree and
  asserts no second worktree is created.

## Out of scope

- Finalize's durable-checkout posture and the cleanup-deletes-my-CWD hazard — that is #75.
- Any change to what the metadata worktree *is*, where it lives, or the branch model.
- Broadening the `.worktrees/<slug>` provenance guard that governs cleanup.

## Open questions

- Fix at the shared resolver (one place, protects every script) or per-script? The shared resolver
  is the obvious candidate, but the blast radius wants checking against the facade's callers.
- Should a script invoked from a non-primary worktree silently retarget the primary root, or warn?
  Silent retargeting is friendlier but hides genuine caller bugs like the one that produced this.
- Should cleanup of a *stray* nested worktree be automatic, or always left to the human? (Removing a
  worktree is destructive and the provenance guard deliberately refuses paths outside
  `.worktrees/<slug>`.)

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
