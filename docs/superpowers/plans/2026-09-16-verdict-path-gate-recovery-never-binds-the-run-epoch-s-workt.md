<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0427 — Verdict-path gate recovery never binds the run epoch's worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-16-0427-verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt.md)**
<!-- docket:backlink:end -->
# Verdict-Path Gate Recovery Worktree Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bind the run epoch's feature worktree in both verdict-path claim-recovery legs of `resolveGateOwnership`, so a run recovered solely through the verdict path is fenceable and cancellable like a normally-claimed run.

**Architecture:** `internal/app/rungate_verdict.go` has two recovery legs that call `ConfirmGateClaim` with an empty worktree, leaving `EpochRecord.Worktree` empty for any run recovered only through them — the mutation fence (`findEpochByWorktree`) and `run.cancel` teardown then skip that epoch entirely. The fix threads `PlanningDeps` into `resolveGateOwnership`, adds one small private helper that derives the logical feature path exactly as the fresh-claim path does (`filepath.Join(repo.PrimaryWorktree, ".worktrees", change.Slug)` from the authoritative metadata corpus and canonical repository discovery), passes that path to both recovery `ConfirmGateClaim` calls, and refuses through the existing `gate-stop … gate-unavailable` / `ReasonGateProofUnavailable` path when identity cannot be resolved — never confirming with an empty path.

**Tech Stack:** Go; existing `internal/app` test fixtures (`newRunVerifyFixture`, `gateMintArmed`, `fakeProofScanner`, `MintEpochRecord`, `runCancel`/`cancelSeams`, `admitWorkflowMutation`).

**Spec:** `docs/superpowers/specs/2026-09-16-verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt-design.md` (on the `docket` metadata branch; readable at `.docket/docs/superpowers/specs/…` from the primary tree). Change file: `docs/changes/active/0427-verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt.md`.

## Global Constraints

- Only the two recovery legs, their directly needed context plumbing/comments, and regression tests change. No new command, configuration, schema, ledger, migration, subsystem, or redesign of cancellation/retry semantics (spec §Scope).
- Derive the path from the selected change's authoritative metadata and canonical primary repository identity — never the caller's directory, a branch spelling, or a scan for candidate worktrees (spec §Fix).
- The feature directory need not exist at bind time; keep the fence's canonicalization at compare time. Do not create or prepare a workspace during verdict recovery (spec §Fix).
- Unresolved repository/change identity refuses before confirming, through the existing `ReasonGateProofUnavailable` stop — never a silent empty-path confirm (spec §Fix).
- Preserve receipt matching, ambiguity refusal, continuation handling, retry accounting, and epoch-less behavior exactly (spec §Fix). Change 0422 independently edits retry handling in this file; if it lands first, compose with it — the two edits touch disjoint regions (`resolveGateOwnership` vs the retry CAS block).
- Every mutation probe runs uncached (`go test … -count=1`) and the fixed source is restored afterward from the uncommitted edit itself — never `git checkout --` over an uncommitted fix (learnings: `cached-runner-serves-a-mutated-tree`, `mutation-restore-needs-a-backup-copy`).
- Comment cross-references anchor on symbol names or verbatim clauses, never line numbers (ADR-0054).
- The final gate runs the complete configured suite through the Go runner (`build.test_command` from `.docket.yml`), and the budget report is read even on green.

---

### Task 1: Helper + unconfirmed-reservation leg (with fence regression)

**Files:**
- Modify: `internal/app/rungate_verdict.go` (`resolveGateOwnership` signature + unconfirmed-reservation branch; new helper `gateRecoveredWorktree`; the `RunGateVerdict` call site)
- Test: `internal/app/rungate_fence_test.go` (new `TestVerdictUnconfirmedRecoveryBindsEpochWorktreeSoFenceActs`)

