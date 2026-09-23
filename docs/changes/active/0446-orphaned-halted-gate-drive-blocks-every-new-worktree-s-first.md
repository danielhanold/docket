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
related: [428, 437, 439, 444, 368]
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

No remedy exists today. `docket gate history cleanup --dry-run` reports recoverable 0, retained 1. `run.cancel` doesn't apply because the drive has no owning epoch. The refusal message points only to those two tools. Change 0428 fixed the neighbouring case (legacy history bound to a different, resolvable worktree), but this shape is still open: a halted drive whose worktree and run evidence both disappeared through normal closeout and temp-dir cleanup. The failure is also time-bombed. The drive halted on 2026-09-18 and only started blocking once its scratchpad run dir was emptied. No merge changed the classifier; it was last touched on 2026-09-14 by change 0428. The timeline:

- 2026-09-18 to 2026-09-19: not blocking, because 368's worktree still existed (step 2, bound to a different worktree).
- 2026-09-19 08:52 to 2026-09-22: not blocking. The worktree was gone after 368's closeout, but the run dir's manifest was still readable, so step 3 could prove teardown. The last successful first admission was 2026-09-22 22:07.
- 2026-09-23 00:00: retained. Every Claude Code session scratchpad last active on 2026-09-18 (8 of them) has its directories stamped at exactly 00:00:00, with run-dir contents removed and recently read files kept. That fits an age-based `/tmp` cleaner (macOS or Claude Code; no available log says which). Without a manifest, `ClassifyRun` returns `invalid` (`internal/process/recover.go`, `classifyRun`).

Any build that halts while its run evidence sits in a session scratchpad becomes a repo-wide blocker a few days later.

## What changes

- Give a halted legacy drive whose worktree no longer exists and whose run evidence is gone or empty a safe, provable resolution path. It must not stay retained forever. The design decides whether this is automatic classification (for example, the owning change is terminal, the worktree is gone, the recorded process group is provably absent) or an explicit, human-authorized `gate history cleanup` action that records an abandoned marker with a reason.
- Make the `legacy-inventory` refusal name a remedy that actually works for this state. Today it points at `gate history cleanup` and `run.cancel`, and neither can act on it.
- Look at why a live drive's durable run evidence sits in a session scratchpad under `/private/tmp`, where it can be emptied under a still-referenced record. Decide whether run evidence belongs under the git common dir, or whether a closeout that removes a change's worktree should also settle that change's halted drives.
- Fix the same "unresolvable legacy record blocks forever" shape in `run.cancel`'s teardown accounting, not only in admission. `ReconcileEpochLaunches` (`internal/gatedrive/reconcile.go:116-127`, called from `internal/app/rungate_cancel.go`'s step 5c) walks **every** drive in the repo-wide registry for every cancellation, regardless of which epoch is being cancelled. When a drive's epoch linkage can't be resolved (a lost scope, or a scopeless admission token the worktree slot no longer matches) it appends `linkage-unresolved:<id>` and fails `Accounted` closed — permanently, since nothing ever re-links or retires that drive's linkage. Confirmed 2026-09-23: cancelling change 444's halted run epoch (`9c0c78c95d02aff4dff08a36145a29b4`, gate key `implement-next-20260923t065151z-64967-5863`) returned `cancellation-pending` with 99 `linkage-unresolved` findings, identical across 4 repeated `run.cancel` invocations — not converging. This epoch owned no participants, no admitted mutations, and no worktree admission slot (none existed for its worktree at all), so there was nothing of *this* epoch's own to account for; every one of the 99 findings named an unrelated, pre-existing drive. The fence itself (`cancelling`) is durably held regardless, and per the run-gate contract the resume-arm still refuses (`cancellation-pending`) until `run.cancel` reports `cancelled` — which this bug makes structurally unreachable whenever the repo's drive registry holds any unresolvable-linkage record, i.e. most real repos over time. The design should decide whether a drive with unresolvable linkage should be scoped out of a cancellation's accounting entirely (it can never have belonged to the epoch being cancelled if its linkage can't even resolve to *an* epoch) versus needing its own recovery/retirement path mirroring the admission-side fix above.
  - **The resume path recomputes the same accounting, so fix both.** `run.gate-before --resume` (`internal/app/rungate_before.go`, the `EpochCancelled` and `EpochSuperseded` branches) calls `validateResumeQuiescence`, which re-derives the registry-wide census independently of the epoch's stored `state`. Confirmed 2026-09-23: after the epoch was hand-set to `cancelled`, the resume arm still refused with `cancellation-pending` ("unresolved cancellation evidence") and listed the same 99 `linkage-unresolved` ids. Fixing only `run.cancel` leaves resume blocked.
  - **Origin: a latent regression from change 0437.** Commit `b33357ce` ("fail closed on unresolved epoch linkage in cancellation census", 2026-09-19) made an unresolved linkage fail `Accounted` closed. It only fires on a cancel or a resume, so runs that go straight to a PR never reach it. Change 444 was the first run since 2026-09-19 that halted and needed a resume. Keep 0437's intent that a drive that *might* belong to the epoch is never silently skipped; only a drive whose linkage resolves to no epoch at all should be ruled out.
- Add a regression test with this exact shape: a HALTED `stopped-not-initiated` drive, a deleted worktree, and an empty run dir. A first admission of an unrelated fresh worktree must succeed, or be refused with a working, named remedy.
- Add a regression test for the cancellation-side shape: an epoch with no participants/slot of its own, cancelled while the repo's drive registry holds at least one drive with unresolvable epoch linkage. `run.cancel` must reach `cancelled`, not `cancellation-pending` forever.
- Add a regression test for the resume-side shape: a `cancelled` epoch with the same unrelated unresolvable-linkage drive present. `run.gate-before --resume` must arm a replacement, not refuse with `cancellation-pending`.
- Resume change 444 after this lands. It is `blocked` on this change, halted at build task 1. Re-arm with `run.gate-before implement-next --resume 444`, then re-dispatch implement-next with id 444.

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
- To unblock change 444's resume, the epoch record this same run left `cancelling` (unable to
  reach `cancelled` for the reason described above) was hand-edited: `.git/docket/rungate/
  implement-next-20260923t065151z-64967-5863/epoch.json`, `state` set from `cancelling` to
  `cancelled` directly, after confirming that epoch owned no participants, no admitted
  mutations, and no worktree admission slot — every `linkage-unresolved` finding it hit named an
  unrelated pre-existing drive, never itself. **This did not unblock the resume**: the resume
  arm's quiescence re-check recomputed the same 99 findings (see *What changes*). The original
  record is backed up beside it as `epoch.json.pre-hand-clear.bak`. Keep both as fixture shapes
  for the cancellation-side and resume-side regression tests above. The epoch now reads
  `cancelled`, so after this change lands the resume should go through `EpochCancelled` →
  quiescence → replacement. Check that path with the fix before re-dispatching 444.
