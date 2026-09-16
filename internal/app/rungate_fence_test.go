package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/testsupport"
	"github.com/danielhanold/docket/internal/workspace"
)

// These are the run-epoch mutation-boundary fence tests (change 0375 Task 11).
// After a run epoch is fenced (cancelling/cancelled/superseded), no NEW workflow
// mutation from that epoch is admitted at the shared boundaries — the transaction
// engine (mechanically, via AdmissionHook), PR creation, and workspace publish —
// and an in-flight mutation is journaled so a cancellation stays PENDING until it is
// reconciled. A worktree no epoch owns is UNFENCED. The barrier tests double as the
// acceptance-criterion-5 barriers and as the fence's mutation evidence: neutering the
// hook (or admitWorkflowMutation) reddens all three block-after-cancel tests.

// mintFenceEpoch mints a gate record and an epoch under repoDir, binds the epoch to
// worktree and drives it to state, and returns the gate key. It is the minimal setup
// admitWorkflowMutation needs to find the owning epoch by worktree.
func mintFenceEpoch(t *testing.T, repoDir, worktree string, state epochState) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:       gateBeforeStoredTarget,
		AttemptLimit: 2,
		Retry:        RetryUnused,
		Disposition:  "gate-armed",
		ParentCap:    "parent-cap-raw",
		ScopeID:      "scope-1",
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	if _, err := MintEpochRecord(repoDir, key, "7"); err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	if err := epochCAS(repoDir, key, func(r *EpochRecord) error {
		r.Worktree = worktree
		r.State = state
		return nil
	}); err != nil {
		t.Fatalf("epochCAS set worktree/state: %v", err)
	}
	return key
}

// fenceStubOp is a SemanticOperation whose key is a change.mark-implemented-shaped
// engine op. It is never invoked: the admission fence fires before planning, so Plan
// panics if the engine ever reaches it (proving the refusal precedes any mutation).
type fenceStubOp struct{}

func (fenceStubOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationChangeMarkImplemented)
}

func (fenceStubOp) Plan(context.Context, transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	panic("fenceStubOp.Plan reached: the admission fence must refuse before planning")
}

// fenceStubLoader is a StateLoader the fenced engine never reaches (Load would only
// run after the fetch that the admission refusal precedes).
type fenceStubLoader struct{}

func (fenceStubLoader) Load(context.Context, transaction.Tree) (transaction.LoadedState, error) {
	panic("fenceStubLoader.Load reached: the admission fence must refuse before load")
}

func (fenceStubLoader) ValidateEvolution(_, _ transaction.LoadedState) []domain.Finding {
	return nil
}

// TestFenceBlocksEngineMutationAfterCancel: a change.mark-implemented-shaped engine
// mutation, driven through an engine wired with the production AdmissionHook against a
// cancelled epoch that owns the worktree, is refused at StageAdmission — before any
// fetch, allocation, plan, or push — so nothing is mutated.
func TestFenceBlocksEngineMutationAfterCancel(t *testing.T) {
	repoDir := newGateRepo(t)
	mintFenceEpoch(t, repoDir, repoDir, EpochCancelling)

	engine, err := transaction.NewEngine(newGitClient(t), testClock())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	engine.AdmissionHook = MutationAdmissionHook(repoDir)

	res, execErr := engine.Execute(context.Background(), transaction.Request{
		Repository: gitcli.Repository{PrimaryWorktree: repoDir, CommonDir: repoDir},
		Remote:     "origin",
		TargetRef:  gitcli.RefName("refs/heads/docket"),
		Loader:     fenceStubLoader{},
		Operation:  fenceStubOp{},
	})

	if res.Disposition != transaction.DispositionInterrupted {
		t.Fatalf("disposition = %q, want interrupted (fenced)", res.Disposition)
	}
	f, ok := transaction.AsFailure(execErr)
	if !ok {
		t.Fatalf("execErr = %v, want a typed *Failure", execErr)
	}
	if f.Stage != transaction.StageAdmission {
		t.Fatalf("failure stage = %q, want %q (refused before any Git work)", f.Stage, transaction.StageAdmission)
	}
	if f.Kind != transaction.KindCancelled {
		t.Fatalf("failure kind = %q, want %q", f.Kind, transaction.KindCancelled)
	}
	if fe, ok := AsMutationFenceError(f.Err); !ok || fe.Reason != "run-cancelled" {
		t.Fatalf("hook error = %v, want a run-cancelled MutationFenceError", f.Err)
	}
}

// TestFenceBlocksPRPublishAfterCancel: with every PRPublish pre-check satisfied, a
// cancelled epoch owning the worktree blocks publication with the run-cancelled
// reason and gh (EnsurePullRequest) is never invoked.
func TestFenceBlocksPRPublishAfterCancel(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	mintFenceEpoch(t, repoDir, repoDir, EpochCancelling)

	reader := prReader(t)
	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	deps := workspaceDepsFor(t, reader)

	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Authored prose.\n", EvidenceRecord: prEvidenceBytes(t, prHead)})

	if res.Result != ResultBlocked {
		t.Fatalf("result = %q (reason %q), want blocked by the fence", res.Result, res.Reason)
	}
	if res.Reason != "run-cancelled" {
		t.Fatalf("reason = %q, want run-cancelled", res.Reason)
	}
	if len(gh.ensureCalls) != 0 {
		t.Fatalf("EnsurePullRequest invoked %d times despite the fence; nothing must be published", len(gh.ensureCalls))
	}
}

