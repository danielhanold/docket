package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	done, err := admitWorkflowMutation(fx.worktree, OperationPRPublish, nil)
	if err != nil {
		t.Fatalf("admitWorkflowMutation on an active epoch: %v", err)
	}

	seams := cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}

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
	done(mutationStatusCompleted, false)

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
	done, err := admitWorkflowMutation(repoDir, OperationPRPublish, nil)
	if err != nil {
		t.Fatalf("no-epoch admit returned error %v, want unfenced", err)
	}
	done(mutationStatusCompleted, false) // must be a safe no-op

	// (b) An epoch that owns a DIFFERENT worktree does not fence this one, even when
	// cancelled.
	other := filepath.Join(repoDir, "other-wt")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatalf("mkdir other worktree: %v", err)
	}
	mintFenceEpoch(t, repoDir, other, EpochCancelling)

	done2, err := admitWorkflowMutation(repoDir, OperationPRPublish, nil)
	if err != nil {
		t.Fatalf("admit refused by an unrelated worktree's epoch: %v", err)
	}
	done2(mutationStatusCompleted, false)
}

// TestFenceRefusesSupersededEpochAsStale: a superseded epoch (a resume replaced it)
// refuses the mutation with the stale-run-epoch reason, distinct from run-cancelled.
func TestFenceRefusesSupersededEpochAsStale(t *testing.T) {
	repoDir := newGateRepo(t)
	mintFenceEpoch(t, repoDir, repoDir, EpochSuperseded)

	_, err := admitWorkflowMutation(repoDir, OperationWorkspacePublish, nil)
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
	_, err = admitWorkflowMutation(canonRepo, OperationPRPublish, nil)
	if fe, ok := AsMutationFenceError(err); !ok || fe.Reason != "run-cancelled" {
		t.Fatalf("err = %v, want run-cancelled across the symlink alias", err)
	}
}

// TestAdmitWorkflowMutationRefusesCompletingEpoch: a completing epoch (a verified
// successful closeout is mid-flight, change 0441) still owns its worktree and refuses
// a new mutation with the distinct run-completed reason — never relabelled as a
// cancellation.
func TestAdmitWorkflowMutationRefusesCompletingEpoch(t *testing.T) {
	repoDir := newGateRepo(t)
	mintFenceEpoch(t, repoDir, repoDir, EpochCompleting)

	_, err := admitWorkflowMutation(repoDir, OperationPRPublish, nil)
	fe, ok := AsMutationFenceError(err)
	if !ok || fe.Reason != "run-completed" {
		t.Fatalf("err = %v, want a MutationFenceError with reason run-completed", err)
	}
}

// TestCompletedEpochExcludedFromAmbientOwnerLookup: a fully completed epoch (change
// 0441) no longer owns the worktree for ambient lookup, so a standalone mutation on
// that worktree is admitted UNFENCED and findEpochByWorktree no longer names it. A
// COMPLETING epoch, in contrast, is still the owner (its closeout has not finished).
func TestCompletedEpochExcludedFromAmbientOwnerLookup(t *testing.T) {
	repoDir := newGateRepo(t)
	key := mintFenceEpoch(t, repoDir, repoDir, EpochCompleted)

	done, err := admitWorkflowMutation(repoDir, OperationPRPublish, nil)
	if err != nil || done == nil {
		t.Fatalf("completed epoch trapped a standalone mutation: err %v done %v", err, done)
	}
	done(mutationStatusCompleted, false) // must be a safe no-op for the unfenced admit

	canon, cerr := canonicalWorktree(repoDir)
	if cerr != nil {
		t.Fatalf("canonicalWorktree(repoDir): %v", cerr)
	}
	if _, found, ferr := findEpochByWorktree(repoDir, canon); ferr != nil || found {
		t.Fatalf("completed epoch still owns the worktree lookup (found %v err %v)", found, ferr)
	}

	// The same epoch, moved back to completing, is still the worktree owner.
	fenceEpoch(t, repoDir, key, EpochCompleting)
	if _, found, ferr := findEpochByWorktree(repoDir, canon); ferr != nil || !found {
		t.Fatalf("completing epoch lost worktree ownership before closeout finished (found %v err %v)", found, ferr)
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
	_, ferr := admitWorkflowMutation(worktree, OperationPRPublish, nil)
	if fe, ok := AsMutationFenceError(ferr); !ok || fe.Reason != "run-cancelled" {
		t.Fatalf("mutation after cancel = %v, want run-cancelled (the fence must locate the fresh epoch)", ferr)
	}
}

// seedPendingEpochMutation journals one admitted-not-completed workflow mutation on
// the epoch at key: a genuinely owned in-flight effect, which keeps the successful-run
// closeout (change 0441) fail-closed with mutation-pending. The two verdict recovery
// tests below use it to hold their epoch at completing so a later explicit
// cancellation is meaningful. They formerly relied on the ABSENT feature directory
// making the slot unreadable; change 0446 (spec §2) addresses a slot through its
// stored identity, so a never-reserved slot now reads as truly absent (safely
// detached) and the closeout would legitimately complete — an absent directory is not
// an obligation, an owned pending mutation is.
func seedPendingEpochMutation(t *testing.T, repo, key string) {
	t.Helper()
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations, AdmittedMutation{
			OpKey:  OperationPRPublish,
			Status: mutationStatusAdmitted,
		})
		return nil
	}); err != nil {
		t.Fatalf("journal a pending mutation: %v", err)
	}
}

