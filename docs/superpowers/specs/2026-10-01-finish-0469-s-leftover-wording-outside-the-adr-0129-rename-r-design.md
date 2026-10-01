<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0482 — Finish 0469's leftover "repair" (relink) and "Step 0" (startup check) wording](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-01-0482-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r.md)**
<!-- docket:backlink:end -->

# Finish 0469's leftover "repair" (relink) and "Step 0" (startup check) wording — design

## Goal

Finish two ADR-0129 family (e) prose rows that change 0469 left partly done. Row 84 renamed "identity repair" to **relink**, and row 80 renamed "Step-0 preamble" to **startup check**. 0469's build changed the spellings those rows quote but left the bare forms: "repair" naming the `change relink` operation, and "Step-0" / "Step 0" naming the shared startup check (`repository.prepare`). 0469's results file reported them as uncovered by any row. The code trace at grooming (2026-10-01) showed they are the same two concepts, so this change only finishes rows 80 and 84 and makes no new rename decision.

## Decisions settled at grooming

- **Full sweep.** Change every site where "repair" names the relink operation, and every site where "Step-0" / "Step 0" names the startup check: user-visible strings, Go comments, test comments, test failure and panic messages, test fixtures and one closure name. Afterwards a grep finds those meanings only in the retirement guards, which must spell the old token.
- **No ADR-0129 edit.** Rows 80 and 84 already name both concepts. This spec records which senses are kept.
- **No new guard.** The retired-vocabulary seal (`internal/repoguard`) cannot ban either word, because both have meanings that stay. "repair" has roughly 200 legitimate uses, and the seal scans skill markdown, where implement-next's own "Step 0" step label legitimately appears. Rows 74-84 are prose rows the seal does not cover, as 0469 already established.
- **The stub missed one user-visible string.** The `change relink` `--id` flag help still reads "whose recorded identity to repair", which is literally row 84's old phrase. It is in scope.
- **No test asserts any of the leftover strings** (whole-repo grep at grooming). The only test edits are the tests' own wording.

## The sense rule

Change a site only when the word names one of these two concepts:

- **"repair" → "relink"** where it names the `change.relink` operation, its transaction, its workspace gate, or its result.
- **"Step-0" / "Step 0" → "startup check"** (hyphenated "startup-check" when used as a modifier) where it names the shared startup check: the `repository.prepare` operation, its contract, its context values, or its config export.

Every other meaning stays. See *Kept* below.

## Sites to change

Derive the final list from a fresh whole-repo grep at build time (AGENTS.md *Guards and tests*). The tables below are the grooming-time inventory: 46 "repair" sites and 14 "Step 0" sites. Anchors are quoted text plus the enclosing symbol, never line numbers.

### "repair" → "relink": user-visible strings (4)

| File | Before | After |
|---|---|---|
| `internal/app/change_relink.go`, stale-evidence refusal after the `ExpectRevision` check | `the change record moved since the approved revision; re-read authoritative context before repairing` | `… before relinking` |
| `internal/app/change_relink.go`, workspace-conflict message in the workspace-gate helper | `an owned workspace targets %q, not the proposed branch %q; the repair would orphan it` | `… the relink would orphan it` |
| `internal/app/change_relink.go`, `transaction.DispositionContended` result | `the change record moved during the repair transaction; re-read authoritative context` | `the change record moved during the relink transaction; re-read authoritative context` |
| `internal/cli/change.go`, `newRelinkSubcommand` `--id` flag | ``change `id` whose recorded identity to repair (required)`` | ``change `id` to relink (required)`` (the `change id to <verb>` form the sibling subcommands use) |

### "repair" → "relink": Go comments naming the operation (4)

| File | Before | After |
|---|---|---|
| `internal/app/derived_views.go`, the declare-only-when-changed comment | `kill, lifecycle, mark-implemented, reclaim, repair, closeout` | `… reclaim, relink, closeout` |
| `internal/app/status.go`, the named-callers comment | `workspace, merge, clear-block, repair, retarget` | `… clear-block, relink, retarget` |
| `internal/app/derived_views_guard_test.go`, the CHANGE-PATH shape comment | `reconcile, repair, halt, resume-halted` | `reconcile, relink, halt, …` |
| `internal/app/named_branch_facts_test.go`, section divider above `TestRelinkWorkspaceClearProbesOnlyOwnStack` | `--- repair workspace ownership gate (fake reader + fake workspace)` | `--- relink workspace ownership gate …` |

### "repair" → "relink": test-only wording (38)

- `internal/app/change_relink_test.go`, 21 sites:
  - 13 fake-method panics of the form `"<Method>: repair must not call this"` → `"<Method>: relink must not call this"`
  - 8 comments and messages: `fake FinalizeGitHub for repair`, `ViewPullRequest the repair reads`, `panics: repair`, `a refused repair opened %d transactions`, `the repair refuses as`, `the repair op's Plan closure`, `the repair envelope`, `an AdoptPRHead repair`
- `internal/app/change_relink_integration_test.go`, 2 sites: `repaired record on origin does not carry the adopted branch` → `relinked record on origin …`
- `internal/app/change_integration_test.go`, 12 sites, in the comments and messages of `TestIntegrationChangeRuntimeRelinkAdoptPRHeadPinsExactRevision`, `TestIntegrationChangeRuntimeRelinkAbsentWorkspaceNoConflict` and the following relink workspace-conflict test: `keying the repair op`, `"repair = (%q, %q)"` (×2), `change 0368's repair`, `the repair proceeds`, `the repair opens`, `record vanished after repair`, `repaired origin record missing`, `survived the repair`, `blocks the repair`, `while the repair`, `lets the repair proceed`
- `internal/app/named_branch_facts_test.go`, 2 sites: `repair workspace gate beside an unprobeable unrelated stack refused` and `repair workspace gate with B's own parent unprobeable passed` → `relink workspace gate …`
- `internal/cli/revision_rename_test.go`, 1 site: the closure `repair := func(value string)` that runs `change relink` → `relink`, along with its two call sites