**Interfaces:**
- Consumes: `ConfirmGateClaim(repoDir, key string, changeID int, requestID, revision, worktree string) error` (rungate_store.go); `PlanningDeps{Reader, Client, Clock}`; `parseCorpus`, `repository.BuildSnapshot`, `snap.Change(domain.ChangeID) (Change, LookupOutcome)`, `c.Slug()`; `deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})`; `gateOwnershipStop(repoDir, key, rec, ReasonGateProofUnavailable)`.
- Produces: `gateRecoveredWorktree(ctx context.Context, deps PlanningDeps, repoDir string, changeID int) (string, bool)` — Task 2 reuses it verbatim in the sole-proof leg; `resolveGateOwnership(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir, key string, rec *GateRecord) *RunGateVerdictResult` (deps added as the second parameter).

- [ ] **Step 1: Write the failing regression test**

Append to `internal/app/rungate_fence_test.go` (same package; helpers `newRunVerifyFixture`, `rvRecord`, `rvPR`, `rvSlug`, `rvPlanPath`, `rvResultsPath`, `rvRecordedPR`, `prEvidenceBytes`, `gateMintArmed`, `fakeProofScanner`, `gateGitCommonDir`, `cancelSeams`, `fakeCancelStopper`, `runCancel` are all package-local). Model on `TestFreshRunClaimBindsEpochWorktreeSoFenceActs` (same file) and `TestVerdictUnconfirmedReservationRecoversFromExactReceipt` (`rungate_verdict_test.go`):

```go
// TestVerdictUnconfirmedRecoveryBindsEpochWorktreeSoFenceActs is the change-0427
// regression for the unconfirmed-reservation recovery leg: a fresh epoch whose
// Worktree is empty (as gate-before mints it — neither claim confirmation nor
// fixture setup pre-binds it), a reservation whose confirm was interrupted, and
// the exact committed receipt. The verdict recovery must confirm WITH the
// change's logical feature worktree — before the directory even exists — so
// LoadEpochRecord shows it bound, and after RunCancel a workflow mutation from
// that worktree is refused specifically run-cancelled. Restoring the empty
// worktree argument at the unconfirmed-reservation ConfirmGateClaim call reddens
// both halves.
func TestVerdictUnconfirmedRecoveryBindsEpochWorktreeSoFenceActs(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	repo := f.repo.invocation
	key := gateMintArmed(t, repo, nil, 1, "ha")

	// A REAL fresh epoch, minted exactly as a fresh (non-resume) arm mints it:
	// change unbound (""), Worktree "". Nothing below pre-binds either.
	ep, err := MintEpochRecord(repo, key, "")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}

	// The interrupted-confirm shape: reserved, never confirmed, with the exact
	// committed receipt (same request id, same context hash).
	if err := ReserveGateClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("ReserveGateClaim: %v", err)
	}
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
	}}

	// The expected LOGICAL feature path, from the same canonical identity
	// production derives it from (never the fixture's spelling of repo).
	repoID, err := f.client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: repo})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	want := filepath.Join(repoID.PrimaryWorktree, ".worktrees", rvSlug)
	if _, serr := os.Stat(want); serr == nil {
		t.Fatalf("precondition: feature dir %q must not exist yet (binding precedes workspace.prepare)", want)
	}

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, repo, key)
	if got, wantLine := res.HumanText(), "gate-done "+key+" run-complete 3"; got != wantLine {
		t.Fatalf("HumanText = %q, want %q (recovery must still succeed)", got, wantLine)
	}

	// (a) The recovery confirm bound the epoch's worktree, with the directory
	// still absent — the bind stores the logical path.
	epAfter, _, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if epAfter.Worktree != want {
		t.Fatalf("epoch worktree = %q, want %q (verdict recovery must bind the run epoch's worktree)", epAfter.Worktree, want)
	}

	// (b) Cancel through RunCancel, then a workflow mutation from that feature
	// worktree is refused run-cancelled. The dir exists by now (as it would after
	// workspace.prepare); the fence canonicalizes at compare time.
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir feature worktree: %v", err)
	}
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	seams := cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}}
	cres := runCancel(seams, repo, key, ep.EpochID, "0427 regression stop")
	if cres.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel disposition = %q (findings %v), want cancelled", cres.Disposition, cres.Findings)
	}
	_, ferr := admitWorkflowMutation(want, OperationPRPublish)
	if fe, ok := AsMutationFenceError(ferr); !ok || fe.Reason != "run-cancelled" {
		t.Fatalf("mutation after cancel = %v, want a run-cancelled MutationFenceError (the fence must locate the recovered epoch)", ferr)
	}
}
```

