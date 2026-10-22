package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// completeSpecPath is the design reference a build-ready proposed fixture
// carries; the spec file itself need not exist on the tree.
const completeSpecPath = "docs/superpowers/specs/2026-08-01-widget-design.md"

// completeRecord renders change 3 ("widget") in status as a results-artifact
// change with the valid FINAL results artifact attached (miResultsPath). A
// proposed fixture also carries a spec so it is build-ready.
func completeRecord(status string) string {
	src := lifecycleChange(3, miSlug, status)
	src = strings.Replace(src, "type: feat\n", "type: feat\ndeliverable: results-artifact\n", 1)
	src = strings.Replace(src, "results:\n", "results: '"+miResultsPath+"'\n", 1)
	if status == "proposed" {
		src = strings.Replace(src, "spec:\n", "spec: '"+completeSpecPath+"'\n", 1)
	}
	return src
}

// completeFiles is the happy tree: the record at its active path plus every
// mi results artifact on the metadata branch.
func completeFiles(record string) map[string]string {
	files := map[string]string{groomPath(3, miSlug): record}
	for p, content := range miMetadataResults() {
		files[p] = content
	}
	return files
}

func baseCompleteOp(surfaces []string) changeCompleteOp {
	return changeCompleteOp{
		changeID:   3,
		path:       groomPath(3, miSlug),
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

// completePlanFor runs one Plan attempt over files, failing the test only on an
// internal error (a refusal is returned for the caller to inspect).
func completePlanFor(t *testing.T, files map[string]string, op changeCompleteOp) (transaction.MutationPlan, transaction.OperationResult) {
	t.Helper()
	tree := newFakeTree(files)
	before, err := newPlanningLoader(op.eff).Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}
	plan, opRes, err := op.Plan(context.Background(), transaction.AttemptState{
		Base: tree.Revision(), State: before, Tree: tree,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return plan, opRes
}

func validCompleteRequest() ChangeCompleteRequest {
	return ChangeCompleteRequest{ChangeID: 3, Path: groomPath(3, miSlug), Revision: blobV}
}

// --- request-shape validation (no engine call) ----------------------------

func TestChangeCompleteRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeCompleteRequest)
		code string
	}{
		{"non-positive change id", func(r *ChangeCompleteRequest) { r.ChangeID = 0 }, "invalid-change_id"},
		{"empty path", func(r *ChangeCompleteRequest) { r.Path = "" }, "empty-path"},
		{"empty revision", func(r *ChangeCompleteRequest) { r.Revision = "" }, "empty-revision"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validCompleteRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			res := ChangeComplete(context.Background(), PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}, "", req)
			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called %d times on a shape failure, want 0", len(engine.calls))
			}
			if !hasFindingCode(res.Findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, res.Findings)
			}
			if res.Operation != OperationChangeComplete {
				t.Errorf("operation = %q, want %q", res.Operation, OperationChangeComplete)
			}
		})
	}
}

// --- plan closure ----------------------------------------------------------

func TestChangeCompletePlanFromProposed(t *testing.T) {
	recPath := groomPath(3, miSlug)
	archivePath := killArchivePath(3, miSlug)
	files := completeFiles(completeRecord("proposed"))
	files["docs/changes/BOARD.md"] = "# Backlog\n\nold\n"

	plan, opRes := completePlanFor(t, files, baseCompleteOp([]string{"inline"}))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		archivePath:             transaction.MutationCreate,
		recPath:                 transaction.MutationDelete,
		miResultsPath:           transaction.MutationReplace,
		"docs/changes/BOARD.md": transaction.MutationReplace,
	})
	if want := "change 0003 completed (done, archived)"; plan.CommitSubject != want {
		t.Errorf("commit subject = %q, want %q", plan.CommitSubject, want)
	}

	rec := killRecordBytes(t, plan, archivePath)
	for _, want := range []string{"status: 'done'", "updated: '2026-08-16'", "deliverable: results-artifact", "docket:artifacts:start"} {
		if !strings.Contains(rec, want) {
			t.Errorf("archived record lacks %q:\n%s", want, rec)
		}
	}
	if strings.Contains(rec, "claimed_at") {
		t.Errorf("completing a proposed record wrote a claimed_at line:\n%s", rec)
	}
	results := killRecordBytes(t, plan, miResultsPath)
	if !strings.Contains(results, "../changes/archive/2026-08-16-0003-widget.md") {
		t.Errorf("results backlink not retargeted to the archive path:\n%s", results)
	}
}

