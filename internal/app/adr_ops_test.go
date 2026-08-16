package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// --- fixtures --------------------------------------------------------------

// adrsIndexPath is the canonical ADR index path every ADR operation rerenders.
const adrsIndexPath = "docs/adrs/README.md"

// adrPath is the canonical adrs-dir record path for an id/slug.
func adrPath(id, slug string) string {
	return "docs/adrs/" + id + "-" + slug + ".md"
}

func validADRRecordRequest() ADRRecordRequest {
	return ADRRecordRequest{
		RequestID:    "adr-00000001",
		Title:        "Record the widget decision",
		Context:      "The widget needs a home.\n",
		Decision:     "Put it in the shed.\n",
		Consequences: "The shed is now full.\n",
		Alternatives: "The garage was considered.\n",
	}
}

// --- request-shape validation (no engine call) -----------------------------

func TestADRRecordRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ADRRecordRequest)
		code string
	}{
		{"short request id", func(r *ADRRecordRequest) { r.RequestID = "short" }, "invalid-request-id"},
		{"empty title", func(r *ADRRecordRequest) { r.Title = "" }, "empty-title"},
		{"blank context", func(r *ADRRecordRequest) { r.Context = "  " }, "empty-context"},
		{"empty decision", func(r *ADRRecordRequest) { r.Decision = "" }, "empty-decision"},
		{"empty consequences", func(r *ADRRecordRequest) { r.Consequences = "" }, "empty-consequences"},
		{"empty alternatives", func(r *ADRRecordRequest) { r.Alternatives = "" }, "empty-alternatives"},
		{"duplicate relates_to", func(r *ADRRecordRequest) { r.RelatesTo = []int{2, 2} }, "duplicate-relates_to"},
		{"non-positive relates_to", func(r *ADRRecordRequest) { r.RelatesTo = []int{0} }, "invalid-relates_to"},
		{"producing change empty path", func(r *ADRRecordRequest) {
			r.Change = &ADRProducingChange{ID: 1, Path: "", Version: blobV}
		}, "empty-change-path"},
		{"producing change empty version", func(r *ADRRecordRequest) {
			r.Change = &ADRProducingChange{ID: 1, Path: "docs/changes/active/0001-first.md", Version: ""}
		}, "empty-change-version"},
		{"producing change non-positive id", func(r *ADRRecordRequest) {
			r.Change = &ADRProducingChange{ID: 0, Path: "docs/changes/active/0001-first.md", Version: blobV}
		}, "invalid-change-id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validADRRecordRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ADRRecordOp(context.Background(), deps, "", req)

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

// --- outcome mapping (engine reached; Discover over a real temp repo) -------

func TestADRRecordAppliedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	receipt := mustMarshal(t, adrRecordReceipt{
		ID: 7, Op: OperationADRRecord, Path: adrPath("0007", "record-the-widget-decision"),
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRRecordOp(context.Background(), deps, repoDir, validADRRecordRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.ID != 7 || res.Path != adrPath("0007", "record-the-widget-decision") {
		t.Errorf("identity from receipt = (%d, %q)", res.ID, res.Path)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}
	if res.Replayed {
		t.Errorf("Replayed = true on a fresh apply")
	}
	if res.Operation != OperationADRRecord {
		t.Errorf("operation = %q, want %q", res.Operation, OperationADRRecord)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationADRRecord {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/main" {
		t.Errorf("target ref = %q, want refs/heads/main", req.TargetRef)
	}
	if req.Idempotency == nil || req.Idempotency.RequestID != "adr-00000001" {
		t.Errorf("idempotency key = %+v", req.Idempotency)
	}
	// No producing change ⇒ no entity expectation.
	if len(req.Expected) != 0 {
		t.Errorf("plain record must carry no entity expectation, got %+v", req.Expected)
	}
}

func TestADRRecordWithProducingChangeCarriesExpectation(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	req := validADRRecordRequest()
	req.Change = &ADRProducingChange{ID: 1, Path: "docs/changes/active/0001-first.md", Version: blobV}

	res := ADRRecordOp(context.Background(), deps, repoDir, req)

	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	call := engine.calls[0]
	if call.Idempotency == nil {
		t.Errorf("record is allocating; it must carry an idempotency key")
	}
	if len(call.Expected) != 1 {
		t.Fatalf("expected 1 entity expectation for the producing change, got %d", len(call.Expected))
	}
	exp := call.Expected[0]
	if string(exp.Path) != "docs/changes/active/0001-first.md" {
		t.Errorf("expectation path = %q", exp.Path)
	}
	if exp.Version.Kind != transaction.VersionBlob || string(exp.Version.ObjectID) != blobV {
		t.Errorf("expectation version = %+v", exp.Version)
	}
}

func TestADRRecordReplayResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	receipt := mustMarshal(t, adrRecordReceipt{
		ID: 4, Op: OperationADRRecord, Path: adrPath("0004", "record-the-widget-decision"),
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionAlreadyApplied,
		AppliedCommit: "0000000000000000000000000000000000000abc",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRRecordOp(context.Background(), deps, repoDir, validADRRecordRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if !res.Replayed {
		t.Errorf("Replayed = false on an already-applied replay")
	}
	if res.ID != 4 {
		t.Errorf("id = %d, want 4 (from the original receipt)", res.ID)
	}
}

func TestADRRecordRefusedMapsInvalidInput(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionRefused}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRRecordOp(context.Background(), deps, repoDir, validADRRecordRequest())

	if res.Result != ResultInvalidInput {
		t.Fatalf("refused disposition mapped to %q, want invalid-input", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

// --- record plan closure ---------------------------------------------------

func baseADRRecordOp(req ADRRecordRequest) adrRecordOp {
	return adrRecordOp{
		req:     req,
		eff:     planningTestConfig([]string{}),
		slug:    "record-the-widget-decision",
		clock:   testClock(),
		link:    render.LinkContext{MetadataBranch: "main"},
		adrsDir: "docs/adrs",
	}
}

func adrRecordPlanFor(t *testing.T, files map[string]string, op adrRecordOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func TestADRRecordPlanFileSetCreatesIndexWhenAbsent(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-a-decision.md": fixtureADR(1, "a-decision"),
	}
	plan, opRes := adrRecordPlanFor(t, files, baseADRRecordOp(validADRRecordRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// max(1)+1 = 2; the plan is EXACTLY the new ADR and the index.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		adrPath("0002", "record-the-widget-decision"): transaction.MutationCreate,
		adrsIndexPath: transaction.MutationCreate,
	})
	if plan.CommitSubject == "" {
		t.Error("empty commit subject")
	}
	assertCanonicalADRReceipt(t, plan.Receipt, 2, adrPath("0002", "record-the-widget-decision"))
}

func TestADRRecordPlanReplacesExistingIndex(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-a-decision.md": fixtureADR(1, "a-decision"),
		adrsIndexPath:                  "# ADR index\n\nold\n",
	}
	plan, opRes := adrRecordPlanFor(t, files, baseADRRecordOp(validADRRecordRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		adrPath("0002", "record-the-widget-decision"): transaction.MutationCreate,
		adrsIndexPath: transaction.MutationReplace,
	})
	// The index reflects the freshly-recorded ADR.
	idx := planFileBytes(t, plan, adrsIndexPath)
	if !strings.Contains(idx, "ADR-0002") {
		t.Errorf("index does not reflect the new ADR:\n%s", idx)
	}
}

func TestADRRecordPlanAllocatesNextIDAcrossGap(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-one.md":  fixtureADR(1, "one"),
		"docs/adrs/0005-five.md": fixtureADR(5, "five"),
	}
	plan, opRes := adrRecordPlanFor(t, files, baseADRRecordOp(validADRRecordRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// max(1, 5) + 1 = 6; the gap at 2, 3, 4 is never backfilled.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		adrPath("0006", "record-the-widget-decision"): transaction.MutationCreate,
		adrsIndexPath: transaction.MutationCreate,
	})
}

func TestADRRecordPlanRefusesDanglingRelatesTo(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-one.md": fixtureADR(1, "one"),
	}
	req := validADRRecordRequest()
	req.RelatesTo = []int{999} // no such ADR
	plan, opRes := adrRecordPlanFor(t, files, baseADRRecordOp(req))
	if !opRes.Refused {
		t.Fatalf("dangling relates_to was not refused")
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", plan.Files)
	}
	if !hasDomainFindingCode(opRes.Findings, "adr-dangling-reference") {
		t.Errorf("missing adr-dangling-reference finding: %v", opRes.Findings)
	}
}

func TestADRRecordPlanRecordContent(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-one.md": fixtureADR(1, "one"),
	}
	plan, _ := adrRecordPlanFor(t, files, baseADRRecordOp(validADRRecordRequest()))
	rec := planFileBytes(t, plan, adrPath("0002", "record-the-widget-decision"))
	for _, want := range []string{
		"id: 2",
		"slug: 'record-the-widget-decision'",
		"title: 'Record the widget decision'",
		"status: 'Accepted'",
		"date: '2026-08-16'",
		"## Context\n\nThe widget needs a home.",
		"## Decision\n\nPut it in the shed.",
		"## Alternatives considered\n\nThe garage was considered.",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
}

func TestADRRecordPlanWithProducingChange(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0001-first.md": lifecycleChange(1, "first", "proposed"),
		"docs/adrs/0001-one.md":             fixtureADR(1, "one"),
	}
	req := validADRRecordRequest()
	req.Change = &ADRProducingChange{ID: 1, Path: "docs/changes/active/0001-first.md", Version: blobV}
	plan, opRes := adrRecordPlanFor(t, files, baseADRRecordOp(req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// The plan is EXACTLY {new ADR, index, producing change}.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		adrPath("0002", "record-the-widget-decision"): transaction.MutationCreate,
		adrsIndexPath:                       transaction.MutationCreate,
		"docs/changes/active/0001-first.md": transaction.MutationReplace,
	})

	// The producing change gains the new ADR id in its typed adrs collection and
	// its artifact block lists the new ADR.
	change := planFileBytes(t, plan, "docs/changes/active/0001-first.md")
	if !strings.Contains(change, "adrs: [2]") {
		t.Errorf("producing change adrs not updated to [2]:\n%s", change)
	}
	if !strings.Contains(change, "updated: '2026-08-16'") {
		t.Errorf("producing change updated date not bumped:\n%s", change)
	}
	if !strings.Contains(change, "| ADRs |") || !strings.Contains(change, adrPath("0002", "record-the-widget-decision")) {
		t.Errorf("producing change artifact block missing the new ADR row:\n%s", change)
	}

	// The new ADR frontmatter records its producing change.
	rec := planFileBytes(t, plan, adrPath("0002", "record-the-widget-decision"))
	if !strings.Contains(rec, "change: 1") {
		t.Errorf("new ADR does not record producing change:\n%s", rec)
	}
}

func TestADRRecordPlanRefusesAbsentProducingChange(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-one.md": fixtureADR(1, "one"),
	}
	req := validADRRecordRequest()
	req.Change = &ADRProducingChange{ID: 42, Path: "docs/changes/active/0042-ghost.md", Version: blobV}
	plan, opRes := adrRecordPlanFor(t, files, baseADRRecordOp(req))
	if !opRes.Refused {
		t.Fatalf("absent producing change was not refused")
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", plan.Files)
	}
}

// assertCanonicalADRReceipt decodes and re-marshals the receipt, asserting its
// identity fields and its canonical, sorted-key compact form.
func assertCanonicalADRReceipt(t *testing.T, receipt []byte, id int, path string) {
	t.Helper()
	var rec adrRecordReceipt
	if err := json.Unmarshal(receipt, &rec); err != nil {
		t.Fatalf("receipt does not decode: %v", err)
	}
	if rec.ID != id || rec.Path != path || rec.Op != OperationADRRecord {
		t.Errorf("receipt = %+v", rec)
	}
	var generic any
	if err := json.Unmarshal(receipt, &generic); err != nil {
		t.Fatalf("receipt generic decode: %v", err)
	}
	remar := mustMarshal(t, generic)
	if string(remar) != string(receipt) {
		t.Errorf("receipt is not canonical:\n got %s\nwant %s", receipt, remar)
	}
}
