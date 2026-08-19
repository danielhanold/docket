package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file drives `change halt`, `change resume-halted`, the run-verify
// run-halted verdict, and the context-implementation halt reporting. The
// transaction plan closures run over an in-memory fakeTree; the acknowledgement
// gate and the quiescence mapping are pure; the reprobe-then-recover round-trip
// runs over a REAL feature workspace with a fake WorkspaceService that scripts
// the reprobed state.

// --- fake WorkspaceService for resume reprobe ------------------------------

// fakeResumeWorkspace scripts the reprobed workspace Kind resume-halted reads to
// decide whether a live writer may still hold the workspace. Prepare/PublishHead
// panic — resume never allocates or publishes.
type fakeResumeWorkspace struct {
	kind workspace.StateKind
	head string
}

func (f fakeResumeWorkspace) Prepare(context.Context, workspace.PrepareRequest) (workspace.Workspace, error) {
	panic("Prepare: resume must not allocate")
}
func (f fakeResumeWorkspace) Inspect(context.Context, workspace.InspectRequest) (workspace.Inspection, error) {
	return workspace.Inspection{Kind: f.kind, HeadCommit: gitcli.ObjectID(f.head)}, nil
}
func (f fakeResumeWorkspace) PublishHead(context.Context, workspace.PublishRequest) (workspace.PublishResult, error) {
	panic("PublishHead: resume must not publish")
}

// --- change halt plan closures ---------------------------------------------

// TestChangeHaltPreservesCheckpoints proves a halt records the bounded report in
// one "## Run halted" section and touches no frontmatter — branch, claim, and
// every other datum stay byte-identical — and that a non-in-progress change is
// refused.
func TestChangeHaltPreservesCheckpoints(t *testing.T) {
	recPath := groomPath(3, "widget")
	inProgress := lifecycleChange(3, "widget", "in-progress")
	files := map[string]string{
		recPath:                 inProgress,
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	op := changeHaltOp{id: 3, report: "Blocked on infra; see run 7.\n", eff: planningTestConfig([]string{"inline"}),
		clock: testClock(), inline: true, changesDir: "docs/changes"}
	plan, opRes := blockPlanFor(t, files, planningTestConfig([]string{"inline"}), op)
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "## Run halted") || !strings.Contains(rec, "Blocked on infra; see run 7.") {
		t.Errorf("halt report not recorded:\n%s", rec)
	}
	// Branch and claim lease byte-identical — halt touches no frontmatter.
	for _, want := range []string{"branch: feat/widget", "claimed_at: 2026-08-02T00:00:00Z", "reconciled: true"} {
		if !strings.Contains(rec, want) {
			t.Errorf("halt altered a preserved field; missing %q:\n%s", want, rec)
		}
	}

	// A non-in-progress change has no run to halt.
	files[recPath] = fixtureChange(3, "widget") // proposed
	_, opRes2 := blockPlanFor(t, files, planningTestConfig(nil),
		changeHaltOp{id: 3, report: "x\n", eff: planningTestConfig(nil), clock: testClock(), changesDir: "docs/changes"})
	if !opRes2.Refused || firstFindingCode(opRes2.Findings) != ReasonHaltNotInProgress {
		t.Fatalf("proposed change: refused=%v code=%q, want %q", opRes2.Refused, firstFindingCode(opRes2.Findings), ReasonHaltNotInProgress)
	}
}

// --- resume: acknowledgement gate + quiescence mapping (pure) --------------

// TestChangeResumeHaltedRequiresAcknowledgement proves resume refuses before any
// effect without the explicit --acknowledge-quiescent human acknowledgement.
func TestChangeResumeHaltedRequiresAcknowledgement(t *testing.T) {
	got := ChangeResumeHalted(context.Background(),
		PlanningDeps{Reader: &fakeReader{}, Clock: testClock()}, WorkspaceDeps{}, "",
		ResumeRequest{ID: 3, Version: blobV, AcknowledgeQuiescent: false})
	if got.Result != ResultBlocked || got.Reason != ReasonResumeNotAcknowledged {
		t.Fatalf("result=%q reason=%q, want blocked/%s", got.Result, got.Reason, ReasonResumeNotAcknowledged)
	}
}

