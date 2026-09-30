<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0474 — Rename re-arm to re-enable and retire the lifecycle 'terminal' and non-run 'fence' names](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0474-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle.md)**
<!-- docket:backlink:end -->

# Rename re-arm to re-enable and retire the lifecycle "terminal" and non-run "fence" names — design

Change 0474 is ADR-0129's family (d), the last of the four family changes (0471 run tracker, 0472 revision, 0473 tiers and the 0477 follow-up have all merged). It owns ADR-0129 rows 53–59. Grooming (2026-09-30) traced the code and extended the family with rows 59a–59c, recorded in ADR-0129 in place.

## Problem

ADR-0129 settled three rules this family applies:

- **Decision 6:** "final" names change-lifecycle end states (`done` / `killed`, and ADR statuses other than Accepted). "terminal" stays only for the process-level meaning, "a run or process has finished".
- **Decision 7:** "fence" means only the run fence. The config coordination-key fence became the **shared-setting guard**.
- **Decision 8:** `change.groom`'s `rearm` outcome becomes `re-enable`.

Rows 53–59 are complete as wire tokens: every code, outcome and vocabulary site is the one the ADR names. The trace found the rules are still broken around them:

1. Change-lifecycle "terminal" is widespread outside the seven codes. Examples: `domain.Status.Terminal()` (the done/killed check), CLI help "Sequence a change's terminal half", refusal text "change 0412 is terminal; …", the commit subject "change 0473 terminal backlinks retargeted to archive", and the skill reference file `docket-convention/references/terminal-close-out.md`. There are also about 100 Go comment lines. 0468's prose sweep (row 64) listed only four phrases.
2. The config shared-setting guard renames only its string code (row 56); the Go machinery still says fence (`CodeFencedIgnored`, `scopeRepoFenced`, `applyFence`, …).
3. "fence" also names five internal checks the ADR never listed: the board-surface check, the learnings check, the capability check, the owned-section check and the owned-ref guard. An agent looking for the run fence in `change_kill.go` finds `inline, err := fenceBoardSurface(eff)` under the comment "Board-surface fence".

## Decisions (settled at grooming)

- **D1 — Full lifecycle-"terminal" sweep.** Every maintained-source "terminal" that means a change or ADR end state is renamed. That covers wire codes, Go identifiers, CLI help, messages, commit subjects, skill text, the reference filename and Go comments. Afterwards, maintained source uses "terminal" only in the process-level sense (ADR-0129 row 59b).
- **D2 — "closing half", not "final half".** The "terminal half" of the lifecycle (finalize, maintenance) is the stretch that *leads to* the end state, not an end state itself, so it becomes "closing half". This is the only deviation from Decision 6's mechanical mapping (row 59b).
- **D3 — `skipped-terminal` → `skipped-final` as written.** It is also emitted for **stacked-merged** children, which are non-final. The human accepted that trade-off at grooming to keep row 59 uniform with the other six codes. The Go constant's comment and the glossary state it explicitly: "also emitted for stacked-merged children (the child no longer needs its PR retargeted)". Do not rename it to anything else.
- **D4 — All non-run "fence" senses are renamed** (row 59c), so Decision 7 holds in the code as well as in the docs. Markdown code fences and `---` frontmatter fences are standard terms and stay.
- **D5 — The skill reference file is renamed and sealed** (row 59a, new kind **path**). Nothing else would catch a stale "read `terminal-close-out.md` now (blocking)".
- **D6 — Hard cut, no aliases** (ADR-0129 Decision 2). `outcome: rearm` is refused as `invalid-outcome` once this lands.
- **D7 — No drain.** No retired token is persisted. None appears in metadata-branch records, in `.git/docket` stores or in a JSON key; the run tracker's `terminal*` JSON keys are the kept process sense. No active record is abstained today.

## Rename inventory

Derive every site with a whole-repo grep at build time. This inventory is the grooming-time trace, not a hand list to trust blindly (AGENTS.md: "Never hand-list the sites of a literal …"). Point-in-time records are never edited: archived changes, results, specs, plans, Accepted ADRs, `testdata/repositories/**` and `internal/repository/testdata/**`.

### A. Wire tokens (hard cut, sealed)