Add any missing imports to `rungate_fence_test.go` (it already has `os`, `filepath`, `gitcli`; it will additionally need `github.com/danielhanold/docket/internal/gatedrive` — copy the import path from `rungate_cancel_test.go`).

- [ ] **Step 2: Run the test to verify it fails on the binding assert**

Run: `go test ./internal/app/ -run 'TestVerdictUnconfirmedRecoveryBindsEpochWorktreeSoFenceActs' -count=1 -v`
Expected: FAIL at `epoch worktree = "" , want …/.worktrees/widget` (the verdict line itself passes; only the bind is missing). If it fails earlier — e.g. the cancel fixture wiring — fix the test scaffolding first; the failure this task exists to see is the empty `Worktree`.

- [ ] **Step 3: Implement the helper and fix the unconfirmed-reservation leg**

In `internal/app/rungate_verdict.go`:

(1) Add `"path/filepath"` and `"github.com/danielhanold/docket/internal/gitcli"` to the imports (`context`, `domain`, `repository` are already imported).

(2) Add the helper (place it after `resolveGateOwnership`'s helper cluster, e.g. below `gateProofsForContext`):

```go
// gateRecoveredWorktree resolves the LOGICAL feature worktree for a change the
// verdict path is recovering — filepath.Join(repo.PrimaryWorktree, ".worktrees",
// slug), the same derivation the fresh-claim path binds (change_claim.go,
// "featureWorktree") and the workspace service's intendedPath use. It reads the
// selected change's authoritative metadata (pin + corpus snapshot) and the
// canonical primary repository identity — never the caller's directory, a branch
// spelling, or a scan for candidate worktrees. The directory need not exist yet:
// the mutation fence canonicalizes the stored value at compare time
// (epochOwnsWorktree), once workspace.prepare has created it. ok=false means
// repository/change identity could not be resolved; the caller must refuse
// (fail closed) rather than confirm with an empty path.
func gateRecoveredWorktree(ctx context.Context, deps PlanningDeps, repoDir string, changeID int) (string, bool) {
	if deps.Reader == nil || deps.Client == nil {
		return "", false
	}
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return "", false
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		return "", false
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return "", false
	}
	c, out := build.Snapshot.Change(domain.ChangeID(changeID))
	if out != domain.LookupFound {
		return "", false
	}
	repo, err := deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return "", false
	}
	return filepath.Join(repo.PrimaryWorktree, ".worktrees", c.Slug()), true
}
```

(3) Thread `deps` through: change the signature to

```go
func resolveGateOwnership(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir, key string, rec *GateRecord) *RunGateVerdictResult {
```

and update its doc comment's last paragraph to note that the two recovery legs resolve the change's logical feature worktree through `gateRecoveredWorktree` before confirming. Update the single call site in `RunGateVerdict`:

```go
	if stop := resolveGateOwnership(ctx, deps, wdeps, repoDir, key, &rec); stop != nil {
		return *stop
	}
```

(4) In the `case hasBinding:` (unconfirmed reservation) branch, replace the empty-worktree confirm and its justifying comment:

```go
		// The committed receipt is authority, so a confirm error still proceeds on the
		// proof (best-effort mirror) — but the worktree it binds must be real: resolve
		// the recovered change's logical feature worktree first (change 0427), and
		// refuse (fail closed, before confirming) when identity cannot be resolved,
		// never confirming with an empty path that would leave the run epoch
		// unfenceable and uncancellable.
		wt, ok := gateRecoveredWorktree(ctx, deps, repoDir, binding.ChangeID)
		if !ok {
			return gateOwnershipStop(repoDir, key, *rec, ReasonGateProofUnavailable)
		}
		_ = ConfirmGateClaim(repoDir, key, binding.ChangeID, binding.RequestID, proof.Revision, wt)
		gateAdoptOwnership(wdeps, repoDir, key, rec, binding.ChangeID, binding.RequestID, proof.Revision)
		return nil
```

Leave the `default:` (sole-proof) branch untouched in this task — Task 2 owns it (its stale `// worktree "": see the confirmed-binding branch above` comment goes there too).

- [ ] **Step 4: Run the new test and the neighboring gate tests**

Run: `go test ./internal/app/ -run 'TestVerdictUnconfirmedRecoveryBindsEpochWorktreeSoFenceActs|TestVerdictUnconfirmed|TestFence|TestFreshRunClaimBindsEpochWorktreeSoFenceActs|TestVerdictConfirmedBinding|TestVerdictNoBinding' -count=1`
Expected: PASS. `TestVerdictUnconfirmedReservationWithoutReceiptStops` and `…SiblingContextHashIsNoAttributableClaim` stay green — they exit on the no-receipt path before the new resolution runs.

- [ ] **Step 5: Mutation-check the fixed call (uncached), then restore**

Back up the fixed file first (`cp internal/app/rungate_verdict.go "${TMPDIR:-/tmp}/rungate_verdict.go.0427-fixed.XXXXXX"` is unnecessary complexity — instead copy to a fixed scratch name under the scratchpad, e.g. `cp internal/app/rungate_verdict.go /tmp-scratch-backup-task1.go` in your session scratchpad directory; never rely on `git checkout --`, which would restore HEAD and destroy the uncommitted fix). Then re-introduce the defect in the unconfirmed-reservation branch only — replace `wt` with `""` in that one `ConfirmGateClaim` call:

Run: `go test ./internal/app/ -run 'TestVerdictUnconfirmedRecoveryBindsEpochWorktreeSoFenceActs' -count=1`
Expected: FAIL — the binding assert (`epoch worktree = ""`) reddens, and had it somehow passed, the post-cancel fence assert would (mutation must actually redden; a green run here is a defect in the test, stop and fix it).

Restore the fixed file from the scratch copy, and re-run the same command uncached.
Expected: PASS.

- [ ] **Step 6: Run the whole package uncached**

Run: `go test ./internal/app/ -count=1`
Expected: PASS. If any pre-existing test now stops `gate-stop gate-unavailable proof-unavailable` because it reached this recovery leg with unresolvable deps (`PlanningDeps{}` or a reader whose corpus lacks the recovered change), fix that test's setup to supply resolvable identity (the rv fixture's reader + real client, as `TestVerdictUnconfirmedReservationRecoversFromExactReceipt` does) — never by weakening the refusal. The audited call sites (`TestVerdictUnconfirmedReservationRecoversFromExactReceipt`, `TestVerdictAbsentBindingAdoptsSoleProof/sole`, `TestVerdictFreshRunBindsScopeChange`) already carry real fixture deps.