### "Step 0" → "startup check" (14)

| File | Before | After |
|---|---|---|
| `internal/cli/repository.go`, `prepare` subcommand short help (user-visible) | `Prepare the repository for a workflow: pin topology and attach or fast-forward the .docket worktree (Step 0)` | `… (the startup check)` |
| `internal/app/repository_prepare.go`, file header comment | `the sole shared Step-0 operation` | `the sole shared startup-check operation` |
| `internal/app/repository_prepare.go`, dropped-transport list (`DOCKET_GI_*` entry) | `not a Step-0 context value` | `not a startup-check context value` |
| `internal/app/repository_prepare.go`, dropped-transport list (`DOCKET_DISPATCH_RETENTION_DAYS` entry) | `not a Step-0 value` | `not a startup-check value` |
| `internal/app/repository_prepare.go`, `RunRepositoryPrepare` doc comment | `RunRepositoryPrepare is the sole shared Step-0 operation` | `… startup-check operation` |
| `internal/app/repository_prepare.go`, `prepareGatherFailure` comment | `the Step-0 contract promises` | `the startup-check contract promises` |
| `internal/app/configrefusal_integration_test.go`, `TestIntegrationRepoPrepareInvalidConfigDiagnostics` doc comment | `Step-0 contract promises` | `startup-check contract promises` |
| `internal/app/repoprepare_integration_test.go`, header comment | `the shared Step-0 \`repository prepare\` service` | `the shared startup-check …` |
| `internal/repoguard/config_read_channel_test.go`, header comment | `the config resolver / Step-0 export` | `… / startup-check export` |
| `internal/repoguard/budgets_test.go`, change-0394 re-baseline note | `the new Step-0 capability bootstrap` | `the new startup-check capability bootstrap` |
| `tests/test_go_integration_app_repoprepare.sh`, header comment | `the shared Step-0 \`repository prepare\` service scenarios` | `the shared startup-check …` |
| `internal/repoguard/skill_handoff_sites_test.go`, `non_vacuity` fixtures `unmarked`, `mention`, `braced` (3) | `from the Step-0 config export.` / `come from the Step-0 export.` | `from the startup-check config export.` / `come from the startup-check export.` (the phrasing the real skills already use) |

## Kept

- **"repair" in every other meaning:**
  - finalize's post-rebase repair and `docket-integration-repair`
  - build's repair-and-rerun and the in-branch fix loop
  - the board / ADR-index "repair notice" (`writeRepairNotice`, `ADRIndexWithRepair`)
  - `reposetup.PlanRepairs`, `repository migrate --repair-frontmatter`, `finalizeCleanupBacklinkRepair`
  - `status.go`'s branch-malformed comment "which repair can work", which means "fix" and covers both relink and a hand edit
  - `change_integration_test.go`'s "the historical corruption this change prevents, not repairs"
- **The retirement guards that must spell the old token:** the retired-vocabulary rows 70-71 and their comment in `internal/repoguard/retired_vocabulary_test.go`, and `TestChangeRelinkRetiresRepairIdentity` in `internal/cli/change_test.go`.
- **"identity" in its which-record sense** inside `change_relink.go` and its tests (`proves identity`, `identity field`), which ADR-0129 family (e) keeps.
- **implement-next's own "Step 0" step label** (`### Step 0 — Sync & sweep`) and every reference to it:
  - `maintenance_preflight.go`, `internal/cli/maintenance.go`
  - `budgets_test.go`'s "Step 0 gained the named-invocation branch"
  - the `prose_contracts_test.go` asserts, `references/edge-paths.md`, `docket-status`'s skill text
  - the convention's "step 0 amended by change 0397"

  0469 kept this label on purpose. It is a step number in a numbered procedure, not the startup check.
- **Other "step 0" numbering:** the harness validation runbook's "Phase 2 step 0".
- **Frozen records:** testdata corpora, results files, archived changes, specs and Accepted ADR bodies.

## Out of scope

- The overloaded gate-drive halt tokens `worktree-changed` / `launch-unconfirmed` (change 0481).
- Any wire token, flag, schema key, operation id or behavior. Only words in strings and comments change.
- Skill and agent-wrapper text. No skill uses either leftover sense today, so consumer repos need no `docket install` re-run.

## Testing

- Run the whole suite at the build gate (`build.test_command`), never only the touched packages.
- `internal/repoguard`'s `skill_handoff_sites_test.go` `non_vacuity` subtest must stay green with the edited fixtures. `classifyHandoffSite` keys on the case-sensitive noun "skill" and the phrase "human is present", and "startup-check" contains neither, so no classification changes.
- Record a closing grep in the results file:
  - `repair` across the files listed under *Sites to change* hits only the *Kept* sites, which are the generic "not repairs" in `change_integration_test.go` and nothing else in those files.
  - A case-sensitive grep for `Step-0` in maintained source, excluding `docs/results`, `docs/superpowers`, `docs/changes`, `docs/adrs` and testdata, hits nothing. The lowercase "step-0" in `prose_contracts_test.go` names implement-next's step and is kept.
  - `Step 0` hits only references to implement-next's step label and the runbook's own numbering.