// reconcilePendingEpochMutations marks every journaled mutation on the epoch at key
// completed — the in-flight effect resolved — so an explicit cancellation can account
// it and reach cancelled.
func reconcilePendingEpochMutations(t *testing.T, repo, key string) {
	t.Helper()
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		for i := range r.AdmittedMutations {
			r.AdmittedMutations[i].Status = mutationStatusCompleted
		}
		return nil
	}); err != nil {
		t.Fatalf("reconcile pending mutations: %v", err)
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
	seedPendingEpochMutation(t, repo, key)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, repo, key)
	// The verdict recovery binds the epoch worktree, then drives the successful-run
	// ownership closeout (change 0441). Here that closeout fails CLOSED on the owned
	// in-flight mutation seeded above (mutation-pending) — the epoch is left durably
	// completing while the WORKTREE BINDING this test guards is already persisted. The
	// recovery (ownership resolution + worktree binding) still succeeded.
	if got, wantLine := res.HumanText(), "gate-stop "+key+" gate-unavailable completion-unaccounted"; got != wantLine {
		t.Fatalf("HumanText = %q, want %q (recovery binding must still land)", got, wantLine)
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

	// (b) Cancel through RunCancel — an explicit cancellation wins even from the
	// completing epoch the blocked closeout left (change 0441) — then a workflow
	// mutation from that feature worktree is refused run-cancelled. The dir exists by
	// now (as it would after workspace.prepare); the fence canonicalizes at compare
	// time, so it must locate the recovered epoch by its bound worktree.
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir feature worktree: %v", err)
	}
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	seams := cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
	reconcilePendingEpochMutations(t, repo, key)
	cres := runCancel(seams, repo, key, ep.EpochID, "0427 regression stop")
	if cres.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel disposition = %q (findings %v), want cancelled", cres.Disposition, cres.Findings)
	}
	_, ferr := admitWorkflowMutation(want, OperationPRPublish, nil)
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
	seedPendingEpochMutation(t, repo, key)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, repo, key)
	// Adoption binds the epoch worktree, then the successful-run closeout (change 0441)
	// fails CLOSED on the owned in-flight mutation seeded above (mutation-pending), and
	// the epoch is left completing while the ADOPTION binding this test guards is
	// already persisted.
	if got, wantLine := res.HumanText(), "gate-stop "+key+" gate-unavailable completion-unaccounted"; got != wantLine {
		t.Fatalf("HumanText = %q, want %q (adoption binding must still land)", got, wantLine)
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
	seams := cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
	// An explicit cancellation wins even from the completing epoch the blocked closeout
	// left (change 0441); the fence must then locate the recovered epoch by its bound
	// worktree and refuse the mutation run-cancelled.
	reconcilePendingEpochMutations(t, repo, key)
	cres := runCancel(seams, repo, key, ep.EpochID, "0427 regression stop")
	if cres.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel disposition = %q (findings %v), want cancelled", cres.Disposition, cres.Findings)
	}
	_, ferr := admitWorkflowMutation(want, OperationPRPublish, nil)
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

// --- change 0446 Task 7: deterministic worktree owner selection and the
// slot-named-epoch rule (spec §§1, 5; AC3, AC6). ---

// seedNamedEpoch writes an epoch record for state bound to worktree under a gate-key
// directory whose NAME the test chooses, so the directory order os.ReadDir yields is
// controlled (a first-match selector would pick the lexically first key). It writes
// the record through the store's own atomic writer and needs no gate record.
func seedNamedEpoch(t *testing.T, repo, key, worktree string, state epochState) EpochRecord {
	t.Helper()
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	dir := filepath.Join(common, "docket", "rungate", key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir gate-key dir: %v", err)
	}
	id, err := epochToken()
	if err != nil {
		t.Fatalf("epochToken: %v", err)
	}
	gen, err := epochToken()
	if err != nil {
		t.Fatalf("epochToken: %v", err)
	}
	rec := EpochRecord{
		SchemaVersion: epochSchemaVersion,
		GateKey:       key,
		ChangeID:      "7",
		State:         state,
		EpochID:       id,
		Worktree:      worktree,
	}
	if err := writeEpochAtomic(dir, storedEpoch{Generation: gen, Record: rec}); err != nil {
		t.Fatalf("writeEpochAtomic: %v", err)
	}
	return rec
}

// epochRecordPath is the epoch.json path for key under repo's rungate root.
func epochRecordPath(t *testing.T, repo, key string) string {
	t.Helper()
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	return filepath.Join(common, "docket", "rungate", key, epochRecordFileName)
}

func mustCanon(t *testing.T, path string) string {
	t.Helper()
	c, err := canonicalWorktree(path)
	if err != nil {
		t.Fatalf("canonicalWorktree(%q): %v", path, err)
	}
	return c
}

