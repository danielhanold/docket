package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// --- Plan-closure helper ---------------------------------------------------

func attachPlanFor(t *testing.T, files map[string]string, op changeAttachOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func baseAttachOp(surfaces []string, id int, kind, artifact string) changeAttachOp {
	return changeAttachOp{
		opKey:      attachOpKey(kind),
		kind:       kind,
		changeID:   id,
		artifact:   artifact,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

// --- TestChangeAttachPlanPatch ---------------------------------------------

// TestChangeAttachPlanPatch proves the transaction stores the plan path in the
// owned plan: field, re-renders the artifact block, refreshes updated, and names
// the record + board surfaces — nothing else in the record moves.
func TestChangeAttachPlanPatch(t *testing.T) {
	recPath := groomPath(3, "widget")
	planPath := "docs/superpowers/plans/2026-08-17-widget-plan.md"
	files := map[string]string{
		recPath:                 lifecycleChange(3, "widget", "in-progress"),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := attachPlanFor(t, files, baseAttachOp([]string{"inline"}, 3, attachKindPlan, planPath))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:                 transaction.MutationReplace,
		"docs/changes/BOARD.md": transaction.MutationReplace,
	})
	rec := lifecycleRecordBytes(t, plan, recPath)
	for _, want := range []string{
		"plan: '" + planPath + "'",
		"updated: '2026-08-16'",
		"docket:artifacts:start",
		"status: in-progress", // untouched, still unquoted
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("attached record missing %q:\n%s", want, rec)
		}
	}
	if strings.Contains(rec, "results: '") {
		t.Errorf("attach-plan wrote a results field:\n%s", rec)
	}
}

// --- TestChangeAttachResultsPatch ------------------------------------------

// TestChangeAttachResultsPatch proves the results transaction stores the results
// path in the owned results: field, leaving plan: untouched.
func TestChangeAttachResultsPatch(t *testing.T) {
	recPath := groomPath(3, "widget")
	resultsPath := "docs/results/2026-08-17-widget-results.md"
	files := map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")}
	plan, opRes := attachPlanFor(t, files, baseAttachOp(nil, 3, attachKindResults, resultsPath))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "results: '"+resultsPath+"'") {
		t.Errorf("attached record missing results field:\n%s", rec)
	}
	if strings.Contains(rec, "plan: '") {
		t.Errorf("attach-results wrote a plan field:\n%s", rec)
	}
}

// --- TestChangeAttachContention --------------------------------------------

// TestChangeAttachContention proves a transaction CAS miss folds to a contended
// result carrying no revision, with Findings marshalling as [] never nil.
func TestChangeAttachContention(t *testing.T) {
	res := attachResultFromOutcome(OperationChangeAttachPlan, attachKindPlan,
		"docs/superpowers/plans/x.md", transaction.Result{Disposition: transaction.DispositionContended}, nil)
	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	if res.Revision != "" {
		t.Errorf("contended result carried a revision %q", res.Revision)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

// --- TestChangeAttachRejectsBadShape ---------------------------------------

// TestChangeAttachRejectsBadShape proves the request-shape check refuses a
// malformed request before any pin/engine work, for every missing scalar.
func TestChangeAttachRejectsBadShape(t *testing.T) {
	valid := ChangeAttachRequest{ID: 3, Version: blobV, Path: "docs/superpowers/plans/x.md", Commit: blobV}
	cases := []struct {
		name string
		mut  func(*ChangeAttachRequest)
		code string
	}{
		{"non-positive id", func(r *ChangeAttachRequest) { r.ID = 0 }, "invalid-id"},
		{"empty path", func(r *ChangeAttachRequest) { r.Path = "" }, "empty-path"},
		{"empty version", func(r *ChangeAttachRequest) { r.Version = "" }, "empty-version"},
		{"empty commit", func(r *ChangeAttachRequest) { r.Commit = " " }, "empty-commit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := valid
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeAttachPlan(context.Background(), deps, WorkspaceDeps{}, "", req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called %d times on a shape failure, want 0", len(engine.calls))
			}
			if !hasFindingCode(res.Findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, res.Findings)
			}
		})
	}
}
