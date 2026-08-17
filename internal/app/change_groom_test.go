package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// groomableChange renders a canonical proposed change record eligible for
// grooming: proposed status, empty spec, trivial false, every relationship
// field present and empty, the empty docket:artifacts block, and the four
// authored proposal sections (including ## Open questions, so its removal is
// observable).
func groomableChange(id int, slug string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoaTest(id) + "\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A change'\n")
	b.WriteString("status: proposed\n")
	b.WriteString("priority: medium\n")
	b.WriteString("type: feat\n")
	b.WriteString("created: 2026-08-01\n")
	b.WriteString("updated: 2026-08-02\n")
	b.WriteString("depends_on: []\n")
	b.WriteString("stacked_on:\n")
	b.WriteString("related: []\n")
	b.WriteString("discovered_from: []\n")
	b.WriteString("adrs: []\n")
	b.WriteString("spec:\n")
	b.WriteString("plan:\n")
	b.WriteString("results:\n")
	b.WriteString("trivial: false\n")
	b.WriteString("---\n\n")
	b.WriteString("## Artifacts\n\n")
	b.WriteString("<!-- docket:artifacts:start (generated — do not hand-edit) -->\n")
	b.WriteString("<!-- docket:artifacts:end -->\n\n")
	b.WriteString("## Why\n\nOriginal why.\n\n")
	b.WriteString("## What changes\n\nOriginal what.\n\n")
	b.WriteString("## Out of scope\n\nOriginal out.\n\n")
	b.WriteString("## Open questions\n\nAn open question.\n")
	return b.String()
}

// groomPath is the active record path for a groomable change fixture.
func groomPath(id int, slug string) string {
	return "docs/changes/active/" + padID(id) + "-" + slug + ".md"
}

func padID(id int) string {
	s := itoaTest(id)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

// validGroomSpecRequest is a well-formed spec-outcome groom request against the
// groomable fixture at id 2 / slug add-a-widget.
func validGroomSpecRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID:     2,
		Path:         groomPath(2, "add-a-widget"),
		Version:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:      GroomSpec,
		SpecMarkdown: "# Design\n\nThe design body.\n",
		Sections: []SectionEditRequest{
			{Heading: "## Why", Intent: "replace", Markdown: "Refined why.\n"},
			{Heading: "## Open questions", Intent: "remove"},
		},
	}
}

// --- request-shape validation (no engine call) ----------------------------

func TestChangeGroomRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		code string
	}{
		{"non-positive change id", func(r *ChangeGroomRequest) { r.ChangeID = 0 }, "invalid-change_id"},
		{"empty path", func(r *ChangeGroomRequest) { r.Path = "" }, "empty-path"},
		{"empty version", func(r *ChangeGroomRequest) { r.Version = "" }, "empty-version"},
		{"unknown outcome", func(r *ChangeGroomRequest) { r.Outcome = "maybe" }, "invalid-outcome"},
		{"spec outcome empty markdown", func(r *ChangeGroomRequest) { r.SpecMarkdown = "" }, "empty-spec_markdown"},
		{"spec outcome unparseable markdown", func(r *ChangeGroomRequest) { r.SpecMarkdown = "---\nid: 1\n" }, "invalid-spec_markdown"},
		{"section unowned heading", func(r *ChangeGroomRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Nope", Intent: "replace", Markdown: "x\n"}}
		}, "invalid-section-heading"},
		{"section unknown intent", func(r *ChangeGroomRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "delete"}}
		}, "invalid-section-intent"},
		{"section non-replace with markdown", func(r *ChangeGroomRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "remove", Markdown: "x\n"}}
		}, "invalid-section-markdown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validGroomSpecRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeGroom(context.Background(), deps, "", req)

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