// TestOwnerSelectionActiveBeatsCancelledRegardlessOfOrder: a cancelled (and a
// cancelling) never-superseded epoch bound to the same path as a fresh ACTIVE run is
// not the ambient owner, whichever sorts first. The fence admits the active run's
// mutation and journals it on the ACTIVE epoch. Before the fix, first-match selection
// returned the cancelled record in the "fenced-first" ordering and refused the live
// run with run-cancelled.
func TestOwnerSelectionActiveBeatsCancelledRegardlessOfOrder(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		cancelled, cancelling string
		active                string
	}{
		{name: "fenced records sort first", cancelled: "aaaa-cancelled", cancelling: "aaab-cancelling", active: "zzzz-active"},
		{name: "active sorts first", cancelled: "yyyy-cancelled", cancelling: "zzzz-cancelling", active: "aaaa-active"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newGateRepo(t)
			seedNamedEpoch(t, repo, tc.cancelled, repo, EpochCancelled)
			seedNamedEpoch(t, repo, tc.cancelling, repo, EpochCancelling)
			seedNamedEpoch(t, repo, tc.active, repo, EpochActive)

			key, found, err := findEpochByWorktree(repo, mustCanon(t, repo))
			if err != nil || !found || key != tc.active {
				t.Fatalf("findEpochByWorktree = (%q, %v, %v), want the active owner %q", key, found, err, tc.active)
			}
			done, err := admitWorkflowMutation(repo, OperationPRPublish, nil)
			if err != nil {
				t.Fatalf("the active run's mutation was refused: %v", err)
			}
			done(mutationStatusCompleted, false)
			ep, _, lerr := LoadEpochRecord(repo, tc.active)
			if lerr != nil {
				t.Fatalf("LoadEpochRecord(active): %v", lerr)
			}
			if len(ep.AdmittedMutations) != 1 || ep.AdmittedMutations[0].Status != mutationStatusCompleted {
				t.Fatalf("active epoch journal = %+v, want exactly the one completed mutation", ep.AdmittedMutations)
			}
		})
	}
}

// TestOwnerSelectionSoleCancelledStillFences: with no active owner, a cancelled or
// cancelling non-superseded epoch bound to the path is still returned, so the fence
// keeps refusing run-cancelled (dropping every terminal epoch from the lookup is not
// a substitute). Several fenced records resolve deterministically to the lexically
// first key.
func TestOwnerSelectionSoleCancelledStillFences(t *testing.T) {
	for _, state := range []epochState{EpochCancelled, EpochCancelling} {
		t.Run(string(state), func(t *testing.T) {
			repo := newGateRepo(t)
			seedNamedEpoch(t, repo, "mmmm-fenced", repo, state)
			key, found, err := findEpochByWorktree(repo, mustCanon(t, repo))
			if err != nil || !found || key != "mmmm-fenced" {
				t.Fatalf("findEpochByWorktree = (%q, %v, %v), want the sole fenced epoch", key, found, err)
			}
			_, aerr := admitWorkflowMutation(repo, OperationWorkspacePublish, nil)
			if fe, ok := AsMutationFenceError(aerr); !ok || fe.Reason != "run-cancelled" {
				t.Fatalf("admit = %v, want run-cancelled from the sole %s owner", aerr, state)
			}
		})
	}

	t.Run("several fenced resolve to the first key", func(t *testing.T) {
		repo := newGateRepo(t)
		seedNamedEpoch(t, repo, "zzzz-cancelled", repo, EpochCancelled)
		seedNamedEpoch(t, repo, "bbbb-cancelling", repo, EpochCancelling)
		key, found, err := findEpochByWorktree(repo, mustCanon(t, repo))
		if err != nil || !found || key != "bbbb-cancelling" {
			t.Fatalf("findEpochByWorktree = (%q, %v, %v), want bbbb-cancelling", key, found, err)
		}
	})
}

// TestOwnerSelectionTwoActiveOwnersAmbiguous: two active (or active + completing)
// epochs bound to one canonical path are a contradiction — a typed
// ErrEpochOwnerAmbiguous naming the worktree, and the mutation is refused, never
// silently admitted against one of them.
func TestOwnerSelectionTwoActiveOwnersAmbiguous(t *testing.T) {
	for _, second := range []epochState{EpochActive, EpochCompleting} {
		t.Run(string(second), func(t *testing.T) {
			repo := newGateRepo(t)
			canon := mustCanon(t, repo)
			seedNamedEpoch(t, repo, "aaaa-owner", repo, EpochActive)
			seedNamedEpoch(t, repo, "bbbb-owner", repo, second)
			seedNamedEpoch(t, repo, "cccc-cancelled", repo, EpochCancelled)

			_, found, err := findEpochByWorktree(repo, canon)
			if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochOwnerAmbiguous || found {
				t.Fatalf("findEpochByWorktree = (found %v, %v), want ErrEpochOwnerAmbiguous", found, err)
			}
			if !strings.Contains(err.Error(), canon) {
				t.Fatalf("ambiguity error %q must name the worktree %q", err, canon)
			}
			_, aerr := admitWorkflowMutation(repo, OperationPRPublish, nil)
			if ee, ok := AsEpochError(aerr); !ok || ee.Kind != ErrEpochOwnerAmbiguous {
				t.Fatalf("admit = %v, want an ErrEpochOwnerAmbiguous refusal", aerr)
			}
			if reason, _ := fenceRefusalReasonMessage(aerr, "change"); reason != string(ErrEpochOwnerAmbiguous) {
				t.Fatalf("refusal reason = %q, want %q (never relabelled run-cancelled)", reason, ErrEpochOwnerAmbiguous)
			}
			for _, k := range []string{"aaaa-owner", "bbbb-owner"} {
				if ep, _, lerr := LoadEpochRecord(repo, k); lerr != nil || len(ep.AdmittedMutations) != 0 {
					t.Fatalf("epoch %s journal = %+v (err %v), want nothing admitted", k, ep.AdmittedMutations, lerr)
				}
			}
		})
	}
}

