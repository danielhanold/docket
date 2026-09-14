package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
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
	alias := filepath.Join(t.TempDir(), "wt-alias")
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