// TestFenceBlocksWorkspacePublishAfterCancel: with the workspace head matching, a
// cancelled epoch blocks the publish with the run-cancelled reason and PublishHead is
// never invoked.
func TestFenceBlocksWorkspacePublishAfterCancel(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	mintFenceEpoch(t, repoDir, repoDir, EpochCancelling)

	const head = "abcdef0000000000000000000000000000000000"
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
		publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(head)},
	}

	res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspacePublishRequest{ID: 7, Head: head})

	if res.Result != ResultBlocked {
		t.Fatalf("result = %q (reason %q), want blocked by the fence", res.Result, res.Reason)
	}
	if res.Reason != "run-cancelled" {
		t.Fatalf("reason = %q, want run-cancelled", res.Reason)
	}
	if len(svc.publishCalls) != 0 {
		t.Fatalf("PublishHead invoked %d times despite the fence; nothing must be published", len(svc.publishCalls))
	}
}

// TestInFlightMutationReconcilesBeforeCancelled: an admitted-not-completed mutation
// keeps a cancellation PENDING; once it is journaled completed, a repeated cancel
// reconciles it and reports cancelled. This is the "already-admitted external actions
// are reconciled, and cancelled is not reported while an unresolved effect remains"
// property.
func TestInFlightMutationReconcilesBeforeCancelled(t *testing.T) {
	fx := newCancelFixture(t, false) // active epoch + authority, no slot to reconcile

	// A workflow mutation is admitted (in flight) but not yet completed.
	done, err := admitWorkflowMutation(fx.worktree, OperationPRPublish)
	if err != nil {
		t.Fatalf("admitWorkflowMutation on an active epoch: %v", err)
	}

	seams := cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}}

	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending while a mutation is in flight", res.Disposition)
	}
	if !hasFinding(res.Findings, "mutation-pending:") {
		t.Fatalf("findings = %v, want a mutation-pending finding", res.Findings)
	}
	if got := loadEpochState(t, fx.repo, fx.key); got != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling (fenced, not yet cancelled)", got)
	}

	// Reconcile the in-flight mutation, then a repeated cancel resumes cleanup and
	// reaches cancelled.
	done(mutationStatusCompleted)

	res2 := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res2.Disposition != CancelDispositionCancelled {
		t.Fatalf("repeat disposition = %q, want cancelled after reconciliation (findings %v)", res2.Disposition, res2.Findings)
	}
	if got := loadEpochState(t, fx.repo, fx.key); got != EpochCancelled {
		t.Fatalf("epoch state = %q, want cancelled", got)
	}
}

// TestStandaloneMutationUnfenced: a worktree no epoch owns admits every mutation
// unfenced (the completion callback is a no-op), and an epoch owning a DIFFERENT
// worktree never fences this one.
func TestStandaloneMutationUnfenced(t *testing.T) {
	repoDir := newGateRepo(t)

	// (a) No epoch at all.
	done, err := admitWorkflowMutation(repoDir, OperationPRPublish)
	if err != nil {
		t.Fatalf("no-epoch admit returned error %v, want unfenced", err)
	}
	done(mutationStatusCompleted) // must be a safe no-op

	// (b) An epoch that owns a DIFFERENT worktree does not fence this one, even when
	// cancelled.
	other := filepath.Join(repoDir, "other-wt")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatalf("mkdir other worktree: %v", err)
	}
	mintFenceEpoch(t, repoDir, other, EpochCancelling)

	done2, err := admitWorkflowMutation(repoDir, OperationPRPublish)
	if err != nil {
		t.Fatalf("admit refused by an unrelated worktree's epoch: %v", err)
	}
	done2(mutationStatusCompleted)
}

// TestFenceRefusesSupersededEpochAsStale: a superseded epoch (a resume replaced it)
// refuses the mutation with the stale-run-epoch reason, distinct from run-cancelled.
func TestFenceRefusesSupersededEpochAsStale(t *testing.T) {
	repoDir := newGateRepo(t)
	mintFenceEpoch(t, repoDir, repoDir, EpochSuperseded)

	_, err := admitWorkflowMutation(repoDir, OperationWorkspacePublish)
	fe, ok := AsMutationFenceError(err)
	if !ok {
		t.Fatalf("err = %v, want a MutationFenceError", err)
	}
	if fe.Reason != "stale-run-epoch" {
		t.Fatalf("reason = %q, want stale-run-epoch", fe.Reason)
	}
}

