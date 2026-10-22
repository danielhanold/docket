<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0575 — Retire claim reclaim and its lease](../../changes/archive/2026-10-22-0575-retire-claim-reclaim-and-its-lease.md)**
<!-- docket:backlink:end -->

# Retire claim reclaim and its lease — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` — one `### Task N` heading per
> worker, one commit per task, tasks in order. Steps use checkbox (`- [ ]`) syntax; nobody ticks them.

**Goal:** Remove `change.reclaim`, the maintenance sweep's reclaim pass, and the `claimed_at` claim
lease end to end, keep `reclaim.lease_ttl` / `reclaim.auto` as warn-and-ignore tombstones, and make
every maintained doc describe stranded-run recovery as resume-by-id with the run tracker deciding.

**Architecture:** This is deletion work, done in five tasks that each build and pass tests on their own.
First the reclaim operation and sweep leg go (they are the only things that read the lease's TTL
verdict). Then the readers stop requiring or forbidding `claimed_at`, which makes it safe for the
writers to stop stamping it. Then the domain field, decoder, lease, and claim receipt fields go. The
docs and skills are cleaned before the config tombstone lands, because once the keys are marked
obsolete, `TestLivingDocsAlignment` and `TestExampleSchemaCorrespondence` flag any surviving
`reclaim.*` spelling.

**Tech Stack:** Go (Cobra CLI, `internal/{domain,app,repository,reposetup,config,cli,document}`),
Markdown skills/docs mirrored into `internal/assets/embedded/tree/` by `go generate ./internal/assets/`.

**Spec:** `docs/superpowers/specs/2026-10-22-retire-claim-reclaim-and-its-lease-design.md` (metadata
branch). Read it alongside this plan: its section numbers (§1–§7) are cited below, and its site lists
are the minimum, not the complete set.

## Global Constraints

- **Build every removal list from a whole-repo grep, never from this plan's lists** (spec "Design"; AGENTS.md
  "Never hand-list the sites"). Grep `reclaim`, `claimed_at`, `ClaimedAt`, `lease`/`Lease`, `refreshes the
  claim`. Then drop the unrelated senses: `--force-with-lease`/`PushLease`/`PushCreateLease`, the
  transaction engine's expected-ref lease, finalize's publication lease, `lease-contended`/`ReasonRunLeaseContended`
  in `run_verify.go`, `install collect` version trees, `transaction/cleanup.go`'s `PruneAbandoned`,
  gate-drive claims, and `claim_eligible`.
- **Do not edit point-in-time records:** `docs/changes/**`, `docs/adrs/**` bodies, `docs/superpowers/**`,
  `docs/results/**`, `docs/release/**`, learnings, `testdata/bash-upgrade/**`, `internal/install/legacydata/**`,
  `internal/install/testdata/legacy/**`, `internal/export/testdata/corpus/**`,
  `internal/repository/testdata/corpus/**`, and **every sample under `testdata/repositories/**`**
  (see *Deviation from the spec* below).
- No `schema_version` bump. Do not add `reclaim.lease_ttl` / `reclaim.auto` rows to
  `internal/repoguard/retired_vocabulary_test.go`. Leave its row 44a (`reclaim-version-missing`) as it is:
  it is the record of a rename, and its `Old` token is still correctly absent.
- A `reclaim:` block in any config layer must never make an operation refuse. A malformed value
  (`auto: maybe`) still gets the same type error it gets today (spec §3), and nothing more.
- Unchanged: resume, `change.resume-halted`, the run tracker, `## Run halted`, `reconciled:`, claim
  eligibility, `branch:`, claim's removal of a leftover `## Run halted`, and the `claim-replaced` verdict.
- Living docs (`docs/**` outside point-in-time dirs, `skills/**`, `agents/**`, `AGENTS.md`) never cite a
  change or PR number, and they describe only current behavior: delete removed features outright.
- Never hand-edit `internal/assets/embedded/tree/`. After any `skills/**` or `.docket.example.yml` edit, run
  `go generate ./internal/assets/` and commit the regenerated tree in the same task.
- Compile check for every task: `go vet -tags integration,e2e ./...` (the `integration` and `e2e`
  build tags gate `change_integration_test.go`, `maintenance_traffic_integration_test.go`,
  `finalize_e2e_test.go`, and others; plain `go vet` misses them). Always pass `-count=1` to `go test`.
- Test asserts that guard a mechanism that survives are re-pointed. Test asserts whose subject is gone are deleted. Neither kind is
  re-gated to green (learning `test-premise-deleted-not-regated`). Mutation-test every new absence pin
  and confirm the mutation landed with `grep -c` before and after (learning `assert-detects-removal-not-replacement`).

### Deviation from the spec (decided at planning — record it in the results file)

Spec §7 says to remove the `reclaim:` block from `testdata/repositories/configs/example-activated` and to
move the `four-layer-collision` fixtures to another key. `testdata/README.md` says these samples are
data docket must keep reading, and must never be edited to make a test pass. An old config carrying
`reclaim:` is exactly what the tombstone exists to keep loading. So the samples stay byte-unchanged, and
their tests change instead (Task 5): the reclaim leaves become obsolete-warning assertions, which gives
the tombstone coverage against a real sample. `testdata/repositories/configs/invalid/alias-merge`
(which uses `reclaim: <<: *base`) also stays unchanged. It still fails on the alias merge.

## Review Focus

1. **Old configs keep working.** A `reclaim:` block in `.docket.yml`, `.docket.local.yml`, or the global
   config loads with one `obsolete-setting` warning per key, and mutation preflight stays allowed (Task 5,
   `TestReclaimKeysAreObsoleteTombstones`, plus the real-sample asserts in `fixtures_test.go`).