// TestOwnerSelectionCompletedNeverOwns: a completed epoch is never the ambient
// owner — alone it leaves the path unfenced, and beside a cancelled epoch the
// cancelled one (not the completed one) is returned.
func TestOwnerSelectionCompletedNeverOwns(t *testing.T) {
	repo := newGateRepo(t)
	canon := mustCanon(t, repo)
	seedNamedEpoch(t, repo, "aaaa-completed", repo, EpochCompleted)
	if key, found, err := findEpochByWorktree(repo, canon); err != nil || found {
		t.Fatalf("findEpochByWorktree = (%q, %v, %v), want no owner for a completed epoch", key, found, err)
	}
	if _, err := admitWorkflowMutation(repo, OperationPRPublish, nil); err != nil {
		t.Fatalf("completed epoch fenced a mutation: %v", err)
	}
	seedNamedEpoch(t, repo, "zzzz-cancelled", repo, EpochCancelled)
	if key, found, err := findEpochByWorktree(repo, canon); err != nil || !found || key != "zzzz-cancelled" {
		t.Fatalf("findEpochByWorktree = (%q, %v, %v), want the cancelled epoch, never the completed one", key, found, err)
	}
}

// TestSlotNamedEpochUnreadableRefusesLocally (AC3): the worktree's execution slot
// names run epoch E. When no readable epoch record carries E — the record is corrupt,
// I/O-unreadable, or gone — the path fence refuses locally with E and the worktree in
// the error instead of admitting unfenced. The same damage to an epoch record NO slot
// names stays diagnostic, and a companion unrelated worktree keeps admitting.
func TestSlotNamedEpochUnreadableRefusesLocally(t *testing.T) {
	damage := map[string]func(t *testing.T, path string){
		"corrupt": func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
				t.Fatalf("corrupt epoch record: %v", err)
			}
		},
		"io-unreadable": func(t *testing.T, path string) {
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatalf("chmod 000 epoch record: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			if f, err := os.Open(path); err == nil {
				f.Close()
				t.Skip("process can read a mode-000 file (running as root); the I/O case is unobservable")
			}
		},
		"absent": func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove epoch record: %v", err)
			}
		},
	}
	for name, apply := range damage {
		t.Run(name, func(t *testing.T) {
			fx := newCancelFixture(t, true) // active epoch E bound to fx.worktree; the slot names E
			canon := mustCanon(t, fx.worktree)

			// Control: while E is readable it is the owner and the mutation is admitted.
			done, err := admitWorkflowMutation(fx.worktree, OperationPRPublish, nil)
			if err != nil {
				t.Fatalf("control admit on a readable active owner: %v", err)
			}
			done(mutationStatusCompleted, false)

			// An UNREFERENCED epoch bound to another worktree (no slot names it),
			// damaged the same way, and a companion worktree with no owner at all.
			unref := filepath.Join(fx.repo, "unreferenced-wt")
			companion := filepath.Join(fx.repo, "companion-wt")
			for _, d := range []string{unref, companion} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", d, err)
				}
			}
			seedNamedEpoch(t, fx.repo, "unreferenced-epoch", unref, EpochActive)

			apply(t, epochRecordPath(t, fx.repo, fx.key))
			apply(t, epochRecordPath(t, fx.repo, "unreferenced-epoch"))

			_, aerr := admitWorkflowMutation(fx.worktree, OperationPRPublish, nil)
			ee, ok := AsEpochError(aerr)
			if !ok || ee.Kind != ErrEpochOwnerUnresolved {
				t.Fatalf("admit on a slot-named %s epoch = %v, want ErrEpochOwnerUnresolved (fail closed, never unfenced)", name, aerr)
			}
			if !strings.Contains(aerr.Error(), fx.epochID) || !strings.Contains(aerr.Error(), canon) {
				t.Fatalf("refusal %q must name epoch %s and worktree %s", aerr, fx.epochID, canon)
			}
			if reason, _ := fenceRefusalReasonMessage(aerr, "workspace"); reason != string(ErrEpochOwnerUnresolved) {
				t.Fatalf("refusal reason = %q, want %q", reason, ErrEpochOwnerUnresolved)
			}
			// The remedy must be valid in this state: name where the epoch records
			// live and that a human repairs them, and never point at run.cancel
			// (which cannot resolve an epoch no readable record carries).
			if msg := aerr.Error(); !strings.Contains(msg, filepath.Join("docket", "rungate")) ||
				!strings.Contains(msg, "human") || !strings.Contains(msg, "run.cancel cannot") {
				t.Fatalf("refusal %q must name the rungate store, human repair, and run.cancel's inapplicability", msg)
			}

			for _, wt := range []string{unref, companion} {
				d, err := admitWorkflowMutation(wt, OperationPRPublish, nil)
				if err != nil {
					t.Fatalf("worktree %s refused by damage to a record no slot of it names: %v", wt, err)
				}
				d(mutationStatusCompleted, false)
			}
		})
	}
}