// TestFenceMatchesWorktreeAcrossSymlinkAlias: the fence canonicalizes both the
// caller's worktree and the epoch's stored Worktree, so a `/tmp`→`/private/tmp`-style
// alias cannot dodge it. Here the epoch stores a symlink spelling of the worktree and
// the mutation runs with the canonical spelling; the fence still matches.
func TestFenceMatchesWorktreeAcrossSymlinkAlias(t *testing.T) {
	repoDir := newGateRepo(t)
	canonRepo, err := canonicalWorktree(repoDir)
	if err != nil {
		t.Fatalf("canonicalWorktree(repoDir): %v", err)
	}

	// A symlink alias pointing at the same worktree, stored as the epoch's Worktree.
	alias := filepath.Join(testsupport.TempDir(t), "wt-alias")
	if err := os.Symlink(canonRepo, alias); err != nil {
		t.Fatalf("symlink alias: %v", err)
	}
	mintFenceEpoch(t, repoDir, alias, EpochCancelling)

	// The mutation runs with the canonical spelling; the fence resolves the epoch's
	// aliased Worktree to the same canonical path and refuses.
	_, err = admitWorkflowMutation(canonRepo, OperationPRPublish)
	if fe, ok := AsMutationFenceError(err); !ok || fe.Reason != "run-cancelled" {
		t.Fatalf("err = %v, want run-cancelled across the symlink alias", err)
	}
}

// TestFreshRunClaimBindsEpochWorktreeSoFenceActs is the BLOCKER regression (change
// 0375): a FRESH (non-resume) run's claim confirmation must bind the epoch's Worktree
// so the mutation fence locates the epoch. It drives the REAL arm→reserve→confirm
// production path (RunGateBefore mints the fresh epoch with Worktree == ""; the claim
// confirmation binds it) rather than a fixture that sets r.Worktree directly, then
// asserts both halves of the defect: the epoch worktree is bound, and a post-cancel
// workflow mutation running in that worktree is refused run-cancelled. Before the fix
// the fresh epoch kept Worktree == "", findEpochByWorktree skipped it, and the mutation
// was admitted UNFENCED even after the epoch was cancelling — the fence and run.cancel
// teardown were both inert for the common first-dispatch case.
func TestFreshRunClaimBindsEpochWorktreeSoFenceActs(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	// A FRESH arm (resumeID 0) mints a run epoch beside the gate record with Worktree "".
	res := RunGateBefore(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Armed {
		t.Fatalf("fresh arm failed: %+v", res)
	}

	// The real production claim→confirm sequence for a fresh dispatch, carrying the
	// change's feature worktree — the same path change_claim.go drives.
	worktree := repo
	if err := ReserveGateClaim(repo, res.Key, 42, "req-1"); err != nil {
		t.Fatalf("ReserveGateClaim: %v", err)
	}
	if err := ConfirmGateClaim(repo, res.Key, 42, "req-1", "revabc123", worktree); err != nil {
		t.Fatalf("ConfirmGateClaim: %v", err)
	}

	// (a) The confirm bound the worktree onto the fresh run's epoch.
	ep, _, err := LoadEpochRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.Worktree != worktree {
		t.Fatalf("fresh-run claim must bind the epoch worktree, got %q want %q", ep.Worktree, worktree)
	}

	// (b) findEpochByWorktree now locates the fresh epoch: cancel it, and a workflow
	// mutation running in that worktree is refused run-cancelled. Before the fix the
	// empty-worktree epoch was skipped and this mutation was admitted unfenced.
	if err := epochCAS(repo, res.Key, func(r *EpochRecord) error {
		r.State = EpochCancelling
		return nil
	}); err != nil {
		t.Fatalf("epochCAS to cancelling: %v", err)
	}
	_, ferr := admitWorkflowMutation(worktree, OperationPRPublish)
	if fe, ok := AsMutationFenceError(ferr); !ok || fe.Reason != "run-cancelled" {
		t.Fatalf("mutation after cancel = %v, want run-cancelled (the fence must locate the fresh epoch)", ferr)
	}
}

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

	// Give the armed record a parent-held authority so RunCancel's authority gate
	// (rec.ParentCap != "") is satisfied later; gateMintArmed leaves it empty. This
	// is cancel-fixture scaffolding, not part of the recovery under test.
	if rec, lerr := LoadGateRecord(repo, key); lerr != nil {
		t.Fatalf("LoadGateRecord: %v", lerr)
	} else {
		rec.ParentCap = "parent-cap-raw"
		if serr := SaveGateRecord(repo, key, rec); serr != nil {
			t.Fatalf("SaveGateRecord: %v", serr)
		}
	}

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

	// Give the armed record a parent-held authority so RunCancel's authority gate
	// (rec.ParentCap != "") is satisfied later; gateMintArmed leaves it empty. This
	// is cancel-fixture scaffolding, not part of the adoption under test.
	if rec, lerr := LoadGateRecord(repo, key); lerr != nil {
		t.Fatalf("LoadGateRecord: %v", lerr)
	} else {
		rec.ParentCap = "parent-cap-raw"
		if serr := SaveGateRecord(repo, key, rec); serr != nil {
			t.Fatalf("SaveGateRecord: %v", serr)
		}
	}

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
