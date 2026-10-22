<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0575 — Retire claim reclaim and its lease](../../changes/archive/2026-10-22-0575-retire-claim-reclaim-and-its-lease.md)**
<!-- docket:backlink:end -->

# Retire claim reclaim and its lease — design

## Why

Claim reclaim returns an `in-progress` change to `proposed` when all of these hold: its `claimed_at` stamp is strictly older than `reclaim.lease_ttl` (72h by default); neither the recorded `branch:` nor the conventional `<type>/<slug>` branch exists locally or on the remote; and no live workspace exists. It runs only when typed (`docket change reclaim`), or from the maintenance sweep when `reclaim.auto: true` (off by default). It arrived with #89 (2026-07-17), borrowed from a competitor's lease-and-reclaim feature, and was ported to Go in #316.

It has never fired, and its one window is already covered:

- No change record has ever carried a `## Reclaim log`. That holds for this repository's full metadata history, checked 2026-10-21, and for the other local repositories using docket. No saved session ran a real `docket change reclaim`, and `reclaim.auto` is set in no config layer.
- In every saved session the sweep reported a stale claim for exactly one change, #422, six times. It had halted with a feature branch, so reclaim would have refused it anyway, and the board already showed it as halted.
- The building clone has a local branch from `workspace.prepare` onward, minutes after the claim, which blocks reclaim. A run that dies before then is recovered by resuming the change by id (which works without a workspace since #368), with the run tracker's verdict deciding any retry.
- From another clone, reclaim is a risk. A live run's branch is invisible there until `workspace.publish`, so a reclaim after the TTL could hand back a change that is still being built. ADR-0159 records this as an accepted risk.

The lease has no other job. `claimed_at` is read only by lease evaluation (reclaim, the sweep's reclaim pass, and the `lease` value echoed in the claim result) and by validation that insists the stamp exists. Nothing else reads it: not the board, `docket status`, `context.implementation`, `change.relink`, or the run tracker, which has bound a dispatch to its claim through claim proofs since #407 (ADR-0111).

The docs describe reclaim wrongly in several places. The guides say it rescues a run that "crashed before pushing a branch", but a local branch or workspace also blocks it. The convention's `halt` fallback says "the reclaim lease self-heals an abandoned claim", which has never been true in Go: every halt in that row happens after `workspace.prepare`, so a branch exists. The convention also says `claimed_at` is "cleared on leaving in-progress", but the code keeps it through blocked, implemented, and stacked-merged and clears it only at kill, done, and complete.

## Goals

- No operation, sweep pass, config key, body section, or doc returns an `in-progress` change to `proposed`.
- `claimed_at` is never written, cleared, required, or read. Records that already carry the line keep loading unchanged.
- **Upgrade together (decided at grooming).** No compatibility step for older binaries. Today's validation treats a missing `claimed_at` on an in-progress, implemented, or stacked-merged record as an error. So a pre-change binary reading a record the new binary claimed reports `change-state-incoherent`, and refuses that change's reconcile, mark-implemented, and finalize writes until it upgrades. On one machine there is no gap, because the binary is rebuilt right after the merge. The results file's human-actions section and the PR body carry an upgrade note for the next release's notes: every machine sharing a repository's docket metadata must upgrade before its next claim.
- An old config carrying `reclaim.lease_ttl` or `reclaim.auto` loads in every layer with one warning per key and never blocks anything.
- Every maintained doc describes the recovery of a stranded run as: resume the change by id, with the run tracker deciding.

## Non-goals and anti-goals

- **No replacement stale-claim signal.** Accepted at grooming. The sweep's skipped-reclaim entries were the only place a stale claim showed up, and the one change they ever named was already flagged as halted. A crashed coordinator surfaces at the next `docket run start implement-next --resume <id>` (`resume-active-run`).
- **No new way back to `proposed`.** After this change, nothing moves `in-progress` to `proposed` (#568 already refuses deferring an in-progress change). A stranded change is resumed, blocked, or killed.
- **No `schema_version` bump.** `SchemaVersion` versions the schema surface's own format, not each operation's fields. #360 removed a whole operation without a bump, and a bump would make installed skills refuse to run.
- **Anti-goal:** a `reclaim:` block in `.docket.yml`, `.docket.local.yml`, the global config, or a private repository's config must never make an operation refuse. Deleting the schema rows outright would do exactly that: `reclaim:` would become an `unknown-key` error, and every operation would fail with `ErrInvalidConfig`.
- **Unchanged:** resume, `change.resume-halted`, the run tracker, `## Run halted`, `reconciled:`, claim eligibility, and `branch:`.
- **Unrelated uses of the word stay:** `docket install collect` reclaiming version trees, `transaction/cleanup.go`'s `PruneAbandoned`, gate-drive claims, and `claim_eligible`.
- **Point-in-time records stay as written:** archived change records, specs, plans, results, release notes, Accepted ADR bodies, learnings war stories, and frozen fixtures (`testdata/bash-upgrade/**`, `internal/install/legacydata/**`, `testdata/repositories/documents/active-change-0158.md`).

## Design

**Build the removal list from a whole-repo grep, not from the lists below.** Grep for `reclaim`, `claimed_at`, `ClaimedAt`, `lease`/`Lease`, and `refreshes the claim`, then sort each hit into executable or prose. Drop the unrelated senses: git exact-lease pushes (`--force-with-lease`, `PushLease`, the transaction engine's expected-ref lease, finalize's publication lease, `lease-contended` in `run_verify.go`), `install collect`'s version trees, `PruneAbandoned`, gate-drive claims, and `claim_eligible`. Then drop the point-in-time records. The lists in each section are the sites known at grooming; the builder check found more, and there may be others.

### 1. Delete the reclaim operation and the sweep's reclaim pass

- **Operation.** Delete `internal/app/change_reclaim.go` whole: the `change.reclaim` op, its request, result, receipt, dispositions, proofs, and `## Reclaim log` writer. Also delete:
  - the CLI subcommand (`newReclaimSubcommand` and its registration in `internal/cli/change.go`);
  - the `"change reclaim"` entry in `internal/cli/install.go`'s asset-independent list;
  - the `change.reclaim` schema binding (`schema_registry.go`) and the `reclaim_dispositions` vocabulary (`schema_vocab.go`).

  Reason consts whose values other operations also define (for example `workspace-probe-failed`, `unknown-change`, `ambiguous-change`) stay alive through those operations.
- **Domain.** Delete `internal/domain/lease.go` whole: `EvaluateReclaim`, `Reclaim`, `blockingBranch`, and its refusal reasons, plus `EvaluateLease` and `LeaseState`. Nothing reads them once the claim's `lease` field goes (section 2). Fix `actions.go`'s "Reclaim is declared in lease.go" comment. Reclaim has no transition enum value, so no switch needs editing.
- **Sweep** (`internal/app/maintenance.go`). Delete:
  - `sweepKindReclaim`;
  - `ReasonSweepReclaimAutoDisabled` and `ReasonSweepReclaimRevisionMissing`;
  - the `sweepOps.reclaim` field and its closure, and the `case sweepKindReclaim` arm;
  - the reclaim list in `sweepWorklist` and `sweepRunReclaim`.

  Keep the workspace service: `sweepAssessHistorical` still uses it. Only the reclaim closure's `wsErr` path goes. Drop `sweepWorklist`'s `eff` and `now` parameters if they become unused, and the `time` import with them. Delete `FCWorkspaceServiceUnavailable` if, as traced, only the reclaim closure mints it.
- **Finding-code registry** (`finding_codes.go`). Remove `branch-still-exists`, `lease-not-expired`, `malformed-claim-stamp`, `missing-claim-stamp`, and any other code only reclaim mints. Shared codes stay.
- **Dead config plumbing.** Delete:
  - `context.finalize`'s `FinalizePolicy.ReclaimAuto` (`reclaim_auto`) and `ReclaimLeaseTTLHours` (`reclaim_lease_ttl_hours`). Nothing reads them.
  - `domain.RepositoryPolicy.ReclaimTTLHours` and its only writer in `internal/repository/build.go`. Nothing reads them.
  - `diagnostic.config`'s effective lines for both keys (`internal/app/config.go`).
  - `config.Effective.Reclaim`, `type Reclaim`, its defaults, and its resolve assignments.
- **Comments that name reclaim:** fix them in `derived_views.go` (the always-board-visible op list), `change_relink.go` ("the same remote branch-facts probe reclaim gathers"), `change_claim.go`, `internal/cli/finalize.go`, and `internal/cli/maintenance.go`.
- **Help text.** `docket maintenance sweep`'s Short drops "and reclaim expired claims". `docket maintenance`'s group Short "Reclaim docket's closing half in batch", and the matching "batch driver that reclaims docket's closing half" comment in `maintenance.go`, are reworded so the word no longer appears (decided at grooming). Exact wording is the builder's call.

### 2. Retire `claimed_at`

Decided at grooming: stop writing it, rather than keep it as a "claimed on" date (git history already records the claim commit) or leave it as a lease nothing reads.

- **Writers.** Delete `setClaimedAt`, `clearClaimedAt`, and `domain.RefreshClaim`:
  - `change.claim` stops stamping;
  - `change.reconcile` and `change.resume-halted` stop re-stamping;
  - `Kill`, `MarkDone`, and `Complete` stop writing a bare `claimed_at:`.

  An existing line in a record is left exactly as it is by every write.
- **Domain and decode.** Remove `ChangeSpec.ClaimedAt` and `Change.ClaimedAt()`, and the `claimed_at` field of `changeWire` in `internal/repository/decode.go`. The decoder already treats unknown frontmatter keys as compatibility data ("Unknown keys are compatibility data and stay in the document; they are not decoded here and never fail the decode"). So an old record's `claimed_at:` line, empty, stamped, or even malformed, loads with no finding.
- **Validation** (`internal/repository/validate.go`):
  - `claimFacts` stops requiring `claimed_at`. `branch` stays required for in-progress, implemented, and stacked-merged. Rename the function or fix its comment ("the claim facts a held lease requires"), since no lease exists.
  - Delete the `change-final-claim-stamp` check and its finding text in `validation_text.go`. If kill, done, and complete no longer clear the stamp, an older record finishing under the new binary would keep one, and an error there would block writes to that record.
- **Repair** (`internal/reposetup/repair.go`). Delete `planClaimedAt`, `claimedAtHoldsValue`, the `drop-final-claimed-at` repair code (`RepairDropClaimedAt`), and the non-repairable "archived but not done/killed, still holds `claimed_at`" variant of `frontmatter-manual-review`. An archived record in a non-final status is still caught by validation's placement check. Other repair codes, and other `frontmatter-manual-review` cases if any exist, are untouched. Update `internal/app/testdata/finding-catalog.txt` to match.
- **Claim receipt and result** (`internal/app/change_claim.go`):
  - Drop `claimed_at` and `lease` from `changeClaimReceipt` (the committed Docket-Result trailer) and from `ChangeClaimResult`.
  - Drop the claim's TTL lookup (`ttlHours: eff.Reclaim.LeaseTTL.Value`).
  - The human line loses its lease part. Before: `change 0575 claimed (refactor/retire-claim-reclaim-and-its-lease) lease fresh — …`. After: `change 0575 claimed (refactor/retire-claim-reclaim-and-its-lease) — …`.

  Older committed receipts that carry both keys still decode (extra keys are ignored), and `claim_proof.go` reads neither. ADR-0129's "committed state keeps its spelling" covers renames, not new receipts carrying fewer keys.
- **Comments that describe the lease.** Fix them in `change_halt.go` (header, `ChangeHalt`, `resumeHaltedOp`), `change_reconcile.go` (header and the patch-step comment), `change_implemented.go` ("never touches claimed_at"), `internal/document/document.go` and `internal/repository/decode.go` (both name "reposetup's claimed_at planner"), and `finalize_context.go`'s `FinalizePolicy` doc comment ("approval/reclaim policy").
- **Before/after on a record.** Before, a claim leaves `claimed_at: '2026-10-22T09:14:03Z'` in the frontmatter, reconcile overwrites it, and a missing line on an in-progress record is a `change-state-incoherent` error. After, a new claim writes no `claimed_at:` line. An older in-progress record that has one keeps it untouched until it is archived, and nothing reads it.

### 3. Tombstone the two config keys

Decided at grooming: follow the existing retired-key path used by `finalize.skip_results_only_delta`, `runtime.bash`, and `metadata_branch`.

- In `internal/config/schema.go`, keep `reclaim.lease_ttl` and `reclaim.auto` as `disp: dispObsolete` rows, with no `def`, the default any-layer scope, and their existing validators (`intLeaf(0)`, `boolLeaf()`). Keeping the rows also keeps `reclaim` an interior node, so a `reclaim:` block still parses.
- In `internal/config/decode.go`, add a `decodeLeaf` case per key, shaped like `finalize.skip_results_only_delta`'s. It emits an `obsolete-setting` warning with `Classification = Obsolete`, a message saying docket no longer returns claimed changes to the backlog and the value is ignored, and the remedy `remove reclaim.<key> from <file>`. It returns before the leaf is appended. Example: `reclaim.auto: docket no longer returns claimed changes to the backlog; it is ignored — remove reclaim.auto from .docket.yml`.
- This covers every layer: the repository `.docket.yml` (read from origin), `.docket.local.yml`, the global config, and a private repository's config. The warning shows in `diagnostic.config`, `docket status` findings, and install warnings. It never resolves into `Effective` and never blocks a mutation preflight. A malformed value (`auto: maybe`) still reports its type error first, as every tombstone's validator does today.
- Remove the `reclaim:` section from `.docket.example.yml`, then run `go generate ./internal/assets/` to refresh the embedded copy. With the rows marked obsolete, `TestLivingDocsAlignment` and `TestExampleSchemaCorrespondence` (Direction C) flag every remaining `reclaim.*` spelling in the living docs. That makes the docs pass in section 5 self-checking.
- Do **not** add `reclaim.lease_ttl` or `reclaim.auto` rows to `internal/repoguard/retired_vocabulary_test.go`. The tombstone needs those exact literals in Go, and `skip_results_only_delta` has no row for the same reason.

### 4. Keep two defensive paths, and fix their wording

- **Claim strips a leftover `## Run halted`** from a `proposed` record. After this change, only a record written by an older docket can reach that state (for example, one deferred while in progress before #568, then revived). Keep the code, because a stale marker on a re-claimed change would make `run.verify` report `run-halted` for a fresh run. Remove only the prose that ties it to reclaim.
- **The run tracker's `claim-replaced` verdict** fires when a newer claim proof exists for the bound change. It never read the lease. Keep it as a fail-closed ownership guard, and reword the "reclaimed and re-claimed" comments in `runtracker_verdict.go` and its two integration tests.

### 5. Skills and docs

Edit the authored `skills/**` files, then regenerate `internal/assets/embedded/tree/` with `go generate ./internal/assets/`; never hand-edit the mirror. For every edit below, a stranded run's recovery reads as: resume the change by id (`docket run start implement-next --resume <id>`). If that reports `resume-active-run`, cancel the old run with `docket run cancel --key <key> --reason <why>` and resume; for a halted change, use `change.resume-halted`. `skills/docket-implement-next/references/edge-paths.md`'s *Resume of an in-progress change* and `docs/concepts/run-tracker.md` already describe this; reuse their wording.

- **`skills/docket-convention/SKILL.md`:**
  - Drop the `reclaim:` lines from the config sample.
  - The `halt` row loses "the reclaim lease self-heals an abandoned claim"; replace it with the resume-halted recovery, or nothing.
  - Drop the `claimed_at:` manifest line, and "survives reclaim" from the `branch_prefix:` line.
  - Delete the `## Reclaim log` body-section bullet.
  - Delete the lifecycle diagram's `lease expired, no branch (reclaim)──▶ proposed` edge.
  - Delete the *Reclaim edge* paragraph.
  - Keep `## Run halted`'s "and by the claim when a halted change is claimed again", which is still accurate as behavior.
- **`skills/docket-implement-next/SKILL.md`:**
  - Claim paragraph: drop "+ `claimed_at:` — the lease the `change.reclaim` operation and the maintenance sweep's reclaim key on", and "lease classification" from what the claim returns.
  - Delete "A change that instead reached `proposed` through the `change.reclaim` operation carries no live claim…".
  - Checkpoint row 2: drop "+ `claimed_at:`".
  - Delete "`claimed_at` is stamped only by the claim, reconcile, and resume-halted transactions."
  - Named-invocation paragraph: drop "or reclaim" from "another change's failing closeout, cleanup, or reclaim".
  - Resume-halted paragraph and the reconcile-transaction sentence: drop "refreshes the claim" from both.
  - The halt bullet: "leaving the branch, lease, workspace, and evidence untouched" drops "lease".
- **`skills/docket-implement-next/references/edge-paths.md`:** drop "and no reclaim" from the bounded dependency closeout. Review "no stolen lease": keep it if it means the claim or ownership, and reword it if it means the claim lease.
- **`skills/docket-status/SKILL.md`:** remove every reclaim mention:
  - the read "never … reclaims", twice;
  - implementation scope's "plus reclaim gating" and its "`reclaim.auto`-gated reclaims";
  - the full sweep's "eligible reclaims";
  - "attempts an eligible reclaim only when `reclaim.auto` is true";
  - the `reclaim-auto-disabled` example of an intentional skip (replace it with a remaining intentional skip if one exists, or drop it);
  - "A stale claim surfaces as a `maintenance.sweep` reclaim entry…".
- **Guides:**
  - `docs/guide/keeping-the-backlog-honest.md`: delete the intro clause about a crashed run going back to the queue, the status and sweep "reclaims" mentions, and the whole *Reclaiming expired claims* section with its YAML fence. The "crashed before pushing a branch" bullet is replaced by the single resume path above. The "crashed after pushing a branch" bullet becomes the general case and gains the `resume-active-run` → `run cancel` step.
  - `docs/guide/capturing-work.md`: drop the backward-edge paragraph, the claim lease, and reclaim. A claim records which branch will carry the work.
  - `docs/guide/building-without-supervision.md`: step 2 drops "and when it was taken" and the claim-lease self-heal sentence.
- **Concepts and reference:**
  - `docs/concepts/change-lifecycle.md`: drop "and when it was taken".
  - `docs/reference/fields.md`: drop "and when it was taken".
  - `docs/reference/glossary.md`: remove `claimed_at` from the manifest field list and "Moved there by … `change reclaim`" from the `proposed` row. Delete the *Claim / claim lease / reclaim* entry, or reduce it to *Claim* without lease or reclaim. Delete the *`reclaim.auto` / `reclaim.lease_ttl`* entry, the `reclaim_dispositions` vocabulary row, the docket-status "reclaim expired claims" phrase, and both index links.
  - `docs/reference/cli.md`: drop "and reclaim expired claims" from `docket maintenance`.
  - `docs/reference/config-keys.md`: remove both rows. Obsolete keys are not listed there (no existing tombstone is).
  - `docs/install/global-config.md`: swap the example key `reclaim.lease_ttl` for another any-layer key.
- **Indexes:** `docs/README.md` and `docs/guide/README.md` drop "reclaiming expired claims".
- **`docs/comparison/ai-native-sdlc-playbook.md`:** this is kept current (edited 2026-10-19 for trivial's retirement). Fix the two rows that list reclaim as a docket capability, and drop "and leases" from the audit-trail row's "receipts and leases" (settled at grooming: there the lease is the claim stamp; pushes leave no trail). The `reclaim-claims.sh` entry in its retired-scripts list is history and may stay. This file is outside the docs-alignment guard's roots, so no test flags a miss here.

### 6. ADR

Record a new ADR through `docket-adr` that **supersedes ADR-0159**, so 0159's status becomes "Superseded by ADR-N". The decision: docket keeps no claim lease and no reclaim. `claimed_at` is never written or read, and old lines are ignored. The two `reclaim.*` keys are obsolete tombstones that warn and are ignored. A stranded run recovers only by resuming it by id, with the run tracker deciding. The consequences and alternatives cover:

- the evidence (never fired; #422 the only stale claim ever reported, already halted);
- the cross-clone risk;
- no path from `in-progress` back to `proposed` (with #568);
- no replacement stale-claim signal.

Considered and rejected: keeping `claimed_at` as a "claimed on" date; leaving the lease in place; deleting the config keys outright; a silent tombstone.

The ADR's context also notes three stale passages, and lists ADR-0059, ADR-0084, and ADR-0101 in `relates_to`:

- ADR-0059's consequence "the existing reclaim lease self-heals an abandoned claim" no longer applies.
- ADR-0101's implementation-scope schedule no longer includes `reclaim.auto`-gated reclaims.
- ADR-0084's `claimed_at` attribution filters were already replaced by ADR-0111's claim proofs.

The bodies of those three ADRs are not edited.

### 7. Tests

Follow the rule that a test whose subject is gone is deleted, while a test guarding a mechanism that survives is re-pointed, not re-gated to green.

- **Delete:**
  - `internal/app/change_reclaim_test.go` whole;
  - the five `TestIntegrationChangeRuntimeReclaim*` functions in `change_integration_test.go`;
  - `internal/domain/lease_test.go` whole, since `EvaluateLease` goes too;
  - in `maintenance_test.go`: `TestSweepReclaimGatedOnAuto`, `TestDisabledReclaimPreparesNothing`, and `TestReclaimRevisionFromObservation`;
  - `TestChangeReclaim*` in `internal/cli/change_test.go`;
  - the `reclaim_dispositions` spec in `schema_vocab_test.go`.
- **Replace, don't just delete:**
  - `TestIntegrationSweepDispatchedReclaimKeepsSetupOnce` in `maintenance_traffic_integration_test.go` is the file's only proof that a sweep-dispatched operation keeps setup to once. Re-point that proof at another dispatched sweep item, such as a closeout, and reword the header comments.
  - `TestE2EHaltResumeAndReclaim` in `finalize_e2e_test.go` keeps its halt/resume half and drops the reclaim half.
- **Edit:**
  - `maintenance_test.go`: the `recordingSweepOps.reclaim` field, `sweepPin`, `sweepInProgressBlob`, and the reclaim legs of the structured-report, reload, implementation-scope, integration-sync no-op, and history-growth tests. The integration-sync test used a skipped reclaim as its no-op entry, so give it another no-op item.
  - The `sweepWorklist` callers in `maintenance_assess_test.go`.
  - `maintenance_preflight_test.go`'s reclaim rows.
  - `finding_codes_test.go`'s `reclaimSkip` constructor pattern.
  - `revision_rename_test.go`'s reclaim rows.
  - `build_test.go`, `transaction/loader_test.go`, and `entities_test.go` (`ReclaimTTLHours`).
  - `finalize_e2e_test.go`'s `reclaim:` config block and its unsupported-config matrix row.
  - `claim_proof_git_integration_test.go`'s receipt fixture fields.
  - `change_integration_test.go`'s claim `Lease` assertion.
- **Claim stamp tests:**
  - Remove the stamp assertions in `actions_test.go` (`TestClaimSetsBranchStampAndReconciled`, `TestRefreshClaimRestampsOnly`, and the clear checks in kill, done, and complete), plus the action-matrix fingerprint field.
  - Update the same assertions in `change_claim_test.go`, `change_reconcile_test.go`, the resume-halted integration tests, `change_kill_test.go`, `change_complete_test.go`, and `finalize_closeout_integration_test.go`.
  - Remove the claim-stamp cases in `validate_test.go`, `decode_test.go`, `repair_test.go`, `repair_text_test.go`, `findingtext_test.go`, `repository_repair_*_test.go`, `repair_remedy_family_test.go`, `validation_text_test.go`, `empty_value_test.go` (if its subject was only `claimed_at`), `tests/test_go_integration_app_reporepair.sh`, and the finding-catalog golden file.
  - **Add** one validation case: an in-progress record with a branch and no `claimed_at:` line is coherent. **Add** one decode case: a record with a stamped or malformed `claimed_at:` line decodes with no finding.
- **Config:**
  - Add `TestReclaimKeysAreObsoleteTombstones`, modeled on `TestSkipResultsOnlyDeltaIsObsoleteTombstone`. It covers both keys across the repository (block and flow), local, and global layers: not declared, exactly one obsolete warning with the exact remedy, preflight allowed, and the capability obsolete and non-blocking.
  - Flip both rows to obsolete in `decode_test.go`'s acceptance table, `capability_test.go`, and `setting_paths_test.go`.
  - Tests that used a `reclaim.*` key as a generic subject move to another key of the same shape and layer scope: `resolve_test.go`'s per-leaf precedence and explicit-default provenance tests, `decode_test.go`'s empty-section test, the `four-layer-collision` fixtures, `repository_set_visibility_integration_test.go` (which reads `Effective.Reclaim.Auto`, a compile break), and optionally `internal/reposetup/visibilityconfig_test.go`.
  - Remove the block from `testdata/repositories/configs/example-activated`.
  - Fix the tests that read `Effective.Reclaim` and stop compiling: `defaults_test.go`, `schema_test.go`'s defaults map (the obsolete rows have no default), and `fixtures_test.go`'s sparse-defaults, example-activated, and four-layer-collision assertions.
- **Run-tracker tests.**
  - `runtracker_ownership_integration_test.go` re-stamps `claimed_at` as its "real reconcile edit" churn. Switch it to an edit reconcile still makes, so the test keeps proving that unrelated churn does not move ownership.
  - Update `runtracker_verdict_integration_test.go`'s header, which still describes the retired `claimed_at` ≥ dispatch-time attribution filter.
- **Comment-only:** the `tests/test_go_integration_app_changeruntime.sh` header and `derived_views_guard_test.go`.
- **Retired-operation guard.** Add `TestChangeReclaimRetired` in `internal/cli/change_test.go`, in the shape of `TestChangeRefreshClaimRetired`: absent from the Cobra tree, the capability catalog, and `app.SchemaFor`, and exit 2.
- **Prose sentinels** (`internal/repoguard/prose_contracts_test.go`):
  - Replace the `change_0360_revision_lookup` presence pin on "stamped by claim, reconcile, and resume-halted" with `absent:` pins on `change.reclaim`, `## Reclaim log`, `reclaim.lease_ttl`, and the `claimed_at:` manifest line in the convention and implement-next skills, plus "refreshes the claim" in implement-next.
  - Mutation-test each pin: restore the removed text and watch it go red.

## Left to the builder

- Exact help-text and doc wording, within the rules above.
- Which key replaces `reclaim.*` in each re-pointed config test, as long as it has the same type, merge kind, and layer scope as the key it replaces.
- Whether to strip now-ignored `claimed_at:` lines from test fixtures. Either way passes, since the decoder ignores them; strip them where the test is about claim facts.
- Whether to re-point retired-vocabulary row 44a (`reclaim-version-missing` → `reclaim-revision-missing`). It names a token that no longer exists, and its `New` value is never checked.
- Whether the glossary keeps a slimmed *Claim* entry or folds it into an existing one.