// TestUnreadableSlotRefusesLocally (review fix): with no readable ambient owner, a
// worktree whose execution slot the store cannot READ (corrupt or I/O-unreadable)
// is not evidence that the slot names no epoch — the path fence refuses with
// ErrEpochOwnerUnresolved naming the worktree instead of admitting unfenced. An
// ABSENT slot still admits unfenced (the standalone contract).
func TestUnreadableSlotRefusesLocally(t *testing.T) {
	damage := map[string]func(t *testing.T, path string){
		"corrupt": func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
				t.Fatalf("corrupt slot: %v", err)
			}
		},
		"io-unreadable": func(t *testing.T, path string) {
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatalf("chmod 000 slot: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			if f, err := os.Open(path); err == nil {
				f.Close()
				t.Skip("process can read a mode-000 file (running as root); the I/O case is unobservable")
			}
		},
	}
	for name, apply := range damage {
		t.Run(name, func(t *testing.T) {
			fx := newCancelFixture(t, true) // the slot names epoch E
			canon := mustCanon(t, fx.worktree)
			// Remove E's record so no ambient owner is readable; the slot is then the
			// only evidence, and it is damaged.
			if err := os.Remove(epochRecordPath(t, fx.repo, fx.key)); err != nil {
				t.Fatalf("remove epoch record: %v", err)
			}
			apply(t, admissionRecordFile(t, fx.common, fx.worktree))

			_, aerr := admitWorkflowMutation(fx.worktree, OperationPRPublish, nil)
			if ee, ok := AsEpochError(aerr); !ok || ee.Kind != ErrEpochOwnerUnresolved {
				t.Fatalf("admit over an unreadable %s slot = %v, want ErrEpochOwnerUnresolved (never unfenced)", name, aerr)
			}
			if !strings.Contains(aerr.Error(), canon) || strings.Contains(aerr.Error(), "run.cancel using") {
				t.Fatalf("refusal %q must name worktree %s and never suggest run.cancel", aerr, canon)
			}
		})
	}
	t.Run("absent-slot-admits", func(t *testing.T) {
		fx := newCancelFixture(t, false)
		if err := os.Remove(epochRecordPath(t, fx.repo, fx.key)); err != nil {
			t.Fatalf("remove epoch record: %v", err)
		}
		done, err := admitWorkflowMutation(fx.worktree, OperationPRPublish, nil)
		if err != nil {
			t.Fatalf("absent slot must admit unfenced: %v", err)
		}
		done(mutationStatusCompleted, false)
	})
}

// TestEpochCarryingFencesUnchangedByOwnerSelection (AC6, separate proof): owner
// selection answers only "who owns this path now". After a NEW active owner binds the
// path, the stale epoch's own epoch-carrying fences still refuse it — the launch gate
// (by id) and the takeover revocation resolver — for a cancelled, superseded, and
// completed stale epoch alike, while ambient lookup names the new owner.
func TestEpochCarryingFencesUnchangedByOwnerSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stale func(t *testing.T, repo, key string)
		want  *MutationFenceError
	}{
		{name: "cancelled", want: ErrRunCancelled, stale: func(t *testing.T, repo, key string) {
			fenceEpoch(t, repo, key, EpochCancelled)
		}},
		{name: "superseded", want: ErrStaleRunEpoch, stale: func(t *testing.T, repo, key string) {
			fenceEpoch(t, repo, key, EpochCancelled)
			if err := SupersedeCancelledEpoch(repo, key, "zzzz-new-owner"); err != nil {
				t.Fatalf("SupersedeCancelledEpoch: %v", err)
			}
		}},
		{name: "completed", want: ErrRunCompleted, stale: func(t *testing.T, repo, key string) {
			fenceEpoch(t, repo, key, EpochCompleted)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, common, key, staleID, worktree := epochGateFixture(t)
			tc.stale(t, repo, key)
			seedNamedEpoch(t, repo, "zzzz-new-owner", worktree, EpochActive)

			if got, found, err := findEpochByWorktree(repo, mustCanon(t, worktree)); err != nil || !found || got != "zzzz-new-owner" {
				t.Fatalf("ambient owner = (%q, %v, %v), want the new active owner", got, found, err)
			}
			calls := 0
			lerr := epochLaunchGate(common)(staleID, worktree, func() error { calls++; return nil })
			if fe, ok := AsMutationFenceError(lerr); !ok || fe != tc.want {
				t.Fatalf("launch gate for the stale %s epoch = %v, want %v", tc.name, lerr, tc.want)
			}
			if calls != 0 {
				t.Fatalf("the stale epoch's reserve ran %d times; it must never run", calls)
			}
			if revoked, err := epochRevokedResolver(common)(staleID); err != nil || !revoked {
				t.Fatalf("takeover resolver for the stale %s epoch = (%v, %v), want revoked", tc.name, revoked, err)
			}
		})
	}
}

// TestPRPublishJournalsPublicationIdentity: a fenced (active-epoch) PR publish
// journals a VALID descriptor carrying the resolved repo identity, exact head
// branch + full commit, base branch, and title/body digests — and the journal
// bytes never contain the raw title or body (digests only). (change 0444)
func TestPRPublishJournalsPublicationIdentity(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceEpoch(t, repoDir, repoDir, EpochActive)

	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	deps := workspaceDepsFor(t, prReader(t))
	const secretTitle = "Add widget SEKRET-TITLE-BYTES"
	const secretBody = "Authored prose SEKRET-BODY-BYTES.\n"

	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: secretTitle, Body: secretBody, EvidenceRecord: prEvidenceBytes(t, prHead)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q (reason %q), want applied", res.Result, res.Reason)
	}
	if len(gh.ensureCalls) != 1 {
		t.Fatalf("EnsurePullRequest calls = %d, want 1", len(gh.ensureCalls))
	}
	call := gh.ensureCalls[0]

	ep, _, err := LoadEpochRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 1 {
		t.Fatalf("journal length = %d, want 1", len(ep.AdmittedMutations))
	}
	m := ep.AdmittedMutations[0]
	if m.Status != mutationStatusCompleted {
		t.Fatalf("status = %q, want completed (observed EnsureCreated)", m.Status)
	}
	if !validPublication(OperationPRPublish, m.Publication) {
		t.Fatalf("journaled descriptor must be valid, got %+v", m.Publication)
	}
	p := m.Publication
	want := prRepo()
	if p.RepoHost != want.Host || p.RepoOwner != want.Owner || p.RepoName != want.Name {
		t.Fatalf("repo identity = %s/%s/%s, want %s", p.RepoHost, p.RepoOwner, p.RepoName, want.Spec())
	}
	if p.HeadCommit != prHead {
		t.Fatalf("HeadCommit = %q, want the requested head %q", p.HeadCommit, prHead)
	}
	// The descriptor carries exactly the identity handed to the adapter.
	if p.HeadRef != call.HeadBranch || p.BaseBranch != call.BaseBranch {
		t.Fatalf("head/base = %q/%q, want the adapter's %q/%q", p.HeadRef, p.BaseBranch, call.HeadBranch, call.BaseBranch)
	}
	if p.TitleDigest != publicationDigest("pr-title", secretTitle) {
		t.Fatal("TitleDigest must be the deterministic digest of the requested title")
	}
	if p.BodyDigest != publicationDigest("pr-body", call.Body) {
		t.Fatal("BodyDigest must digest the fully assembled body handed to the adapter")
	}
	if p.BodyDigest == publicationDigest("pr-body", secretBody) {
		// The assembled body (backlink + evidence woven in) differs from the raw prose.
		t.Fatal("BodyDigest must digest the assembled body, not the raw authored prose")
	}

	// Leak check on the durable record bytes: digests only, never content.
	common, cerr := gateGitCommonDir(repoDir)
	if cerr != nil {
		t.Fatalf("gateGitCommonDir: %v", cerr)
	}
	raw, rerr := os.ReadFile(filepath.Join(rungateRootOf(common), key, epochRecordFileName))
	if rerr != nil {
		t.Fatalf("read epoch record: %v", rerr)
	}
	for _, secret := range []string{"SEKRET-TITLE-BYTES", "SEKRET-BODY-BYTES"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("journal bytes leak %q", secret)
		}
	}
}

