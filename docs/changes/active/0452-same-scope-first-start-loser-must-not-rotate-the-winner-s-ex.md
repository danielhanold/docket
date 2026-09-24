---
id: 452
slug: 'same-scope-first-start-loser-must-not-rotate-the-winner-s-ex'
title: 'Same-scope first-start loser must not rotate the winner''s executing worktree slot'
status: 'in-progress'
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
plan: 'docs/superpowers/plans/2026-09-24-0452-same-scope-first-start-loser-must-not-rotate.md'
results:
trivial: true
auto_groomable:
branch_prefix:
branch: 'fix/same-scope-first-start-loser-must-not-rotate-the-winner-s-ex'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-24T16:20:57Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-09-24-0452-same-scope-first-start-loser-must-not-rotate.md](https://github.com/danielhanold/docket/blob/fix/same-scope-first-start-loser-must-not-rotate-the-winner-s-ex/docs/superpowers/plans/2026-09-24-0452-same-scope-first-start-loser-must-not-rotate.md) |
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

## Open questions

None. This is trivial: the root cause is traced and reproduced (about 48% failures under `GOMAXPROCS=1 -race`), and the fix is a single receipt guard in one switch arm. A scratch run of it passed 500/500 plus the full `gatedrive` package. The only remaining work is the guard, a deterministic regression test, and a comment update. No design choice is open.

## Reconcile log

### 2026-09-24

Reconciled against main 9d4cb1fe: admitScopedWorktree (internal/gatedrive/driver.go) still rotates an executing same-scope slot for any start, receipt or not; isSameScopeRaceLoss unchanged. Related 437/446/375/405 are done and do not touch this arm. Scope stands as written: receipt guard in the admissionExecuting case, deterministic late-loser regression test with mutation check, doc-comment update.
