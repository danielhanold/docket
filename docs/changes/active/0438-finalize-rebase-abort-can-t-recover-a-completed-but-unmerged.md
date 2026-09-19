---
id: 438
slug: 'finalize-rebase-abort-can-t-recover-a-completed-but-unmerged'
title: 'finalize.rebase-abort can''t recover a completed-but-unmerged rebase whose base later moved'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-19'
updated: '2026-09-19'
depends_on: []
stacked_on:
related: [368]
discovered_from: []
adrs: []
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
<!-- docket:artifacts:end -->

## Why

On change 0368 (PR #313), an interrupted finalize attempt left an orphaned rebase receipt: an earlier finalize.rebase hit a conflict, ran the resolver once, and COMPLETED the rebase against the then-current main, but the attempt was interrupted before merge/publish. main then advanced further. A later finalize run's finalize.rebase correctly refused to adopt the now-stale base (blocked base-moved-under-receipt), but finalize.rebase-abort also failed (blocked abort-restore-failed, "no rebase in progress") because AbortRebase (internal/gitcli/rebase.go:258) only knows how to unwind an in-progress git rebase, not restore orig_head after a completed-but-unmerged rebase whose base later moved. No catalog operation can clear the orphaned state, so finalize loops on base-moved-under-receipt until a human manually resets the branch to the receipt's orig_head, deletes refs/docket/finalize/<id>/orig and refs/docket/finalize/<id>/base, and removes the stale .git/docket/workspaces/<hash>/rebase-receipt.json by hand. Secondary finding: finalize.block returned invalid-input/"empty commit subject" instead of a clean no-op when called against a change that already has a recorded ## Finalize blocked section for the same attempt token — the idempotency path is not clean.

## What changes

Add a catalog operation (or extend finalize.rebase-abort) that can detect and autonomously recover the "completed-but-unmerged rebase with a since-moved base" state: restore the branch to the receipt's orig_head, clear the owned anchor refs, and remove the stale receipt, without a human hand-editing Docket-owned git state. Also make finalize.block's idempotency path a clean no-op when an identical ## Finalize blocked record for the same attempt token already exists, instead of returning invalid-input.

## Out of scope

Design of the recovery operation's exact mechanics/schema; not fixing 0368 itself (handled separately by manual recovery).