func TestChangeCompletePlanFromInProgressWithAndWithoutRunHalted(t *testing.T) {
	for _, halted := range []bool{false, true} {
		name := "plain"
		if halted {
			name = "run halted"
		}
		t.Run(name, func(t *testing.T) {
			src := completeRecord("in-progress")
			if halted {
				src += "\n## Run halted\n\nThe run stopped at the gate.\n"
			}
			plan, opRes := completePlanFor(t, completeFiles(src), baseCompleteOp([]string{}))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			rec := killRecordBytes(t, plan, killArchivePath(3, miSlug))
			if !strings.Contains(rec, "status: 'done'") {
				t.Errorf("status not done:\n%s", rec)
			}
			if !strings.Contains(rec, "\nclaimed_at: 2026-08-02T00:00:00Z\n") {
				t.Errorf("completion rewrote the claimed_at line:\n%s", rec)
			}
			if !strings.Contains(rec, "branch: feat/widget") {
				t.Errorf("branch not kept:\n%s", rec)
			}
			if strings.Contains(rec, "## Run halted") {
				t.Errorf("run-halted section survived the completion:\n%s", rec)
			}
			if !strings.Contains(rec, "## Why\n\nOriginal why.\n") {
				t.Errorf("authored ## Why section not preserved:\n%s", rec)
			}
		})
	}
}

func TestChangeCompletePlanRefusals(t *testing.T) {
	const otherResults = "docs/results/2026-08-17-widget-elsewhere.md"
	withResults := func(src, p string) string {
		return strings.Replace(src, "results: '"+miResultsPath+"'\n", "results: '"+p+"'\n", 1)
	}
	cases := []struct {
		name   string
		files  map[string]string
		reason string
	}{
		{"pr deliverable", completeFiles(strings.Replace(completeRecord("in-progress"), "deliverable: results-artifact\n", "deliverable: pr\n", 1)), "not-results-artifact"},
		{"absent deliverable", completeFiles(strings.Replace(completeRecord("in-progress"), "deliverable: results-artifact\n", "", 1)), "not-results-artifact"},
		{"blocked", completeFiles(completeRecord("blocked")), "illegal-source-status"},
		{"deferred", completeFiles(completeRecord("deferred")), "illegal-source-status"},
		{"implemented", completeFiles(completeRecord("implemented")), "illegal-source-status"},
		{"needs-grooming proposed", completeFiles(strings.Replace(completeRecord("proposed"), "spec: '"+completeSpecPath+"'\n", "spec:\n", 1)), "not-ready-needs-grooming"},
		{"open dependency", func() map[string]string {
			f := completeFiles(strings.Replace(completeRecord("proposed"), "depends_on: []\n", "depends_on: [5]\n", 1))
			f[groomPath(5, "base")] = lifecycleChange(5, "base", "proposed")
			return f
		}(), "not-ready-waiting-dependency"},
		{"pr recorded", completeFiles(strings.Replace(completeRecord("in-progress"), "blocked_by:\n", "blocked_by:\npr: 'https://github.com/acme/widget/pull/42'\n", 1)), "pr-recorded"},
		{"no results", completeFiles(strings.Replace(completeRecord("in-progress"), "results: '"+miResultsPath+"'\n", "results:\n", 1)), "results-missing"},
		{"results file absent", completeFiles(withResults(completeRecord("in-progress"), otherResults)), ReasonImplementedResultsIdentity},
		{"foreign backlink", completeFiles(withResults(completeRecord("in-progress"), miResultsMismatchPath)), ReasonImplementedResultsIdentity},
		{"final contract failure", completeFiles(withResults(completeRecord("in-progress"), miResultsInvalidPath)), ReasonImplementedResultsInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, opRes := completePlanFor(t, c.files, baseCompleteOp([]string{"inline"}))
			if !opRes.Refused {
				t.Fatalf("completion was not refused; planned %v", planPaths(plan))
			}
			if len(plan.Files) != 0 {
				t.Errorf("a refusal planned files: %v", planPaths(plan))
			}
			if !hasDomainFindingCode(opRes.Findings, c.reason) {
				t.Errorf("missing refusal %q; got %v", c.reason, opRes.Findings)
			}
		})
	}
}

