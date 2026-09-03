---
id: 327
slug: stack-closeout-must-prove-integration-reachability
title: 'Stacked-merged close-out can stamp `done` after a stale-worktree rebase clobbers the child — prove reachability in git, not metadata'
status: proposed
priority: high
type: fix
created: 2026-08-18
updated: 2026-08-18
depends_on: []
stacked_on:
related: [298, 316]
discovered_from: []
adrs: [92]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — 2026-08-18, live close-out in `cet-terraform`. Change 13 (`stacked_on: 12`) was
finalize-merged into its parent (`gh pr merge 268 --rebase`; tip `de87420` carried the child's
tree). Seven minutes later, finalize of stack-root 12 rebased a **stale local worktree** of
`feat/alertmanager-template-configmaps` — still only the parent's original four commits, never
fetched after 268 — onto `origin/main`, `git push --force-with-lease`d it (lease compared against
the stale remote-tracking ref, so it succeeded), and merged PR 267. `stack-closeout` then promoted
13 from `stacked-merged` to `done`. `de87420` is not an ancestor of `origin/main`; the child's
catalog files are absent from the integration branch. Docket's board and archive now claim 13
shipped. It did not.

**Opportunity** — change 0298's governing invariant is already the right sentence: *`done` means
the change's code is reachable from the integration branch.* The shipped path never tests that.
`stacked-changes.md` *asserts* "a child already at `stacked-merged` … its code is in the parent's
branch and rides the merge." The parent-finalize open-children gate is `--open-only`, so a
`stacked-merged` child is invisible. The rebase-retest gate rebases the **local** `feat/<slug>`
worktree after fetching the integration branch, not the remote feature branch. `stack-closeout.sh`
promotes on metadata (child `stacked-merged` + root merged). `--force-with-lease` without a fetch
of that same ref only protects against updates the worktree has already seen. An agent can follow
finalize + stacked-changes as written and still clobber a stack-parent merge, then stamp `done` on
unreachable code.

**Independent value** — every stacked close-out is one stale parent worktree away from a false
`done` and a force-push that deletes the only copy of the child's commits on the parent branch
(the child's own `feat/<slug>` is kept until stack close-out, which is the only reason this
incident was recoverable). Prose will not hold this; the 0298 suite never mutated a parent
worktree that lagged `origin/<parent-branch>` after a child merge.

**Boundary** — git-level proofs around (1) refreshing the parent worktree after a stack-parent
merge, (2) refusing to rebase/force-push a stack root whose `stacked-merged` children's merge
tips are not ancestors of the branch about to move, (3) refusing to promote a descendant to
`done` unless its merge commit is an ancestor of `origin/<integration_branch>`. Also the
fetch-before-lease rule for the feature ref the lease is supposed to protect. Skill prose is the
trigger; the checks belong in scripts the suite can redden.

## What changes

Close the hole that lets a skill-compliant finalize of a stack root drop a `stacked-merged`
child's tree and still archive that child as `done`.

Candidate mechanical checks (design owns which script vs skill, not this stub):

1. After a stack-parent merge, the parent's worktree must match `origin/<parent-branch>` (or the
   run must refuse to use a worktree behind that ref).
2. Before rebasing a stack root, fetch the feature branch and require every `stacked-merged`
   child's merge tip (`gh pr view --json mergeCommit`) to be an ancestor of the branch about to
   be rebased. If not: halt, do not force-push.
3. Before promoting a descendant to `done`, require
   `git merge-base --is-ancestor <child-merge-commit> origin/<integration_branch>`. Metadata-only
   promotion is how 13 became `done` with no code on `main`.
4. `--force-with-lease` of a feature branch must fetch that same ref first; a lease against a
   stale tracking ref is not a concurrent-update guard.

## Out of scope

- Recovering the `cet-terraform` incident (the human is rebasing the surviving child branch
  onto current `main` and correcting that repo's docket status).
- Reopening 0298's stacking model, `stacked_on:`, or ADR-0092's base-resolution arms.
- GitHub's delete-time PR retargeting (already out of 0298).
- Batch mode / several changes on one branch (change 0158).

## Open questions

- Script helper vs additional skill prose: this incident is the evidence that prose-only will
  not hold; groom should default to a tested script, with skill lines as the trigger.
- Whether change 0316's Go finalize/stack-close-out port must land the same proofs on day one,
  or this ships in Bash first and 0316 inherits the contract.
- Whether fetch-before-lease is a general finalize-gate rule (any feature branch) or only the
  stack-root path that can race a just-merged child.
- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-target: Go closeout already proves the root's merge commit reachable from a fresh integration tip (`finalize_closeout.go`) and rebase fetches the remote head (`finalize_rebase.go`), but descendant carry in `internal/domain/stackcloseout.go` is still proven via merged-PR destinations, not git ancestry. Move to: refuse to rewrite a stack root whose stacked-merged children's merge commits are not ancestors of the head being rebased, and prove each descendant by ancestry at closeout. Drop the `stack-closeout.sh` open question.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
