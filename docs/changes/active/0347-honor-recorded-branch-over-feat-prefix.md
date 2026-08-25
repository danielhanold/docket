---
id: 347
slug: honor-recorded-branch-over-feat-prefix
title: Honor recorded branch names instead of reconstructing feat/<slug>
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-25
updated: '2026-08-25'
depends_on: []
related: [316, 327, 336, 344]
discovered_from: []
adrs: [35, 92, 97]
spec: docs/superpowers/specs/2026-08-25-recorded-branch-identity-design.md
plan: 'docs/superpowers/plans/2026-08-25-honor-recorded-branch-over-feat-prefix.md'
results:
trivial: false
auto_groomable:
branch: 'feat/honor-recorded-branch-over-feat-prefix'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-25T23:46:49Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-25-recorded-branch-identity-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-25-recorded-branch-identity-design.md) |
| Plan | [2026-08-25-honor-recorded-branch-over-feat-prefix.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-25-honor-recorded-branch-over-feat-prefix.md) |
| ADRs | [ADR-0035](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0035-cleanup-teardown-fail-closed.md), [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0097](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0097-pr-identity-is-verified-by-parsed-pr-number.md) |
<!-- docket:artifacts:end -->

## Why

Claim, workspace, and finalize currently share a hidden assumption that every feature branch is
`feat/<slug>`. The Go runtime reconstructs that name even though the change record already carries
the actual branch in `branch:`.

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

This is not only a finalize defect. Implementation context, workspace resume, stack operations,
and cleanup can also act on a reconstructed name. Divergence is legitimate in Git Flow
repositories, on human-minted stack roots, after an approved rename, and whenever a change needs a
one-off prefix. Docket must mint a name once, record it, and consume that record thereafter.

## What changes

Make recorded `branch:` the feature-head source of truth for every post-claim operation. Pass it
explicitly through implementation context, workspace targets, PR operations, finalize, stack
retarget/closeout, and cleanup; missing or conflicting identity must stop before effects.

Replace the `feat/<slug>` mint rule with `<type>/<slug>`. Add optional per-change
`branch_prefix:`—captured from a human's natural-language instruction during change creation—to
override the type for that claim. Retain the field across reclaim, but make it inert after claim.

Read finalize's exact recorded PR by number rather than discovering it by head. A missing branch or
PR-head mismatch becomes an interactive repair checkpoint: the human may adopt the exact PR head,
supply a correct PR matching the recorded branch, or abort. Repairs are version-pinned, reload and
re-probe before continuing, and never search for a likely matching branch or PR. An existing Docket
workspace that still names the other branch blocks repair; renaming or migrating that checkout is
out of scope.

## Out of scope

- Forcing every consuming repo onto Conventional Branch `feat/`
- A repository-wide branch-prefix configuration
- Searching for a branch or PR that merely resembles the change slug
- Renaming branches or migrating an existing Docket workspace to another branch
- A CircleCI / Docker-tag sanitizer in a consumer repo
- Changing `integration_branch` or stacked-on merge policy (ADR-0092)
- Inventing a new lifecycle state

## Reconcile log

### 2026-08-25

2026-08-25 — Reconciled at claim time against current `main`/`docket` reality. Confirmed every premise still holds in the Go runtime: `domain.BranchForSlug` is reconstructed at all the sites the spec names (`implementation_context.go`, `finalize_context.go` context + block, `finalize_retarget.go`, `finalize_cleanup.go`, `finalize_closeout.go`, `finalize_merge.go`, `change_reclaim.go`), `FindOpenPullRequestsByHead` is still the head-discovery path in `finalize_publish.go`/`pr_publish.go`/`change_implemented.go`/`finalize_block.go`, and `finalize_context.ProbePR` still takes a reconstructed `headBranch`. The live `pr-closed` misclassification path (a clean head-search miss read as closed) is unchanged. Related changes 316/327/336/344 and ADRs 35/92/97 remain the correct context. No scope adjustment, no drops; proceeding to plan under the spec as written.
