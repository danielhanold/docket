---
id: 336
slug: finalize-s-go-merge-verb-hardcodes-merge-honor-the-repo-s-al
title: 'Finalize selects the best merge method permitted by repository and branch policy'
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-21
updated: '2026-08-21'
depends_on: []
stacked_on:
related: [316, 327, 330]
discovered_from: [316]
adrs: [10, 11, 43]
spec: docs/superpowers/specs/2026-08-21-finalize-effective-merge-method-design.md
plan: 'docs/superpowers/plans/2026-08-21-finalize-effective-merge-method.md'
results:
trivial: false
auto_groomable:
branch: 'feat/finalize-s-go-merge-verb-hardcodes-merge-honor-the-repo-s-al'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-21T19:20:59Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-21-finalize-effective-merge-method-design.md` |
| Plan | `docs/superpowers/plans/2026-08-21-finalize-effective-merge-method.md` |
| ADRs | `docs/adrs/0010-finalize-merge-gate-split-agents.md`, `docs/adrs/0011-finalize-consent-model.md`, `docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md` |
<!-- docket:artifacts:end -->

## Why

docket's Go finalize runtime always invokes `gh pr merge --merge`. Repositories that disable merge
commits reject the final effect even when rebase or squash is allowed. This repository permits
rebase and squash but not merge commits, so the defect blocked change 0330's closeout until its PR
was merged manually with `--rebase`.

The documented CLI workflow also promises that an omitted `--repo-dir` means the current directory,
but handlers pass an empty string into application operations and GitHub discovery rejects it.
`context finalize` consequently reports real PRs as `pr-unknown`, and finalize verbs cannot reach
the method-selection path without an explicit flag the skill does not supply. Both seams must be
closed for the rebase-first finalize path to work end to end.

## What changes

Fulfil the advertised `--repo-dir` default through one shared CLI-boundary resolver used by every
command that promises current-directory behavior.

Before the single merge effect, read repository-enabled methods and the active rules for the PR's
actual base branch, intersect them, and select the first available method in the fixed order
**rebase → merge commit → squash**. Add no config knob. A known empty intersection blocks before any
merge with a specific `merge-method-unavailable` reason; an unobservable policy remains unknown.

Attempt exactly one selected method, report which method Docket attempted, and never interpret a
generic GitHub denial as permission to try another mutation. Preserve exact-head matching,
authorized `--admin`, authoritative reprobe, destination reachability verification, and the rest of
the finalize sequence unchanged. Verify all three merge graph shapes without assuming that the
result is a two-parent merge commit.

## Out of scope

- Changing repo-side merge-method configuration.
- Adding a merge-method config knob or configurable priority order.
- Retrying a lower-priority method after GitHub rejects an attempted merge.
- Reworking the broader finalize sequence, the rebase/gate/publish steps, or the merged-recovery
  archive path — only the merge-method selection is in scope.

## Reconcile log

### 2026-08-21

2026-08-21 — Reconciled at claim. Confirmed against current `docket` HEAD: `internal/githubcli/merge.go` `MergePullRequest` still hardcodes `--merge` (line ~159), and the empty-`--repo-dir` seam is live (a `docket change claim` without `--repo-dir` fails `gitcli discover: invalid-request: invocation path is empty`, reproduced during this claim). No `MergeMethod` type, repository/branch capability probe, or effective-set selection exists yet. Related changes 316, 330, 331 are all `done` (archived); their finalize-recovery, results-append, and evidence-remint work is complete and does not overlap this change's merge-method-selection scope. Spec and proposal scope are accurate as written; no section or relation edits required.

## Run halted

### 2026-08-21

Halted 2026-08-21 by docket-implement-next (Step 5 build).

## Why halted

The build reached Task 6 (e2e coverage) and surfaced a genuine spec-internal contradiction that no plan task covers and that cannot be resolved without touching a file the spec explicitly places out of scope. This needs a human scope/spec decision, not more implementation.

## The contradiction

This change makes the fixed preference order **rebase → merge commit → squash** the merge policy, so **rebase becomes the default** merge method for this repository (rebase + squash enabled, merge commits disabled). That is the spec's core intent and is already implemented and green through Task 5.

But once rebase is the effective default, `internal/app/finalize_cleanup.go` `finalizeCleanupLocalRef` blocks local feature-ref deletion:

- Line 476: `expectedTip := gitcli.ObjectID(facts.HeadOID)` — the ORIGINAL PR head.
- Line 502: `git.IsAncestor(ctx, cc.repo, expectedTip, rev.Commit)` — requires the original head to be an ancestor of the integration tip.
- Line 508: on failure, `ReasonCleanupUnreachable = "tip-not-in-merge-chain"`, the local ref is retained, and finalize cleanup returns `blocked`.

Under a rebase or squash merge the integration tip is a fresh commit chain with new object ids, so the original PR head is NOT an ancestor. Two pre-existing e2e tests (`TestE2EOrdinaryFinalize`, `TestE2ENoPathDocketDependency`) fail at `finalize cleanup = "blocked"` / `tip-not-in-merge-chain`. This is a real functional regression (the local feature branch would never be cleaned up after a rebase-first finalize), not a fake-gh artifact.

## Why this is out of scope to fix autonomously

- The spec's `## Out of scope` explicitly names "branch cleanup."
- The spec's graph-shape-independence clause is scoped to post-merge VERIFICATION (`verifyMerge`), a different code path than the cleanup ancestry check — so it does not authorize changing cleanup.
- No plan task is allocated to `internal/app/finalize_cleanup.go`.

Silently expanding scope into an explicitly out-of-scope file, inside the same run, is the scope creep the design review process exists to prevent. The build worker (docket-build-standard) correctly returned BLOCKED rather than committing; a stronger model hits the identical spec/scope wall — this is not a capability gap.

## What is already built (green, committed on feat/finalize-s-go-merge-verb-hardcodes-merge-honor-the-repo-s-al)

- Task 1 (0b24e3fe): shared `--repo-dir` current-directory resolver at the CLI boundary.
- Task 2 (62b60412): closed `MergeMethod` vocabulary + fixed-priority selection.
- Task 3 (fcdf71aa): fail-closed repository + branch-rule capability probes.
- Task 4 (20347b96): `MergePullRequest` selects the effective method behind a `MergeResult` value.
- Task 5 (a7f3fb77): app maps `merge-method-unavailable` → blocked, reports the attempted method.

`go test ./internal/githubcli/ ./internal/app/` is green at feature HEAD a7f3fb77. Task 6's own new targets pass; only the two pre-existing cleanup tests fail once rebase is the default. Task 6's e2e changes are left UNCOMMITTED in the worktree (`internal/app/finalize_e2e_test.go`) for inspection; they were not adopted.

## Recommended resolution (human decision)

Amend the spec/plan to bring the cleanup containment predicate in scope, then add a task to make `finalizeCleanupLocalRef` graph-shape-independent: key the containment proof on the recorded merge-result commit (`facts.MergeCommit`, already certified reachable by `verifyMerge`) instead of the original head — replace `IsAncestor(expectedTip, rev.Commit)` at finalize_cleanup.go:502 with `IsAncestor(gitcli.ObjectID(facts.MergeCommit), rev.Commit)`, keeping the `tip == expectedTip` local-ref-identity check (line 490) and the `DeleteLocalBranchChecked(expectedTip)` lease (line 525) as-is. Then rerun Task 6 and the full suite.
