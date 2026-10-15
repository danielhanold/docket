---
id: 551
slug: 'check-out-a-change-built-on-another-machine-into-its-own-doc'
title: 'Check out a change built on another machine into its own docket worktree'
status: 'in-progress'
priority: 'medium'
type: 'chore'
created: '2026-10-15'
updated: '2026-10-15'
depends_on: []
stacked_on:
related: [368, 429]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-15-check-out-a-change-built-on-another-machine-into-its-own-doc-design.md'
plan: 'docs/superpowers/plans/2026-10-15-check-out-a-change-built-on-another-machine-into-its-own-doc.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/check-out-a-change-built-on-another-machine-into-its-own-doc'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-15T02:35:29Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-15-check-out-a-change-built-on-another-machine-into-its-own-doc-design.md](../../superpowers/specs/2026-10-15-check-out-a-change-built-on-another-machine-into-its-own-doc-design.md) |
| Plan | [2026-10-15-check-out-a-change-built-on-another-machine-into-its-own-doc.md](../../superpowers/plans/2026-10-15-check-out-a-change-built-on-another-machine-into-its-own-doc.md) |
<!-- docket:artifacts:end -->

## Why

Finalize only works on the machine whose docket created the change's worktree. When a PR was built on another machine, or another tool checked the branch out (for example `gh pr checkout` or a hand-made `git worktree add`), finalize stops with a misleading `workspace-probe-failed` ("failed to start git"), and nothing docket offers can recover. `workspace prepare` refuses a branch that already exists on the remote, and merge needs a local worktree head even with `finalize.gate: off`. A downstream change 21 hit this: its foreign worktree was clean and identical to the PR, yet finalize could not proceed without hand surgery.

## What changes

- New `docket workspace checkout --id <id>` command. It puts an `implemented` change into its own docket-owned worktree at `.worktrees/<slug>` on any machine, checked against the head the PR and the remote branch agree on. It attaches to a branch that exists only on the remote, reuses or fast-forwards a local branch that is behind, and removes a foreign worktree only when it is provably clean and at that head. Everything else is blocked with a plain remedy.
- The finalize skill runs this command before any step that reads the worktree, in every gate mode.
- `finalize rebase` reports a missing or foreign worktree as `workspace-not-ready` with the real reason, never as a git start failure.
- An ADR records when docket may adopt local state it did not create.
- The guide explains how to work on a change built on another machine.

## Out of scope

- Continuing an `in-progress` change on another machine.
- Repairing a broken docket-owned worktree.
- Any change to `workspace prepare`.
- Cleaning up foreign worktrees that do not hold the change's branch.

## Reconcile log

### 2026-10-15

2026-10-15: Reconciled against main a9321f6e3. The spec was groomed today and its premises still hold: `FinalizeRebase` still calls `RebaseState` in `rc.wsDir` before `requireReadyWorkspace` (internal/app/finalize_rebase.go), and no `workspace.checkout` operation exists in the catalog. Recent main work (metadata-checkout follow, groom priority) touches neither the workspace package nor finalize rebase. No scope change.