- [ ] **Step 7: Commit**

```bash
git add internal/app/rungate_verdict.go internal/app/rungate_fence_test.go
git commit -m "fix(0427): unconfirmed-reservation verdict recovery binds the epoch worktree"
```

---

### Task 2: Sole-proof adoption leg + unresolved-identity refusals

**Files:**
- Modify: `internal/app/rungate_verdict.go` (the `default:` no-binding branch of `resolveGateOwnership`)
- Test: `internal/app/rungate_fence_test.go` (new `TestVerdictSoleProofAdoptionBindsEpochWorktreeSoFenceActs`, `TestVerdictRecoveryUnresolvedIdentityStopsBeforeConfirm`)

**Interfaces:**
- Consumes: `gateRecoveredWorktree(ctx, deps, repoDir, changeID) (string, bool)` from Task 1; `ReserveGateClaim`, `ConfirmGateClaim`, `gateAdoptOwnership`, `gateOwnershipStop`, `ReasonGateProofUnavailable`; the Task 1 test's fixture pattern.
- Produces: both recovery legs bind the worktree; unresolved identity in either leg is a terminal `gate-stop <key> gate-unavailable proof-unavailable` with no confirm (and, for the sole-proof leg, no reservation write either).

- [ ] **Step 1: Write the two failing tests**

Append to `internal/app/rungate_fence_test.go`:

