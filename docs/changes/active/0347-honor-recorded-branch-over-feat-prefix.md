---
id: 347
slug: honor-recorded-branch-over-feat-prefix
title: Honor recorded branch names instead of reconstructing feat/<slug>
status: proposed
priority: medium
type: fix
created: 2026-08-25
updated: 2026-08-25
depends_on: []
related: [327, 336]
discovered_from: []
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

`domain.BranchForSlug` is a one-liner: `"feat/" + slug`. Claim stamps that name. Workspace
prepare cuts that ref. Finalize then **rebuilds** the same string to find the PR, rebase,
retarget children, close out a stack parent, and delete the feature branch.

The change file already has `branch:` (and `pr:`). Stack-base resolution already trusts a
live parent's **recorded** branch (domain stack rule 4; `stack-base.sh`). The board renders
`c.Branch()`. Reclaim prefers the recorded name and only falls back to `feat/<slug>`.
Finalize does not.

Live failure, 2026-08-25, in a Git Flow consumer (`feature/…`, not `feat/…`):

- Change 0001 recorded `branch: feature/keda-qarch-consumer-scaledobjects` and
  `pr:` pointing at an **open** GitHub PR (head `feature/…`, base `feature/eks-consumers`).
- Stack-base for that child correctly resolved to `feature/eks-consumers`.
- `docket context finalize --id 1` skipped with **`pr-closed`**. Explicit `--id` cannot
  override that. No rebase, no merge.
- GitHub still had the PR open. Finalize probed `FindOpenPullRequestsByHead(feat/<slug>)`,
  found nothing, and treated a clean miss as closed (`finalize_context.go` `ProbePR`).
  It parsed the PR number from `pr:` and then ignored it on the open path.

This is not "only finalize," but finalize is where it **hurts**. A fresh `implement-next`
claim would mint `feat/<slug>` and never notice. Divergence happens when a human (or CI
workaround) records a real branch that is not the constructor's spelling: Git Flow
`feature/`, a renamed head, a long-lived stack-root branch that is not `feat/<parent-slug>`.
Those are legal git names. Docket already stored them. Go then pretended they did not exist.

## What changes

Treat a present `branch:` as the feature-head source of truth for every Go operation that
currently calls `BranchForSlug` on a **landed** record (finalize context/probe/merge/retarget/
closeout/cleanup; implementation context; workspace target when resuming an existing
claim). Keep `BranchForSlug` as the **mint default** when claim first writes `branch:`,
unless design later adds a configurable prefix.

`ProbePR` already has the PR number. An open PR should be read by that number (as the
merged path already is), not by a reconstructed head. A head mismatch against recorded
`branch:` is a data defect to surface, not a silent `pr-closed`.

## Out of scope

- Forcing every consuming repo onto Conventional Branch `feat/`
- A CircleCI / Docker-tag sanitizer in a consumer repo
- Changing `integration_branch` or stacked-on merge policy (ADR-0092)
- Inventing a new lifecycle state

## Open questions

- Configurable mint prefix (`feat/` vs `feature/` vs Git Flow) vs recorded-name-only after
  claim? A prefix knob does not fix rename-after-claim unless recorded `branch:` wins.
- Should `workspace.NewTarget` stay derived-from-slug for a brand-new cut, and only honor
  recorded `branch:` when the remote ref already exists?
- Is a recorded name that is not `feat/<slug>` a supported first-class case, or an error
  until renamed? The live stack-root (`feature/eks-consumers` on a hand-minted parent)
  needs the first-class case or stacking on existing long-lived branches cannot close out.