// TestResumeQuiescenceMapping proves the reprobed-state mapping: an allocating,
// foreign, or mismatched workspace has a writer that may be live and refuses; a
// ready or dirty-owned (the prior worker's checkpoints) workspace resumes.
func TestResumeQuiescenceMapping(t *testing.T) {
	cases := []struct {
		state  workspace.StateKind
		refuse bool
	}{
		{workspace.StateResumable, true},
		{workspace.StateForeign, true},
		{workspace.StateMismatch, true},
		{workspace.StateReady, false},
		{workspace.StateDirty, false},
		{workspace.StateBranchGone, false},
	}
	for _, tc := range cases {
		reason, _ := resumeQuiescenceRefusal(string(tc.state))
		if (reason != "") != tc.refuse {
			t.Errorf("state %q: refuse=%v (reason %q), want refuse=%v", tc.state, reason != "", reason, tc.refuse)
		}
		if tc.refuse && reason != ReasonResumeWorkspaceActive {
			t.Errorf("state %q: reason=%q, want %q", tc.state, reason, ReasonResumeWorkspaceActive)
		}
	}
}

// --- resume: reprobe then recover (real git) -------------------------------

// TestChangeResumeHalted proves the full recovery: a live-writer reprobe refuses
// and leaves the marker; a version drift is contended; a quiescent reprobe
// refreshes the claim, removes exactly the marker section, and preserves every
// other byte.
func TestChangeResumeHalted(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			// A live writer (allocating workspace) refuses; the marker stays.
			t.Run("live-writer-refuses", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateResumable, head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: f.version, AcknowledgeQuiescent: true})
				if got.Reason != ReasonResumeWorkspaceActive {
					t.Fatalf("reason=%q, want %q", got.Reason, ReasonResumeWorkspaceActive)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if !strings.Contains(rec, "## Run halted") {
					t.Errorf("marker removed on a refused resume:\n%s", rec)
				}
			})

			// A version drift is a lost race: contended, marker retained.
			t.Run("version-drift-contended", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateReady, head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: strings.Repeat("b", 40), AcknowledgeQuiescent: true})
				if got.Result != ResultContended {
					t.Fatalf("result=%q disp=%q, want contended", got.Result, got.Disposition)
				}
			})

			// A quiescent reprobe recovers: claim refreshed, marker removed, other
			// bytes preserved.
			t.Run("quiescent-resumes", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateReady, head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: f.version, AcknowledgeQuiescent: true})
				if got.Result != ResultApplied || got.Disposition != HaltDispResumed {
					t.Fatalf("result=%q disp=%q reason=%q", got.Result, got.Disposition, got.Reason)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if strings.Contains(rec, "## Run halted") {
					t.Errorf("marker not removed on resume:\n%s", rec)
				}
				if !strings.Contains(rec, "claimed_at: '2026-08-16T12:00:00Z'") {
					t.Errorf("claim lease not refreshed:\n%s", rec)
				}
				// Preserved: branch and the authored ## Why section byte-identical.
				for _, want := range []string{"branch: feat/widget", "## Why\n\nOriginal why."} {
					if !strings.Contains(rec, want) {
						t.Errorf("resume altered a preserved byte; missing %q:\n%s", want, rec)
					}
				}
			})
		})
	}
}

// setupHaltedFixture builds a coherent in-progress feature workspace whose record
// carries a durable "## Run halted" section — the state resume-halted recovers.
func setupHaltedFixture(t *testing.T, m planRepoMode) *rebaseFixture {
	t.Helper()
	f := setupRebaseFixtureStatus(t, m, "in-progress")
	halted := strings.TrimRight(lifecycleChange(f.id, f.slug, "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused pending infra.\n"
	f.repo.writerAdvance(t, f.branch, map[string]string{groomPath(f.id, f.slug): halted})
	f.version = blobVersionAt(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
	return f
}

// TestHaltResultFromOutcomeFailedCarriesCause proves a halt transaction that
// fails mid-flight is dispositioned `failed` (not mislabeled as a refusal) and
// carries its typed cause in the envelope's failure diagnosis.
func TestHaltResultFromOutcomeFailedCarriesCause(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageVerifyDelta,
		Kind:   transaction.KindInvalidState,
		Detail: "an undeclared path changed in the worktree",
	}
	out := haltResultFromOutcome(OperationChangeHalt,
		transaction.Result{Disposition: transaction.DispositionFailed}, execErr,
		HaltDispHalted, ReasonHaltNotInProgress)

	if out.Disposition != HaltDispFailed {
		t.Errorf("disposition = %q, want %q — a failed transaction is not a refusal", out.Disposition, HaltDispFailed)
	}
	if out.Failure == nil || out.Failure.Detail == "" {
		t.Fatalf("failure diagnosis missing or empty: %+v", out.Failure)
	}
}
