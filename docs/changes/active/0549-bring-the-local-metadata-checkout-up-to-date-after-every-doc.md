---
id: 549
slug: 'bring-the-local-metadata-checkout-up-to-date-after-every-doc'
title: 'Bring the local metadata checkout up to date after every docket write'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-14'
updated: '2026-10-14'
depends_on: []
stacked_on:
related: [523, 531, 360]
discovered_from: []
adrs: [142, 89]
spec: 'docs/superpowers/specs/2026-10-14-bring-the-local-metadata-checkout-up-to-date-after-every-doc-design.md'
plan: 'docs/superpowers/plans/2026-10-14-bring-the-local-metadata-checkout-up-to-date-after-every-doc.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/bring-the-local-metadata-checkout-up-to-date-after-every-doc'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-14T23:34:41Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-14-bring-the-local-metadata-checkout-up-to-date-after-every-doc-design.md](../../superpowers/specs/2026-10-14-bring-the-local-metadata-checkout-up-to-date-after-every-doc-design.md) |
| Plan | [2026-10-14-bring-the-local-metadata-checkout-up-to-date-after-every-doc.md](../../superpowers/plans/2026-10-14-bring-the-local-metadata-checkout-up-to-date-after-every-doc.md) |
| ADRs | [ADR-0142](../../adrs/0142-private-visibility-keeps-the-single-metadata-layout-and-vari.md), [ADR-0089](../../adrs/0089-shared-metadata-worktree-contention-survivable-not-impossible.md) |
<!-- docket:artifacts:end -->

## Why

Every typed metadata operation (create, groom, claim, implemented, finalize, kill, and the rest) commits in a throwaway transaction worktree and pushes straight to the metadata remote. It never moves the local metadata branch or the metadata checkout on disk. Only `repository prepare` fast-forwards that checkout (#523), and it runs at the start of the next workflow or `docket open`.

In a shared repository nobody notices, because the board is read on GitHub and GitHub reads the pushed branch. In a private repository the checkout under `~/.local/share/dckt/<owner>-<repo>/checkouts/<clone-id>/` is the only place a human can read the board. So after finalize regenerates BOARD.md, the file on disk still shows the old state until something runs prepare. To the human it looks like private mode forgot to refresh the board.

Evidence (2026-10-14): this repo's `.docket` checkout and local `docket` branch sat one commit behind `origin/docket` for two days after the 0545 kill, until the next prepare. In the private `dckt-test` repo every board-visible operation did rewrite BOARD.md on the branch, so the board on the branch is correct. Only the local copy lags.

The human also asked for a command to refresh the board. `docket open board` already syncs the checkout and opens BOARD.md, and `repository repair` re-renders a board that has drifted on the branch itself, which is a different problem. Once writes keep the local copy current, a dedicated refresh command may not be needed.

## What changes

- After a docket write's push lands, move the local metadata branch and checkout (shared `.docket/`, private `checkouts/<clone-id>/`) forward to the pushed commit, reusing the in-place fast-forward from #523.
- One optional after-apply hook on the transaction engine, wired where the engine is built, so every typed write is covered; `repository repair`, which pushes outside the engine, calls the same step.
- Best-effort and silent: a checkout with edits, its own commits, or one already moved further is left alone, and the next `prepare` handles it as today.
- Same behavior in shared and private repositories. No new command: `docket open board` stays the manual way to sync and view the board.
- One sentence in the metadata-location guide describing the new behavior.

## Out of scope

- Changing how the board is rendered or when an operation decides to rewrite it; generation is already correct on the branch.
- Changing `repository repair`'s job (repairing a board that drifted on the branch).
- Moving transactions off their throwaway worktree.

## Reconcile log

### 2026-10-14

2026-10-14: Reconciled against current main (50420ee). Traced the cited seams: transaction.Engine still carries only AdmissionHook (engine.go), newPlanningDepsOver in internal/cli/change.go is still the single engine assembly point, gitcli.Client.FastForwardCheckedOutBranch exists and is already used by repository prepare, and executeRepositoryRepair still pushes outside the engine. No overlapping work landed since grooming; scope and spec unchanged.
