<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0574 — Recognize the docket branch by its shape in the everyday repository commands](../../changes/archive/2026-10-20-0574-stop-repository-prepare-re-walking-all-of-main-s-history-to.md)**
<!-- docket:backlink:end -->

# Recognize the docket branch by its shape in the everyday repository commands Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` (one tier agent per task under the docket-build-task contract, one whole-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `repository prepare`, `check`, `repair`, `configure-tests`, and `configure-harnesses` decide metadata-branch ownership from the two structural checks only (one parentless root, no shared history with the integration branch), so prepare on this repository takes seconds; `init` and `migrate` keep the full content proof unchanged.

**Architecture:** Factor the two structural steps that `verifyMetadataOwnership` already runs first into a new `verifyMetadataShape` (the "quick check") in `internal/app/metadata_ownership.go`; the full verifier calls it first, so the shared steps have one source. `augmentCheckFacts` (check, both configure commands, private init's final report) and `prepareAugment` call the quick check directly. `migrateRoute`, shared by migrate and repair, takes the ownership check as a parameter: migrate passes the full verifier, repair passes the quick one. `reconcileResumeSeed` (migrate only) and `repository_init.go` are untouched.

**Tech Stack:** Go; real-git integration tests behind the `integration` build tag (`internal/app`, shard prefix `TestIntegrationRepoOwnership`); unit tests in `internal/app` and `internal/reposetup`.

**Spec:** `docs/superpowers/specs/2026-10-20-stop-repository-prepare-re-walking-all-of-main-s-history-to-design.md` (in the feature worktree; read it with this plan).

## Global Constraints

- A probe that fails is `RootUnknown` with the error retained; unknown refuses exactly as today. Never read a git error as owned or foreign.
- No cache, stored proof, or after-the-fact receipt. The quick check reads no receipt and no file contents.
- `repository init` and `repository migrate` behavior must not change (both keep `verifyMetadataOwnership`, history search included). Migrate's and init's refusal text is unchanged.
- The finding code stays `metadata-root-foreign`.
- The dirty, `local-metadata-ahead`, and `local-metadata-diverged` refusals stay.
- No wall-clock assertion in the suite; speed is accepted by hand and recorded in the results file.
- The ADR (spec decision 5) is recorded by the coordinator after build. No task in this plan writes an ADR.
- Cross-references in maintained source anchor on symbol names, never line numbers (`TestCommentAnchorStyle`).
- Mutation probes run with `-count=1` and restore from a backup copy (`cp` before mutating), never `git checkout --` over uncommitted work.

## Review Focus

1. **Migrate silently picking up the quick check.** `migrateRoute` and `reconcileResumeSeed` must both still run the full verifier for `repository migrate`; a receiptless nonempty root that matches no snapshot must still be refused by migrate. Pinned by Task 2's `TestIntegrationRepoOwnershipEverydayCommandsSkipContentProof` (migrate leg) and the unchanged `TestIntegrationRepoOwnershipMigrate*` tests.
2. **A removed integration-tip guard hiding behind a git error.** Dropping the `integrationTip == ""` guard would still yield `RootUnknown` (the ancestry probe errors), so the test must pin the guard's own error text. Pinned in Task 1's `unknown integration tip` subtest.
3. **The full verifier's Unknown default lost in the refactor.** After `verifyMetadataShape` returns owned, the full verifier must reset `Shape` to `RootUnknown` before its receipt/content probes, or a later probe error returns owned. Pinned by the existing `TestIntegrationRepoOwnershipLegacyUnreadableHistoryIsUnknown` (must stay green) plus Task 1's mutation step.
4. **Repair refusing a quick-owned branch at a later gate.** Repair must reach `phaseAlreadyMigrated` and pin the crafted tip, not just "not refuse as foreign". Pinned in Task 2 by asserting `res.SourceRevision` equals the crafted tip.
5. **Two copies of the reworded finding drifting.** Prepare and check must show the same words. Task 3 defines the text once in `reposetup` and both sites reference it; pinned by the prepare unit test comparing against the shared constants.

---

### Task 1: The quick ownership check

**Files:**
- Modify: `internal/app/metadata_ownership.go` (`seedProof` constants, `verifyMetadataOwnership`, new `verifyMetadataShape`, new `ownershipCheck` type, new `quickOwnershipCheck`)
- Test: `internal/app/repoownership_integration_test.go`

**Interfaces:**
- Produces:
  - `const proofShape seedProof` — appended after `proofLegacyEquivalent`; comment: structural only (one parentless root, disjoint from integration), set by the quick check used by the everyday commands.
  - `func verifyMetadataShape(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, tip, integrationTip gitcli.ObjectID) metadataOwnership` — returns `Tip` always; `Root` once proven sole; `Shape`: `RootForeign` when `len(roots) != 1` or shared ancestry; `RootUnknown` with `Err` on any probe error or empty `integrationTip` (keep today's error text `"integration tip unknown; cannot prove disjoint ancestry"`); otherwise `RootParentless` with `Proof = proofShape`.
  - `type ownershipCheck func(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, tip, integrationTip gitcli.ObjectID, defaultBranch string) metadataOwnership` — `verifyMetadataOwnership` already satisfies it.
  - `func quickOwnershipCheck(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, tip, integrationTip gitcli.ObjectID, _ string) metadataOwnership` — returns `verifyMetadataShape(ctx, git, repo, tip, integrationTip)`.

- [ ] **Step 1: Write the failing test**

In `repoownership_integration_test.go` add a fixture helper beside `verify`:

```go
func (f *ownFixture) verifyShape(t *testing.T, tip, integration gitcli.ObjectID) metadataOwnership {
	t.Helper()
	return verifyMetadataShape(context.Background(), newGitClient(t), gitcli.Repository{PrimaryWorktree: f.dir}, tip, integration)
}
```

Add `TestIntegrationRepoOwnershipShapeCheck`, a table of subtests, each on its own `newOwnFixture(t)`, built with the existing `commitTree`/`mktree`/`emptyTree`/`seedMessage` helpers:

| subtest | fixture | assertions |
|---|---|---|
| `two parentless roots` | merge of two orphan roots (as `TestIntegrationRepoOwnershipTwoParentlessRoots`) | `Shape == RootForeign` |
| `shared ancestry` | commit with parent `mainTip()` carrying an `OpInitRoot` receipt (as `TestIntegrationRepoOwnershipSharedAncestryWithIntegration`) | `Shape == RootForeign` |
| `unreadable tip` | tip `strings.Repeat("a", 40)` | `Shape == RootUnknown`, `Err != nil` |
| `unknown integration tip` | init-receipt empty root, integration `""` | `Shape == RootUnknown`, `Err != nil`, `strings.Contains(Err.Error(), "integration tip unknown")` |
| `receiptless nonempty root with no matching snapshot is owned` | root over `mktree({"docs/changes/active/zzz.md": "x\n"})`, no trailers, one descendant on top | quick: `Shape == RootParentless`, `Proof == proofShape`, `Root == root`, `Tip == descendant`; full (`f.verify`) on the same tip: `Shape == RootForeign` (positive evidence the fixture exercises the content-proof gap) |
| `invalid receipt is owned` | root carrying `opTrailer(reposetup.OpInitRoot)` over a NONEMPTY tree | quick: `RootParentless`; full: `RootForeign` |

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoOwnershipShapeCheck$' ./internal/app`
Expected: build failure, `undefined: verifyMetadataShape`.

- [ ] **Step 3: Implement**

Move the topology block of `verifyMetadataOwnership` (from `RootCommits` through the shared-ancestry check) into `verifyMetadataShape`, ending with `RootParentless`/`proofShape`. `verifyMetadataOwnership` starts with:

```go
own := verifyMetadataShape(ctx, git, repo, tip, integrationTip)
if own.Shape != reposetup.RootParentless {
	return own
}
own.Shape, own.Proof = reposetup.RootUnknown, proofNone // receipt/content proofs below decide
```

and keeps the rest unchanged. Add `ownershipCheck` and `quickOwnershipCheck`. Update the doc comments: `verifyMetadataShape` names the everyday commands (prepare, check, repair, configure-tests, configure-harnesses) as its callers and says it reads no receipt or file contents; `verifyMetadataOwnership` says it is reserved for init and migrate and runs `verifyMetadataShape` first.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoOwnership' ./internal/app`
Expected: PASS (new test plus every existing verifier test, unchanged).

- [ ] **Step 5: Mutation-test each structural check**

Back up `metadata_ownership.go` with `cp`, then one mutation at a time, re-running Step 4's command and restoring from the backup after each:
1. delete the `len(roots) != 1` branch → `two parentless roots` must FAIL;
2. delete the `if shared` branch → `shared ancestry` must FAIL;
3. delete the `integrationTip == ""` guard → `unknown integration tip` must FAIL (on the error-text assertion);
4. delete the `own.Shape, own.Proof = reposetup.RootUnknown, proofNone` reset in `verifyMetadataOwnership` → at least one existing verifier test (for example `TestIntegrationRepoOwnershipLegacyUnreadableHistoryIsUnknown` or a `RootForeign` receipt test) must FAIL.
Record each result for the results file. Confirm `git diff` shows only the intended edit after restoring.

- [ ] **Step 6: Commit**

```bash
git add internal/app/metadata_ownership.go internal/app/repoownership_integration_test.go
git commit -m "refactor(repository): factor the structural ownership steps into a quick check"
```

---

### Task 2: Everyday commands use the quick check

**Files:**
- Modify: `internal/app/repository_check.go` (`augmentCheckFacts` call and its comment block)
- Modify: `internal/app/repository_prepare.go` (`prepareAugment` call and its comment block)
- Modify: `internal/app/repository_migrate.go` (`migrateRoute` signature and doc comment; `migratePhaseDispatch` call)
- Modify: `internal/app/repository_repair.go` (`repairPreflight` call)
- Test: `internal/app/repoownership_integration_test.go`; header comment of `internal/app/repoprepare_integration_test.go`

**Interfaces:**
- Consumes: `verifyMetadataShape`, `ownershipCheck`, `quickOwnershipCheck` (Task 1); `verifyMetadataOwnership` (unchanged).
- Produces: `func migrateRoute(ctx context.Context, git *gitcli.Client, facts reposetup.Facts, sc *setupContext, checkOwnership ownershipCheck) (migratePhase, *RepositoryMigrateResult)`.

- [ ] **Step 1: Write the failing tests**

Add a fixture helper in `repoownership_integration_test.go`:

```go
// publishReceiptlessLegacyRoot replaces origin/docket with a parentless,
// receiptless, NONEMPTY root (one valid change record) that matches no
// integration snapshot, then moves the invocation clone's .docket worktree
// (and so its local docket branch) to it, so local and remote stay in sync.
func (r *initRepo) publishReceiptlessLegacyRoot(t *testing.T) string
```

Build it with `wf := &ownFixture{t: t, dir: r.writer}`, tree `wf.mktree({"docs/changes/active/0001-legacy.md": changeRecord(1, "legacy", "Legacy seed")})`, `wf.commitTree(tree, nil, "legacy seed")`, `r.publishCraftedDocket`; then in `filepath.Join(r.invocation, ".docket")`: `git fetch -q origin docket` and `git reset -q --hard FETCH_HEAD`. Return the remote tip.

Add `TestIntegrationRepoOwnershipEverydayCommandsSkipContentProof` on one `newHealthyRepo(t)`:
- `tip := r.publishReceiptlessLegacyRoot(t)`; oracle: `verifyMetadataOwnership` against `r.invocation` at `tip` with integration tip `rev-parse origin/main` returns `RootForeign` (fatal otherwise: the fixture would prove nothing).
- prepare (`runPrepareAt(t, r.invocation)`): `Disposition == PrepareDispositionNoOp`, `!prepareFinding(res, "metadata-root-foreign")`.
- check (`r.runCheck(t)`): `RepositoryState == healthy`, `!findingWithCode(res, "metadata-root-foreign")`, `CheckExitCode() == 0`.
- configure-tests (`r.runConfigureTests(t)`): `res.Result != ResultInvalidState`, `res.RepositoryState != string(reposetup.StateConflict)`.
- repair preview (`r.runRepair(t, RepairOptions{})`): `res.RepositoryState != string(reposetup.StateConflict)` and `res.SourceRevision == tip` (proves `phaseAlreadyMigrated` pinned the crafted tip).
- migrate, last (`r.runMigrate(t, MigrateOptions{})`): `RepositoryState == conflict` and `HumanText()` contains `"not a verified docket metadata branch"`.

Add `TestIntegrationRepoOwnershipEverydayCommandsRefuseStructuralForeign` with two subtests, each on its own `newHealthyRepo(t)`, publishing via `publishCraftedDocket` (no local sync; the foreign verdict comes first in every route):
- `two parentless roots`: in `r.writer`, `git fetch -q origin docket`, `root1 := rev-parse FETCH_HEAD`, `root2` a parentless commit over a one-file tree, tip = `commitTree(emptyTree, {root1, root2}, ...)`.
- `shared ancestry`: `git fetch -q origin main`, tip = commit over a one-change-record tree with parent `rev-parse FETCH_HEAD`.
For each: prepare `Disposition == PrepareDispositionRefused` and `prepareFinding(res, "metadata-root-foreign")`; check `findingWithCode(res, "metadata-root-foreign")`; repair `RepositoryState == conflict` and `HumanText()` contains `"not a verified docket metadata branch"`.

Flip `TestIntegrationRepoOwnershipCheckForeignNonemptyRoot`: rename to `TestIntegrationRepoOwnershipCheckReceiptlessNonemptyRootIsOwned`, keep its fixture, assert `!findingWithCode(res, "metadata-root-foreign")` (state may be conflict from the unsynced local branch; do not assert healthy), rewrite its doc comment to say the everyday check no longer runs the content proof and migrate still refuses such a root.

The unknown cases (unreadable tip, unknown integration tip) are pinned at the function all three commands share (Task 1), and their per-command routing is already pinned by `TestRepositoryPrepareUnknownRootRefusesAsReachability`, `TestIntegrationRepoOwnershipCheckFailedFetchIsUnknown`, and `migrateRoute`'s unchanged `RootUnknown` branch.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoOwnership(EverydayCommands|CheckReceiptless)' ./internal/app`
Expected: `EverydayCommandsSkipContentProof` and `CheckReceiptlessNonemptyRootIsOwned` FAIL (metadata-root-foreign reported, prepare refused, repair refused); `RefuseStructuralForeign` passes already.

- [ ] **Step 3: Implement**

- `augmentCheckFacts` and `prepareAugment`: replace `verifyMetadataOwnership(ctx, git, sc.repo, rev.Commit, gitcli.ObjectID(sc.sourceRevision), sc.defaultBranch)` with `verifyMetadataShape(ctx, git, sc.repo, rev.Commit, gitcli.ObjectID(sc.sourceRevision))`. Rewrite both comment blocks: they now run the quick structural check (one parentless root, no shared history with the integration branch), the content proof is reserved for init and migrate, and fetch errors stay `RootUnknown`.
- `migrateRoute`: add the `checkOwnership ownershipCheck` parameter and call it in place of `verifyMetadataOwnership`. Doc comment: migrate passes `verifyMetadataOwnership`, repair passes `quickOwnershipCheck`; only `own.Shape` is consumed. `migratePhaseDispatch` passes `verifyMetadataOwnership`; `repairPreflight` passes `quickOwnershipCheck`. Do not touch `reconcileResumeSeed` (repair refuses `phaseResumePrune` before it).
- Update the header comment of `repoprepare_integration_test.go`, which says prepare consumes `verifyMetadataOwnership`, to name `verifyMetadataShape`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepo(Ownership|Prepare|Repair|Migration|Check)' ./internal/app`
Expected: PASS.

- [ ] **Step 5: Mutation-test the wiring**

With a `cp` backup of each file, one at a time: put `verifyMetadataOwnership` back in `augmentCheckFacts` → the check and configure-tests legs FAIL; back in `prepareAugment` → the prepare leg FAILS; pass `verifyMetadataOwnership` from `repairPreflight` → the repair leg FAILS; pass `quickOwnershipCheck` from `migratePhaseDispatch` → the migrate leg FAILS. Restore each from the backup, confirm with `git diff`, and record the results.

- [ ] **Step 6: Commit**

```bash
git add internal/app/repository_check.go internal/app/repository_prepare.go internal/app/repository_migrate.go internal/app/repository_repair.go internal/app/repoownership_integration_test.go internal/app/repoprepare_integration_test.go
git commit -m "fix(repository): recognize the docket branch by its shape in prepare, check, repair, and configure"
```

---

### Task 3: Reword the foreign finding and the ownership docs

**Files:**
- Modify: `internal/reposetup/health.go` (`findingFor`, case `"metadata-root-foreign"`; new exported constants)
- Modify: `internal/app/repository_prepare.go` (`prepareRoute`'s `RootForeign` finding)
- Modify: `internal/reposetup/probe.go` (`RootShape` constant comments)
- Modify: `internal/reposetup/classify.go` (the partial-state comment in `Classify`)
- Test: `internal/reposetup/health_test.go`, `internal/app/repository_prepare_test.go`; comment in `internal/reposetup/classify_test.go`

**Interfaces:**
- Produces, in package `reposetup`:
  - `const MetadataRootForeignMessage = "The remote docket branch has more than one starting commit, or shares history with the integration branch, so docket will not treat it as its own."`
  - `const MetadataRootForeignRemedy = "Leave the remote docket branch unchanged and resolve it with a human, then run ` + "`docket repository check`" + `."`
  (Exact wording may be adjusted within spec decision 4; both sites must use these constants.)

- [ ] **Step 1: Write the failing tests**

`internal/reposetup/health_test.go`, new `TestMetadataRootForeignFindingNamesStructuralReason`: `f := findingFor("metadata-root-foreign", Facts{})`; assert `f.Code == "metadata-root-foreign"`, `f.Message == MetadataRootForeignMessage`, `f.Remedy == MetadataRootForeignRemedy`, `strings.Contains(f.Message, "more than one starting commit")`, `strings.Contains(f.Message, "shares history")`, and `!strings.Contains(f.Message, "docket-created")` (detects the removed wording).

`internal/app/repository_prepare_test.go`, extend `TestRepositoryPrepareForeignRootRefusesAsForeign`: `v.finding.Message == reposetup.MetadataRootForeignMessage` and `v.finding.Remedy == reposetup.MetadataRootForeignRemedy`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run 'TestMetadataRootForeignFindingNamesStructuralReason|TestRepositoryPrepareForeignRootRefusesAsForeign' ./internal/reposetup ./internal/app`
Expected: build failure, `undefined: MetadataRootForeignMessage`.

- [ ] **Step 3: Implement**

- Add the two constants to `health.go`; use them in `findingFor`'s `"metadata-root-foreign"` case and in `prepareRoute`'s `RootForeign` finding.
- `probe.go`: `RootParentless` reads "proven owned by the caller's ownership check: the quick structural check (prepare, check, repair, configure-tests, configure-harnesses) or the full content proof (init, migrate)"; `RootForeign` reads "readable evidence the caller's check rejects: more than one root or shared history with the integration branch (both checks), or no valid receipt and no legacy-equivalent tree (full check only)". Keep `RootUnknown` as is.
- `classify.go`: rewrite the comment above the `metadata-seeded-live-surface` partial rule so it no longer claims the `RootForeign` case already claimed every non-corresponding seed tree. Say instead that under the quick check a branch only the full check would reject can land here, and the remedy it gets (`repository migrate`) runs the full check and refuses it, so the route is safe.
- `classify_test.go`: reword the case comment "The seed tree does not correspond: the gatherer sets RootForeign" to "The caller's ownership check reports RootForeign" (behavior unchanged).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/reposetup ./internal/app`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/reposetup/health.go internal/reposetup/health_test.go internal/reposetup/probe.go internal/reposetup/classify.go internal/reposetup/classify_test.go internal/app/repository_prepare.go internal/app/repository_prepare_test.go
git commit -m "fix(repository): name the structural reason in the foreign docket branch finding"
```

---

### Task 4: Whole-suite gate and acceptance timing

**Files:** none modified (evidence for the results file only).

- [ ] **Step 1: Sweep remaining references**

Run: `grep -rn "verifyMetadataOwnership\|RootForeign\|metadata-root-foreign\|docket-created" --include='*.go' internal cmd`
Expected: `verifyMetadataOwnership` called only from `repository_init.go`, `migratePhaseDispatch` (as the passed value), `reconcileResumeSeed`, and tests; no `docket-created` in finding text; every comment that described prepare/check as running the full verifier is updated.

- [ ] **Step 2: Run the whole suite**

Run (the configured `build.test_command`, from the feature worktree): `go run ./cmd/docket development test`
Expected: green. Read the budget report; the `tests/test_go_integration_app_repoownership.sh` row (30s ceiling) gained four fixtures, so compare its measured time against the ceiling and report any breach.

- [ ] **Step 3: Time the commands on this repository by hand**

```bash
b=$(mktemp -d "${TMPDIR:-/tmp}/docket-574.XXXXXX")/docket
go build -o "$b" ./cmd/docket
time "$b" repository prepare --repo-dir /Users/homer/dev/docket --json
time "$b" repository check --repo-dir /Users/homer/dev/docket --json
time "$b" repository repair --repo-dir /Users/homer/dev/docket --json
```

Expected: each finishes in seconds (baseline: prepare 68.7s on 2026-10-20), prepare's disposition is `applied` or `no-op`, and repair without `--yes` is a preview that writes nothing. Record the three timings and dispositions for the results file. If any command still takes about a minute, that is a red result even with a green suite.
