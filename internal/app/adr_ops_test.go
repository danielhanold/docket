package app

import (
	"context"
	"encoding/json"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
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

// --- adr supersede / reverse ----------------------------------------------

// fixtureADRWithStatus renders a minimal well-formed ADR record carrying an
// arbitrary raw status value (unquoted), for driving the not-Accepted refusal.
func fixtureADRWithStatus(id int, slug, status string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoaTest(id) + "\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A decision'\n")
	b.WriteString("status: " + status + "\n")
	b.WriteString("date: 2026-08-01\n")
	b.WriteString("---\n\n## Decision\n\nBody.\n")
	return b.String()
}

func validADRReplaceRequest() ADRReplaceRequest {
	return ADRReplaceRequest{
		RequestID: "adr-replace-0001",
		Target:    ADRTarget{ID: 1, Path: "docs/adrs/0001-one.md", Version: blobV},
		Successor: ADRRecordRequest{
			// RequestID intentionally empty: the outer key governs, this is ignored.
			Title:        "Supersede the widget decision",
			Context:      "The widget moved.\n",
			Decision:     "Put it in the garage.\n",
			Consequences: "The shed is empty now.\n",
			Alternatives: "Leaving it was considered.\n",
		},
	}
}

func baseADRReplaceOp(opKey string, reverses bool, req ADRReplaceRequest) adrReplaceOp {
	return adrReplaceOp{
		opKey:    opKey,
		req:      req,
		reverses: reverses,
		eff:      planningTestConfig([]string{}),
		slug:     slugifyTitle(req.Successor.Title),
		clock:    testClock(),
		link:     render.LinkContext{MetadataBranch: "main"},
		adrsDir:  "docs/adrs",
	}
}

func adrReplacePlanFor(t *testing.T, files map[string]string, op adrReplaceOp) (transaction.MutationPlan, transaction.OperationResult) {
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

// --- request-shape validation (no engine call) -----------------------------

func TestADRSupersedeRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ADRReplaceRequest)
		code string
	}{
		{"short outer request id", func(r *ADRReplaceRequest) { r.RequestID = "short" }, "invalid-request-id"},
		{"non-positive target id", func(r *ADRReplaceRequest) { r.Target.ID = 0 }, "invalid-target-id"},
		{"empty target path", func(r *ADRReplaceRequest) { r.Target.Path = "" }, "empty-target-path"},
		{"empty target version", func(r *ADRReplaceRequest) { r.Target.Version = "" }, "empty-target-version"},
		{"empty successor title", func(r *ADRReplaceRequest) { r.Successor.Title = "" }, "empty-title"},
		{"empty successor decision", func(r *ADRReplaceRequest) { r.Successor.Decision = "" }, "empty-decision"},
		{"duplicate successor relates_to", func(r *ADRReplaceRequest) { r.Successor.RelatesTo = []int{3, 3} }, "duplicate-relates_to"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validADRReplaceRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ADRSupersede(context.Background(), deps, "", req)

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

// --- outcome mapping (engine reached) --------------------------------------

// --- replace plan closure --------------------------------------------------

func TestADRSupersedePlanFileSet(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-one.md": fixtureADR(1, "one"),
	}
	plan, opRes := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRSupersede, false, validADRReplaceRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// max(1)+1 = 2; plan is EXACTLY {new ADR, old ADR (status flip), index}.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		adrPath("0002", "supersede-the-widget-decision"): transaction.MutationCreate,
		"docs/adrs/0001-one.md":                          transaction.MutationReplace,
		adrsIndexPath:                                    transaction.MutationCreate,
	})
	// The receipt names the new ADR id, the supersede op key, and the new path,
	// in canonical sorted-key compact form.
	var rec adrRecordReceipt
	if err := json.Unmarshal(plan.Receipt, &rec); err != nil {
		t.Fatalf("receipt decode: %v", err)
	}
	if rec.ID != 2 || rec.Op != OperationADRSupersede || rec.Path != adrPath("0002", "supersede-the-widget-decision") {
		t.Errorf("receipt = %+v", rec)
	}
	var generic any
	if err := json.Unmarshal(plan.Receipt, &generic); err != nil {
		t.Fatalf("receipt generic decode: %v", err)
	}
	if remar := mustMarshal(t, generic); string(remar) != string(plan.Receipt) {
		t.Errorf("receipt not canonical:\n got %s\nwant %s", plan.Receipt, remar)
	}
}

func TestADRSupersedeOldADRStatusOnlyFlip(t *testing.T) {
	src := fixtureADR(1, "one")
	files := map[string]string{"docs/adrs/0001-one.md": src}
	plan, opRes := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRSupersede, false, validADRReplaceRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	old := planFileBytes(t, plan, "docs/adrs/0001-one.md")
	// The frozen body is preserved: the old record differs from source ONLY in the
	// status value.
	want := strings.Replace(src, "status: Accepted", "status: 'Superseded by ADR-0002'", 1)
	if old != want {
		t.Errorf("old ADR changed more than its status value:\n got %q\nwant %q", old, want)
	}
}

func TestADRSupersedeNewADRRecordsSupersedesEdge(t *testing.T) {
	files := map[string]string{"docs/adrs/0001-one.md": fixtureADR(1, "one")}
	plan, _ := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRSupersede, false, validADRReplaceRequest()))
	rec := planFileBytes(t, plan, adrPath("0002", "supersede-the-widget-decision"))
	for _, want := range []string{
		"id: 2",
		"status: 'Accepted'",
		"supersedes: [1]",
		"reverses: []",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("new ADR missing %q:\n%s", want, rec)
		}
	}
}

