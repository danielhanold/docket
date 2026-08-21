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
plan:
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