```go
// TestVerdictSoleProofAdoptionBindsEpochWorktreeSoFenceActs is the change-0427
// regression for the absent-binding recovery leg: a fresh epoch with an empty
// Worktree and NO binding file, with exactly one committed proof carrying the
// record's context hash. Adoption must reserve + confirm WITH the change's
// logical feature worktree (directory still absent), bind the epoch, and after
// RunCancel a workflow mutation from that worktree is refused run-cancelled.
// Restoring the empty worktree argument at the sole-proof ConfirmGateClaim call
// reddens both halves.
func TestVerdictSoleProofAdoptionBindsEpochWorktreeSoFenceActs(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	repo := f.repo.invocation
	key := gateMintArmed(t, repo, nil, 1, "ha")
	ep, err := MintEpochRecord(repo, key, "")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	// NO ReserveGateClaim: the no-binding branch adopts the sole matching proof.
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
	}}

	repoID, err := f.client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: repo})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	want := filepath.Join(repoID.PrimaryWorktree, ".worktrees", rvSlug)
	if _, serr := os.Stat(want); serr == nil {
		t.Fatalf("precondition: feature dir %q must not exist yet", want)
	}

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, repo, key)
	if got, wantLine := res.HumanText(), "gate-done "+key+" run-complete 3"; got != wantLine {
		t.Fatalf("HumanText = %q, want %q (adoption must still succeed)", got, wantLine)
	}
	b, ok, berr := LoadGateClaimBinding(repo, key)
	if berr != nil || !ok || !b.Confirmed || b.ChangeID != 3 {
		t.Fatalf("binding = %+v ok=%v err=%v, want confirmed change 3 after adoption", b, ok, berr)
	}
	epAfter, _, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if epAfter.Worktree != want {
		t.Fatalf("epoch worktree = %q, want %q (sole-proof adoption must bind the run epoch's worktree)", epAfter.Worktree, want)
	}

	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir feature worktree: %v", err)
	}
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	seams := cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}}
	cres := runCancel(seams, repo, key, ep.EpochID, "0427 regression stop")
	if cres.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel disposition = %q (findings %v), want cancelled", cres.Disposition, cres.Findings)
	}
	_, ferr := admitWorkflowMutation(want, OperationPRPublish)
	if fe, ok := AsMutationFenceError(ferr); !ok || fe.Reason != "run-cancelled" {
		t.Fatalf("mutation after cancel = %v, want a run-cancelled MutationFenceError", ferr)
	}
}

// TestVerdictRecoveryUnresolvedIdentityStopsBeforeConfirm: when either recovery
// leg cannot resolve repository/change identity (here: empty PlanningDeps — no
// reader, no client), the verdict refuses gate-stop gate-unavailable
// proof-unavailable BEFORE any confirm — it never substitutes an empty worktree.
// The unconfirmed reservation stays intact-unconfirmed; the sole-proof leg
// writes NO reservation at all (resolution precedes ReserveGateClaim).
func TestVerdictRecoveryUnresolvedIdentityStopsBeforeConfirm(t *testing.T) {
	t.Run("unconfirmed reservation leg", func(t *testing.T) {
		repo := newGateRepo(t)
		key := gateMintArmed(t, repo, nil, 1, "ha")
		if _, err := MintEpochRecord(repo, key, ""); err != nil {
			t.Fatalf("MintEpochRecord: %v", err)
		}
		if err := ReserveGateClaim(repo, key, 3, "claim-3-v"); err != nil {
			t.Fatalf("ReserveGateClaim: %v", err)
		}
		wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
			{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
		}}}

		res := RunGateVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
		if res.Decision != GateDecisionStop || res.Outcome != GateOutcomeUnavailable || res.Reason != ReasonGateProofUnavailable {
			t.Fatalf("got %q/%q/%q, want gate-stop/gate-unavailable/%s (refuse before confirming)", res.Decision, res.Outcome, res.Reason, ReasonGateProofUnavailable)
		}
		b, ok, err := LoadGateClaimBinding(repo, key)
		if err != nil || !ok || b.Confirmed {
			t.Fatalf("binding = %+v ok=%v err=%v, want the reservation intact and UNCONFIRMED", b, ok, err)
		}
		ep, _, err := LoadEpochRecord(repo, key)
		if err != nil {
			t.Fatalf("LoadEpochRecord: %v", err)
		}
		if ep.Worktree != "" {
			t.Fatalf("epoch worktree = %q, want empty (nothing may bind on a refusal)", ep.Worktree)
		}
	})

	t.Run("sole-proof adoption leg", func(t *testing.T) {
		repo := newGateRepo(t)
		key := gateMintArmed(t, repo, nil, 1, "ha")
		if _, err := MintEpochRecord(repo, key, ""); err != nil {
			t.Fatalf("MintEpochRecord: %v", err)
		}
		wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
			{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
		}}}

		res := RunGateVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
		if res.Decision != GateDecisionStop || res.Outcome != GateOutcomeUnavailable || res.Reason != ReasonGateProofUnavailable {
			t.Fatalf("got %q/%q/%q, want gate-stop/gate-unavailable/%s", res.Decision, res.Outcome, res.Reason, ReasonGateProofUnavailable)
		}
		if res.AttributedID != 0 {
			t.Errorf("AttributedID = %d, want 0 (nothing adopted on a refusal)", res.AttributedID)
		}
		if _, ok, err := LoadGateClaimBinding(repo, key); err != nil || ok {
			t.Fatalf("binding present=%v err=%v, want NO reservation written (resolution precedes ReserveGateClaim)", ok, err)
		}
	})
}
```