func TestChangeGroomTrivialRequiresRationale(t *testing.T) {
	req := ChangeGroomRequest{
		ChangeID: 2,
		Path:     groomPath(2, "add-a-widget"),
		Version:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:  GroomTrivial,
		// No section carrying an authored rationale.
	}
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", req)

	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a missing trivial rationale")
	}
	if !hasFindingCode(res.Findings, "missing-rationale") {
		t.Errorf("missing finding missing-rationale; got %v", res.Findings)
	}
}

func TestChangeGroomFencesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline", "github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", validGroomSpecRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a fenced board surface")
	}
}

// --- outcome mapping (engine reached; Discover over a real temp repo) ------

func TestChangeGroomAppliedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	specPath := "docs/superpowers/specs/2026-08-16-add-a-widget-design.md"
	receipt := mustMarshal(t, changeGroomReceipt{
		ID: 2, Op: OperationChangeGroom, Outcome: string(GroomSpec), SpecPath: specPath,
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, repoDir, validGroomSpecRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.ID != 2 || res.SpecPath != specPath {
		t.Errorf("identity from receipt = (%d, %q)", res.ID, res.SpecPath)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationChangeGroom {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/main" {
		t.Errorf("target ref = %q, want refs/heads/main", req.TargetRef)
	}
	if req.Idempotency != nil {
		t.Errorf("groom is non-allocating; it must carry no idempotency key, got %+v", req.Idempotency)
	}
	if len(req.Expected) != 1 {
		t.Fatalf("expected %d entity expectations, want 1", len(req.Expected))
	}
	exp := req.Expected[0]
	if string(exp.Path) != groomPath(2, "add-a-widget") {
		t.Errorf("expectation path = %q", exp.Path)
	}
	if exp.Version.Kind != transaction.VersionBlob ||
		string(exp.Version.ObjectID) != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("expectation version = %+v", exp.Version)
	}
}

func TestChangeGroomContendedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, repoDir, validGroomSpecRequest())

	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

func TestChangeGroomRefusedMapsInvalidState(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings:    nil,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, repoDir, validGroomSpecRequest())

	if res.Result != ResultInvalidState {
		t.Fatalf("refused disposition mapped to %q, want invalid-state", res.Result)
	}
}

// --- plan closure ----------------------------------------------------------

func groomPlanFor(t *testing.T, files map[string]string, op changeGroomOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func baseGroomOp(surfaces []string, req ChangeGroomRequest) changeGroomOp {
	return changeGroomOp{
		req:        req,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

func groomedRecordBytes(t *testing.T, plan transaction.MutationPlan, path string) []byte {
	t.Helper()
	for _, f := range plan.Files {
		if string(f.Path) == path {
			return f.Bytes
		}
	}
	t.Fatalf("record %q not planned; files: %v", path, planPaths(plan))
	return nil
}

func planPaths(plan transaction.MutationPlan) []string {
	var out []string
	for _, f := range plan.Files {
		out = append(out, string(f.Path))
	}
	return out
}

func TestChangeGroomPlanSpecOutcomeFileSet(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, validGroomSpecRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	specPath := "docs/superpowers/specs/2026-08-16-add-a-widget-design.md"
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		specPath:                     transaction.MutationCreate,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})

	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "spec: '"+specPath+"'") {
		t.Errorf("spec field not set to the spec path:\n%s", rec)
	}
	if !strings.Contains(rec, "| Spec |") || !strings.Contains(rec, specPath) {
		t.Errorf("artifact block missing the Spec row:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	if strings.Contains(rec, "Original why.") || !strings.Contains(rec, "Refined why.") {
		t.Errorf("## Why section not replaced:\n%s", rec)
	}
	if strings.Contains(rec, "## Open questions") {
		t.Errorf("## Open questions not removed:\n%s", rec)
	}

	spec := string(groomedRecordBytes(t, plan, specPath))
	if !strings.Contains(spec, "docket:backlink:start") {
		t.Errorf("spec file missing backlink block:\n%s", spec)
	}
	if !strings.Contains(spec, "The design body.") {
		t.Errorf("spec file missing the submitted markdown:\n%s", spec)
	}
	if !strings.Contains(spec, groomPath(2, "add-a-widget")) {
		t.Errorf("spec backlink does not target the change record path:\n%s", spec)
	}
}

func TestChangeGroomPlanTrivialOutcomeFileSet(t *testing.T) {
	req := ChangeGroomRequest{
		ChangeID: 2,
		Path:     groomPath(2, "add-a-widget"),
		Version:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:  GroomTrivial,
		Sections: []SectionEditRequest{
			{Heading: "## Why", Intent: "replace", Markdown: "Too small to design.\n"},
		},
	}
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "trivial: true") {
		t.Errorf("trivial not set true:\n%s", rec)
	}
	if strings.Contains(rec, "spec: '") {
		t.Errorf("trivial groom must not set a spec:\n%s", rec)
	}
}