2. **Old records keep loading.** An in-progress or archived record whose `claimed_at:` holds a stamp,
   an empty value, or a malformed value decodes with no finding. Validation reports nothing about it, the repair
   planner offers nothing, and kill/done/complete leave the line byte-identical (Tasks 2 and 3).
3. **An expired in-progress record produces no sweep entry at all.** No skipped or reclaim entry, no
   preparation, no dispatch. The implementation-scope preflight therefore never shows a stale-claim
   item (Task 1, `TestSweepIgnoresInProgressRecords`).
4. **Idempotent claim replay of a pre-change receipt.** A committed claim receipt that still carries
   `claimed_at` and `lease` still decodes, and replays as `already-claimed` with id, status, and branch (Task 3,
   `TestDecodeChangeClaimReceiptToleratesRetiredKeys`).
5. **Old callers of the removed command.** `docket change reclaim …` exits 2 as an unknown command.
   `change.reclaim` is absent from the capability catalog and from `app.SchemaFor` (Task 1,
   `TestChangeReclaimRetired`).

---

### Task 1: Delete the reclaim operation and the sweep's reclaim pass

**Build tier:** standard

**Files:**
- Delete: `internal/app/change_reclaim.go`, `internal/app/change_reclaim_test.go`
- Modify (domain): `internal/domain/lease.go`: delete `ReclaimVerdict`, `EvaluateReclaim`, `blockingBranch`,
  `Reclaim`, `leaseRefusalReason`, and the four `reason*` consts. **Keep** `LeaseState`, its consts, and
  `EvaluateLease`, because `change_claim.go` still reads them until Task 3.
  `internal/domain/actions.go`: delete the "Reclaim is declared in lease.go" comment.
  `internal/domain/lease_test.go`: delete the `Reclaim`/`EvaluateReclaim` cases and keep the `EvaluateLease` cases.
- Modify (app): `maintenance.go`, `schema_registry.go`, `schema_vocab.go`, `finding_codes.go`,
  `derived_views.go`, `change_relink.go`, `change_claim.go` (header comment only: "reclaim is 0316")
- Modify (cli): `internal/cli/change.go`, `install.go`, `maintenance.go`, `finalize.go` (comment)
- Tests: `internal/cli/change_test.go`, `internal/cli/revision_rename_test.go`,
  `internal/app/{maintenance_test,maintenance_assess_test,maintenance_preflight_test,maintenance_traffic_integration_test,change_integration_test,finalize_e2e_test,schema_vocab_test,finding_codes_test,derived_views_guard_test}.go`,
  `tests/test_go_integration_app_changeruntime.sh` (header comment)

**Interfaces:**
- Produces: `sweepWorklist(snap domain.Snapshot, queue []domain.FinalizeCandidate, scope SweepScope) (items []sweepWorkItem, deferredHistorical int)`.
  The `eff` and `now` parameters are dropped, and so is the `time` import if nothing else uses it.
  `sweepOps` has no `reclaim` field. The sweep kinds are only `sweepKindCloseout` and `sweepKindCleanup`.
- `domain.EvaluateLease` / `domain.LeaseState` still exist; Task 3 deletes them.

- [ ] **Step 1: Write the failing tests**

In `internal/cli/change_test.go`, add `TestChangeReclaimRetired`, a copy of the
`TestChangeRefreshClaimRetired` shape with `"reclaim"` / `"change.reclaim"` substituted. Keep the same
non-vacuity checks on `change.claim`, and expect exit 2 from
`runCLI(t, "change", "reclaim", "--id", "1", "--revision", "x", "--json")`.

In `internal/app/maintenance_test.go`, add `TestSweepIgnoresInProgressRecords`. The corpus holds one
`sweepInProgressBlob(50, "stale")` whose stamp is long expired. The reader uses a pin from
`sweepPin`; Step 6 drops `sweepPin`'s reclaim auto/TTL parameters, so call it in that reduced form. Run `maintenanceSweep` once in
each of `SweepScopeFull` and `SweepScopeImplementation`, and assert for both:
```go
if len(res.Entries) != 0 { t.Fatalf("an in-progress record must yield no sweep entry, got %+v", res.Entries) }
if len(ops.prepared) != 0 || len(ops.calls) != 0 { t.Fatalf("nothing may be prepared or dispatched; prepared=%d calls=%v", len(ops.prepared), ops.calls) }
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 ./internal/cli/ -run TestChangeReclaimRetired` and
`go test -count=1 ./internal/app/ -run TestSweepIgnoresInProgressRecords`
Expected: both FAIL. The command is still registered, and the sweep emits a `reclaim`/`reclaim-auto-disabled` entry.

- [ ] **Step 3: Delete the operation, CLI, schema, and vocabulary**

- Delete `change_reclaim.go`.
- Remove `newReclaimSubcommand` and its registration from `internal/cli/change.go`, and the `"change reclaim"`
  entry from `internal/cli/install.go`'s asset-independent list.
- Remove the `change.reclaim` row from `schema_registry.go` and `v["reclaim_dispositions"]` from `schema_vocab.go`.
- In `finding_codes.go`, remove `branch-still-exists`, `lease-not-expired`, `malformed-claim-stamp`, and
  `missing-claim-stamp`. Remove `FCWorkspaceServiceUnavailable` only if a grep proves the reclaim closure was
  its sole minter (the spec traced that it is). Also update the census comment in `finding_codes_test.go`
  that lists `workspace-service-unavailable`, and drop `reclaimSkip` from `ctorLit`.
- Reason consts whose values other operations also define (`unknown-change`, `ambiguous-change`,
  `workspace-probe-failed`) stay alive through those operations.

- [ ] **Step 4: Delete the sweep's reclaim pass** (spec §1 *Sweep*)