- [ ] **Step 2: Run to verify the expected failures**

Run: `go test ./internal/app/ -run 'TestVerdictSoleProofAdoptionBindsEpochWorktreeSoFenceActs|TestVerdictRecoveryUnresolvedIdentityStopsBeforeConfirm' -count=1 -v`
Expected: `SoleProofAdoption…` FAILs on `epoch worktree = ""`. `UnresolvedIdentity…/unconfirmed reservation leg` PASSes already (Task 1 built that refusal); `…/sole-proof adoption leg` FAILs (today that leg still adopts with `""` — the verdict comes back `gate-stop … unknown-verdict` or a run-verify refusal, not `proof-unavailable`, and a reservation IS written).

- [ ] **Step 3: Fix the sole-proof leg**

In `resolveGateOwnership`'s `default:` branch, `case 1:`, replace the adoption block (resolve BEFORE the reserve, so a refusal writes nothing):

```go
		case 1:
			p := matches[0]
			// Adopt the sole proof: resolve the change's logical feature worktree
			// FIRST (change 0427) — a refusal here writes nothing, neither
			// reservation nor confirm — then reserve + confirm best-effort with
			// that worktree, then mirror. Confirming with an empty path would
			// leave the recovered run epoch unfenceable and uncancellable.
			wt, ok := gateRecoveredWorktree(ctx, deps, repoDir, p.ChangeID)
			if !ok {
				return gateOwnershipStop(repoDir, key, *rec, ReasonGateProofUnavailable)
			}
			_ = ReserveGateClaim(repoDir, key, p.ChangeID, p.RequestID)
			_ = ConfirmGateClaim(repoDir, key, p.ChangeID, p.RequestID, p.Revision, wt)
			gateAdoptOwnership(wdeps, repoDir, key, rec, p.ChangeID, p.RequestID, p.Revision)
			return nil
```

(The old `// worktree "": see the confirmed-binding branch above — recovery binds no worktree.` comment is deleted by this replacement.)

- [ ] **Step 4: Run both new tests plus the recovery/ownership neighbors**

Run: `go test ./internal/app/ -run 'TestVerdictSoleProof|TestVerdictRecoveryUnresolvedIdentity|TestVerdictAbsentBinding|TestVerdictFreshRunBindsScopeChange|TestVerdictOwnershipIgnoresBeforeSetAndEpoch|TestVerdictNoBindingNoProof' -count=1`
Expected: PASS (the two-proofs conflict case and the no-proof cases exit before resolution; `TestVerdictFreshRunBindsScopeChange` carries real fixture deps and now also binds a worktree, which it does not assert against).

- [ ] **Step 5: Mutation-check each fixed call separately (uncached), then restore**

Copy the fixed `internal/app/rungate_verdict.go` to a scratch backup (never restore via `git checkout --`).