func TestChangeGroomPlanRefusesNonGroomable(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(string) string
	}{
		{"already has spec", func(s string) string { return strings.Replace(s, "spec:\n", "spec: 'docs/x.md'\n", 1) }},
		{"already trivial", func(s string) string { return strings.Replace(s, "trivial: false", "trivial: true", 1) }},
		{"not proposed", func(s string) string {
			return strings.Replace(s, "status: proposed\n", "status: blocked\nblocked_by: 'waiting on infra'\n", 1)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{
				groomPath(2, "add-a-widget"): c.mutate(groomableChange(2, "add-a-widget")),
			}
			_, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validGroomSpecRequest()))
			if !opRes.Refused {
				t.Fatalf("%s: expected a refusal, got none", c.name)
			}
		})
	}
}

func TestChangeGroomPlanRefusesExistingSpecPath(t *testing.T) {
	specPath := "docs/superpowers/specs/2026-08-16-add-a-widget-design.md"
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		specPath:                     "# already here\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validGroomSpecRequest()))
	if !opRes.Refused {
		t.Fatalf("expected a refusal on a pre-existing spec path")
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", planPaths(plan))
	}
}

func TestChangeGroomPlanSourcePreservation(t *testing.T) {
	// A groomable record carrying an unknown frontmatter field and an unknown
	// authored body section: both must survive byte-identically.
	src := groomableChange(2, "add-a-widget")
	src = strings.Replace(src, "trivial: false\n", "trivial: false\ncustom_field: 'unknown survives'\n", 1)
	src += "\n## Custom notes\n\nUnknown section survives.\n"

	files := map[string]string{groomPath(2, "add-a-widget"): src}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validGroomSpecRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "custom_field: 'unknown survives'") {
		t.Errorf("unknown frontmatter field did not survive:\n%s", rec)
	}
	if !strings.Contains(rec, "## Custom notes\n\nUnknown section survives.\n") {
		t.Errorf("unknown body section did not survive byte-identically:\n%s", rec)
	}
}

func TestChangeGroomPlanRelationshipsWritten(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"):        groomableChange(2, "add-a-widget"),
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	req := validGroomSpecRequest()
	req.DependsOn = []int{1}
	req.ADRs = []int{}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "depends_on: [1]") {
		t.Errorf("depends_on not written as the complete desired value:\n%s", rec)
	}
}

// TestChangeGroomPlanToleratesMissingUpdatedField pins that a groom over a record
// lacking the updated: field inserts it rather than internal-erroring, matching
// the ADR ops' upsert of the same field (a bare SetField returns
// KindMissingPatchTarget on an absent target).
func TestChangeGroomPlanToleratesMissingUpdatedField(t *testing.T) {
	src := groomableChange(2, "add-a-widget")
	src = strings.Replace(src, "updated: 2026-08-02\n", "", 1)
	if strings.Contains(src, "updated:") {
		t.Fatalf("fixture still carries an updated field:\n%s", src)
	}

	files := map[string]string{groomPath(2, "add-a-widget"): src}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validGroomSpecRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not inserted from the clock on a record lacking it:\n%s", rec)
	}
}