| ADR row | Old | New | Sites (grooming trace) |
|---|---|---|---|
| 54 | `change.groom` `outcome: rearm` (`groom_outcomes` vocabulary) | `re-enable` | `internal/app/change_groom.go` (`GroomRearm`), `schema_vocab.go` |
| 55 | `nothing-to-rearm` (also in the `finding_codes` vocabulary) | `nothing-to-re-enable` | `internal/app/finding_codes.go` |
| 56 | `fenced-setting-ignored` | `shared-setting-ignored` | `internal/config/config.go` |
| 57 | `terminal-backlink-pending` | `final-backlink-pending` | `finalize_closeout.go` (`ReasonCloseoutBacklinkPending`), `finalize_cleanup.go` (`ReasonCleanupBacklinkPending`) |
| 57 | `terminal-notes-frozen` | `final-notes-frozen` | `finalize_closeout.go` (`ReasonCloseoutNotesFrozen`) |
| 58 | `change-terminal-claim-stamp` | `change-final-claim-stamp` | `internal/repository/validate.go` |
| 58 | `drop-terminal-claimed-at` | `drop-final-claimed-at` | `internal/reposetup/repair.go` (`RepairDropClaimedAt`) |
| 59 | `not-terminal` (finalize cleanup) | `not-final` | `finalize_cleanup.go` (`ReasonCleanupNotTerminal`) |
| 59 | `skipped-terminal` (retarget child outcome) | `skipped-final` (D3) | `finalize_retarget.go` (`childOutcomeSkippedDone`) |
| 59 | `adr-update-after-terminal` | `adr-update-after-final` | `internal/repository/evolution.go` |
| 59a | `skills/docket-convention/references/terminal-close-out.md` | `skills/docket-convention/references/close-out.md` | the file itself; references in docket-convention, docket-implement-next, docket-new-change and docket-status SKILL.md; `internal/repoguard/budgets_test.go` |

### B. Go identifiers (renamed, not sealed)

**re-enable (rows 53–55):**
- `GroomRearm` → `GroomReEnable`; `FCNothingToRearm` → `FCNothingToReEnable`; test helper `rearmRequest` → `reEnableRequest`.
- Tests: `TestChangeGroomPlanRearmAppliesSectionEditsInOneRecord`, `TestChangeGroomPlanRearmArmsWithoutABlockedSection` (→ `TestChangeGroomPlanReEnableEnablesWithoutABlockedSection`), `TestChangeGroomPlanRearmClearsSectionSetsFlagAndBoard`, `TestChangeGroomPlanRearmRefusals`, `TestChangeGroomPlanRearmRemovesSectionBeforeFollowingSection`, `TestChangeGroomRearmFencedMarkerAgreesWithBoard` (keep "Fenced": a Markdown code fence), `TestChangeGroomRearmShapeValidation`, `TestChangeGroomResultHumanTextRearm`, `TestIntegrationRecordOpsChangeGroomAbstainThenRearmRealGit`. In each, `Rearm` → `ReEnable`.

**final (rows 57–59, 59b):**
- `domain.Status.Terminal()` → `Final()` (callers in `internal/domain/actions.go`, `finalize.go`; `internal/app/finalize_block.go`, `finalize_closeout.go`, `finalize_context.go`; `internal/reposetup/repair.go`; `internal/repository/validate.go`).
- `render.boardTerminalStatuses` → `boardFinalStatuses`; `repository.isTerminalADRStatus` → `isFinalADRStatus`; `reposetup.terminalStatus` → `finalStatus`; `closeoutNotesMatchTerminal` → `closeoutNotesMatchArchived` (row 64: "terminal record" → "archived record").
- `CodeADRUpdateAfterTerminal` → `CodeADRUpdateAfterFinal`; `CodeChangeTerminalClaimStamp` → `CodeChangeFinalClaimStamp`; `ReasonCleanupNotTerminal` → `ReasonCleanupNotFinal`. The code constants whose names carry no "terminal" (`ReasonCloseoutBacklinkPending`, `ReasonCleanupBacklinkPending`, `ReasonCloseoutNotesFrozen`, `RepairDropClaimedAt`, `childOutcomeSkippedDone`) keep their names.
- Tests: `TestStatusTerminal`, `TestRetargetChildrenSkipsTerminalChildren`, `TestFinalizeCleanupOnlyAfterTerminal` and its integration twin (plus the `non-terminal-refused` subtest → `non-final-refused`), `TestValidateEvolutionRejectsUpdateAfterTerminalStatus`. In each, `Terminal` → `Final`.
- Keep the process-sense names (`TestIntegrationRecordOpsClaimTerminalGateRefused`, `TestHealthTerminalFallbackNamesEveryCause`, the githubcli terminal-state PR tests, everything in run tracker / gatedrive / process / supervisor).