// TestWorkspacePublishJournalsPublicationIdentity: an active-epoch workspace
// publish journals a VALID workspace descriptor: canonical repo identity, remote
// name, exact feature ref, and the full intended commit. (change 0444)
func TestWorkspacePublishJournalsPublicationIdentity(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceEpoch(t, repoDir, repoDir, EpochActive)

	const head = "abcdef0000000000000000000000000000000000"
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
		publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(head)},
	}
	res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspacePublishRequest{ID: 7, Head: head})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q (reason %q), want applied", res.Result, res.Reason)
	}
	if len(svc.publishCalls) != 1 {
		t.Fatalf("PublishHead calls = %d, want 1", len(svc.publishCalls))
	}
	call := svc.publishCalls[0]

	ep, _, err := LoadEpochRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 1 {
		t.Fatalf("journal length = %d, want 1", len(ep.AdmittedMutations))
	}
	m := ep.AdmittedMutations[0]
	if m.Status != mutationStatusCompleted {
		t.Fatalf("status = %q, want completed (observed PublishPublished)", m.Status)
	}
	if !validPublication(OperationWorkspacePublish, m.Publication) {
		t.Fatalf("journaled descriptor must be valid, got %+v", m.Publication)
	}
	p := m.Publication
	if p.HeadCommit != head || p.Remote != "origin" {
		t.Fatalf("descriptor = %+v, want head %q on origin", p, head)
	}
	// The descriptor carries exactly the identity handed to the adapter.
	if p.Remote != string(call.Remote) || p.HeadRef != string(call.Target.FeatureRef) || p.RepoDir != call.Repository.CommonDir {
		t.Fatalf("descriptor = %+v, want the adapter's remote %q, ref %q, common dir %q",
			p, call.Remote, call.Target.FeatureRef, call.Repository.CommonDir)
	}
	if p.HeadRef == "" || p.RepoDir == "" {
		t.Fatalf("descriptor must carry the exact feature ref and canonical repo dir: %+v", p)
	}
}