In `maintenance.go`:
- delete `sweepKindReclaim`, `ReasonSweepReclaimAutoDisabled`, `ReasonSweepReclaimRevisionMissing`,
  the `sweepOps.reclaim` field and its production closure, `case sweepKindReclaim`, the `reclaims`
  list in `sweepWorklist`, and `sweepRunReclaim`;
- keep `sweepObservedRevision` (closeout and cleanup still call it);
- keep building `wdeps` for `sweepAssessHistorical`, but drop `wsErr`. Go rejects a variable that is
  only ever assigned, so on a `NewService` error or a nil client leave `wdeps` at its zero value, which
  is what assessment already got on that path.

Then reword every comment there that names reclaim: the file header ("the batch driver that reclaims docket's
closing half" → wording without the word, decided at grooming), `SweepScope`, `sweepOps`, `MaintenanceEntry`, `MaintenanceSweep`, and
`sweepWorklist`.

- [ ] **Step 5: Fix the help text and comments**

- `internal/cli/maintenance.go`: the group Short `"Reclaim docket's closing half in batch (docket status stays read-only)"`
  and the sweep Short `"… and reclaim expired claims in one pass"` lose the word. Pick the exact wording yourself,
  for example `"Close out merged changes and retry close-out cleanup in one pass"`. Fix the package comments that list "reclaim".
- Fix the comments in `internal/cli/finalize.go`, `derived_views.go` (the always-board-visible op list), `change_relink.go`
  ("the same remote branch-facts probe reclaim gathers"), and the `change_claim.go` header.

- [ ] **Step 6: Update the tests whose subject is gone, and re-point the ones that guard a surviving mechanism**

- Delete: `TestIntegrationChangeRuntimeReclaim*` (5 functions) in `change_integration_test.go`; `TestSweepReclaimGatedOnAuto`,
  `TestDisabledReclaimPreparesNothing`, and `TestReclaimRevisionFromObservation` in `maintenance_test.go`; `TestChangeReclaim*`
  in `internal/cli/change_test.go`; the `reclaim_dispositions` row in `schema_vocab_test.go`; the `reclaim` rows in
  `revision_rename_test.go`; the `Reclaim`/`EvaluateReclaim` cases in `lease_test.go`.
- Edit `maintenance_test.go`. Remove `recordingSweepOps.reclaim` and its seam closure. Drop the reclaim legs of the
  structured-report, reload, implementation-scope, and history-growth tests. In
  `TestSweepIntegrationSyncAdvanceUpgradesNoOpToApplied`, give the no-op entry another non-applying item: a
  closeout or cleanup whose fake result is a `skipped`/no-op disposition. Keep that test's assertion that the
  sync upgrades a no-op sweep to applied.
- Update the `sweepWorklist` callers in `maintenance_assess_test.go` to the new signature, and delete the `reclaim`
  rows from `maintenance_preflight_test.go`. Keep each table's other rows and any non-vacuity floor it carries.
- **Re-point** `TestIntegrationSweepDispatchedReclaimKeepsSetupOnce` (`maintenance_traffic_integration_test.go`) to
  `TestIntegrationSweepDispatchedCloseoutKeepsSetupOnce`. It is that file's only proof that a *dispatched* sweep operation keeps
  full setup to one run. Drive a real dispatched closeout: an `implemented` record whose PR the `trafficGH`
  fake reports merged, the same fixture shape the file's other closeout tests use. Keep the
  `wantProbes := 1 + syncSuffixDefaultBranchProbes` default-branch-probe assertion. Re-derive the
  `ls-remote --heads` count from what a closeout and its cleanup suffix require, and state in a comment why
  that count is operation-specific proof rather than per-history fan-out. Reword the header comments.
- `finalize_e2e_test.go`: rename `TestE2EHaltResumeAndReclaim` to `TestE2EHaltResume`. Keep its halt/resume half,
  delete the reclaim half (the `change reclaim` call and its config), and drop the `reclaim` row from the
  argv matrix (`{"reclaim", []string{"change", "reclaim", …}}`).
- Comment-only: `derived_views_guard_test.go` and the `tests/test_go_integration_app_changeruntime.sh` header.

- [ ] **Step 7: Verify**

Run: `go vet -tags integration,e2e ./... && go test -count=1 ./internal/domain/ ./internal/cli/ && go test -count=1 ./internal/app/ -run 'Sweep|Maintenance|Schema|FindingCode|Reclaim'`
then `go test -count=1 -tags integration ./internal/app/ -run 'TestIntegrationSweep|TestIntegrationChangeRuntime'`
and `go test -count=1 -tags e2e ./internal/app/ -run TestE2EHaltResume`.
Expected: all PASS. Then confirm that
`git grep -n -i 'reclaim' -- internal/app internal/cli internal/domain tests ':!*_test.go'` shows no hit in
the operation, sweep, or lease sense. Remaining hits are claim-stamp code (Tasks 2–3), `eff.Reclaim` config plumbing
(Task 5), or an unrelated sense listed in the Global Constraints.

- [ ] **Step 8: Commit**

```bash
git rm internal/app/change_reclaim.go internal/app/change_reclaim_test.go
git add internal/domain internal/app internal/cli tests/test_go_integration_app_changeruntime.sh
git commit -m "refactor: delete change.reclaim and the maintenance sweep's reclaim pass"
```

---

### Task 2: Stop validation and repair from requiring or forbidding `claimed_at`

**Build tier:** standard

**Files:**
- Modify: `internal/repository/validate.go`, `internal/app/validation_text.go`,
  `internal/reposetup/repair.go`, `internal/app/repository_check.go`, `internal/app/repository_migrate.go`,
  `internal/document/document.go` (comment)
- Modify: `internal/app/testdata/finding-catalog.txt` (regenerated)
- Tests: `internal/repository/validate_test.go`, `internal/reposetup/{repair_test,repair_text_test,findingtext_test}.go`,
  `internal/app/{validation_text_test,finding_catalog_test,repair_remedy_family_test,repository_repair_test,repository_repair_integration_test,config_diagnostics_test}.go`,
  `internal/document/empty_value_test.go` (comment), `tests/test_go_integration_app_reporepair.sh` (header)

**Interfaces:**
- Produces: `reposetup.PlanRepairs(path string, src []byte, loc RecordLocator) ([]RepairFinding, error)`. The
  `archived bool` parameter goes because only the claimed_at planner read it. Update both production
  callers and every test caller.
- Removed: `repository.CodeChangeFinalClaimStamp`, `reposetup.RepairDropClaimedAt`, `planClaimedAt`,
  `claimedAtHoldsValue`, and the `RepairDropClaimedAt` arm of `buildRepair`.
- The writers still stamp and clear `claimed_at` after this task. Only the readers change.

- [ ] **Step 1: Write the failing tests**

In `validate_test.go`, add `TestInProgressWithoutClaimStampIsCoherent`: an in-progress record with
`branch: 'feat/x'` and no `claimed_at:` line yields no `CodeChangeStateIncoherent` finding. Also add
`TestArchivedFinalRecordWithClaimStampHasNoFinding`: an archived `done` record carrying
`claimed_at: '2026-05-20T10:00:00Z'` yields no finding whose field is `claimed_at`.
In `repair_test.go`, add `TestPlanRepairsIgnoresClaimedAt`: `PlanRepairs` over an archived `done`, an archived
`proposed`, and an active `in-progress` record, each carrying a non-empty `claimed_at`, returns no finding with
`Field == "claimed_at"`. Write it against the new two-argument-plus-loc signature, so its failure is the
compile error that drives Step 3.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 ./internal/repository/ -run 'ClaimStamp' && go test -count=1 ./internal/reposetup/ -run TestPlanRepairsIgnoresClaimedAt`
Expected: FAIL. Validation reports `claimed_at` missing or final-stamped, and reposetup does not compile against the new signature.

- [ ] **Step 3: Implement**

- `validate.go`: `claimFacts` keeps only the `branch` requirement. Rename it `branchFacts` or keep the name, and
  either way fix its comment ("the claim facts a held lease requires": no lease exists). Delete the
  `CodeChangeFinalClaimStamp` const and its check in the archive-placement switch.
- `validation_text.go`: delete the `CodeChangeFinalClaimStamp` entry.
- `repair.go`: delete the `drop-final-claimed-at` roster line in the header, `RepairDropClaimedAt`, `planClaimedAt`,
  `claimedAtHoldsValue`, its call in `PlanRepairs`, the `buildRepair` arm, and the `archived` parameter. Leave every other
  `frontmatter-manual-review` case alone. An archived record in a non-final status is still caught by validation's
  placement check.
- `repository_check.go` / `repository_migrate.go`: drop the argument. If `repository_migrate.go`'s `archived` local
  becomes unused, delete it.
- `document.go` comment on `EmptyValue`: it no longer names "reposetup's claimed_at planner". The repository decoder
  is now its only caller.

- [ ] **Step 4: Update the existing tests**

Delete the claim-stamp cases. In `validate_test.go`, that means the `CodeChangeFinalClaimStamp` row, its spelling pin, and the
"missing claimed_at" coherence case. In `repair_test.go`, delete `TestRepairDropClaimedAt*` (all five). Also delete the
`drop-final-claimed-at` rows in `repair_text_test.go`, `findingtext_test.go`, `validation_text_test.go`, and
`repair_remedy_family_test.go`. In `config_diagnostics_test.go`'s `appendFindingBlock`, swap the code for another repair
code such as `quote-unsafe-scalar`; the test is about block formatting.
`repository_repair_test.go` and `repository_repair_integration_test.go` use `drop-final-claimed-at` as their repairable
subject. Re-point them to `quote-unsafe-scalar` (an archived record with `title: yes`) so they keep proving
repair plan/apply. Do not delete them.
`finding_catalog_test.go`: remove the `drop-final-claimed-at` and `claimed_at, not done` rows from the shapes table,
and `RepairDropClaimedAt` from the floor. Then regenerate the golden:
`go test -count=1 ./internal/app/ -run TestFindingCatalogGolden -update`, and review the diff. Only the
`change-final-claim-stamp` and `drop-final-claimed-at` sections may disappear.
Update the comment headers in `tests/test_go_integration_app_reporepair.sh` and `empty_value_test.go`.

- [ ] **Step 5: Verify**

Run: `go vet -tags integration,e2e ./... && go test -count=1 ./internal/repository/... ./internal/reposetup/ ./internal/document/ && go test -count=1 ./internal/app/ -run 'Finding|Validation|Repair|Diagnostic' && go test -count=1 -tags integration ./internal/app/ -run 'Repair'`
Expected: PASS. `git grep -n 'drop-final-claimed-at\|change-final-claim-stamp' -- internal tests` shows only
`retired_vocabulary_test.go` (row 58, a rename record, and a non-vacuity sample).

- [ ] **Step 6: Commit**

```bash
git add internal/repository internal/reposetup internal/document internal/app tests/test_go_integration_app_reporepair.sh
git commit -m "refactor: stop requiring or repairing claimed_at in validation and repository repair"
```

---

### Task 3: Stop writing `claimed_at`; delete the lease and the claim's lease fields

**Build tier:** premium. The spec names a consequential risk here: the upgrade-together compatibility of
new records and old receipts. The run-tracker ownership churn test also needs to stay meaningful.

**Files:**
- Delete: `internal/domain/lease.go`, `internal/domain/lease_test.go`
- Modify: `internal/domain/actions.go`, `internal/domain/entities.go`, `internal/repository/decode.go`,
  `internal/app/change_claim.go`, `internal/app/change_reconcile.go`, `internal/app/change_halt.go`,
  `internal/app/change_implemented.go` (comment), `internal/app/runtracker_verdict.go` (comments)
- Tests: `internal/domain/{actions_test,entities_test}.go`, `internal/repository/decode_test.go`,
  `internal/app/{change_claim_test,change_reconcile_test,change_kill_test,change_complete_test,change_halt_test,workflow_integration_test,claim_workflow_git_test,claim_proof_git_integration_test,change_integration_test,finalize_closeout_integration_test,runtracker_ownership_integration_test,runtracker_verdict_integration_test,run_verify_test}.go`

**Interfaces:**
- Consumes: Task 1 (no reclaim code reads `ClaimedAt`) and Task 2 (validation no longer requires the stamp).
- Produces: `domain.Claim(c Change) (ActionResult, *PolicyFailure)`, which drops the unused `now`. `domain.RefreshClaim`,
  `Change.ClaimedAt()`, `ChangeSpec.ClaimedAt`, `setClaimedAt`, `clearClaimedAt`, both `stampLayout` consts,
  `EvaluateLease`, and `LeaseState` are gone. `ChangeClaimResult` and `changeClaimReceipt` have no `ClaimedAt`/`Lease`;
  `changeClaimOp` has no `ttlHours`. `domain.OptionalTime` stays (it has other users).

- [ ] **Step 1: Write the failing tests**

- `change_claim_test.go`, `TestClaimWritesNoClaimStamp`: claim a proposed record and assert that the committed record
  bytes contain no `claimed_at:` line, that `ChangeClaimResult.HumanText()` equals
  `"change 0001 claimed (feat/<slug>) — " + revision` (spec §2: `change 0575 claimed (refactor/…) — …`), and that the
  receipt JSON has no `claimed_at` or `lease` key.
- `change_claim_test.go`, `TestDecodeChangeClaimReceiptToleratesRetiredKeys`: `decodeChangeClaimReceipt` on
  `{"branch":"feat/x","claimed_at":"2026-08-02T00:00:00Z","gate_context_hash":"","id":7,"lease":"fresh","op":"change.claim","status":"in-progress"}`
  returns `ok == true` with `ID == 7`, `Branch == "feat/x"`, `Status == "in-progress"`.
- `change_reconcile_test.go` and the resume-halted test in `change_halt_test.go`: given a record carrying
  `claimed_at: 2026-08-02T00:00:00Z`, the line is byte-identical after the operation.
- `change_kill_test.go` / `change_complete_test.go` / `finalize_closeout_integration_test.go`: a record carrying a stamped
  `claimed_at:` keeps that exact line after kill, complete, or closeout. Today it is rewritten to a bare `claimed_at:`.
- `decode_test.go`, `TestDecodeIgnoresClaimedAt`: records whose `claimed_at:` is stamped, empty, or malformed
  (`claimed_at: not-a-time`) each decode with zero findings.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestClaimWritesNoClaimStamp|TestDecodeChangeClaimReceiptToleratesRetiredKeys|Reconcile|ResumeHalted|Kill|Complete' && go test -count=1 ./internal/repository/ -run TestDecodeIgnoresClaimedAt`
Expected: FAIL. Claim stamps the line, the human line says `lease fresh`, kill/complete blank the line, and a malformed stamp produces a finding.

- [ ] **Step 3: Implement the domain and decoder**

- `actions.go`: `Claim` no longer stamps (fix its doc comment and drop `now`). Delete `RefreshClaim`, `setClaimedAt`,
  `clearClaimedAt`, and `stampLayout`. `killedResult`, `MarkDone`, and `Complete` stop calling `clearClaimedAt`; fix their doc
  comments ("a killed record holds no lease", "an archived record holds no lease"). Remove `ClaimedAt` from `newChangeBuilder`.
- `entities.go`: remove `ChangeSpec.ClaimedAt` and `Change.ClaimedAt()`.
- Delete `lease.go`.
- `decode.go`: remove `changeWire.ClaimedAt`, the `spec.ClaimedAt` assignment, and `stampLayout` (fix the `dateLayout`
  comment that pairs them) along with the "reposetup's claimed_at planner" comment. The unknown-keys rule
  then covers old lines.

- [ ] **Step 4: Implement the app layer**

- `change_claim.go`: drop `ClaimedAt`/`Lease` from `ChangeClaimResult` and `changeClaimReceipt` (keep the receipt's
  json keys alphabetical, and fix the field-order comment that names `claimed_at`). Drop `ttlHours` and its
  `eff.Reclaim.LeaseTTL.Value` assignment, the `EvaluateLease` call, and the `out.ClaimedAt`/`out.Lease` copies.
  The applied `HumanText` becomes `"change %04d %s (%s) — %s"` over `ID, Disposition, Branch, Revision`.
  Fix the header ("The lease is re-stamped only by claim, …") and the patch-pass comment ("several absent fields
  (branch, claimed_at)" → name the fields a claim does insert).
- `change_reconcile.go`: delete the `domain.RefreshClaim` call and its FieldChange loop. Fix the header ("refreshes
  the claim (re-stamps claimed_at)") and the step-3 comment.
- `change_halt.go`: `resumeHaltedOp.Plan` drops `RefreshClaim`; it parses `edited` only if it still needs to and
  writes the marker-removed bytes. Remove `resumeHaltedOp.clock` if nothing reads it. Fix the comments on the header,
  `ChangeHalt`, and `resumeHaltedOp` that mention "claim lease".
- `change_implemented.go` ("never touches claimed_at"): name only `branch:`. `runtracker_verdict.go`: reword the two
  "reclaimed and re-claimed" comments. The guard fires when a newer claim proof exists for the bound change.

- [ ] **Step 5: Update the existing tests**

- `actions_test.go`: remove the stamp assertions from `TestClaimSetsBranchStampAndReconciled` (rename it to drop "Stamp"),
  delete `TestRefreshClaimRestampsOnly`, remove the clear checks in kill/done/complete and the `ClaimedAt` field of the
  action-matrix fingerprint, and update every `Claim(c, now)` call. Remove the `ClaimedAt` cases from `entities_test.go`.
- `change_claim_test.go`, `change_reconcile_test.go`, `change_kill_test.go`, `change_complete_test.go`,
  `finalize_closeout_integration_test.go`, and the resume-halted integration tests: replace the stamp/clear assertions with the
  byte-identical-line assertions from Step 1.
- `claim_proof_git_integration_test.go`: drop `ClaimedAt`/`Lease` from its receipt fixture struct, and keep at least one fixture
  that is a pre-change receipt carrying both keys. `change_integration_test.go`: delete the claim `Lease` assertion.
- `workflow_integration_test.go` (the test near "reconcile after a claim re-stamps the record's claimed_at") and
  `claim_workflow_git_test.go` use the re-stamp as proof that the record genuinely changed. Re-point both to fields
  reconcile still changes a day later: `updated:` and the new `## Reconcile log` entry. Keep the property under test.
- `runtracker_ownership_integration_test.go` re-stamps `claimed_at` as its "real reconcile edit" churn. Switch it
  to an edit reconcile still makes (`updated:`, `reconciled:`, or a reconcile-log append), so the test keeps proving
  that unrelated churn does not move ownership. Mutation-check it: make the churn a no-op and the test must
  fail (or prove that the churn byte-changes the record before the ownership assert).
- `runtracker_verdict_integration_test.go`: reword the header that describes the retired `claimed_at` ≥ dispatch-time filter.
  Leave the `claimed_at` lines in fixture strings (`runtracker_verdict_helpers_test.go`, `run_verify_test.go`, and others) alone
  unless a test is about claim facts. The decoder ignores them.

- [ ] **Step 6: Verify**

Run: `go vet -tags integration,e2e ./... && go test -count=1 ./internal/domain/ ./internal/repository/... && go test -count=1 ./internal/app/ -run 'Claim|Reconcile|Halt|Resume|Kill|Complete|Closeout|RunTracker|Verdict|Ownership' && go test -count=1 -tags integration ./internal/app/ -run 'Claim|Reconcile|Resume|Closeout|Ownership|Verdict|Workflow'`
Expected: PASS. `git grep -n 'ClaimedAt\|RefreshClaim\|EvaluateLease\|LeaseState' -- internal` returns nothing.
`git grep -n 'claimed_at' -- 'internal/*.go' ':!*_test.go'` returns only Task 5's config sites, or nothing.

- [ ] **Step 7: Commit**

```bash
git rm internal/domain/lease.go internal/domain/lease_test.go
git add internal/domain internal/repository internal/app
git commit -m "refactor: stop stamping claimed_at and drop the claim lease from the claim result"
```

---

### Task 4: Skills and docs describe resume-by-id as the only recovery

**Build tier:** standard

**Files:**
- Modify: `skills/docket-convention/SKILL.md`, `skills/docket-implement-next/SKILL.md`,
  `skills/docket-implement-next/references/edge-paths.md`, `skills/docket-status/SKILL.md`
- Modify: `docs/guide/{keeping-the-backlog-honest,capturing-work,building-without-supervision,README}.md`,
  `docs/concepts/change-lifecycle.md`, `docs/reference/{fields,glossary,cli,config-keys}.md`,
  `docs/install/global-config.md`, `docs/README.md`, `docs/comparison/ai-native-sdlc-playbook.md`
- Modify: `internal/repoguard/prose_contracts_test.go`, `internal/repoguard/budgets_test.go`
- Regenerate: `internal/assets/embedded/tree/**`

**Interfaces:**
- Consumes: Tasks 1–3 (the behavior the docs now describe).
- Produces: no living doc spells `reclaim.lease_ttl`, `reclaim.auto`, or a `reclaim:` block. Task 5 relies on this,
  because marking the keys obsolete makes `TestLivingDocsAlignment` fail on any surviving spelling.

- [ ] **Step 1: Write the failing prose pins**

In `prose_contracts_test.go`, replace the `change_0360_revision_lookup` row for `skills/docket-convention/SKILL.md`.
Drop its `present: "stamped by claim, reconcile, and resume-halted"` and keep `absent: "refreshed at phase boundaries"`.
Then add `change_0575_no_reclaim` rows with these `absent:` pins:
- `skills/docket-convention/SKILL.md`: `` "`change.reclaim`" ``, `"## Reclaim log"`, `"reclaim.lease_ttl"`, `"lease_ttl:"`,
  `"claimed_at:"`, `"self-heals"`, `"Reclaim edge"`
- `skills/docket-implement-next/SKILL.md`: `` "`change.reclaim`" ``, `"claimed_at"`, `"refreshes the claim"`, `"lease classification"`
- `skills/docket-implement-next/references/edge-paths.md`: `"refreshes the claim"`, `"no reclaim"`
- `skills/docket-status/SKILL.md`: `"reclaim"` (every sense here is the retired one)

Each row also gets one `present:` pin on a phrase the edit keeps, so the row is not vacuous. For the convention,
use `"and by the claim when a halted change is claimed again"` (spec §5 keeps it). For implement-next and
edge-paths, use the resume-by-id wording you write in Step 3, for example `"docket run start implement-next --resume <id>"`.
Comment the block `// change 0575 — claim reclaim and the claimed_at lease are retired; …`.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 ./internal/repoguard/ -run TestProseContracts`, using the name the table's driver test
actually has (`grep -n 'range proseContracts'`).
Expected: FAIL on every new absent pin.

- [ ] **Step 3: Edit the skills** (spec §5, bullet by bullet)

Apply every convention, implement-next, edge-paths, and docket-status bullet in spec §5. A stranded run recovers by
resuming the change by id (`docket run start implement-next --resume <id>`); on `resume-active-run`, cancel
with `docket run cancel --key <key> --reason <why>` and resume; for a halted change, `change.resume-halted`.
Reuse the wording of edge-paths' *Resume of an in-progress change* and of `docs/concepts/run-tracker.md`. In the
convention's `halt` row, replace "the reclaim lease self-heals an abandoned claim" with the resume-halted recovery,
or drop it. In edge-paths, keep "no stolen lease" if it means claim ownership; reword it if it means the stamp.
In docket-status, replace the `reclaim-auto-disabled` example of an intentional skip with a skip reason
`maintenance.go` still emits (`grep -n 'Reason.*= "' internal/app/maintenance.go`), or drop the example.
Then run `go generate ./internal/assets/`.

- [ ] **Step 4: Edit the docs** (spec §5 *Guides*, *Concepts and reference*, *Indexes*, playbook)

Apply each listed edit. Before you edit, grep every listed file, because the spec's per-file list is a floor. In
`docs/reference/cli.md`, line 42's `docket install collect … reclaims` is the unrelated sense and stays. Leave
`docs/install/install.md` and `keeping-current.md` alone (version-tree sense). In `config-keys.md`, remove both
rows (obsolete keys are not listed there). In `global-config.md`, swap `reclaim.lease_ttl` for another any-layer
key such as `review.max_fix_tasks`. In the glossary, keep a slimmed *Claim* entry (no lease, no reclaim), and
delete the `reclaim.*` entry, the `reclaim_dispositions` row, the docket-status "reclaim expired claims" phrase,
both index links, and `change reclaim` from the `proposed` row. In the playbook, fix rows 121 and 171, and change
"receipts and leases" to "receipts" on the audit-trail row. The `reclaim-claims.sh` retired-scripts entry stays.
Write no change or PR citations.

- [ ] **Step 5: Re-baseline the skill budgets**

Run `go test -count=1 ./internal/repoguard/ -run TestSkillSizeBudgets -v`, then measure each edited skill file
(`wc -l` and `wc -w`, matching `wcLines`/`wcWords`). Lower each edited file's `skillBudgets` row to its exact new
counts, and prepend a `0575: reclaim and the claim lease are retired (old -> new);` note to that row's comment.

- [ ] **Step 6: Verify, and mutation-test the pins**

Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/`
Expected: PASS. Then, for each new absent pin, re-insert the removed phrase into a scratch copy of the file in place
(copy the file aside first, since `git checkout --` would destroy the edit; learning `mutation-restore-needs-a-backup-copy`).
Confirm with `grep -c` that the phrase is present, rerun the prose test, watch it fail, and restore from the copy.
Finish with `git grep -n -i 'reclaim\|claimed_at\|claim lease' -- skills docs/guide docs/concepts docs/reference docs/install docs/README.md docs/comparison`.
Its only hits may be the `install collect` version-tree sense and the playbook's retired-scripts history.

- [ ] **Step 7: Commit**

```bash
git add skills docs/guide docs/concepts docs/reference docs/install/global-config.md docs/README.md docs/comparison internal/repoguard internal/assets/embedded
git commit -m "docs: describe resume-by-id as the only recovery for a stranded run"
```

---

### Task 5: Tombstone the two config keys and delete the dead config plumbing

**Build tier:** standard

**Files:**
- Modify: `internal/config/{schema,decode,config,defaults,resolve}.go`, `internal/app/config.go`,
  `internal/app/finalize_context.go`, `internal/domain/entities.go`, `internal/repository/build.go`,
  `.docket.example.yml`, `testdata/repositories/configs/PROVENANCE.md` (note only; samples untouched)
- Regenerate: `internal/assets/embedded/tree/**`
- Tests: `internal/config/{decode_test,capability_test,setting_paths_test,resolve_test,defaults_test,schema_test,fixtures_test}.go`,
  `internal/app/repository_set_visibility_integration_test.go`, `internal/reposetup/visibilityconfig_test.go` (optional),
  `internal/repository/build_test.go`, `internal/repository/transaction/loader_test.go`, `internal/domain/entities_test.go`,
  `internal/app/finalize_e2e_test.go` (reclaim config block and unsupported-config matrix row, if Task 1 left any)

**Interfaces:**
- Consumes: Tasks 1 and 3 (no code reads `eff.Reclaim` except the plumbing deleted here), and Task 4 (docs are clean).
- Produces: `config.Effective` has no `Reclaim` field, and `type Reclaim` is gone. `FinalizePolicy` has no `ReclaimAuto` or
  `ReclaimLeaseTTLHours` (so `context.finalize` drops `reclaim_auto` and `reclaim_lease_ttl_hours`). `domain.RepositoryPolicy`
  has no `ReclaimTTLHours`. `diagnostic.config` prints no `reclaim.*` effective line.

- [ ] **Step 1: Write the failing test**

In `internal/config/decode_test.go`, add `TestReclaimKeysAreObsoleteTombstones`, modeled on
`TestSkipResultsOnlyDeltaIsObsoleteTombstone`, with the same per-source asserts: not declared, exactly one
`CodeObsoleteSetting` warning with `Classification == Obsolete`, `Remedy == "remove " + path + " from " + src.Name`,
no deferred or shared-ignored diagnostics, `PreflightMutation(snap).Allowed`, and the capability inactive and
non-blocking. Run it for both paths, and for each over these sources:
`srcR("reclaim:\n  lease_ttl: 24\n")`, `srcR("reclaim: {auto: true}\n")`, `srcL(…)`, and `srcG(…)` (block and flow).
Also assert that the message contains `"docket no longer returns claimed changes to the backlog; it is ignored"`, and
that a `srcR("reclaim:\n  auto: maybe\n")` source still yields its type-error diagnostic, as it does today.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -count=1 ./internal/config/ -run TestReclaimKeysAreObsoleteTombstones`
Expected: FAIL. Both keys are still supported and resolve as declarations.

- [ ] **Step 3: Implement the tombstone** (spec §3)

- `schema.go`: both rows become `disp: dispObsolete` with no `def` and no explicit `scope` (default any-layer), keeping
  `intLeaf(0)` and `boolLeaf()`. Replace the `// 13-14: reclaim.` comment with a tombstone comment shaped like
  `skip_results_only_delta`'s.
- `decode.go`: add one `decodeLeaf` case per key, shaped like the `finalize.skip_results_only_delta` case. Message:
  `"docket no longer returns claimed changes to the backlog; it is ignored"`. Remedy: `"remove reclaim.<key> from " + d.src.Name`.
  Return before the leaf is appended.
- Delete `Effective.Reclaim`, `type Reclaim`, the defaults entry, and the two `resolve.go` assignments.
- Delete the two `diagnostic.config` lines in `internal/app/config.go`, `FinalizePolicy.ReclaimAuto`/`ReclaimLeaseTTLHours`
  and their assignments in `finalize_context.go` (and "approval/reclaim policy" in its doc comment), and
  `RepositoryPolicy.ReclaimTTLHours` and its writer in `build.go`.
- `.docket.example.yml`: remove the `reclaim:` section. Then run `go generate ./internal/assets/`.
- `testdata/repositories/configs/PROVENANCE.md`: under `example-activated/` and `four-layer-collision/`, add one sentence
  saying their `reclaim.*` leaves are now obsolete settings, kept so old configs are proven to keep loading.

- [ ] **Step 4: Update the tests**

- Flip both rows to obsolete in `decode_test.go`'s acceptance table, `capability_test.go`, and `setting_paths_test.go`.
- Re-point tests that used a `reclaim.*` key as a generic subject. Pick a key with the same kind, merge kind, and
  any-layer scope: for `reclaim.auto` (nested bool, default false), `finalize.require_pr_approval`; for
  `reclaim.lease_ttl` (nested int), `review.max_fix_tasks` or `finalize.resolver_max_attempts`. Confirm the row
  in `schema.go` first. This applies to `resolve_test.go`'s per-leaf precedence and explicit-default provenance tests,
  `decode_test.go`'s empty-section test, and `repository_set_visibility_integration_test.go`
  (`privateSwitchLocalYML`, the `Effective.Reclaim.Auto` read, and the `ConfigLeafValues`/`"reclaim"` checks). It
  optionally applies to `internal/reposetup/visibilityconfig_test.go`.
- `defaults_test.go`, `schema_test.go` (defaults map; obsolete rows have no default), `build_test.go`, `loader_test.go`,
  and `entities_test.go`: delete the `Reclaim`/`ReclaimTTLHours` reads.
- `fixtures_test.go` (samples unchanged, see *Deviation*): drop the two `reclaim.*` rows from the sparse-defaults and
  example-activated explicit-leaf tables and from the built-in drift check. Then add to `TestFixtureExampleActivated`:
  ```go
  for _, p := range []string{"reclaim.lease_ttl", "reclaim.auto"} {
      // exactly one CodeObsoleteSetting diagnostic on p, Classification Obsolete; PreflightMutation(snap).Allowed
  }
  ```
  In `TestFixtureFourLayerCollision`, replace the `reclaim.lease_ttl == 200` / repository-provenance block with an assertion
  that `reclaim.lease_ttl` yields one obsolete warning from each of the repository and global files and never
  resolves. Then name, in a comment there, the `resolve_test.go` test that now carries repository-beats-global for
  a nested scalar the local layer does not enter, so that coverage is relocated rather than lost.

- [ ] **Step 5: Verify**

Run: `go vet -tags integration,e2e ./... && go test -count=1 ./internal/config/ ./internal/repository/... ./internal/domain/ ./internal/assets/ ./internal/repoguard/ && go test -count=1 ./internal/app/ -run 'Config|Finalize|Context' && go test -count=1 -tags integration ./internal/app/ -run 'SetVisibility'`
Expected: PASS, including `TestExampleSchemaCorrespondence` and `TestLivingDocsAlignment`. Then run the residual
whole-repo grep from Global Constraints over every maintained path and fix any straggler in its own sense.
Finally run the whole suite once, the way the gate will: `go run ./cmd/docket development test`. Expected:
exit 0. Read the budget report.

- [ ] **Step 6: Commit**

```bash
git add internal .docket.example.yml testdata/repositories/configs/PROVENANCE.md
git commit -m "refactor: make reclaim.lease_ttl and reclaim.auto obsolete tombstones"
```

---

## After the build (controller-owned, not build tasks)

- **ADR (Step 6, through `docket-adr` with this change as the producing change):** record a new ADR that **supersedes
  ADR-0159**, carrying the decision, consequences, and rejected alternatives from spec §6, with `relates_to:
  [59, 84, 101]` and the three stale passages noted in its context. ADR-0159 is already in this change's `adrs:`,
  so its status flip ships with the change (learning `adr-update-delivery`). Do not edit the bodies of
  ADR-0059, ADR-0084, or ADR-0101.
- **Results file and PR body:** carry the upgrade note: every machine sharing a repository's docket metadata
  must upgrade before its next claim, because a pre-change binary reports `change-state-incoherent` on a record the new
  binary claimed. Record the *Deviation from the spec* above (testdata samples kept byte-unchanged).
