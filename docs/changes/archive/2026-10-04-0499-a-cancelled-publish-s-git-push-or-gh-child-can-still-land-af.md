---
id: 499
slug: 'a-cancelled-publish-s-git-push-or-gh-child-can-still-land-af'
title: 'A cancelled publish''s git push or gh child can still land after cancel'
status: 'killed'
priority: 'medium'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [494]
discovered_from: [494]
adrs: [137]
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
| ADRs | [ADR-0137](../../adrs/0137-the-publish-journal-blocks-only-on-a-publisher-that-may-stil.md) |
<!-- docket:artifacts:end -->

## Why

Change 0494 made the publish journal stop blocking once the publisher (`pr.publish` / `workspace.publish`) is provably gone. Killing the docket publisher process, though, does not kill the `git push` or `gh` child it started. That child can still push the branch or edit the PR after `run.cancel` has reported `cancelled`, so a cancelled run can still change the remote. 0494 documented this in the glossary and ADR-0137 but did not fix it in code.

## What changes

Make sure a publisher's external child processes (`git push`, `gh`) cannot outlive a cancel or a publisher death unnoticed. For example, run them in the publisher's process group so cancel's teardown reaches them, or have the abandonment check also confirm the children are gone. Then `cancelled` and `mutation-abandoned` mean no publish mutation can still land.

## Out of scope

Rolling back a mutation that already landed on the remote. Changing the journal's blocking rule from 0494/ADR-0137 except where needed to account for the children. Other operations' child processes outside the two publish operations.

## Why killed

Decided against at grooming on 2026-10-04, after tracing main at 20bc0a36a. The behavior stays as documented in the glossary (`mutation-abandoned`) and the ADR-0137 Update note.

- **Cancel never kills the publisher.** `reconcileRunTeardown` stops only registered gate-scope and raw-run participants; the native-task adapter is unwired (`productionCancelSeams`). A publisher dies only from outside causes.
- **The common outside causes take the child down too.** `gitcli` and `githubcli` start `git`/`gh` with no `Setpgid`, so the child shares the publisher's process group and Ctrl-C reaches both. Stopping a Claude Code Bash task also killed its child process (checked empirically while grooming). An orphan needs the docket process alone to die: a targeted `kill` of its pid, a crash, or the OS killing just that process.
- **A late landing converges.** The push is a lease push, so it cannot overwrite a newer remote head. GitHub refuses a second open PR for the same head, and `EnsurePullRequest` adopts what landed. The one leftover is an orphaned `gh pr edit` re-applying an old title or body. That needs a resumed run to reach its own edit within the seconds the orphan lives.
- **The stub's first idea conflicts with an existing rule.** Docket never signals a process group once its owner is dead, because the group id may have been reused (the reason 0492 made `tree-survives` informational).
- **The best fix was not worth its cost.** Passing the publish lock to the `git push`/`gh` child would make a free lock prove the child gone too. But it adds a new way for cancel to stay pending: a long-lived grandchild (for example git's credential-cache daemon on https remotes) would hold the lock until it exits. Recording the child's pid instead adds a journal write after spawn, leaves a gap, and has the same reuse ambiguity.

Revisit only if an orphaned publish child is ever observed landing after cancel.