// TestProductionUncertainThenIdenticalRetryThenCancel (change 0444 acceptance 7
// and 8): descriptors journaled by the REAL PRPublish boundary — an uncertain first
// attempt (an external/transport adapter failure), then an identical successful
// retry, both in one run epoch — are matched by the REAL cancel path, which reaches
// cancelled while issuing NO GitHub call (the capture adapters' counters do not
// move during cancellation) and leaking no title/body bytes into the durable
// journal or the cancel findings.
func TestProductionUncertainThenIdenticalRetryThenCancel(t *testing.T) {
	fx := newCancelFixture(t, true)
	// The fixture epoch owns fx.worktree (a real directory inside the fixture's git
	// repository), so PRPublish invoked at that worktree resolves the admission
	// fence to exactly this epoch and journals into it.
	repoDir := fx.worktree

	const secretTitle = "Add widget SEKRET-TITLE-BYTES"
	const secretBody = "Authored prose SEKRET-BODY-BYTES.\n"
	deps := workspaceDepsFor(t, prReader(t))
	req := PRPublishRequest{ID: 7, Head: prHead, Title: secretTitle, Body: secretBody, EvidenceRecord: prEvidenceBytes(t, prHead)}

	// Attempt 1: the adapter fails externally (outcome unobservable) -> uncertain.
	ghFail := &fakeGitHub{repo: prRepo(), ensureErr: &githubcli.Failure{
		Op: "ensure-pull-request", Stage: githubcli.StageInvoke, Kind: githubcli.KindExternal, Detail: "transport reset",
	}}
	if res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: ghFail}, repoDir, req); res.Result != ResultExternalFailed {
		t.Fatalf("first attempt result = %q (reason %q), want external-failed", res.Result, res.Reason)
	}
	// Attempt 2: the identical request succeeds -> completed, identical descriptor.
	ghOK := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	if res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: ghOK}, repoDir, req); res.Result != ResultApplied {
		t.Fatalf("retry result = %q (reason %q), want applied", res.Result, res.Reason)
	}
	if len(ghFail.ensureCalls) != 1 || len(ghOK.ensureCalls) != 1 {
		t.Fatalf("ensure calls = %d/%d, want 1/1", len(ghFail.ensureCalls), len(ghOK.ensureCalls))
	}

	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 2 ||
		ep.AdmittedMutations[0].Status != mutationStatusUncertain ||
		ep.AdmittedMutations[1].Status != mutationStatusCompleted {
		t.Fatalf("journal = %+v, want [uncertain, completed]", ep.AdmittedMutations)
	}
	// Both descriptors come from the production boundary and must be valid and
	// field-for-field identical — that is what the cancel path matches on.
	a, b := ep.AdmittedMutations[0].Publication, ep.AdmittedMutations[1].Publication
	if !validPublication(OperationPRPublish, a) || !validPublication(OperationPRPublish, b) || *a != *b {
		t.Fatalf("production descriptors = %+v / %+v, want two identical valid descriptors", a, b)
	}

	// Cancellation matches through the PRODUCTION-journaled descriptors. It takes no
	// GitHub deps at all, and every adapter capture counter must hold still.
	snapshot := func() [6]int {
		return [6]int{
			len(ghFail.ensureCalls), ghFail.discoverCalls, len(ghFail.probeCalls),
			len(ghOK.ensureCalls), ghOK.discoverCalls, len(ghOK.probeCalls),
		}
	}
	before := snapshot()
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q (findings %v), want cancelled via the production-journaled match", res.Disposition, res.Findings)
	}
	if after := snapshot(); after != before {
		t.Fatalf("adapter calls moved during cancellation %v -> %v; reconciliation must issue no GitHub call", before, after)
	}

	ep, _, err = LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord after cancel: %v", err)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatalf("original entry status = %q, want durably completed", ep.AdmittedMutations[0].Status)
	}

	// Leak audit: neither the durable journal bytes nor the cancel findings carry
	// title/body content — digests and bounded tokens only.
	raw, rerr := os.ReadFile(filepath.Join(rungateRootOf(fx.common), fx.key, epochRecordFileName))
	if rerr != nil {
		t.Fatalf("read epoch record: %v", rerr)
	}
	for _, secret := range []string{"SEKRET-TITLE-BYTES", "SEKRET-BODY-BYTES"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("journal bytes leak %q", secret)
		}
		for _, f := range res.Findings {
			if bytes.Contains([]byte(f), []byte(secret)) {
				t.Fatalf("cancel finding %q leaks %q", f, secret)
			}
		}
	}
}

// assertUnverifiedRetryLeavesOriginalPending runs the REAL cancel path over a
// journal the production boundary wrote as [uncertain original, admitted-then-
// resolved identical retry] and asserts the retry verified NOTHING it could settle
// with: the original stays uncertain, cancellation stays cancellation-pending with
// the exclusion finding, and no settlement is reported (change 0444 review
// blocker: a refused/failed retry must never count as proof).
func assertUnverifiedRetryLeavesOriginalPending(t *testing.T, fx cancelFixture, op string) {
	t.Helper()
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 2 || ep.AdmittedMutations[0].Status != mutationStatusUncertain {
		t.Fatalf("journal = %+v, want [uncertain original, resolved retry]", ep.AdmittedMutations)
	}
	a, b := ep.AdmittedMutations[0].Publication, ep.AdmittedMutations[1].Publication
	if !validPublication(op, a) || !validPublication(op, b) || *a != *b {
		t.Fatalf("descriptors = %+v / %+v, want two identical valid descriptors (else the case is vacuous)", a, b)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition == CancelDispositionCancelled {
		t.Fatalf("disposition = cancelled (findings %v): an unverified retry settled the uncertain original", res.Findings)
	}
	if !hasFinding(res.Findings, "mutation-pending:"+op) {
		t.Fatalf("findings = %v, want mutation-pending:%s (exclusion retained)", res.Findings, op)
	}
	if hasFinding(res.Findings, "mutation-settled") {
		t.Fatalf("findings = %v; an unverified retry must settle nothing", res.Findings)
	}
	ep, _, err = LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord after cancel: %v", err)
	}
	if s := ep.AdmittedMutations[0].Status; s != mutationStatusUncertain {
		t.Fatalf("original = %q after cancel, want uncertain (missing evidence never counts as success)", s)
	}
}

