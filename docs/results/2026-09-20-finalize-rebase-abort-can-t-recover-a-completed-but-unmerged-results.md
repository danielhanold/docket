<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0438 — finalize.rebase-abort can't recover a completed-but-unmerged rebase whose base later moved](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0438-finalize-rebase-abort-can-t-recover-a-completed-but-unmerged.md)**
<!-- docket:backlink:end -->
# finalize.rebase-abort can't recover a completed-but-unmerged rebase whose base later moved — Results

## Outcome

`finalize.rebase` now recovers a completed, clean, owned, **unpublished** rewrite whose effective
base has since advanced, instead of refusing on the stale recorded base and then failing to abort a
rebase Git no longer has in progress. The recovery **forward-rebases the current head onto the newer
base**, preserving prior conflict resolutions and all committed work — the branch is never reset to
its pre-first-rebase head. The refresh reuses the existing `workspace.RebaseReceipt` fields,
`gitcli.Client.BeginRebase`, the per-workspace operation lock, and the existing
gate/checkpoint/publication machinery; **no new CLI operation, argument, config key, receipt field,
or lifecycle state was added** (spec §6).

Delivered behavior:

- **Classification before mutation** (`recoverFromReceipt`): conflicted and foreign in-progress
  states settle against the **recorded** base first (a new owned-base-anchor agreement check); a
  durable-before-effect crash (receipt persisted, Git not started, clean head still equal to the
  receipt's `OrigHead`) resumes `BeginRebase` against the recorded target; only a completed,
  quiescent, unpublished rewrite whose head descends the recorded base is classified for forward
  refresh.
- **Forward refresh** (`refreshOwnedRewrite`): admission conjuncts (no unsettled reservation/
  continuation, clean registered workspace, remote head still equals the recorded publication lease,
  and a fast-forward-only new base) are all checked **before** the operation lock; then a
  decide-and-act-on-the-same-copy reload under the lock refuses `refresh-contended`
  (`ResultContended`) if the on-disk receipt diverged. The refreshed receipt preserves
  `OrigRemoteHead` (the exact publication lease) and the resolver budget verbatim (base movement
  never replenishes dispatch capacity), advances `OrigHead`/`BaseHead`, mints a fresh `Attempt`
  token, and clears the gate pair and all six `PublishCheckpoint*` fields so nothing recorded
  against the old base can authorize publishing the refreshed rewrite.
- **Always retest on the new base**: recovery composes the gate with `allowEvidenceSkip=false` and
  the refresh uses `forceRetest=true`, so a base refresh always runs the configured finalize suite
  on the resulting head — even for a mechanically-unchanged rebase — and never reuses a
  superseded-base checkpoint or PR-body evidence. A checkpoint recorded for the refreshed
  head/base/command/PR is reusable on a response-loss replay.
- **Attempt-guarded receipt writers** (`mutateReceiptForAttempt`): the three lock-free post-gate
  writers now compare the observed attempt token against the on-disk receipt before writing, so a
  late gate/resolver write for a superseded rewrite cannot clobber a refreshed receipt.
- **Real no-op finalize.block / finalize.clear-block plans**: a repeated same-attempt block and an
  absent-marker clear-block now return populated no-op plans (valid commit subject + canonical
  `blockReceipt`, zero file mutations) so `validatePlan` admits them and the engine's empty-`Files`
  path reports a genuine no-op with the requested change id, instead of `invalid-input`. The
  transaction engine was not weakened.
- **Guidance**: `skills/docket-finalize-change/SKILL.md` and `references/gate-failure.md` describe
  forward recovery and the narrowed meaning of a `base-moved-under-receipt` block (base diverged/
  rewritten, or unsettled recorded-attempt resolver work — retained, halted, a human is needed;
  never permission to reset). The change-0411 `rebase-continue` reconciliation remedy and change-0439
  admission diagnostics are preserved. Embedded assets were regenerated.

## Verification performed

- **Build gate (full suite, from source `go run ./cmd/docket development test`)**: green on head
  `ad6500eb` — `SUITE files=52 passed=52 failed=0`. An earlier run reddened only on
  `TestSkillSizeBudgets` (the finalize skill files exceeded their line/word budgets after the
  guidance additions); the guidance was slimmed back under budget (SKILL.md 235/5412 lines/words vs
  239/5421; gate-failure.md 145/1892 vs 147/1901) and the suite re-ran green.
- **Budget report** (read on the green run): only `BUDGET WATCH` screening lines under parallel
  `-j11` at consecutive-overrun streak 1/5 (including `test_go_integration_app_rebaserecovery.sh`,
  which the new real-Git tests extend). No `SERIAL CONFIRMED OVER BUDGET` line — not an authoritative
  breach, no action taken.
- **Mutation testing of the new guards** (CLAUDE.md guard-is-code rule): 7 of 9 guards reddened
  their named tests directly (remote-lease refusal, divergent-base refusal, attempt-token
  preservation, budget preservation, `mutateReceiptForAttempt` skip, `allowEvidenceSkip`, the
  block/clear-block no-op plans). Two guards initially did not redden — see Findings; the pre-start
  resume conjunct was subsequently covered by a dedicated test, and the `refresh-contended` guard by
  a concurrent-re-entry test added during the review fix loop.
- **Whole-branch review** (deep rung; premium build tier bumped for a 2228-line diff): 0 blockers,
  1 important, 1 minor — see Findings.
- **Review fix loop**: the important finding (missing `refresh-contended`/concurrent-re-entry
  coverage that spec §3 required) was fixed in-branch as a test-only addition
  (`TestIntegrationFinalizeRebaseRecoveryForwardRefreshContended`), mutation-confirmed against the
  reload/compare guard with product code left byte-identical.

## Findings and limitations

### Checkpoint-clearing on refresh is redundant defense-in-depth (untested crash window)

Removing the three `refreshed.PublishCheckpoint* = ""` clearing lines left the recovery integration
suite green: the observable behavior (no stale-checkpoint reuse) is already guaranteed by
`forceRetest=true` forcing a re-gate and by the fresh checkpoint overwriting the stale fields against
the new base. The clearing is belt-and-suspenders against a crash between the receipt write and gate
completion, a window no test exercises. The guard is correct and present; the limitation is that its
crash-window value is not independently asserted.

### Crash-before-Git resume of a moved-base refresh does not set `forceRetest` (efficiency-only, recorded)

The shared durable-before-effect resume path passes `rebaseMapOptions{}`. If a refresh crashed after
persisting the refreshed (moved-base) receipt but before Git started, and the resumed `BeginRebase`
returns `RebaseUnchanged` because the completed rewrite already contains the new base, no publish
checkpoint is recorded, so a later replay re-runs the suite rather than reusing a checkpoint. **Safety
is fully preserved** — recovery always runs the suite and never honors stale evidence
(`allowEvidenceSkip=false`) — so this is efficiency/consistency only, on a rare crash∧already-contained
intersection. It was **not fixed in-branch** because the reviewer's suggested fix (carrying a flag
into the resume) would require a new persistent flag/receipt field, which spec §6 explicitly forbids
("interruption does not require a new persistent flag"). Recorded for merge-time judgment; the current
behavior is safe and spec-compliant.
