---
id: 452
slug: 'same-scope-first-start-loser-must-not-rotate-the-winner-s-ex'
title: 'Same-scope first-start loser must not rotate the winner''s executing worktree slot'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-24'
updated: '2026-09-24'
depends_on: []
stacked_on:
related: [437, 446, 375, 405]
discovered_from: [448, 444]
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

`TestBarrierSameScopeFirstStartContention` fails intermittently in GitHub Actions (four failed PR runs on 2026-09-24: changes 0444 and 0448), with `the winner's worktree slot must be executing, got "reserved"`. It reproduces locally about 48% of the time under `GOMAXPROCS=1 go test ./internal/gatedrive/ -run TestBarrierSameScopeFirstStartContention -race -count=500` and passes on fast multi-core machines, which is why it only shows up on CI runners.

The failure is a real admission bug, not a test defect. In `admitScopedWorktree`, when two same-scope first starts race and the loser arrives after the winner has already confirmed the worktree slot to `executing`, the loser takes the successor branch and rotates the winner's executing slot to `reserved` under the loser's own token. The loser then loses `reserveScopeDrive` with `ErrScopeSecondDrive`. `isSameScopeRaceLoss` treats that as "a peer adopted my reservation", so the loser releases nothing. The winner's live run ends up under a `reserved` slot with a foreign token, which breaks the one-execution-per-worktree invariant and leaves the winner unable to release its own slot.

## What changes

- In `admitScopedWorktree`'s `admissionExecuting` case, only a start carrying a predecessor receipt (`req.PredecessorDriveID != ""`) may rotate the slot. A receipt-less first start refuses typed `ErrScopeSecondDrive` without touching the slot. A scratch run of this guard went 500/500 green under `GOMAXPROCS=1 -race` and the full `gatedrive` package passed.
- Add a deterministic regression test for the late-loser interleaving (loser admits only after the winner confirms `executing`), so the case no longer depends on scheduler timing. Mutation-check it: without the guard the test must fail.
- Update the `admitScopedWorktree` doc comment so it says rotation is successor-only.

## Out of scope

- Broader rework of worktree/scope admission arbitration.
- A general CI change to run the suite under `GOMAXPROCS=1`. If that is wanted, it is a separate change.