**shared-setting guard (row 56, internal/config):**
- `CodeFencedIgnored` → `CodeSharedSettingIgnored`; `scopeRepoFenced` → `scopeRepoOnly` (matches the documented `repo-only` scope tag and its sibling `scopeLocalOnly`); `applyFence` → `applySharedSettingGuard`; `keepUnfencedSurfaces` → `keepUnguardedSurfaces`; `fenced()` → `ignoreSharedSetting()`; `fenceCase` / `fenceCases` → `guardCase` / `guardCases`; locals `fenced` / `wantFenced` → `guarded` / `wantGuarded`.
- Tests: `TestEveryFence` → `TestEverySharedSettingGuard`, `TestEveryFenceCoversTheRegistry` → `TestEverySharedSettingGuardCoversTheRegistry`, `TestRegistryFencedSet` → `TestRegistryRepoOnlySet`, `TestFencedPathStillReachesEffective` → `TestGuardedPathStillReachesEffective`, `TestRuntimeBashCommittedFence` → `TestRuntimeBashCommittedGuard`, `TestBoardGithubTokenMachineFence` → `TestBoardGithubTokenMachineGuard`, `TestTolerateUnknownKeysLeavesFenceIntact` → `TestTolerateUnknownKeysLeavesGuardIntact`, `TestFixtureFencedMachineKeys` → `TestFixtureGuardedMachineKeys` (it still loads the frozen fixture directory `fenced-machine-keys`), `TestClassifyFencedDeclarationsAreNotCapabilities` → `TestClassifyGuardedDeclarationsAreNotCapabilities`.

**other fences (row 59c):**

| Old | New |
|---|---|
| `fenceBoardSurface` (`internal/app/planning.go`, ~15 callers) | `resolveBoardSurface` |
| `haltPinAndFence` (`change_halt.go`) | `haltPreflight` |
| `fenceLearningsEnabled` (`learning_ops.go`) | `requireLearningsEnabled` |
| `TestChange{Kill,Block,Defer,Unblock,Revive,Create,Groom}FencesGithubBoardSurface` | `…RefusesGithubBoardSurface` |
| `TestFenceBoardSurface` | `TestResolveBoardSurface` |
| `TestLearning{Record,Update}FencesWhenLearningsDisabled` | `…RefusesWhenLearningsDisabled` |
| `TestE2EUnsupportedConfigFence` | `TestE2EUnsupportedConfigRefused` |
| `TestChangeReconcileOwnedFieldFence` | `TestChangeReconcileRefusesNonOwnedSection` |
| `TestIntegrationRepoOwnedRefFence` (`internal/gitcli`) | `TestIntegrationRepoOwnedRefRefusesForeignRef` |

Check each renamed test against its shard script's `SHARD_PREFIX` (`tests/test_go_*.sh`). Every rename above keeps its `TestIntegration<Shard>` prefix; confirm that none leaves its shard.

### C. Text

**re-enable:**
- `skills/docket-groom-next/SKILL.md`: the description ("… a defer, or a re-enable"), exit 6 **Re-arm** → **Re-enable** with `outcome: re-enable` and `nothing-to-re-enable`, the Retitle paragraph's "6 (re-enable)", and Step 5's "a re-enable returns an abstained row …".
- `skills/docket-convention/SKILL.md`: the `## Auto-groom blocked` bullet (`outcome: re-enable`) and the *Autonomous grooming* abstain rule ("Re-enable = a human supplies …", "a re-enabled stub").
- `skills/docket-auto-groom/SKILL.md`: "the human's re-enable cue".
- `internal/cli/change.go` groom help: "(abstain, re-enable)". `change_groom.go` result text: "change %04d re-enabled for auto-groom — %s". The refusal and shape messages name `re-enable`, and the outcome list reads "spec, trivial, revise, abstain, re-enable".
- `docs/reference/glossary.md`: heading "Abstain / re-enable" with its anchor `#abstain--re-enable` and index entry, the `groom_outcomes` table row, and the other mentions. `docs/guide/designing-before-building.md`.

**final / closing (row 59b):**
- CLI help: `internal/cli/finalize.go` "Sequence a change's closing half: rebase, publish, merge, and closeout"; `internal/cli/maintenance.go` "Reclaim docket's closing half in batch (docket status stays read-only)" and "Close out merged changes, retry close-out cleanup, and reclaim expired claims in one pass". These are mirrored in `docs/reference/cli.md` and `docs/reference/config-keys.md` ("the closing-half sequencer's knobs").
- Messages: `finalize_block.go` ×2 "change %04d is final; there is no finalize attempt to block"; `finalize_context.go` "(final or without a PR reference)"; `finalize_cleanup.go` "change is not final and carries no aborted-rebase scratch to clear; nothing to clean" and "the final backlink interior could not be rendered; retry cleanup"; `maintenance_assess.go` "the record is neither done nor stacked-merged", "the record's final backlink could not be rendered", "artifact … carries a malformed final backlink block".
- Commit subjects: `finalize_closeout.go` "change %04d final backlinks retargeted to archive"; `finalize_cleanup.go` "change N final backlinks verified (cleanup)". No code parses these subjects (grooming trace); re-check at build.
- `skills/docket-finalize-change/SKILL.md` "drives … through its closing half"; `tests/test_go_finalize_e2e.sh` header "whole closing half".
- Go comments in the lifecycle sense (about 100 lines, heaviest in `finalize_closeout.go`, `finalize_cleanup.go`, `render/board.go`, `repository/evolution.go`, `maintenance*.go`, `reposetup/repair.go`).