// --- replay ----------------------------------------------------------------

func TestChangeCompleteReplaysAlreadyCompleted(t *testing.T) {
	archivePath := killArchivePath(3, miSlug)
	archived := strings.Replace(completeRecord("done"), "claimed_at: 2026-08-02T00:00:00Z\n", "", 1)
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{{
		Kind: repository.KindChange, Location: repository.LocationArchive,
		Path: archivePath, Revision: blobV, Data: []byte(archived),
	}}}
	engine := &recordingEngine{}

	res := ChangeComplete(context.Background(), PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}, "", validCompleteRequest())

	if res.Result != ResultNoOp {
		t.Fatalf("result = %q, want no-op (findings %v)", res.Result, res.Findings)
	}
	if res.Disposition != CompleteDispAlreadyCompleted || res.ArchivePath != archivePath || res.ID != 3 {
		t.Fatalf("replay = (%q, %q, %d), want (already-completed, %q, 3)", res.Disposition, res.ArchivePath, res.ID, archivePath)
	}
	if len(engine.calls) != 0 {
		t.Fatalf("engine called %d times on a replay", len(engine.calls))
	}
	if got := res.HumanText(); got != "change 0003 already completed ("+archivePath+")" {
		t.Errorf("HumanText = %q", got)
	}
}

// --- outcome mapping -------------------------------------------------------

func TestChangeCompleteResultFromOutcome(t *testing.T) {
	applied := changeCompleteResultFromOutcome(transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       mustMarshal(t, changeCompleteReceipt{ArchivePath: killArchivePath(3, miSlug), ID: 3, Op: OperationChangeComplete}),
	}, nil)
	if applied.Result != ResultApplied || applied.Disposition != CompleteDispCompleted || applied.ID != 3 || applied.ArchivePath != killArchivePath(3, miSlug) {
		t.Fatalf("applied = %+v", applied)
	}

	contended := changeCompleteResultFromOutcome(transaction.Result{Disposition: transaction.DispositionContended}, nil)
	if contended.Result != ResultContended || contended.Disposition != CompleteDispContended {
		t.Fatalf("contended = %+v", contended)
	}

	refused := changeCompleteResultFromOutcome(transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings:    []domain.Finding{{Code: "not-results-artifact", Severity: domain.SeverityError}},
	}, nil)
	if refused.Result != ResultInvalidState || refused.Disposition != "" || firstStatusFindingCode(refused.Findings) != "not-results-artifact" {
		t.Fatalf("refused = %+v", refused)
	}

	failed := changeCompleteResultFromOutcome(transaction.Result{Disposition: transaction.DispositionFailed}, &transaction.Failure{
		Stage: transaction.StageLoadAfter, Kind: transaction.KindInvalidState, Detail: "boom",
	})
	if failed.Disposition != CompleteDispFailed || failed.Failure == nil {
		t.Fatalf("failed = %+v", failed)
	}
}

// TestChangeCompletePlanFromProposedHeld: completing a held proposed change
// names it by id, so the hold does not refuse it.
func TestChangeCompletePlanFromProposedHeld(t *testing.T) {
	src := strings.Replace(completeRecord("proposed"), "trivial: false\n", "trivial: false\nhold: true\n", 1)
	plan, opRes := completePlanFor(t, completeFiles(src), baseCompleteOp([]string{"inline"}))
	if opRes.Refused {
		t.Fatalf("complete of a held change refused: %v", opRes.Findings)
	}
	rec := killRecordBytes(t, plan, killArchivePath(3, miSlug))
	if !strings.Contains(rec, "status: 'done'") {
		t.Errorf("archived record not done:\n%s", rec)
	}
}