// TestProductionUnverifiedPRRetryNeverSettles (change 0444 review blocker): an
// identical pr.publish retry admitted after an uncertain first attempt, which then
// resolves contended, refused (invalid-state / invalid-input), or with an internal
// error, verified no postcondition — so it never settles the original. The applied
// positive control lives in TestProductionUncertainThenIdenticalRetryThenCancel.
func TestProductionUnverifiedPRRetryNeverSettles(t *testing.T) {
	cases := []struct {
		name  string
		retry *fakeGitHub
		want  Result
	}{
		{"contended", &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureContended}}, ResultContended},
		{"invalid-state", &fakeGitHub{repo: prRepo(), ensureErr: &githubcli.Failure{
			Op: "ensure-pull-request", Stage: githubcli.StageDecode, Kind: githubcli.KindInvalidState, Detail: "unexpected pull request shape"}}, ResultInvalidState},
		{"invalid-input", &fakeGitHub{repo: prRepo(), ensureErr: &githubcli.Failure{
			Op: "ensure-pull-request", Stage: githubcli.StageValidate, Kind: githubcli.KindInvalidInput, Detail: "bad request"}}, ResultInvalidInput},
		{"internal-error", &fakeGitHub{repo: prRepo(), ensureErr: errors.New("adapter panic recovered")}, ResultInternalError},
		{"unknown-disposition", &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: "bogus"}}, ResultInternalError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t, true)
			deps := workspaceDepsFor(t, prReader(t))
			req := PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Prose.\n", EvidenceRecord: prEvidenceBytes(t, prHead)}
			ghFail := &fakeGitHub{repo: prRepo(), ensureErr: &githubcli.Failure{
				Op: "ensure-pull-request", Stage: githubcli.StageInvoke, Kind: githubcli.KindExternal, Detail: "transport reset"}}
			if res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: ghFail}, fx.worktree, req); res.Result != ResultExternalFailed {
				t.Fatalf("first attempt result = %q (reason %q), want external-failed", res.Result, res.Reason)
			}
			if res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: tc.retry}, fx.worktree, req); res.Result != tc.want {
				t.Fatalf("retry result = %q (reason %q), want %q", res.Result, res.Reason, tc.want)
			}
			if len(tc.retry.ensureCalls) != 1 {
				t.Fatalf("retry ensure calls = %d, want 1 (the retry must be admitted and reach the adapter)", len(tc.retry.ensureCalls))
			}
			assertUnverifiedRetryLeavesOriginalPending(t, fx, OperationPRPublish)
		})
	}
}

// TestProductionUnverifiedWorkspaceRetryNeverSettles (change 0444 review
// blocker): the workspace.publish analog. An identical retry that PublishHead
// resolves contended, refuses locally (invalid-state "workspace is not in a ready
// phase" from its reinspection), fails with an internal error, or reports a head
// other than the journaled one never settles the uncertain original; an applied retry (the positive control) does, proving the
// fixture journals through the real fence.
func TestProductionUnverifiedWorkspaceRetryNeverSettles(t *testing.T) {
	const head = "abcdef0000000000000000000000000000000000"
	const movedHead = "fedcba0000000000000000000000000000000000"
	ready := workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)}
	cases := []struct {
		name   string
		retry  *fakeWorkspaceService
		want   Result
		settle bool
	}{
		{"applied (positive control)", &fakeWorkspaceService{inspection: ready,
			publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(head)}}, ResultApplied, true},
		{"already-published (positive control)", &fakeWorkspaceService{inspection: ready,
			publishRes: workspace.PublishResult{Disposition: workspace.PublishAlreadyPublished, Head: gitcli.ObjectID(head)}}, ResultNoOp, true},
		// Moved head (change 0444 review finding): PublishHead reads its own local
		// head under its lock, after the app-level Inspect, so a commit landing in
		// between makes it push a head other than the journaled req.Head. That
		// completion observed a postcondition for a DIFFERENT commit and must never
		// settle an uncertain entry for req.Head — nor may a result carrying no head.
		{"published a moved head", &fakeWorkspaceService{inspection: ready,
			publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(movedHead)}}, ResultApplied, false},
		{"already-published a moved head", &fakeWorkspaceService{inspection: ready,
			publishRes: workspace.PublishResult{Disposition: workspace.PublishAlreadyPublished, Head: gitcli.ObjectID(movedHead)}}, ResultNoOp, false},
		{"published with no head", &fakeWorkspaceService{inspection: ready,
			publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished}}, ResultApplied, false},
		{"contended", &fakeWorkspaceService{inspection: ready,
			publishRes: workspace.PublishResult{Disposition: workspace.PublishContended, Head: gitcli.ObjectID(head)}}, ResultContended, false},
		{"invalid-state", &fakeWorkspaceService{inspection: ready,
			publishErr: &workspace.Failure{Op: "publish-head", Kind: workspace.KindInvalidState, Detail: "workspace is not in a ready phase"}}, ResultInvalidState, false},
		{"invalid-input", &fakeWorkspaceService{inspection: ready,
			publishErr: &workspace.Failure{Op: "publish-head", Kind: workspace.KindInvalidInput, Detail: "bad target"}}, ResultInvalidInput, false},
		{"internal-error", &fakeWorkspaceService{inspection: ready,
			publishRes: workspace.PublishResult{Disposition: "bogus"}}, ResultInternalError, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t, true)
			reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
			deps := workspaceDepsFor(t, reader)
			req := WorkspacePublishRequest{ID: 7, Head: head}
			first := &fakeWorkspaceService{inspection: ready, publishRes: workspace.PublishResult{Disposition: workspace.PublishUnknown}}
			if res := WorkspacePublish(context.Background(), deps, WorkspaceDeps{Service: first}, fx.worktree, req); res.Result != ResultExternalFailed {
				t.Fatalf("first attempt result = %q (reason %q), want external-failed", res.Result, res.Reason)
			}
			if res := WorkspacePublish(context.Background(), deps, WorkspaceDeps{Service: tc.retry}, fx.worktree, req); res.Result != tc.want {
				t.Fatalf("retry result = %q (reason %q), want %q", res.Result, res.Reason, tc.want)
			}
			if len(tc.retry.publishCalls) != 1 {
				t.Fatalf("retry PublishHead calls = %d, want 1 (the retry must be admitted and reach the adapter)", len(tc.retry.publishCalls))
			}
			if !tc.settle {
				assertUnverifiedRetryLeavesOriginalPending(t, fx, OperationWorkspacePublish)
				return
			}
			stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
			res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.epochID, "human stop")
			if res.Disposition != CancelDispositionCancelled {
				t.Fatalf("disposition = %q (findings %v), want cancelled via a verified identical retry", res.Disposition, res.Findings)
			}
		})
	}
}
