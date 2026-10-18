---
id: 568
slug: 'add-a-typed-way-to-clear-branch-on-a-proposed-change'
title: 'Stop deferring a change that is already being built'
status: 'in-progress'
priority: 'low'
type: 'fix'
deliverable: 'pr'
created: '2026-10-18'
updated: '2026-10-18'
depends_on: []
stacked_on:
related: [563, 565]
discovered_from: [563]
adrs: []
spec: 'docs/superpowers/specs/2026-10-18-add-a-typed-way-to-clear-branch-on-a-proposed-change-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/add-a-typed-way-to-clear-branch-on-a-proposed-change'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-18T16:23:26Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-18-add-a-typed-way-to-clear-branch-on-a-proposed-change-design.md](../../superpowers/specs/2026-10-18-add-a-typed-way-to-clear-branch-on-a-proposed-change-design.md) |
<!-- docket:artifacts:end -->

## Why

Any change can be deferred today, including one that is being built (`in-progress`). Deferring keeps everything the build recorded (`branch:`, claim time, plan, results), and reviving it later puts those leftovers on a proposed change. That is the only normal way a proposed change ends up with a `branch:`, and it leads to dead ends in grooming: `change.groom` then refuses to restack it (`not-restackable`, from #563) or retitle it (`not-retitleable`), and no typed operation clears `branch:`. Deleting the git branch doesn't help. No docket workflow defers a change mid-build; only a person typing `docket change defer` can, and it has never happened in this backlog. Closing that path removes the dead ends at their source, so nothing needs a new "clear branch" operation.

## What changes

A change that is being built can only be finished, blocked, or killed, never deferred. `change.defer` on an in-progress change refuses and writes nothing. The refusal gets its own finding code and plain instructions: block it with a reason to pause it and keep its branch and work (`change.unblock` resumes it), or kill it to drop it (its git branch and worktree stay behind to delete by hand). The four lifecycle commands (block, defer, unblock, revive) also print a refusal's message in terminal output, not just `invalid-state`. Kill also owns up to what it leaves: when the killed record had a `branch:`, the kill result warns, naming that branch and saying its local and remote branch and any worktree are left in place to review and delete by hand. The warning shows in terminal output too. The lifecycle docs and the code comments that describe the old path are updated. Deferring a proposed change is unchanged.

## Out of scope

A typed operation to clear `branch:`. Changing the `not-restackable` and `not-retitleable` refusals or their wording; they stay as a safety net for hand-edited or legacy records. What block, unblock, kill, reclaim, revive, or complete do. A new impact/remedy pipeline for typed-operation refusals. The killed-stack-parent policy (#565).

## Reconcile log

### 2026-10-18

2026-10-18: Re-read against current main (7df884687). domain.Defer still accepts proposed and in-progress; the convention Rules sentence still says deferred may be entered from in-progress; related #563 is done and #565 still proposed and separate. Spec holds as written; no scope change.