**shared-setting guard and other fences:** about 40 comment lines in `internal/config`; the glossary entry "Coordination key / scope tag / shared-setting guard" also names the `shared-setting-ignored` warning. About 16 comment phrases elsewhere: "board fence", "Board-surface fence", "deferred-capability fence", "Go v1 capability fence", "owned-section fence", "learnings.enabled fence".

Update the embedded copies under `internal/assets/embedded/tree/` for every skill edit.

### D. Explicitly kept (added to ADR-0129 "Explicitly not renamed")

- Process-level "terminal": gate run, supervisor, participant and drive terminals, `terminal_receipt`, the JSON `terminal` / `terminal_status` / `terminal_turn` / `terminal_observed_at` keys, `record-terminal`, `incumbent-nonterminal`, a run's "Terminal disposition" (implement-next, finalize-change), "terminal result/envelope" (status, build).
- A GitHub pull request's "terminal state" (closed or merged).
- `terminal_publish` and the obsolete "terminal publish" / "terminal publication" feature name.
- Markdown code fences and `---` frontmatter fences (`fenceRun`, `isBareFence`, `isFrontmatterFence`, "pre-fence insertion point", …).
- The frozen fixture directory `testdata/repositories/v0.9.2/fenced-machine-keys/` and ADR-0019's filename `0019-global-config-fence-classification.md`.
- The unrelated `internal/suiterunner` signal "re-arm", and generic "re-enabling" (learnings, recursive self-dispatch).

## ADR-0129

Edited in place at grooming (the 0473 precedent: no `## Amendment` section). The build does not touch it. If the build finds another missed site, it records the fact through the normal ADR path.

## Seal

Append family (d)'s rows to `retiredVocabulary` in `internal/repoguard/retired_vocabulary_test.go`, all `kindToken`, each naming its replacement: `rearm` (54), `nothing-to-rearm` (55), `fenced-setting-ignored` (56), `terminal-backlink-pending` and `terminal-notes-frozen` (57), `change-terminal-claim-stamp` and `drop-terminal-claimed-at` (58), `not-terminal`, `skipped-terminal` and `adr-update-after-terminal` (59), and `terminal-close-out.md` (59a). Update the header comment's family list and raise the table-integrity floor.

- `testRetiredNonVacuity` already plants every row in every site of its kind. Confirm each new row is detected at each planted site and names its replacement. `rearm` also matches inside `nothing-to-rearm`; the non-vacuity check counts only the planted row's own hits.
- Hand mutation for the results file: reintroduce `outcome: rearm` into a skill line, watch the seal go red naming `re-enable`, then revert.
- No guard for Go identifiers or prose (0468/0473 precedent: a guard keyed on passing mentions violates the guard rule). Results records the closing whole-repo grep instead: every remaining `terminal`, `fence` and `re-arm`/`rearm` hit in maintained source, each classified as a D-list keep or a point-in-time record.

## Testing

- The whole suite through `build.test_command`; read the budget report.
- Retargeted assertions: `schema_vocab_test` expects `groom_outcomes` = spec, trivial, revise, abstain, re-enable. Add a hard-cut test: `outcome: rearm` is refused with `invalid-outcome`. Config tests expect `shared-setting-ignored`. `prose_contracts_test` expects `` `outcome: re-enable` `` and `` `nothing-to-re-enable` ``. `budgets_test` moves its entry to `docket-convention/references/close-out.md` (same budgets).
- The embedded-tree sync check passes after the skill edits.
- After `docket install` (or `development.install`), the installed docket-convention skill tree carries `references/close-out.md` and no stale `terminal-close-out.md`. Record the check in results.

## Landing

- No drain (D7). Between merge and the post-merge rebuild, a groom run with the new skill against the old binary gets a loud `invalid-outcome`. The rebuild required by AGENTS.md closes the window.
- Results `**Human action:**` tells consumer repos to re-run `docket install`, because their installed skills still name `outcome: rearm` and `terminal-close-out.md`.

## Out of scope

- Aliases, deprecation windows or dual spellings (Decision 2).
- Renaming config keys (`terminal_publish`), agent names, frontmatter fields or persisted storage (Decisions 3, 9).
- Editing point-in-time records, including frozen testdata corpora.
- Rows owned by other families or by change 0469.
- Semantic changes: every rename keeps behavior identical, and `not-final` is reported for exactly the statuses `not-terminal` was. If a killed change can reach `finalize cleanup`'s default branch, its "not final" message is as inaccurate as the old "not terminal" one. Record that in results as a follow-up; do not fix it here.
