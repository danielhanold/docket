package app

import (
	"context"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
)

// claimableChange renders a proposed, build-ready change: the canonical proposed
// record with trivial: true (so it carries a design outcome) and no claim
// fields yet. Claiming it must INSERT branch/claimed_at/reconciled.
func claimableChange(id int, slug string) string {
	return strings.Replace(lifecycleChange(id, slug, "proposed"), "trivial: false\n", "trivial: true\n", 1)
}

// stackedOn rewrites a record's empty stacked_on edge to point at parent.
func stackedOn(src string, parent int) string {
	return strings.Replace(src, "stacked_on:\n", "stacked_on: "+itoaTest(parent)+"\n", 1)
}

// --- Plan-closure helper ---------------------------------------------------

func claimPlanFor(t *testing.T, files map[string]string, op changeClaimOp) (transaction.MutationPlan, transaction.OperationResult) {
	t.Helper()
	tree := newFakeTree(files)
	loader := newPlanningLoader(op.eff)
	before, err := loader.Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}
	if before.Report.HasErrors() {
		t.Fatalf("before-state has errors: %v", before.Report.Findings())
	}
	plan, opRes, err := op.Plan(context.Background(), transaction.AttemptState{
		Base: tree.Revision(), State: before, Tree: tree,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return plan, opRes
}

func baseClaimOp(surfaces []string, id int, facts domain.BranchFacts) changeClaimOp {
	return changeClaimOp{
		opKey:      OperationChangeClaim,
		changeID:   id,
		facts:      facts,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

func baseRefreshOp(surfaces []string, id int) changeClaimOp {
	op := baseClaimOp(surfaces, id, domain.BranchFacts{})
	op.opKey = OperationChangeRefreshClaim
	op.refresh = true
	return op
}

// --- TestChangeClaimApplies ------------------------------------------------

// --- TestChangeClaimRefusals -----------------------------------------------

// --- TestChangeClaimRetryConvergence ---------------------------------------

// --- TestChangeRefreshClaimStampsOnly --------------------------------------

// --- TestChangeRefreshClaimSkipsUnchangedBoard ------------------------------

// TestChangeRefreshClaimSkipsUnchangedBoard: a refresh re-stamps only
// claimed_at and updated — neither is board-visible — so its board re-render
// can be byte-identical to the committed BOARD.md. Declaring an unchanged
// path trips the engine's verify-delta guard ("a declared path is not an
// actual change") and fails the whole refresh (change 0335), so the plan must
// declare the board only when it truly changes the tree: absent -> create,
// differing -> replace, byte-identical -> not declared at all.
func TestChangeRefreshClaimSkipsUnchangedBoard(t *testing.T) {
	recPath := groomPath(3, "widget")
	src := lifecycleChange(3, "widget", "in-progress")
	const boardPath = "docs/changes/BOARD.md"

	// Pass 1: no committed board. The refresh must still declare the board as
	// a create — and its declared bytes are the canonical render for this
	// corpus at the fixed test clock, which seeds the byte-identical case.
	plan, opRes := claimPlanFor(t, map[string]string{recPath: src}, baseRefreshOp([]string{"inline"}, 3))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:   transaction.MutationReplace,
		boardPath: transaction.MutationCreate,
	})
	var boardBytes []byte
	for _, f := range plan.Files {
		if string(f.Path) == boardPath {
			boardBytes = f.Bytes
		}
	}
	if len(boardBytes) == 0 {
		t.Fatal("absent-board refresh declared no board bytes")
	}

	t.Run("byte-identical committed board is not declared", func(t *testing.T) {
		files := map[string]string{recPath: src, boardPath: string(boardBytes)}
		plan, opRes := claimPlanFor(t, files, baseRefreshOp([]string{"inline"}, 3))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		// Exactly the record — a declared-but-unchanged board is the 0335 bug.
		assertPlanPaths(t, plan, map[string]transaction.MutationKind{
			recPath: transaction.MutationReplace,
		})
	})

	t.Run("stale committed board is still declared", func(t *testing.T) {
		files := map[string]string{recPath: src, boardPath: "# Backlog\n\nstale\n"}
		plan, opRes := claimPlanFor(t, files, baseRefreshOp([]string{"inline"}, 3))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		assertPlanPaths(t, plan, map[string]transaction.MutationKind{
			recPath:   transaction.MutationReplace,
			boardPath: transaction.MutationReplace,
		})
	})
}

func TestClaimResultFromOutcomeFailedCarriesCause(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageVerifyDelta,
		Kind:   transaction.KindInvalidState,
		Detail: "an undeclared path changed in the worktree",
	}
	res := transaction.Result{Disposition: transaction.DispositionFailed}

	out := claimResultFromOutcome(OperationChangeRefreshClaim, res, execErr)

	if out.Result != ResultInvalidState {
		t.Fatalf("result = %q, want %q", out.Result, ResultInvalidState)
	}
	if out.Disposition != ClaimDispositionFailed {
		t.Errorf("disposition = %q, want %q", out.Disposition, ClaimDispositionFailed)
	}
	if out.Disposition == string(out.Result) {
		t.Errorf("disposition %q merely restates the result — the tautology is back", out.Disposition)
	}
	if out.Failure == nil {
		t.Fatal("failure diagnosis missing on a failed disposition — the Failure was dropped again")
	}
	if out.Failure.Detail == "" {
		t.Error("failure.detail is empty")
	}
	if out.Failure.Stage != string(transaction.StageVerifyDelta) || out.Failure.Kind != string(transaction.KindInvalidState) {
		t.Errorf("failure = %+v, want stage %q kind %q", out.Failure, transaction.StageVerifyDelta, transaction.KindInvalidState)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings = %v, want empty — findings are the refusal channel, not the failure channel", out.Findings)
	}

	ok := claimResultFromOutcome(OperationChangeClaim, transaction.Result{Disposition: transaction.DispositionApplied}, nil)
	if ok.Failure != nil {
		t.Errorf("failure must be nil on an applied outcome, got %+v", ok.Failure)
	}
}
