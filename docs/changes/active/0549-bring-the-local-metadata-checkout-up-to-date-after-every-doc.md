---
id: 549
slug: 'bring-the-local-metadata-checkout-up-to-date-after-every-doc'
title: 'Bring the local metadata checkout up to date after every docket write'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-14'
updated: '2026-10-14'
depends_on: []
stacked_on:
related: [523, 531, 360]
discovered_from: []
adrs: [142]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0142](../../adrs/0142-private-visibility-keeps-the-single-metadata-layout-and-vari.md) |
<!-- docket:artifacts:end -->

## Why

Every typed metadata operation (create, groom, claim, implemented, finalize, kill, and the rest) commits in a throwaway transaction worktree and pushes straight to the metadata remote. It never moves the local metadata branch or the metadata checkout on disk. Only `repository prepare` fast-forwards that checkout (#523), and it runs at the start of the next workflow or `docket open`.

In a shared repository nobody notices, because the board is read on GitHub and GitHub reads the pushed branch. In a private repository the checkout under `~/.local/share/dckt/<owner>-<repo>/checkouts/<clone-id>/` is the only place a human can read the board. So after finalize regenerates BOARD.md, the file on disk still shows the old state until something runs prepare. To the human it looks like private mode forgot to refresh the board.

Evidence (2026-10-14): this repo's `.docket` checkout and local `docket` branch sat one commit behind `origin/docket` for two days after the 0545 kill, until the next prepare. In the private `dckt-test` repo every board-visible operation did rewrite BOARD.md on the branch, so the board on the branch is correct. Only the local copy lags.

The human also asked for a command to refresh the board. `docket open board` already syncs the checkout and opens BOARD.md, and `repository repair` re-renders a board that has drifted on the branch itself, which is a different problem. Once writes keep the local copy current, a dedicated refresh command may not be needed.

## What changes

- After a typed metadata operation's push applies, bring the local metadata branch and checkout up to the pushed commit, reusing prepare's in-place fast-forward from #523 (no hooks, refuses rather than discards).
- Best-effort: if the local copy can't be fast-forwarded (dirty, ahead, locked), the operation still succeeds and the next `prepare` handles it as it does today.
- Same behavior in shared and private repositories.
- Decide at grooming whether any dedicated board-refresh command is still warranted, given `docket open board` and the above.

## Out of scope

- Changing how the board is rendered or when an operation decides to rewrite it; generation is already correct on the branch.
- Changing `repository repair`'s job (repairing a board that drifted on the branch).
- Moving transactions off their throwaway worktree.
