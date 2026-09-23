---
id: 446
slug: 'orphaned-halted-gate-drive-blocks-every-new-worktree-s-first'
title: 'Orphaned halted gate drive blocks every new worktree''s first gate admission'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-23'
depends_on: []
stacked_on:
related: [428, 439, 444, 368]
discovered_from: [444]
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

One stale gate drive can stop every new change from building. When a worktree reserves its execution slot for the first time, the gate driver runs a legacy inventory (`inventoryLegacyDrives`) over every drive record in the repository. If any record comes back as retained, the whole admission is refused with `unresolved-execution`, stage `legacy-inventory`. implement-next creates a fresh worktree for every change, so each new change's first `gate.drive.start` hits this check.

Observed 2026-09-23 on change 444, which halted at build task 1. The blocking drive was `b66ce1405cd813e5519c344360f61bd4`, a build-phase suite gate for change 368 (done, archived 2026-09-19). Its record says `last_outcome: HALTED`, `last_cause: stopped-not-initiated`. Its worktree (`.worktrees/resume-halted-preallocation-recovery`) was removed at 368's closeout. Its raw run dir lives under a session scratchpad in `/private/tmp`, and it is now an empty directory. `classifyLegacyDrive` retains it for two reasons:

- Step 2 can't resolve the deleted worktree path. An unresolvable path is deliberately not treated as proof the drive belongs to a different worktree.
- Step 3's `ClassifyRun` on the emptied run dir returns `invalid`. Only `terminal`, `stopped`, `already-abandoned`, and `abandoned-marked` count as teardown proof.

The outcome is `retained` with reason "halted run not provably torn down (invalid)".

No remedy exists today. `docket gate history cleanup --dry-run` reports recoverable 0, retained 1. `run.cancel` doesn't apply because the drive has no owning epoch. The refusal message points only to those two tools. Change 0428 fixed the neighbouring case (legacy history bound to a different, resolvable worktree), but this shape is still open: a halted drive whose worktree and run evidence both disappeared through normal closeout and temp-dir cleanup. The failure is also time-bombed. The drive halted on 2026-09-18 and only started blocking once its scratchpad run dir was emptied.

## What changes

- Give a halted legacy drive whose worktree no longer exists and whose run evidence is gone or empty a safe, provable resolution path. It must not stay retained forever. The design decides whether this is automatic classification (for example, the owning change is terminal, the worktree is gone, the recorded process group is provably absent) or an explicit, human-authorized `gate history cleanup` action that records an abandoned marker with a reason.
- Make the `legacy-inventory` refusal name a remedy that actually works for this state. Today it points at `gate history cleanup` and `run.cancel`, and neither can act on it.
- Look at why a live drive's durable run evidence sits in a session scratchpad under `/private/tmp`, where it can be emptied under a still-referenced record. Decide whether run evidence belongs under the git common dir, or whether a closeout that removes a change's worktree should also settle that change's halted drives.
- Add a regression test with this exact shape: a HALTED `stopped-not-initiated` drive, a deleted worktree, and an empty run dir. A first admission of an unrelated fresh worktree must succeed, or be refused with a working, named remedy.
- Resume change 444 after this lands. It is halted at build task 1 on this blocker.

## Out of scope

- Treating an unresolvable worktree path as proof of unrelatedness in general. That fail-closed rule from 0428 stays unless the design gives a narrower proof.
- Hand-deleting or editing drive records in `.git/docket/gate-drives/v1` as the fix. That's a one-off unblock, not the change.
- Redesigning worktree admission or the run-epoch fence.

## Open questions

- To unblock change 444's own admission on 2026-09-23, the retaining drive record was
  hand-moved out of `.git/docket/gate-drives/v1/` into a quarantine folder, preserved (not
  deleted) at `/Users/homer/dev/docket-quarantine/gate-drives/b66ce1405cd813e5519c344360f61bd4/`
  (`record.json` + `lock`, byte-identical to the retained record captured above). This is
  fixture evidence for this change's regression test, not a remedy — `docket gate history
  cleanup --dry-run` confirmed `retained: 0` immediately after the move. Restore it (or a copy)
  into the drive store as the fixture for the exact-shape regression test this change adds, then
  decide its disposition once the resolution path is designed.