Mutation A — restore `""` at the unconfirmed-reservation `ConfirmGateClaim` only:
Run: `go test ./internal/app/ -run 'TestVerdictUnconfirmedRecoveryBindsEpochWorktreeSoFenceActs|TestVerdictSoleProofAdoptionBindsEpochWorktreeSoFenceActs' -count=1`
Expected: exactly the unconfirmed-reservation test reddens; the sole-proof test stays green. Restore.

Mutation B — restore `""` at the sole-proof `ConfirmGateClaim` only:
Run the same command.
Expected: exactly the sole-proof test reddens. Restore, and re-run the same command uncached to confirm both PASS on the restored source.

- [ ] **Step 6: Run the whole package uncached**

Run: `go test ./internal/app/ -count=1`
Expected: PASS. Same repair rule as Task 1 Step 6 if any pre-existing test surfaces: give it resolvable identity, never weaken the refusal. Retain — do not touch — the existing sibling-proof and ambiguous-proof rejection tests (`TestVerdictUnconfirmedReservationSiblingContextHashIsNoAttributableClaim`, `TestVerdictAbsentBindingAdoptsSoleProof/two proofs is binding-conflict`), which the spec's acceptance keeps as-is.

- [ ] **Step 7: Commit**

```bash
git add internal/app/rungate_verdict.go internal/app/rungate_fence_test.go
git commit -m "fix(0427): sole-proof verdict adoption binds the epoch worktree and refuses unresolved identity"
```

---

### Task 3: Full-suite gate and budget report

**Files:**
- No source changes expected; fixes only if the suite reddens.

**Interfaces:**
- Consumes: the configured build gate — read the exact command from `build.test_command` in `.docket.yml` at the worktree root (never a second copy of it); the Go runner (`internal/suiterunner`) is the sole channel.
- Produces: a green full suite from this checkout, with the budget report read.

- [ ] **Step 1: Read the configured gate command**

Run: `grep -n "test_command" .docket.yml` (from the feature worktree root) and use the `build:` section's value verbatim.

- [ ] **Step 2: Run the complete configured suite from this checkout**

Run the exact `build.test_command` from the feature worktree root.
Expected: the suite's own summary line reports PASS (trust the summary line, not a piped exit code).

- [ ] **Step 3: Read the budget report even on green**

Inspect the run's budget report output: a `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding to note in the results; a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach — serially confirm per `tests/README.md` before acting. The new tests drive real git fixtures plus a cancel each; if `internal/app` shows a watch-line regression attributable to them, note it — do not restructure the suite in this change.

- [ ] **Step 4: Commit (only if the gate forced a fix)**

```bash
git add -u
git commit -m "test(0427): suite-gate repair"
```

If nothing changed, there is nothing to commit — the gate run itself is recorded in the build evidence.

---

## Self-Review Notes

- Spec §Fix coverage: path derivation + threading (Task 1 Steps 3), both recovery calls (Task 1 Step 3, Task 2 Step 3), comment corrections (both replacement blocks delete the stale empty-argument justifications), refuse-on-unresolved (both legs, `ReasonGateProofUnavailable`), no workspace creation (helper only joins a path), fence canonicalization untouched.
- Spec §Acceptance coverage: both recovery shapes with a real fresh empty-worktree epoch (`MintEpochRecord(repo, key, "")`, nothing pre-binds), expected-path storage assert, `RunCancel` + `run-cancelled` mutation refusal, recovery-before-directory-exists (the `os.Stat` precondition asserts the dir is absent at bind time), unresolved-identity refusal before confirmation, sibling/ambiguous rejection tests retained, per-call mutation checks uncached with restore, full configured suite + budget report (Task 3).
- Spec §Scope: no new subsystems; `resolveGateOwnership` gains one parameter and one helper; 0422 composition noted in Global Constraints.
- Type consistency: `gateRecoveredWorktree(ctx, deps, repoDir, changeID) (string, bool)` is defined in Task 1 and consumed identically in Task 2; `resolveGateOwnership`'s new `deps PlanningDeps` second parameter matches the updated `RunGateVerdict` call site.