func TestADRReversePlanFlipsAndRecordsEdge(t *testing.T) {
	src := fixtureADR(1, "one")
	files := map[string]string{"docs/adrs/0001-one.md": src}
	plan, opRes := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRReverse, true, validADRReplaceRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	old := planFileBytes(t, plan, "docs/adrs/0001-one.md")
	want := strings.Replace(src, "status: Accepted", "status: 'Reversed by ADR-0002'", 1)
	if old != want {
		t.Errorf("reversed old ADR changed more than its status value:\n got %q\nwant %q", old, want)
	}
	rec := planFileBytes(t, plan, adrPath("0002", "supersede-the-widget-decision"))
	if !strings.Contains(rec, "reverses: [1]") || !strings.Contains(rec, "supersedes: []") {
		t.Errorf("reversing successor missing reverses edge / carries a supersedes edge:\n%s", rec)
	}
}

func TestADRReversePlanFileSet(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-one.md": fixtureADR(1, "one"),
	}
	plan, opRes := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRReverse, true, validADRReplaceRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// Atomic index ownership: the reverse transaction lands the new ADR, the old
	// ADR's status flip, AND the re-rendered index in ONE plan (commit) — never a
	// caller-owned follow-up render. max(1)+1 = 2.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		adrPath("0002", "supersede-the-widget-decision"): transaction.MutationCreate,
		"docs/adrs/0001-one.md":                          transaction.MutationReplace,
		adrsIndexPath:                                    transaction.MutationCreate,
	})
	// The re-rendered index in the same plan reflects the reversal and the new ADR.
	idx := planFileBytes(t, plan, adrsIndexPath)
	if !strings.Contains(idx, "ADR-0002") {
		t.Errorf("index does not reflect the new ADR-0002:\n%s", idx)
	}
	if !strings.Contains(idx, "Reversed by ADR-0002") {
		t.Errorf("index does not show the target's reversed status:\n%s", idx)
	}
}

func TestADRSupersedeIndexReflectsFlipAndNewADR(t *testing.T) {
	files := map[string]string{"docs/adrs/0001-one.md": fixtureADR(1, "one")}
	plan, _ := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRSupersede, false, validADRReplaceRequest()))
	idx := planFileBytes(t, plan, adrsIndexPath)
	if !strings.Contains(idx, "Superseded / Reversed") {
		t.Errorf("index missing the Superseded / Reversed group:\n%s", idx)
	}
	if !strings.Contains(idx, "ADR-0002") {
		t.Errorf("index does not reflect the new ADR-0002:\n%s", idx)
	}
	if !strings.Contains(idx, "Superseded by ADR-0002") {
		t.Errorf("index does not show the target's flipped status:\n%s", idx)
	}
}

func TestADRSupersedePlanRefusesNonAcceptedTarget(t *testing.T) {
	files := map[string]string{
		"docs/adrs/0001-one.md": fixtureADRWithStatus(1, "one", "Deprecated"),
	}
	plan, opRes := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRSupersede, false, validADRReplaceRequest()))
	if !opRes.Refused {
		t.Fatalf("superseding a non-Accepted target was not refused")
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", plan.Files)
	}
	if !hasDomainFindingCode(opRes.Findings, adrNotAcceptedReason) {
		t.Errorf("missing %q finding: %v", adrNotAcceptedReason, opRes.Findings)
	}
}

func TestADRSupersedePlanRefusesDanglingSuccessorRelatesTo(t *testing.T) {
	files := map[string]string{"docs/adrs/0001-one.md": fixtureADR(1, "one")}
	req := validADRReplaceRequest()
	req.Successor.RelatesTo = []int{999}
	plan, opRes := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRSupersede, false, req))
	if !opRes.Refused {
		t.Fatalf("dangling successor relates_to was not refused")
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", plan.Files)
	}
	if !hasDomainFindingCode(opRes.Findings, "adr-dangling-reference") {
		t.Errorf("missing adr-dangling-reference finding: %v", opRes.Findings)
	}
}

func TestADRSupersedePlanWithProducingChange(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0003-third.md": lifecycleChange(3, "third", "proposed"),
		"docs/adrs/0001-one.md":             fixtureADR(1, "one"),
	}
	req := validADRReplaceRequest()
	req.Successor.Change = &ADRProducingChange{ID: 3, Path: "docs/changes/active/0003-third.md", Version: blobV}
	plan, opRes := adrReplacePlanFor(t, files, baseADRReplaceOp(OperationADRSupersede, false, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// The plan is EXACTLY {new ADR, old ADR, index, producing change}.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		adrPath("0002", "supersede-the-widget-decision"): transaction.MutationCreate,
		"docs/adrs/0001-one.md":                          transaction.MutationReplace,
		adrsIndexPath:                                    transaction.MutationCreate,
		"docs/changes/active/0003-third.md":              transaction.MutationReplace,
	})
	change := planFileBytes(t, plan, "docs/changes/active/0003-third.md")
	if !strings.Contains(change, "adrs: [2]") {
		t.Errorf("producing change adrs not updated to [2]:\n%s", change)
	}
	if !strings.Contains(change, "| ADRs |") || !strings.Contains(change, adrPath("0002", "supersede-the-widget-decision")) {
		t.Errorf("producing change artifact block missing the new ADR row:\n%s", change)
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
