---
id: 35
slug: cleanup-teardown-fail-closed
title: docket's feature-branch teardown is fail-closed, never half-destructive
status: Accepted
date: 2026-07-14
supersedes: []
reverses: []
relates_to: [34]
change: 75
---

## Context

`cleanup-feature-branch.sh` tears down a finished change's feature worktree and its
local + remote `feat/<slug>` branch. Before change 0075 it was structured as a
best-effort sequence that could destroy irreversibly and still report failure: from a
linked-worktree CWD its CWD-relative target never resolved, so the worktree removal was
skipped, `git branch -D` fell into `|| true`, and execution still REACHED
`git push --delete` — which SUCCEEDED, destroying the remote branch — before dying at a
postcondition with exit 1. The operator saw a failure and an already-deleted remote
branch (unrecoverable; the repo forbids force-push to main and the branch was gone). A
related shape survives on the `--worktrees-dir` override path: a target that does not
exist while the local branch is still checked out elsewhere would skip removal, fall
through `|| true`, and still delete the remote. Separately,
`git worktree remove --force` succeeds while the caller's own CWD is inside the target —
orphaning the caller's CWD so its next command cannot start, after the destructive step
already landed.

## Decision

Destructive teardown is fail-closed — it refuses cleanly rather than performing a
partial, irreversible action. Three concrete invariants:

1. **REFUSE BEFORE ANY DESTRUCTIVE STEP when the caller's CWD is at or inside the target
   worktree** — the refusal is placed before BOTH the worktree removal and the remote
   delete, and `caller_pwd` is captured before any `cd` (a `cd` first would compare the
   root against itself and the guard could never fire). Both sides of the comparison are
   canonicalized (`pwd -P`) because `git worktree list` prints realpaths.
2. **NEVER DELETE THE REMOTE BRANCH WHILE THE LOCAL BRANCH STILL EXISTS** — after the
   `git branch -D` attempt, if `feat/<slug>` still resolves locally, die before
   `git push --delete`. The remote stays intact and the operator can re-run after
   resolving why the local branch survived (usually: still checked out in a worktree).
3. The existing `.worktrees/<slug>` provenance guard (refuse to remove a worktree
   outside `<root>/.worktrees/`) is retained unchanged and not broadened.

Rationale: given the pre-0075 behavior, refusing takes away nothing that worked — from
these states the old script destroyed the remote branch and failed anyway. Fail-closed
converts a partial irreversible destruction that reports failure into a clean,
recoverable stop. The skill-side complement (recorded here as the reason the script
guard is a backstop, not the whole fix): `docket-finalize-change` runs its close-out
from the durable REPO_ROOT so the agent is never standing inside the worktree cleanup
removes — a child process cannot change its parent's CWD, so this half is irreducibly
skill-side.

## Consequences

Enables: cleanup is now safe to invoke from any CWD (from `.docket/` it runs the full
happy path; from inside the target it refuses with the remote intact), and the whole CWD
class is now regression-tested (previously every cleanup test ran from the main root —
the one CWD where the relative target happened to resolve — so the entire failure class
was untested). Costs: a caller standing inside the target, or with a still-checked-out
local branch, now gets an explicit refusal it must act on (cd to the repo root, or
remove the lingering worktree) rather than an opaque partial failure — a louder stop,
deliberately. Gives up: nothing that previously worked.
