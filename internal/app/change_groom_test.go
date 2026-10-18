package app

import (
	"context"
	"encoding/json"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
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

// groomBacklinkedSpecMarkdown is a spec body that still carries a
// docket:backlink managed block — the file as read, not the body the groom
// operation expects.
const groomBacklinkedSpecMarkdown = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
	"> old backlink\n" +
	"<!-- docket:backlink:end -->\n\n# Design\n\nThe design body.\n"

// validGroomSpecRequest is a well-formed spec-outcome groom request against the
// groomable fixture at id 2 / slug add-a-widget.
func validGroomSpecRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID:     2,
		Path:         groomPath(2, "add-a-widget"),
		Revision:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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
		{"empty revision", func(r *ChangeGroomRequest) { r.Revision = "" }, "empty-revision"},
		{"unknown outcome", func(r *ChangeGroomRequest) { r.Outcome = "maybe" }, "invalid-outcome"},
		{"spec outcome empty markdown", func(r *ChangeGroomRequest) { r.SpecMarkdown = "" }, "empty-spec_markdown"},
		{"spec outcome unparseable markdown", func(r *ChangeGroomRequest) { r.SpecMarkdown = "---\nid: 1\n" }, "invalid-spec_markdown"},
		// The writer prepends the backlink block itself; an authored one would
		// commit a duplicate marker pair.
		{"spec outcome markdown carrying a backlink block", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = groomBacklinkedSpecMarkdown
		}, "invalid-spec_markdown"},
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
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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

func TestChangeGroomRefusesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline", "github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", validGroomSpecRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a refused github board surface")
	}
}

// --- outcome mapping (engine reached; Discover over a real temp repo) ------

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
	// The spec backlink links the record relatively from the spec's directory.
	if !strings.Contains(spec, "](../../changes/active/0002-add-a-widget.md)**") {
		t.Errorf("spec backlink does not target the change record path:\n%s", spec)
	}
}

func TestChangeGroomPlanTrivialOutcomeFileSet(t *testing.T) {
	req := ChangeGroomRequest{
		ChangeID: 2,
		Path:     groomPath(2, "add-a-widget"),
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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

// revisableChange renders a proposed change already groomed to a spec: the
// groomable fixture with spec: linked to specPath.
func revisableChange(id int, slug, specPath string) string {
	return strings.Replace(groomableChange(id, slug), "spec:\n", "spec: '"+specPath+"'\n", 1)
}

// trivialChange renders a proposed change already groomed by trivial verdict.
func trivialChange(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "trivial: false", "trivial: true", 1)
}

// fakeTreeBlobID is the uniform blob id newFakeTree reports for every path, so
// it is the spec_revision that matches the linked spec on a fake tree.
const fakeTreeBlobID = "a"

// validReviseRequest is a well-formed revise request (sections + spec body)
// against the revisable fixture at id 2 / slug add-a-widget.
func validReviseRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID:     2,
		Path:         groomPath(2, "add-a-widget"),
		Revision:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:      GroomRevise,
		SpecMarkdown: "# Design\n\nThe revised design body.\n",
		SpecRevision: fakeTreeBlobID,
		Sections: []SectionEditRequest{
			{Heading: "## What changes", Intent: "replace", Markdown: "Narrowed what.\n"},
		},
	}
}

func TestChangeGroomReviseShapeValidation(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		code string // "" means the request must pass shape validation
	}{
		{"valid revise passes", func(r *ChangeGroomRequest) {}, ""},
		{"sections-only revise passes", func(r *ChangeGroomRequest) { r.SpecMarkdown, r.SpecRevision = "", "" }, ""},
		{"spec-only revise passes", func(r *ChangeGroomRequest) { r.Sections = nil }, ""},
		{"empty revise refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.Sections = nil
		}, "empty-revise"},
		{"all-preserve revise refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "preserve"}}
		}, "empty-revise"},
		{"relationships-only revise passes", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecRevision = "", ""
			r.Sections = nil
			r.DependsOn = []int{1}
		}, ""},
		{"unparseable revise spec_markdown refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = "---\nid: 1\n"
		}, "invalid-spec_markdown"},
		// An author resubmitting the spec file as read keeps its backlink block;
		// the revise re-renders that block itself, so the resubmission is refused
		// rather than committed with a duplicate marker pair.
		{"revise spec_markdown carrying a backlink block refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = groomBacklinkedSpecMarkdown
		}, "invalid-spec_markdown"},
		// A spec-body revise overwrites the spec file, so it must pin its revision.
		{"spec revise without spec_revision refused", func(r *ChangeGroomRequest) {
			r.SpecRevision = ""
		}, "empty-spec_revision"},
		{"sections-only revise without spec_revision passes", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecRevision = "", ""
		}, ""},
		// spec_revision pins only a spec-body revise; anywhere else it would be
		// silently unchecked, so it is refused.
		{"sections-only revise with spec_revision refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
		}, "invalid-spec_revision"},
		{"spec outcome with spec_revision refused", func(r *ChangeGroomRequest) {
			r.Outcome = GroomSpec
		}, "invalid-spec_revision"},
		// abstain and re-enable own the abstain marker; a revise that replaced or
		// removed it would desynchronize it from auto_groomable:.
		{"revise replacing ## Auto-groom blocked refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecRevision = "", ""
			r.Sections = []SectionEditRequest{{Heading: "## Auto-groom blocked", Intent: "replace", Markdown: "x\n"}}
		}, "invalid-section-heading"},
		{"revise removing ## Auto-groom blocked refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecRevision = "", ""
			r.Sections = []SectionEditRequest{{Heading: "## Auto-groom blocked", Intent: "remove"}}
		}, "invalid-section-heading"},
		{"spec-body revise also naming ## Auto-groom blocked refused", func(r *ChangeGroomRequest) {
			r.Sections = append(r.Sections, SectionEditRequest{Heading: "## Auto-groom blocked", Intent: "remove"})
		}, "invalid-section-heading"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validReviseRequest()
			c.mut(&req)
			findings := validateChangeGroomShape(req)
			if c.code == "" {
				if len(findings) != 0 {
					t.Fatalf("unexpected shape findings: %v", findings)
				}
				return
			}
			if !hasFindingCode(findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, findings)
			}
		})
	}
}

// TestChangeGroomReviseAutoGroomBlockedRefusedWithoutEngineCall pins that the
// guard fires at the real entry point before any engine call, so nothing is
// written whatever state the target change is in (stub or groomed).
func TestChangeGroomReviseAutoGroomBlockedRefusedWithoutEngineCall(t *testing.T) {
	req := validReviseRequest()
	req.SpecMarkdown, req.SpecRevision = "", ""
	req.Sections = []SectionEditRequest{{Heading: "## Auto-groom blocked", Intent: "remove"}}
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
	if !hasFindingCode(res.Findings, "invalid-section-heading") {
		t.Errorf("missing invalid-section-heading; got %v", res.Findings)
	}
}

func TestChangeGroomEmptyReviseRefusedWithoutEngineCall(t *testing.T) {
	req := validReviseRequest()
	req.SpecMarkdown = ""
	req.Sections = nil
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", req)

	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on an empty revise, want 0", len(engine.calls))
	}
	if !hasFindingCode(res.Findings, "empty-revise") {
		t.Errorf("missing finding empty-revise; got %v", res.Findings)
	}
}

// TestChangeGroomSpecReviseWithoutSpecRevisionRefusedWithoutEngineCall pins the
// review finding: a spec-body revise that does not pin the spec file's blob
// revision is a shape refusal, never an unpinned whole-body replace.
func TestChangeGroomSpecReviseWithoutSpecRevisionRefusedWithoutEngineCall(t *testing.T) {
	req := validReviseRequest()
	req.SpecRevision = ""
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", req)

	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on an unpinned spec revise, want 0", len(engine.calls))
	}
	if !hasFindingCode(res.Findings, "empty-spec_revision") {
		t.Errorf("missing finding empty-spec_revision; got %v", res.Findings)
	}
}

// TestChangeGroomSpecRevisionMismatchMapsToContended pins the result mapping: a
// plan refused with spec-revision-mismatch is the spec analogue of a stale record
// pin, so the caller sees contended (re-read and retry), not invalid-state.
func TestChangeGroomSpecRevisionMismatchMapsToContended(t *testing.T) {
	_, opRes := groomPlanFor(t, reviseFixtureFiles(), baseGroomOp([]string{}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("precondition: a matching spec_revision must plan; got %v", opRes.Findings)
	}
	res := changeGroomResultFromOutcome(transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings: []domain.Finding{{
			Code: "spec-revision-mismatch", Severity: domain.SeverityError,
			Entity: domain.EntityRef{Kind: domain.EntityChange},
		}},
	}, nil)
	if res.Result != ResultContended {
		t.Errorf("spec-revision-mismatch refusal = %q, want contended", res.Result)
	}
	other := changeGroomResultFromOutcome(transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings:    []domain.Finding{{Code: "not-revisable", Severity: domain.SeverityError}},
	}, nil)
	if other.Result != ResultInvalidState {
		t.Errorf("not-revisable refusal = %q, want invalid-state", other.Result)
	}
}

const reviseSpecPath = "docs/superpowers/specs/2026-08-01-add-a-widget-design.md"

// reviseFixtureFiles is the fake tree for a revisable spec'd change: the
// record with spec: linked, and the spec file itself with a backlink block.
func reviseFixtureFiles() map[string]string {
	return map[string]string{
		groomPath(2, "add-a-widget"): revisableChange(2, "add-a-widget", reviseSpecPath),
		reviseSpecPath: "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
			"> old backlink\n" +
			"<!-- docket:backlink:end -->\n\n# Design\n\nThe original design body.\n",
	}
}

// reviseFixtureAtStatus is reviseFixtureFiles with the record moved off
// proposed to status at recPath (a final status lives under archive/), extra
// carrying whatever frontmatter that status requires to load coherently —
// spec acceptance item 7's non-proposed refusal rows.
func reviseFixtureAtStatus(recPath, status, extra string) map[string]string {
	files := reviseFixtureFiles()
	src := groomPath(2, "add-a-widget")
	rec := strings.Replace(files[src], "status: proposed\n", "status: "+status+"\n"+extra, 1)
	if status == "implemented" {
		rec = strings.Replace(rec, "plan:\n", "plan: 'docs/superpowers/plans/2026-08-10-add-a-widget.md'\n", 1)
	}
	delete(files, src)
	files[recPath] = rec
	return files
}

const (
	reviseArchivePath       = "docs/changes/archive/2026-08-10-0002-add-a-widget.md"
	reviseClaimFields       = "branch: 'feat/add-a-widget'\nclaimed_at: '2026-08-10T00:00:00Z'\n"
	reviseImplementedFields = reviseClaimFields + "pr: 'https://github.com/o/r/pull/7'\nreconciled: true\n"
)

// assertGroomReceiptSpecPath decodes the plan's canonical receipt and pins its
// spec_path field.
func assertGroomReceiptSpecPath(t *testing.T, plan transaction.MutationPlan, want string) {
	t.Helper()
	var rec changeGroomReceipt
	if err := json.Unmarshal(plan.Receipt, &rec); err != nil {
		t.Fatalf("decoding receipt %s: %v", plan.Receipt, err)
	}
	if rec.SpecPath != want {
		t.Errorf("receipt spec_path = %q, want %q (receipt %s)", rec.SpecPath, want, plan.Receipt)
	}
}

func TestChangeGroomPlanReviseSectionsOnly(t *testing.T) {
	files := reviseFixtureFiles()
	files["docs/changes/BOARD.md"] = "# Backlog\n\nold\n"
	req := validReviseRequest()
	req.SpecMarkdown = "" // sections only
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// Spec item 1: record + board replaced; the spec file is NOT in the plan,
	// so it stays byte-identical by construction.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if strings.Contains(rec, "Original what.") || !strings.Contains(rec, "Narrowed what.") {
		t.Errorf("## What changes section not replaced:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	// Spec item 9: revise never flips the groomed-outcome fields.
	if !strings.Contains(rec, "spec: '"+reviseSpecPath+"'") {
		t.Errorf("spec field changed under revise:\n%s", rec)
	}
	if !strings.Contains(rec, "trivial: false") {
		t.Errorf("trivial field changed under revise:\n%s", rec)
	}
	// A sections-only revise replaced no spec body, so its receipt names none.
	assertGroomReceiptSpecPath(t, plan, "")
}

func TestChangeGroomPlanReviseSpecBodyOnly(t *testing.T) {
	files := reviseFixtureFiles()
	before := files[groomPath(2, "add-a-widget")]
	req := validReviseRequest()
	req.Sections = nil // spec body only
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// Spec item 2: the spec file is REPLACED at the existing path, never created
	// at a new dated path.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		reviseSpecPath:               transaction.MutationReplace,
	})
	spec := string(groomedRecordBytes(t, plan, reviseSpecPath))
	if !strings.Contains(spec, "docket:backlink:start") {
		t.Errorf("revised spec file missing backlink block:\n%s", spec)
	}
	if !strings.Contains(spec, "The revised design body.") || strings.Contains(spec, "The original design body.") {
		t.Errorf("spec body not replaced:\n%s", spec)
	}
	// Spec item 2: the record's sections are byte-identical apart from
	// updated:. The docket:artifacts block legitimately re-renders (the same
	// call every groom outcome makes — the empty fixture block gains a Spec
	// row), so compare the authored body AFTER the artifacts block, plus the
	// frontmatter fields, rather than the whole file.
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	bodyAfterArtifacts := func(s string) string {
		i := strings.Index(s, "docket:artifacts:end")
		if i < 0 {
			t.Fatalf("record lacks the artifacts end marker:\n%s", s)
		}
		return s[i:]
	}
	if got, want := bodyAfterArtifacts(rec), bodyAfterArtifacts(before); got != want {
		t.Errorf("authored body changed under a spec-only revise:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped:\n%s", rec)
	}
	if !strings.Contains(rec, "spec: '"+reviseSpecPath+"'") || !strings.Contains(rec, "trivial: false") {
		t.Errorf("groomed-outcome fields changed under a spec-only revise:\n%s", rec)
	}
	// The receipt names the existing linked spec path the revise replaced.
	assertGroomReceiptSpecPath(t, plan, reviseSpecPath)
}

func TestChangeGroomPlanReviseBoth(t *testing.T) {
	// Spec item 3: both edits land in one plan.
	plan, opRes := groomPlanFor(t, reviseFixtureFiles(), baseGroomOp([]string{}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		reviseSpecPath:               transaction.MutationReplace,
	})
}

func TestChangeGroomPlanReviseTrivialRationale(t *testing.T) {
	// Spec item 4: sections-only revise of a trivial-verdicted change.
	files := map[string]string{
		groomPath(2, "add-a-widget"): trivialChange(2, "add-a-widget"),
	}
	req := validReviseRequest()
	// A trivial change links no spec, so there is no spec file to pin.
	req.SpecMarkdown, req.SpecRevision = "", ""
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "trivial: true") {
		t.Errorf("trivial verdict lost under revise:\n%s", rec)
	}
	if strings.Contains(rec, "spec: '") {
		t.Errorf("revise of a trivial change wrote a spec link:\n%s", rec)
	}
}

func TestChangeGroomPlanReviseRefusals(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		mut   func(*ChangeGroomRequest)
		code  string
	}{
		// Spec item 5: spec_markdown against a trivial-only (no-spec) change.
		{"spec-not-linked", map[string]string{
			groomPath(2, "add-a-widget"): trivialChange(2, "add-a-widget"),
		}, func(r *ChangeGroomRequest) {}, "spec-not-linked"},
		// A needs-grooming stub is revisable, but it links no spec, so a
		// spec-body revise of it refuses before any mutation is assembled.
		{"spec-not-linked needs-grooming", map[string]string{
			groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		}, func(r *ChangeGroomRequest) {}, "spec-not-linked"},
		// Spec item 7: a non-proposed change.
		{"not-revisable blocked", map[string]string{
			groomPath(2, "add-a-widget"): strings.Replace(
				revisableChange(2, "add-a-widget", reviseSpecPath),
				"status: proposed\n", "status: blocked\nblocked_by: 'waiting'\n", 1),
		}, func(r *ChangeGroomRequest) {}, "not-revisable"},
		{"not-revisable in-progress", reviseFixtureAtStatus(groomPath(2, "add-a-widget"), "in-progress", reviseClaimFields),
			func(r *ChangeGroomRequest) {}, "not-revisable"},
		{"not-revisable deferred", reviseFixtureAtStatus(groomPath(2, "add-a-widget"), "deferred", ""),
			func(r *ChangeGroomRequest) {}, "not-revisable"},
		{"not-revisable implemented", reviseFixtureAtStatus(groomPath(2, "add-a-widget"), "implemented", reviseImplementedFields),
			func(r *ChangeGroomRequest) {}, "not-revisable"},
		{"not-revisable done", reviseFixtureAtStatus(reviseArchivePath, "done", ""),
			func(r *ChangeGroomRequest) { r.Path = reviseArchivePath }, "not-revisable"},
		{"not-revisable killed", reviseFixtureAtStatus(reviseArchivePath, "killed", ""),
			func(r *ChangeGroomRequest) { r.Path = reviseArchivePath }, "not-revisable"},
		// Review Focus 3: dangling spec link — spec: names a path absent from
		// the tree; never silently mint a file.
		{"spec-file-missing", map[string]string{
			groomPath(2, "add-a-widget"): revisableChange(2, "add-a-widget", reviseSpecPath),
		}, func(r *ChangeGroomRequest) {}, "spec-file-missing"},
		// The spec file is pinned at the path the record links: a stale
		// spec_revision refuses rather than overwriting a newer spec body.
		{"spec-revision-mismatch", reviseFixtureFiles(), func(r *ChangeGroomRequest) {
			r.SpecRevision = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}, "spec-revision-mismatch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validReviseRequest()
			c.mut(&req)
			plan, opRes := groomPlanFor(t, c.files, baseGroomOp([]string{}, req))
			if !opRes.Refused {
				t.Fatalf("expected a refusal, got plan files %v", planPaths(plan))
			}
			found := false
			for _, f := range opRes.Findings {
				if f.Code == c.code {
					found = true
				}
			}
			if !found {
				t.Errorf("missing refusal code %q; got %v", c.code, opRes.Findings)
			}
			if len(plan.Files) != 0 {
				t.Errorf("refused plan still carries files: %v", planPaths(plan))
			}
		})
	}
}

// optedOutStub is the groomable fixture with an explicit auto_groomable: false,
// so a revise that wrote the flag (either way) is observable.
func optedOutStub(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "trivial: false\n", "trivial: false\nauto_groomable: false\n", 1)
}

// stubSectionReviseRequest is a sections-only revise of the stub at id 2.
func stubSectionReviseRequest() ChangeGroomRequest {
	r := validReviseRequest()
	r.SpecMarkdown, r.SpecRevision = "", ""
	r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "Sharpened why.\n"}}
	return r
}

// TestChangeGroomPlanReviseNeedsGroomingStub pins that revise edits a
// needs-grooming stub's owned sections and leaves its groom state alone: no
// spec: written, trivial: still false, auto_groomable: untouched, a fresh
// updated:, the artifacts block intact, and the board row still needs-grooming.
func TestChangeGroomPlanReviseNeedsGroomingStub(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): optedOutStub(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, stubSectionReviseRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if strings.Contains(rec, "Original why.") || !strings.Contains(rec, "Sharpened why.") {
		t.Errorf("## Why section not replaced:\n%s", rec)
	}
	if strings.Contains(rec, "spec: '") {
		t.Errorf("revise of a stub wrote a spec link:\n%s", rec)
	}
	if !strings.Contains(rec, "trivial: false") || strings.Contains(rec, "trivial: true") {
		t.Errorf("revise of a stub changed trivial:\n%s", rec)
	}
	if !strings.Contains(rec, "auto_groomable: false") || strings.Contains(rec, "auto_groomable: true") {
		t.Errorf("revise of a stub changed auto_groomable:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	if strings.Count(rec, "docket:artifacts:start") != 1 || strings.Count(rec, "docket:artifacts:end") != 1 {
		t.Errorf("artifacts block not intact after the re-render:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "needs-grooming") || strings.Contains(board, "build-ready") {
		t.Errorf("board row is no longer needs-grooming:\n%s", board)
	}
	var receipt changeGroomReceipt
	if err := json.Unmarshal(plan.Receipt, &receipt); err != nil || receipt.Outcome != "revise" {
		t.Errorf("receipt = %s (%v), want outcome revise", plan.Receipt, err)
	}
	assertGroomReceiptSpecPath(t, plan, "")
}

// TestChangeGroomPlanTitleOnlyReviseOfNeedsGroomingStub pins that a title alone
// retitles a stub. A stub links no spec, so only the record (and the board) are
// written, and the slug stays put.
func TestChangeGroomPlanTitleOnlyReviseOfNeedsGroomingStub(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, titleOnlyReviseRequest("Renamed widget")))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	for _, want := range []string{"title: 'Renamed widget'", "slug: add-a-widget", "trivial: false"} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
	if strings.Contains(rec, "spec: '") {
		t.Errorf("title-only revise of a stub wrote a spec link:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "| Renamed widget |") || !strings.Contains(board, "needs-grooming") {
		t.Errorf("board row not retitled or no longer needs-grooming:\n%s", board)
	}
}

// TestChangeGroomPlanReviseNeedsGroomingStubRelationships pins that a stub revise
// writes relationship collections as complete desired values, as on a groomed
// change.
func TestChangeGroomPlanReviseNeedsGroomingStubRelationships(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"):        groomableChange(2, "add-a-widget"),
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	req := stubSectionReviseRequest()
	req.Related = []int{1}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "related: [1]") {
		t.Errorf("related not written as the complete desired value:\n%s", rec)
	}
	if strings.Contains(rec, "spec: '") || strings.Contains(rec, "trivial: true") {
		t.Errorf("stub revise changed its groom state:\n%s", rec)
	}
}

// TestChangeGroomPlanReviseAbstainedStubKeepsMarker pins that a revise of an
// abstained stub on an unrelated section leaves the ## Auto-groom blocked
// marker and auto_groomable: false exactly as they were, so the board keeps
// reading "auto-groom blocked — needs you".
func TestChangeGroomPlanReviseAbstainedStubKeepsMarker(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, stubSectionReviseRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "Sharpened why.") {
		t.Errorf("## Why section not replaced:\n%s", rec)
	}
	if strings.Count(rec, "## Auto-groom blocked") != 1 || !strings.Contains(rec, "First note.") {
		t.Errorf("abstain marker section not preserved:\n%s", rec)
	}
	if !strings.Contains(rec, "auto_groomable: false") || strings.Contains(rec, "auto_groomable: true") {
		t.Errorf("auto_groomable changed under revise:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "auto-groom blocked — needs you") {
		t.Errorf("board lost the auto-groom blocked cell:\n%s", board)
	}
}

// TestChangeGroomPlanReviseSpecNotLinkedMessage pins the reworded refusal: it
// covers both a trivial change and a needs-grooming stub, so it names neither.
func TestChangeGroomPlanReviseSpecNotLinkedMessage(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget")}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validReviseRequest()))
	if !opRes.Refused || len(plan.Files) != 0 {
		t.Fatalf("want a refusal writing nothing, got refused=%v files=%v", opRes.Refused, planPaths(plan))
	}
	want := "change 0002 has no linked spec to revise"
	found := false
	for _, f := range opRes.Findings {
		found = found || (f.Code == "spec-not-linked" && f.Detail["message"] == want)
	}
	if !found {
		t.Errorf("want spec-not-linked with message %q; got %v", want, opRes.Findings)
	}
}

func TestChangeGroomPlanReviseRepeatable(t *testing.T) {
	// Spec item 11 (plan level): a second revise over the first revise's own
	// output succeeds — no one-shot marker exists.
	files := reviseFixtureFiles()
	plan1, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("first revise refused: %v", opRes.Findings)
	}
	files[groomPath(2, "add-a-widget")] = string(groomedRecordBytes(t, plan1, groomPath(2, "add-a-widget")))
	files[reviseSpecPath] = string(groomedRecordBytes(t, plan1, reviseSpecPath))
	req2 := validReviseRequest()
	req2.SpecMarkdown = "# Design\n\nThe twice-revised body.\n"
	plan2, opRes2 := groomPlanFor(t, files, baseGroomOp([]string{}, req2))
	if opRes2.Refused {
		t.Fatalf("second revise refused: %v", opRes2.Findings)
	}
	spec := string(groomedRecordBytes(t, plan2, reviseSpecPath))
	if !strings.Contains(spec, "The twice-revised body.") {
		t.Errorf("second revise did not land:\n%s", spec)
	}
}

// reviseSettledFiles runs validReviseRequest once and feeds its output back as
// the tree: the record's updated: already equals the clock date and its
// docket:artifacts block is already rendered, so a follow-up revise that
// changes nothing in the record yields record bytes identical to the source.
func reviseSettledFiles(t *testing.T) map[string]string {
	t.Helper()
	files := reviseFixtureFiles()
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("settling revise refused: %v", opRes.Findings)
	}
	files[groomPath(2, "add-a-widget")] = string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	files[reviseSpecPath] = string(groomedRecordBytes(t, plan, reviseSpecPath))
	return files
}

// TestChangeGroomPlanReviseOmitsUnchangedRecord pins the review blocker: the
// engine's verifyActualDelta rejects a declared path whose bytes did not change,
// so a spec-only revise whose record re-renders byte-identical (updated: already
// today, artifacts already rendered) must declare ONLY the spec file.
func TestChangeGroomPlanReviseOmitsUnchangedRecord(t *testing.T) {
	files := reviseSettledFiles(t)
	req := validReviseRequest()
	req.Sections = nil
	req.SpecMarkdown = "# Design\n\nA different body.\n"
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		reviseSpecPath: transaction.MutationReplace,
	})
}

// TestChangeGroomPlanReviseIdenticalIsNoOp pins that a revise whose spec body
// and section text equal what is already on the tree declares nothing: an
// empty plan the engine commits as a clean no-op, never an unchanged replace
// the delta verifier fails.
func TestChangeGroomPlanReviseIdenticalIsNoOp(t *testing.T) {
	files := reviseSettledFiles(t)
	files["docs/changes/BOARD.md"] = "# Backlog\n\nold\n"
	// Settle the board too, so inline rendering has nothing to change.
	boardPlan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("board-settling revise refused: %v", opRes.Findings)
	}
	files["docs/changes/BOARD.md"] = string(groomedRecordBytes(t, boardPlan, "docs/changes/BOARD.md"))

	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
	}{
		{"identical spec and section", func(r *ChangeGroomRequest) {}},
		{"identical spec only", func(r *ChangeGroomRequest) { r.Sections = nil }},
		{"identical section only", func(r *ChangeGroomRequest) { r.SpecMarkdown = "" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validReviseRequest()
			c.mut(&req)
			plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			if len(plan.Files) != 0 {
				t.Errorf("identical revise declared files %v, want an empty (no-op) plan", planPaths(plan))
			}
		})
	}
}

func TestChangeGroomResultHumanTextRevise(t *testing.T) {
	r := newChangeGroomResult(ResultApplied, ChangeGroomResult{
		ID: 7, Outcome: string(GroomRevise), SpecPath: "docs/superpowers/specs/x.md",
		Revision: "cafebabecafebabecafebabecafebabecafebabe",
	})
	got := r.HumanText()
	want := "change 0007 revised — cafebabecafebabecafebabecafebabecafebabe"
	if got != want {
		// Review Focus 4: a revise carrying a spec path must NOT render as
		// "groomed (spec …)".
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}

// abstainRequest is a well-formed abstain request against the groomable fixture
// at id 2 / slug add-a-widget. The note uses a ### subsection, which is legal.
func abstainRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID:    2,
		Path:        groomPath(2, "add-a-widget"),
		Revision:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:     GroomAbstain,
		BlockedNote: "The storage decision needs a human.\n\n### What to supply\n\nPick the backend.\n",
	}
}

// abstainedChange is the groomable fixture after one earlier abstain: an
// explicit auto_groomable: false and a one-entry ## Auto-groom blocked section.
func abstainedChange(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "trivial: false\n", "trivial: false\nauto_groomable: false\n", 1) +
		"\n## Auto-groom blocked\n\nRecorded 2026-08-01 (UTC).\n\nFirst note.\n"
}

func TestChangeGroomAbstainShapeValidation(t *testing.T) {
	one := 1
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		code string // "" means the request must pass shape validation
	}{
		{"valid abstain passes", func(r *ChangeGroomRequest) {}, ""},
		{"blank blocked_note", func(r *ChangeGroomRequest) { r.BlockedNote = "  \n" }, "empty-blocked_note"},
		{"blocked_note smuggling a structural heading", func(r *ChangeGroomRequest) {
			r.BlockedNote = "Context.\n\n## Why\n\nsmuggled\n"
		}, "invalid-blocked_note"},
		{"blocked_note with an unterminated fence", func(r *ChangeGroomRequest) {
			r.BlockedNote = "Context.\n\n```\nnever closed\n"
		}, "invalid-blocked_note"},
		{"abstain with sections", func(r *ChangeGroomRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "rewrite\n"}}
		}, "invalid-sections"},
		{"abstain with spec_markdown", func(r *ChangeGroomRequest) { r.SpecMarkdown = "# Design\n" }, "invalid-spec_markdown"},
		{"abstain with spec_revision", func(r *ChangeGroomRequest) { r.SpecRevision = "a" }, "invalid-spec_revision"},
		{"abstain with depends_on", func(r *ChangeGroomRequest) { r.DependsOn = []int{1} }, "invalid-depends_on"},
		{"abstain with an explicit empty related", func(r *ChangeGroomRequest) { r.Related = []int{} }, "invalid-related"},
		{"abstain with discovered_from", func(r *ChangeGroomRequest) { r.DiscoveredFrom = []int{1} }, "invalid-discovered_from"},
		{"abstain with adrs", func(r *ChangeGroomRequest) { r.ADRs = []int{1} }, "invalid-adrs"},
		{"abstain with stacked_on", func(r *ChangeGroomRequest) { r.StackedOn = &one }, "invalid-stacked_on"},
		{"blocked_note on the spec outcome", func(r *ChangeGroomRequest) {
			r.Outcome, r.SpecMarkdown = GroomSpec, "# Design\n"
		}, "invalid-blocked_note"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := abstainRequest()
			c.mut(&req)
			findings := validateChangeGroomShape(req)
			if c.code == "" {
				if len(findings) != 0 {
					t.Fatalf("want no findings, got %v", findings)
				}
				return
			}
			if !hasFindingCode(findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, findings)
			}
		})
	}
}

func TestChangeGroomAbstainBadNoteRefusedWithoutEngineCall(t *testing.T) {
	req := abstainRequest()
	req.BlockedNote = "Context.\n\n## Why\n\nsmuggled\n"
	engine := &recordingEngine{}
	deps := PlanningDeps{Engine: engine, Reader: &fakeChangeReader{pin: mainModePin([]string{"inline"})}, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", req)

	if res.Result != ResultInvalidInput || len(engine.calls) != 0 {
		t.Fatalf("result = %q with %d engine calls, want invalid-input and none", res.Result, len(engine.calls))
	}
	// The refusal must come from the shape check itself: an empty repoDir also
	// yields invalid-input further down, so the result alone would pass even if
	// blocked_note were never validated.
	if !hasFindingCode(res.Findings, "invalid-blocked_note") {
		t.Errorf("missing invalid-blocked_note; got %v", res.Findings)
	}
}

func TestChangeGroomPlanAbstainSetsFlagSectionAndBoard(t *testing.T) {
	cases := []struct {
		name string
		rec  string
	}{
		// Review Focus 4: a record with no auto_groomable key gets it inserted.
		{"field absent", groomableChange(2, "add-a-widget")},
		{"field armed true", strings.Replace(groomableChange(2, "add-a-widget"), "trivial: false\n", "trivial: false\nauto_groomable: true\n", 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{
				groomPath(2, "add-a-widget"): c.rec,
				"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
			}
			plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, abstainRequest()))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			assertPlanPaths(t, plan, map[string]transaction.MutationKind{
				groomPath(2, "add-a-widget"): transaction.MutationReplace,
				"docs/changes/BOARD.md":      transaction.MutationReplace,
			})
			rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
			for _, want := range []string{
				"\nauto_groomable: false\n",
				"updated: '2026-08-16'",
				"## Auto-groom blocked\n\nRecorded 2026-08-16 (UTC).\n\nThe storage decision needs a human.",
				"### What to supply",
				"\nspec:\n", "trivial: false", // groom scalars untouched
				"Original why.", "An open question.", // proposal untouched
			} {
				if !strings.Contains(rec, want) {
					t.Errorf("record missing %q:\n%s", want, rec)
				}
			}
			if strings.Contains(rec, "auto_groomable: true") {
				t.Errorf("abstain left the stub armed:\n%s", rec)
			}
			board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
			if !strings.Contains(board, "auto-groom blocked — needs you") {
				t.Errorf("board row did not flip to auto-groom blocked in the same plan:\n%s", board)
			}
			var receipt changeGroomReceipt
			if err := json.Unmarshal(plan.Receipt, &receipt); err != nil || receipt.Outcome != "abstain" || receipt.SpecPath != "" {
				t.Errorf("receipt = %s (%v), want outcome abstain and no spec_path", plan.Receipt, err)
			}
		})
	}
}

func TestChangeGroomPlanAbstainAppendsSecondEntry(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget")}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, abstainRequest()))
	if opRes.Refused {
		t.Fatalf("a second abstain must append, not refuse: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if n := strings.Count(rec, "## Auto-groom blocked"); n != 1 {
		t.Fatalf("section heading appears %d times, want exactly 1:\n%s", n, rec)
	}
	first := strings.Index(rec, "Recorded 2026-08-01 (UTC).\n\nFirst note.")
	second := strings.Index(rec, "Recorded 2026-08-16 (UTC).\n\nThe storage decision needs a human.")
	if first < 0 || second < 0 || first > second {
		t.Errorf("entries missing or out of order (first=%d second=%d):\n%s", first, second, rec)
	}
}

func TestChangeGroomPlanAbstainRefusesNonGroomable(t *testing.T) {
	cases := []struct {
		name string
		rec  string
	}{
		{"spec'd", revisableChange(2, "add-a-widget", reviseSpecPath)},
		{"trivial", trivialChange(2, "add-a-widget")},
		{"not proposed", strings.Replace(groomableChange(2, "add-a-widget"), "status: proposed\n", "status: blocked\nblocked_by: 'waiting on infra'\n", 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{groomPath(2, "add-a-widget"): c.rec}
			if c.name == "spec'd" {
				files[reviseSpecPath] = reviseFixtureFiles()[reviseSpecPath]
			}
			plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, abstainRequest()))
			if !opRes.Refused || len(plan.Files) != 0 {
				t.Fatalf("want a refusal writing nothing, got refused=%v files=%v", opRes.Refused, planPaths(plan))
			}
			found := false
			for _, f := range opRes.Findings {
				found = found || f.Code == "not-groomable"
			}
			if !found {
				t.Errorf("missing not-groomable; got %v", opRes.Findings)
			}
		})
	}
}

func TestChangeGroomResultHumanTextAbstain(t *testing.T) {
	r := newChangeGroomResult(ResultApplied, ChangeGroomResult{ID: 7, Outcome: string(GroomAbstain), Revision: "cafe"})
	if got, want := r.HumanText(), "change 0007 auto-groom abstained — cafe"; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}

// reEnableRequest is a well-formed re-enable request against the fixture at id 2.
func reEnableRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID: 2,
		Path:     groomPath(2, "add-a-widget"),
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:  GroomReEnable,
	}
}

func TestChangeGroomReEnableShapeValidation(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		code string
	}{
		{"bare re-enable passes", func(r *ChangeGroomRequest) {}, ""},
		{"re-enable with owned-section edits passes", func(r *ChangeGroomRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Open questions", Intent: "replace", Markdown: "Resolved.\n"}}
		}, ""},
		{"re-enable with blocked_note", func(r *ChangeGroomRequest) { r.BlockedNote = "x\n" }, "invalid-blocked_note"},
		{"re-enable with spec_markdown", func(r *ChangeGroomRequest) { r.SpecMarkdown = "# Design\n" }, "invalid-spec_markdown"},
		{"re-enable with spec_revision", func(r *ChangeGroomRequest) { r.SpecRevision = "a" }, "invalid-spec_revision"},
		// Review Focus 5: the op removes this section itself.
		{"re-enable editing ## Auto-groom blocked", func(r *ChangeGroomRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Auto-groom blocked", Intent: "replace", Markdown: "x\n"}}
		}, "invalid-section-heading"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := reEnableRequest()
			c.mut(&req)
			findings := validateChangeGroomShape(req)
			if c.code == "" {
				if len(findings) != 0 {
					t.Fatalf("want no findings, got %v", findings)
				}
				return
			}
			if !hasFindingCode(findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, findings)
			}
		})
	}
}

// TestChangeGroomRetiredRearmOutcomeRefused pins the hard cut (ADR-0129
// Decision 2, row 54): the retired spelling is not an alias of re-enable, and
// the refusal names only the live outcome set.
func TestChangeGroomRetiredRearmOutcomeRefused(t *testing.T) {
	req := reEnableRequest()
	req.Outcome = GroomOutcome("rearm")
	findings := validateChangeGroomShape(req)
	var msg string
	for _, f := range findings {
		if f.Code == "invalid-outcome" {
			msg = f.Message
		}
	}
	if msg == "" {
		t.Fatalf("outcome rearm: want invalid-outcome, got %v", findings)
	}
	// The message echoes the offending value via %q ("rearm"), so assert on the
	// listed outcome set only: it must end with the live spelling.
	if !strings.HasSuffix(msg, "must be one of spec, trivial, revise, abstain, re-enable") {
		t.Errorf("invalid-outcome message %q must list exactly the live outcomes ending in re-enable", msg)
	}
}

func TestChangeGroomPlanReEnableClearsSectionSetsFlagAndBoard(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, reEnableRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if strings.Contains(rec, "## Auto-groom blocked") || strings.Contains(rec, "First note.") {
		t.Errorf("re-enable left the marker section behind:\n%s", rec)
	}
	if !strings.Contains(rec, "\nauto_groomable: true\n") || strings.Contains(rec, "auto_groomable: false") {
		t.Errorf("re-enable did not set auto_groomable: true:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if strings.Contains(board, "auto-groom blocked — needs you") || !strings.Contains(board, "needs-grooming") {
		t.Errorf("board row did not return to needs-grooming in the same plan:\n%s", board)
	}
	var receipt changeGroomReceipt
	if err := json.Unmarshal(plan.Receipt, &receipt); err != nil || receipt.Outcome != "re-enable" {
		t.Errorf("receipt = %s (%v), want outcome re-enable", plan.Receipt, err)
	}
}

func TestChangeGroomPlanReEnableAppliesSectionEditsInOneRecord(t *testing.T) {
	req := reEnableRequest()
	req.Sections = []SectionEditRequest{{Heading: "## Open questions", Intent: "replace", Markdown: "Resolved: use SQLite.\n"}}
	files := map[string]string{groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget")}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{groomPath(2, "add-a-widget"): transaction.MutationReplace})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "Resolved: use SQLite.") || strings.Contains(rec, "An open question.") || strings.Contains(rec, "## Auto-groom blocked") {
		t.Errorf("section edit and section removal did not both land:\n%s", rec)
	}
}

func TestChangeGroomPlanReEnableEnablesWithoutABlockedSection(t *testing.T) {
	cases := []struct {
		name string
		rec  string
	}{
		// Review Focus 4: the key is inserted when absent (inherit ⇒ explicit true).
		{"field absent", groomableChange(2, "add-a-widget")},
		{"opted out", strings.Replace(groomableChange(2, "add-a-widget"), "trivial: false\n", "trivial: false\nauto_groomable: false\n", 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, opRes := groomPlanFor(t, map[string]string{groomPath(2, "add-a-widget"): c.rec}, baseGroomOp([]string{}, reEnableRequest()))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			if rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget"))); !strings.Contains(rec, "\nauto_groomable: true\n") {
				t.Errorf("auto_groomable: true not written:\n%s", rec)
			}
		})
	}
}

func TestChangeGroomPlanReEnableRefusals(t *testing.T) {
	armed := strings.Replace(groomableChange(2, "add-a-widget"), "trivial: false\n", "trivial: false\nauto_groomable: true\n", 1)
	cases := []struct {
		name  string
		files map[string]string
		code  string
	}{
		{"nothing to re-enable", map[string]string{groomPath(2, "add-a-widget"): armed}, "nothing-to-re-enable"},
		{"trivial", map[string]string{groomPath(2, "add-a-widget"): trivialChange(2, "add-a-widget")}, "not-groomable"},
		{"spec'd", reviseFixtureFiles(), "not-groomable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, opRes := groomPlanFor(t, c.files, baseGroomOp([]string{}, reEnableRequest()))
			if !opRes.Refused || len(plan.Files) != 0 {
				t.Fatalf("want a refusal writing nothing, got refused=%v files=%v", opRes.Refused, planPaths(plan))
			}
			found := false
			for _, f := range opRes.Findings {
				found = found || f.Code == c.code
			}
			if !found {
				t.Errorf("missing %q; got %v", c.code, opRes.Findings)
			}
		})
	}
}

// TestChangeGroomReEnableFencedMarkerAgreesWithBoard — a heading-shaped
// "## Auto-groom blocked" line inside fenced code is not the abstain section:
// the decoded record (which the board's cell keys on) and re-enable's section scan
// must agree, so the board never shows "needs you" on a record re-enable reports
// as having nothing to re-enable.
func TestChangeGroomReEnableFencedMarkerAgreesWithBoard(t *testing.T) {
	rec := strings.Replace(groomableChange(2, "add-a-widget"), "trivial: false\n", "trivial: false\nauto_groomable: true\n", 1) +
		"\n## Notes\n\n```md\n## Auto-groom blocked\n```\n"
	files := map[string]string{groomPath(2, "add-a-widget"): rec}
	before, err := newPlanningLoader(planningTestConfig([]string{})).Load(context.Background(), newFakeTree(files))
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}
	c, out := before.Snapshot.Change(2)
	if out != domain.LookupFound {
		t.Fatalf("change 2 lookup = %v", out)
	}
	if c.HasAutoGroomBlocked() {
		t.Errorf("decoded record reports a fenced heading as the abstain marker; the board would show it blocked")
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, reEnableRequest()))
	found := false
	for _, f := range opRes.Findings {
		found = found || f.Code == "nothing-to-re-enable"
	}
	if !opRes.Refused || len(plan.Files) != 0 || !found {
		t.Errorf("re-enable over a fenced marker: refused=%v files=%v findings=%v, want nothing-to-re-enable", opRes.Refused, planPaths(plan), opRes.Findings)
	}
}

func TestChangeGroomResultHumanTextReEnable(t *testing.T) {
	r := newChangeGroomResult(ResultApplied, ChangeGroomResult{ID: 7, Outcome: string(GroomReEnable), Revision: "cafe"})
	if got, want := r.HumanText(), "change 0007 re-enabled for auto-groom — cafe"; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}

// followedSection is a non-final section placed after ## Auto-groom blocked so
// abstain-append and re-enable-removal are proven not to consume what follows.
const followedSection = "## Reconcile log\n\nKept entry.\n"

func TestChangeGroomPlanAbstainAppendsBeforeFollowingSection(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget") + "\n" + followedSection}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, abstainRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	first := strings.Index(rec, "Recorded 2026-08-01 (UTC).\n\nFirst note.")
	second := strings.Index(rec, "Recorded 2026-08-16 (UTC).\n\nThe storage decision needs a human.")
	if first < 0 || second < 0 || first > second {
		t.Errorf("entries missing or out of order (first=%d second=%d):\n%s", first, second, rec)
	}
	if strings.Count(rec, followedSection) != 1 || !strings.HasSuffix(rec, followedSection) {
		t.Errorf("following section not preserved byte-identically:\n%s", rec)
	}
}

func TestChangeGroomPlanReEnableRemovesSectionBeforeFollowingSection(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget") + "\n" + followedSection}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, reEnableRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if strings.Contains(rec, "## Auto-groom blocked") || strings.Contains(rec, "First note.") {
		t.Errorf("re-enable left the blocked section behind:\n%s", rec)
	}
	if strings.Count(rec, followedSection) != 1 || !strings.HasSuffix(rec, followedSection) {
		t.Errorf("following section not preserved byte-identically:\n%s", rec)
	}
}

// titleOnlyReviseRequest is a revise carrying nothing but a title — no spec
// body, no spec_revision, no section edits (change 0461).
func titleOnlyReviseRequest(title string) ChangeGroomRequest {
	r := validReviseRequest()
	r.SpecMarkdown, r.SpecRevision, r.Sections = "", "", nil
	r.Title = title
	return r
}

// titledTrivialRequest is a well-formed trivial groom of the groomable fixture
// at id 2 that also retitles it.
func titledTrivialRequest(title string) ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID: 2,
		Path:     groomPath(2, "add-a-widget"),
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:  GroomTrivial,
		Title:    title,
		Sections: []SectionEditRequest{
			{Heading: "## Why", Intent: "replace", Markdown: "Trivial: a rename only.\n"},
		},
	}
}

func TestChangeGroomTitleShapeValidation(t *testing.T) {
	withTitle := func(r ChangeGroomRequest, title string) ChangeGroomRequest {
		r.Title = title
		return r
	}
	cases := []struct {
		name string
		req  ChangeGroomRequest
		code string // "" means the request must pass shape validation
	}{
		{"spec accepts title", withTitle(validGroomSpecRequest(), "Renamed widget"), ""},
		{"trivial accepts title", titledTrivialRequest("Renamed widget"), ""},
		{"re-enable accepts title", withTitle(reEnableRequest(), "Renamed widget"), ""},
		{"revise accepts a title alone", titleOnlyReviseRequest("Renamed widget"), ""},
		{"revise accepts title with sections", withTitle(validReviseRequest(), "Renamed widget"), ""},
		{"abstain refuses title", withTitle(abstainRequest(), "Renamed widget"), "invalid-title"},
		{"whitespace-only title", titleOnlyReviseRequest("   "), "empty-title"},
		{"multi-line title", titleOnlyReviseRequest("Renamed\nwidget"), "invalid-title"},
		{"control-character title", titleOnlyReviseRequest("Renamed\x00widget"), "invalid-title"},
		{"multi-line title on spec outcome", withTitle(validGroomSpecRequest(), "a\r\nb"), "invalid-title"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings := validateChangeGroomShape(c.req)
			if c.code == "" {
				if len(findings) != 0 {
					t.Fatalf("unexpected shape findings: %v", findings)
				}
				return
			}
			if !hasFindingCode(findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, findings)
			}
		})
	}
}

// TestChangeGroomEmptyReviseNamesTitle pins the widened empty-revise rule: with
// no spec_markdown, no effective section edit, and no title the revise is still
// refused, and the diagnostic names all three inputs.
func TestChangeGroomEmptyReviseNamesTitle(t *testing.T) {
	findings := validateChangeGroomShape(titleOnlyReviseRequest(""))
	var msg string
	for _, f := range findings {
		if f.Code == string(FCEmptyRevise) {
			msg = f.Message
		}
	}
	if msg == "" {
		t.Fatalf("empty revise not refused; findings %v", findings)
	}
	for _, want := range []string{"spec_markdown", "section edit", "title"} {
		if !strings.Contains(msg, want) {
			t.Errorf("empty-revise message %q does not name %q", msg, want)
		}
	}
}

func TestChangeGroomBadTitleRefusedWithoutEngineCall(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, "", titleOnlyReviseRequest("two\nlines"))

	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on a shape failure, want 0", len(engine.calls))
	}
	if !hasFindingCode(res.Findings, "invalid-title") {
		t.Errorf("missing invalid-title; got %v", res.Findings)
	}
}

// reviseSpecBody is the fixture spec's bytes after its backlink block — the part
// a title re-stamp must leave byte-identical.
const reviseSpecBody = "<!-- docket:backlink:end -->\n\n# Design\n\nThe original design body.\n"

func TestChangeGroomPlanTitleOnlyReviseRestampsSpecBacklink(t *testing.T) {
	files := reviseFixtureFiles()
	files["docs/changes/BOARD.md"] = "# Backlog\n\nold\n"
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, titleOnlyReviseRequest("Renamed widget")))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// One commit: the record, the spec (backlink re-stamp only), and the board.
	// The record path itself is unchanged — a retitle renames nothing.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		reviseSpecPath:               transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	for _, want := range []string{"title: 'Renamed widget'", "updated: '2026-08-16'", "slug: add-a-widget", "spec: '" + reviseSpecPath + "'"} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
	spec := string(groomedRecordBytes(t, plan, reviseSpecPath))
	if !strings.Contains(spec, "Change 0002 — Renamed widget") {
		t.Errorf("spec backlink not re-stamped with the new title:\n%s", spec)
	}
	if strings.Contains(spec, "old backlink") {
		t.Errorf("old backlink line survived:\n%s", spec)
	}
	if !strings.HasSuffix(spec, reviseSpecBody) {
		t.Errorf("spec bytes outside the backlink block changed:\n%s", spec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "| Renamed widget |") || strings.Contains(board, "A change") {
		t.Errorf("board row not retitled:\n%s", board)
	}
	// A title-only revise replaced no spec body, so the receipt names none.
	assertGroomReceiptSpecPath(t, plan, "")
}

// TestChangeGroomPlanTitleRoundTripsThroughWriter pins ADR-0071 for the new
// field: YAML-hostile punctuation lands writer-quoted and reads back as the
// exact string (the re-stamped backlink is rendered from the reparsed record).
func TestChangeGroomPlanTitleRoundTripsThroughWriter(t *testing.T) {
	title := "Fix: the '#1' bug"
	plan, opRes := groomPlanFor(t, reviseFixtureFiles(), baseGroomOp([]string{}, titleOnlyReviseRequest(title)))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "title: 'Fix: the ''#1'' bug'\n") {
		t.Errorf("title not writer-quoted:\n%s", rec)
	}
	if spec := string(groomedRecordBytes(t, plan, reviseSpecPath)); !strings.Contains(spec, "Change 0002 — "+title) {
		t.Errorf("title did not round-trip into the backlink:\n%s", spec)
	}
}

func TestChangeGroomPlanTitleOnSpecOutcome(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget")}
	req := validGroomSpecRequest()
	req.Title = "Renamed widget"
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// The new spec path is still minted from the unchanged slug.
	newSpec := "docs/superpowers/specs/2026-08-16-add-a-widget-design.md"
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		newSpec:                      transaction.MutationCreate,
	})
	if spec := string(groomedRecordBytes(t, plan, newSpec)); !strings.Contains(spec, "Change 0002 — Renamed widget") {
		t.Errorf("new spec's backlink lacks the new title:\n%s", spec)
	}
	if rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget"))); !strings.Contains(rec, "title: 'Renamed widget'") {
		t.Errorf("record not retitled:\n%s", rec)
	}
}

func TestChangeGroomPlanTitleOnTrivialOutcome(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget")}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, titledTrivialRequest("Renamed widget")))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// No spec file is touched.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
	})
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "title: 'Renamed widget'") || !strings.Contains(rec, "trivial: true") {
		t.Errorf("record not retitled/trivialled:\n%s", rec)
	}
}

// TestChangeGroomPlanTitleWithSpecBodyRevise pins Review Focus 4: the
// whole-body replace already renders the backlink from the groomed record, so
// the spec path is declared exactly once, carrying both the new body and title.
func TestChangeGroomPlanTitleWithSpecBodyRevise(t *testing.T) {
	req := validReviseRequest()
	req.Title = "Renamed widget"
	plan, opRes := groomPlanFor(t, reviseFixtureFiles(), baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	n := 0
	for _, p := range planPaths(plan) {
		if p == reviseSpecPath {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("spec path declared %d times, want 1: %v", n, planPaths(plan))
	}
	spec := string(groomedRecordBytes(t, plan, reviseSpecPath))
	if !strings.Contains(spec, "Change 0002 — Renamed widget") || !strings.Contains(spec, "The revised design body.") {
		t.Errorf("spec lacks the new title or the new body:\n%s", spec)
	}
}

// TestChangeGroomPlanTitleOnlyReviseOfTrivialChange pins Review Focus 5: a
// trivial-verdicted change links no spec, so a retitle writes only the record.
func TestChangeGroomPlanTitleOnlyReviseOfTrivialChange(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): trivialChange(2, "add-a-widget")}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, titleOnlyReviseRequest("Renamed widget")))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
	})
}

// revivedReviseFiles is the revise fixture for a proposed change that still
// carries feature-branch artifacts (a hand-edited or legacy record): it is an
// already-groomed proposed change with branch: and plan: set, whose
// backlinks embed the title and are identity-checked downstream.
func revivedReviseFiles() map[string]string {
	const planPath = "docs/superpowers/plans/2026-08-10-add-a-widget.md"
	files := reviseFixtureFiles()
	src := groomPath(2, "add-a-widget")
	rec := strings.Replace(files[src], "status: proposed\n", "status: proposed\nbranch: 'feat/add-a-widget'\n", 1)
	files[src] = strings.Replace(rec, "plan:\n", "plan: '"+planPath+"'\n", 1)
	files[planPath] = "# Plan\n"
	return files
}

// TestChangeGroomPlanRetitleRefusedWithFeatureArtifacts pins that a retitle of a
// proposed change still carrying branch:/plan:/results: refuses not-retitleable
// and writes nothing: those artifacts' backlinks embed the title and are
// identity-checked by mark-implemented (results-identity-broken), so a retitle
// would silently break them. A request
// repeating the current title changes nothing and is not refused.
func TestChangeGroomPlanRetitleRefusedWithFeatureArtifacts(t *testing.T) {
	plan, opRes := groomPlanFor(t, revivedReviseFiles(), baseGroomOp([]string{}, titleOnlyReviseRequest("Renamed widget")))
	if !opRes.Refused {
		t.Fatalf("retitle of a change with feature-branch artifacts planned: %v", planPaths(plan))
	}
	found := false
	for _, f := range opRes.Findings {
		found = found || f.Code == string(FCNotRetitleable)
	}
	if !found {
		t.Errorf("missing not-retitleable; got %v", opRes.Findings)
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", planPaths(plan))
	}

	req := validReviseRequest()
	req.Title = "A change" // the fixture's current title: no retitle
	_, opRes = groomPlanFor(t, revivedReviseFiles(), baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Errorf("unchanged title on a record with feature-branch artifacts refused: %v", opRes.Findings)
	}
}

func TestChangeGroomPlanTitleRestampRefusals(t *testing.T) {
	rec := revisableChange(2, "add-a-widget", reviseSpecPath)
	cases := []struct {
		name  string
		files map[string]string
		code  string
	}{
		// The re-stamp never silently inserts a block.
		{"spec lacks a backlink block", map[string]string{
			groomPath(2, "add-a-widget"): rec,
			reviseSpecPath:               "# Design\n\nNo backlink here.\n",
		}, "spec-backlink-missing"},
		// A dangling start marker fails the document parse.
		{"spec backlink markers malformed", map[string]string{
			groomPath(2, "add-a-widget"): rec,
			reviseSpecPath:               "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> dangling\n\n# Design\n",
		}, "spec-backlink-malformed"},
		// A dangling spec link reuses the existing refusal.
		{"spec file missing", map[string]string{
			groomPath(2, "add-a-widget"): rec,
		}, "spec-file-missing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, opRes := groomPlanFor(t, c.files, baseGroomOp([]string{}, titleOnlyReviseRequest("Renamed widget")))
			if !opRes.Refused {
				t.Fatalf("expected a refusal, got plan files %v", planPaths(plan))
			}
			found := false
			for _, f := range opRes.Findings {
				if f.Code == c.code {
					found = true
				}
			}
			if !found {
				t.Errorf("missing refusal code %q; got %v", c.code, opRes.Findings)
			}
			if len(plan.Files) != 0 {
				t.Errorf("refused plan still carries files: %v", planPaths(plan))
			}
		})
	}
}

// TestChangeGroomPlanUnchangedTitleSkipsRestamp pins Review Focus 1: the
// re-stamp is keyed on the title CHANGING, so resending the current title with
// a section edit never probes the spec and never refuses a block-less spec.
func TestChangeGroomPlanUnchangedTitleSkipsRestamp(t *testing.T) {
	files := reviseFixtureFiles()
	files[reviseSpecPath] = "# Design\n\nNo backlink here.\n"
	req := titleOnlyReviseRequest("A change") // the fixture's current title
	req.Sections = []SectionEditRequest{{Heading: "## What changes", Intent: "replace", Markdown: "Narrowed what.\n"}}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
	if opRes.Refused {
		t.Fatalf("unchanged title refused: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
	})
}

// TestChangeGroomPlanSameTitleIsNoOp pins that a title-only revise resending the
// current title over a settled tree declares nothing — the engine's clean no-op.
func TestChangeGroomPlanSameTitleIsNoOp(t *testing.T) {
	files := reviseSettledFiles(t)
	files["docs/changes/BOARD.md"] = "# Backlog\n\nold\n"
	boardPlan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("board-settling revise refused: %v", opRes.Findings)
	}
	files["docs/changes/BOARD.md"] = string(groomedRecordBytes(t, boardPlan, "docs/changes/BOARD.md"))

	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, titleOnlyReviseRequest("A change")))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	if len(plan.Files) != 0 {
		t.Errorf("same-title revise declared files %v, want an empty (no-op) plan", planPaths(plan))
	}
}

// priorityOnlyReviseRequest is a revise carrying nothing but a priority — no
// spec body, no spec_revision, no section edits, no title (change 0550).
func priorityOnlyReviseRequest(priority string) ChangeGroomRequest {
	r := validReviseRequest()
	r.SpecMarkdown, r.SpecRevision, r.Sections = "", "", nil
	r.Priority = priority
	return r
}

func TestChangeGroomPriorityShapeValidation(t *testing.T) {
	withPriority := func(r ChangeGroomRequest, p string) ChangeGroomRequest {
		r.Priority = p
		return r
	}
	cases := []struct {
		name string
		req  ChangeGroomRequest
		code string // "" means the request must pass shape validation
		not  string // a code that must NOT be emitted ("" = no check)
	}{
		{"spec accepts priority", withPriority(validGroomSpecRequest(), "high"), "", ""},
		{"trivial accepts priority", withPriority(titledTrivialRequest("Renamed widget"), "low"), "", ""},
		{"re-enable accepts priority", withPriority(reEnableRequest(), "critical"), "", ""},
		{"revise accepts a priority alone", priorityOnlyReviseRequest("high"), "", ""},
		{"revise accepts priority with sections", withPriority(validReviseRequest(), "medium"), "", ""},
		{"unknown word", priorityOnlyReviseRequest("urgent"), "unknown-priority", ""},
		{"capitalized", priorityOnlyReviseRequest("High"), "unknown-priority", ""},
		{"upper-case", priorityOnlyReviseRequest("HIGH"), "unknown-priority", ""},
		{"leading space", priorityOnlyReviseRequest(" high"), "unknown-priority", ""},
		{"trailing space", priorityOnlyReviseRequest("high "), "unknown-priority", ""},
		{"unknown on spec outcome", withPriority(validGroomSpecRequest(), "urgent"), "unknown-priority", ""},
		{"abstain refuses a valid priority", withPriority(abstainRequest(), "high"), "invalid-priority", "unknown-priority"},
		{"abstain refuses a bogus priority once", withPriority(abstainRequest(), "urgent"), "invalid-priority", "unknown-priority"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings := validateChangeGroomShape(c.req)
			if c.code == "" {
				if len(findings) != 0 {
					t.Fatalf("unexpected shape findings: %v", findings)
				}
				return
			}
			if !hasFindingCode(findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, findings)
			}
			if c.not != "" && hasFindingCode(findings, c.not) {
				t.Errorf("unexpected finding %q alongside %q; got %v", c.not, c.code, findings)
			}
		})
	}
}

// TestChangeGroomUnknownPriorityMessageMatchesCreate pins the shared wording
// with change.create's unknown-priority finding.
func TestChangeGroomUnknownPriorityMessageMatchesCreate(t *testing.T) {
	findings := validateChangeGroomShape(priorityOnlyReviseRequest("urgent"))
	want := `unknown priority "urgent"; valid values are critical, high, medium, low`
	for _, f := range findings {
		if f.Code == string(FCUnknownPriority) {
			if f.Message != want {
				t.Errorf("message = %q, want %q", f.Message, want)
			}
			return
		}
	}
	t.Fatalf("no unknown-priority finding; got %v", findings)
}

// TestChangeGroomEmptyReviseNamesPriority pins the widened empty-revise rule:
// with no spec_markdown, no effective section edit, no title, and no priority
// the revise is refused, and the diagnostic names priority too.
func TestChangeGroomEmptyReviseNamesPriority(t *testing.T) {
	findings := validateChangeGroomShape(priorityOnlyReviseRequest(""))
	var msg string
	for _, f := range findings {
		if f.Code == string(FCEmptyRevise) {
			msg = f.Message
		}
	}
	if msg == "" {
		t.Fatalf("empty revise not refused; findings %v", findings)
	}
	for _, want := range []string{"spec_markdown", "section edit", "title", "priority"} {
		if !strings.Contains(msg, want) {
			t.Errorf("empty-revise message %q does not name %q", msg, want)
		}
	}
}

func TestChangeGroomBadPriorityRefusedWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		req  ChangeGroomRequest
		code string
	}{
		{"unknown priority", priorityOnlyReviseRequest("urgent"), "unknown-priority"},
		{"abstain with priority", func() ChangeGroomRequest { r := abstainRequest(); r.Priority = "high"; return r }(), "invalid-priority"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeGroom(context.Background(), deps, "", c.req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called %d times on a shape failure, want 0 (nothing may be written)", len(engine.calls))
			}
			if !hasFindingCode(res.Findings, c.code) {
				t.Errorf("missing %q; got %v", c.code, res.Findings)
			}
		})
	}
}

// assertPriorityLanded pins that the record carries the new priority (and not
// the old medium) and that the inline board row shows it.
func assertPriorityLanded(t *testing.T, plan transaction.MutationPlan, want string) {
	t.Helper()
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if !strings.Contains(rec, "\npriority: '"+want+"'\n") {
		t.Errorf("record priority not rewritten to %q:\n%s", want, rec)
	}
	if strings.Contains(rec, "\npriority: medium\n") {
		t.Errorf("old priority survived:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "| A change | `"+want+"` |") {
		t.Errorf("board row does not show priority %q:\n%s", want, board)
	}
	if strings.Contains(board, "| A change | `medium` |") {
		t.Errorf("board row still shows the old priority:\n%s", board)
	}
}

// TestChangeGroomPlanPriorityLandsOnEveryAcceptingOutcome pins spec test 1:
// spec, trivial, revise, and re-enable each write the new priority to the
// record and the board row in the same plan.
func TestChangeGroomPlanPriorityLandsOnEveryAcceptingOutcome(t *testing.T) {
	trivial := ChangeGroomRequest{
		ChangeID: 2, Path: groomPath(2, "add-a-widget"),
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Outcome: GroomTrivial,
		Sections: []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "Too small to design.\n"}},
	}
	cases := []struct {
		name string
		rec  string
		req  ChangeGroomRequest
	}{
		{"spec", groomableChange(2, "add-a-widget"), validGroomSpecRequest()},
		{"trivial", groomableChange(2, "add-a-widget"), trivial},
		{"revise", groomableChange(2, "add-a-widget"), priorityOnlyReviseRequest("")},
		{"re-enable", abstainedChange(2, "add-a-widget"), reEnableRequest()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := c.req
			req.Priority = "high"
			files := map[string]string{
				groomPath(2, "add-a-widget"): c.rec,
				"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
			}
			plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			assertPriorityLanded(t, plan, "high")
			if rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget"))); !strings.Contains(rec, "updated: '2026-08-16'") {
				t.Errorf("updated not bumped:\n%s", rec)
			}
		})
	}
}

// TestChangeGroomPlanPriorityOnlyReviseKeepsGroomState pins spec test 2: a
// revise carrying only a priority passes shape validation (the widened
// empty-revise rule) and changes no groom state.
func TestChangeGroomPlanPriorityOnlyReviseKeepsGroomState(t *testing.T) {
	req := priorityOnlyReviseRequest("critical")
	if findings := validateChangeGroomShape(req); len(findings) != 0 {
		t.Fatalf("priority-only revise refused at the shape stage: %v", findings)
	}
	files := map[string]string{
		groomPath(2, "add-a-widget"): optedOutStub(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		groomPath(2, "add-a-widget"): transaction.MutationReplace,
		"docs/changes/BOARD.md":      transaction.MutationReplace,
	})
	assertPriorityLanded(t, plan, "critical")
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	for _, want := range []string{"\nspec:\n", "trivial: false", "auto_groomable: false", "title: 'A change'", "slug: add-a-widget", "Original why."} {
		if !strings.Contains(rec, want) {
			t.Errorf("priority-only revise changed more than priority; missing %q:\n%s", want, rec)
		}
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "needs-grooming") {
		t.Errorf("board row is no longer needs-grooming:\n%s", board)
	}
}

// TestChangeGroomPlanOmittedPriorityLeavesFieldByteIdentical pins spec test 3:
// a groom without priority never touches the stored priority: line (the
// fixture stores it unquoted, so any rewrite would re-quote it).
func TestChangeGroomPlanOmittedPriorityLeavesFieldByteIdentical(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget")}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, stubSectionReviseRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	// Non-vacuity: this is the groomed record (the section edit landed).
	if !strings.Contains(rec, "Sharpened why.") {
		t.Fatalf("not reading the groomed record:\n%s", rec)
	}
	if !strings.Contains(rec, "\npriority: medium\n") || strings.Contains(rec, "priority: 'medium'") {
		t.Errorf("priority: line not byte-identical when priority is omitted:\n%s", rec)
	}
}

// TestChangeGroomPlanPriorityWithTitleAndSections pins Review Focus 3: one
// revise carrying a title, a priority, and a section edit lands all three in
// the single record write and the board row.
func TestChangeGroomPlanPriorityWithTitleAndSections(t *testing.T) {
	req := stubSectionReviseRequest()
	req.Title = "Renamed widget"
	req.Priority = "low"
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	for _, want := range []string{"title: 'Renamed widget'", "\npriority: 'low'\n", "Sharpened why."} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if !strings.Contains(board, "| Renamed widget | `low` |") {
		t.Errorf("board row lacks the new title and priority together:\n%s", board)
	}
}

// TestChangeGroomPlanSamePriorityIsNoOp pins Review Focus 4: resending the
// stored priority over a settled tree declares nothing, the engine's clean
// no-op, never an unchanged replace the delta verifier rejects.
func TestChangeGroomPlanSamePriorityIsNoOp(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	settle, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, priorityOnlyReviseRequest("high")))
	if opRes.Refused {
		t.Fatalf("settling revise refused: %v", opRes.Findings)
	}
	files[groomPath(2, "add-a-widget")] = string(groomedRecordBytes(t, settle, groomPath(2, "add-a-widget")))
	files["docs/changes/BOARD.md"] = string(groomedRecordBytes(t, settle, "docs/changes/BOARD.md"))

	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, priorityOnlyReviseRequest("high")))
	if opRes.Refused {
		t.Fatalf("same-priority revise refused: %v", opRes.Findings)
	}
	if len(plan.Files) != 0 {
		t.Errorf("same-priority revise declared files %v, want an empty (no-op) plan", planPaths(plan))
	}
}

// TestChangeGroomPlanPriorityOnNonProposedRefused pins Review Focus 5: the
// revise gate still admits only proposed changes; a deferred change must be
// revived before it can be re-prioritized.
func TestChangeGroomPlanPriorityOnNonProposedRefused(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"in-progress", reviseFixtureAtStatus(groomPath(2, "add-a-widget"), "in-progress", reviseClaimFields)},
		{"deferred", reviseFixtureAtStatus(groomPath(2, "add-a-widget"), "deferred", "")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, opRes := groomPlanFor(t, c.files, baseGroomOp([]string{}, priorityOnlyReviseRequest("high")))
			if !opRes.Refused || len(plan.Files) != 0 {
				t.Fatalf("want a refusal writing nothing, got refused=%v files=%v", opRes.Refused, planPaths(plan))
			}
			found := false
			for _, f := range opRes.Findings {
				found = found || f.Code == "not-revisable"
			}
			if !found {
				t.Errorf("missing not-revisable; got %v", opRes.Findings)
			}
		})
	}
}

// --- deliverable, type, and branch_prefix (change 0559) --------------------

func strPtr(s string) *string { return &s }

func intPtrTest(n int) *int { return &n }

// fieldOnlyReviseRequest is a revise carrying nothing but what mut sets — no
// spec body, no spec_revision, no section edits.
func fieldOnlyReviseRequest(mut func(*ChangeGroomRequest)) ChangeGroomRequest {
	r := validReviseRequest()
	r.SpecMarkdown, r.SpecRevision, r.Sections = "", "", nil
	mut(&r)
	return r
}

// stackedChange is the groomable fixture at id/slug with stacked_on set.
func stackedChange(id int, slug string, parent int) string {
	return strings.Replace(groomableChange(id, slug), "stacked_on:\n", "stacked_on: "+itoaTest(parent)+"\n", 1)
}

// resultsArtifactChange is the groomable fixture carrying deliverable: results-artifact.
func resultsArtifactChange(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "type: feat\n", "type: feat\ndeliverable: results-artifact\n", 1)
}

func TestChangeGroomEditsDeliverableTypeAndPrefixOnEveryAcceptingOutcome(t *testing.T) {
	trivial := ChangeGroomRequest{
		ChangeID: 2, Path: groomPath(2, "add-a-widget"),
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Outcome: GroomTrivial,
		Sections: []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "Too small to design.\n"}},
	}
	cases := []struct {
		name string
		rec  string
		req  ChangeGroomRequest
	}{
		{"spec", groomableChange(2, "add-a-widget"), validGroomSpecRequest()},
		{"trivial", groomableChange(2, "add-a-widget"), trivial},
		{"revise", groomableChange(2, "add-a-widget"), fieldOnlyReviseRequest(func(*ChangeGroomRequest) {})},
		{"re-enable", abstainedChange(2, "add-a-widget"), reEnableRequest()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := c.req
			req.Deliverable = "results-artifact"
			req.Type = "chore"
			req.BranchPrefix = strPtr("spike")
			if findings := validateChangeGroomShape(req); len(findings) != 0 {
				t.Fatalf("shape refused: %v", findings)
			}
			files := map[string]string{groomPath(2, "add-a-widget"): c.rec}
			plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
			for _, want := range []string{"\ndeliverable: 'results-artifact'\n", "\ntype: 'chore'\n", "\nbranch_prefix: 'spike'\n"} {
				if !strings.Contains(rec, want) {
					t.Errorf("record missing %q:\n%s", want, rec)
				}
			}
			if strings.Contains(rec, "\ntype: feat\n") {
				t.Errorf("old type survived:\n%s", rec)
			}
		})
	}
}

func TestChangeGroomEachNewFieldAloneIsAValidRevise(t *testing.T) {
	cases := map[string]func(*ChangeGroomRequest){
		"deliverable":         func(r *ChangeGroomRequest) { r.Deliverable = "results-artifact" },
		"type":                func(r *ChangeGroomRequest) { r.Type = "chore" },
		"branch_prefix set":   func(r *ChangeGroomRequest) { r.BranchPrefix = strPtr("x") },
		"branch_prefix clear": func(r *ChangeGroomRequest) { r.BranchPrefix = strPtr("") },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := validateChangeGroomShape(fieldOnlyReviseRequest(mut)); len(findings) != 0 {
				t.Fatalf("single-field revise refused: %v", findings)
			}
		})
	}
	findings := validateChangeGroomShape(fieldOnlyReviseRequest(func(*ChangeGroomRequest) {}))
	var msg string
	for _, f := range findings {
		if f.Code == string(FCEmptyRevise) {
			msg = f.Message
		}
	}
	if msg == "" {
		t.Fatalf("empty revise not refused; findings %v", findings)
	}
	for _, want := range []string{"deliverable", "type", "branch_prefix"} {
		if !strings.Contains(msg, want) {
			t.Errorf("empty-revise message %q does not name %q", msg, want)
		}
	}
}

func TestChangeGroomAbstainRefusesNewFields(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		code string
	}{
		{"deliverable", func(r *ChangeGroomRequest) { r.Deliverable = "results-artifact" }, "invalid-deliverable"},
		{"bogus deliverable once", func(r *ChangeGroomRequest) { r.Deliverable = "research" }, "invalid-deliverable"},
		{"type", func(r *ChangeGroomRequest) { r.Type = "chore" }, "invalid-type"},
		{"branch_prefix", func(r *ChangeGroomRequest) { r.BranchPrefix = strPtr("spike") }, "invalid-branch_prefix"},
		{"branch_prefix clear", func(r *ChangeGroomRequest) { r.BranchPrefix = strPtr("") }, "invalid-branch_prefix"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := abstainRequest()
			c.mut(&req)
			findings := validateChangeGroomShape(req)
			if !hasFindingCode(findings, c.code) {
				t.Errorf("missing %q; got %v", c.code, findings)
			}
			if hasFindingCode(findings, string(FCUnknownDeliverable)) {
				t.Errorf("abstain also reported unknown-deliverable; got %v", findings)
			}
		})
	}
	// abstain refuses type at the shape stage, before the config check.
	req := abstainRequest()
	req.Type = "nope"
	engine := &recordingEngine{}
	deps := PlanningDeps{Engine: engine, Reader: &fakeChangeReader{pin: mainModePin([]string{"inline"})}, Clock: testClock()}
	res := ChangeGroom(context.Background(), deps, "", req)
	if res.Result != ResultInvalidInput || !hasFindingCode(res.Findings, "invalid-type") || len(engine.calls) != 0 {
		t.Errorf("abstain with type: result %q findings %v engine calls %d", res.Result, res.Findings, len(engine.calls))
	}
}

func TestChangeGroomBranchPrefixThreeStates(t *testing.T) {
	withPrefix := strings.Replace(groomableChange(2, "add-a-widget"), "trivial: false\n", "trivial: false\nbranch_prefix: keep\n", 1)
	run := func(t *testing.T, bp *string) string {
		t.Helper()
		req := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.Priority = "high"; r.BranchPrefix = bp })
		if findings := validateChangeGroomShape(req); len(findings) != 0 {
			t.Fatalf("shape refused: %v", findings)
		}
		plan, opRes := groomPlanFor(t, map[string]string{groomPath(2, "add-a-widget"): withPrefix}, baseGroomOp([]string{}, req))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		return string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	}
	if rec := run(t, nil); !strings.Contains(rec, "\nbranch_prefix: keep\n") {
		t.Errorf("nil branch_prefix changed the stored line:\n%s", rec)
	}
	if rec := run(t, strPtr("")); !strings.Contains(rec, "\nbranch_prefix:\n") || strings.Contains(rec, "keep") {
		t.Errorf("empty branch_prefix did not clear the field:\n%s", rec)
	}
	if rec := run(t, strPtr(" Spike/ ")); !strings.Contains(rec, "\nbranch_prefix: 'spike'\n") {
		t.Errorf("branch_prefix not normalized and set:\n%s", rec)
	}

	bad := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.BranchPrefix = strPtr("a/b") })
	var groomMsg string
	for _, f := range validateChangeGroomShape(bad) {
		if f.Code == string(FCInvalidBranchPrefix) {
			groomMsg = f.Message
		}
	}
	creq := validChangeCreateRequest()
	creq.BranchPrefix = "a/b"
	var createMsg string
	for _, f := range validateChangeCreateShape(creq) {
		if f.Code == string(FCInvalidBranchPrefix) {
			createMsg = f.Message
		}
	}
	if groomMsg == "" || groomMsg != createMsg {
		t.Errorf("groom invalid-branch_prefix message %q, want create's %q", groomMsg, createMsg)
	}
}

func TestChangeGroomRefusesUnconfiguredType(t *testing.T) {
	engine := &recordingEngine{}
	deps := PlanningDeps{Engine: engine, Reader: &fakeChangeReader{pin: mainModePin([]string{"inline"})}, Clock: testClock()}
	res := ChangeGroom(context.Background(), deps, "", fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.Type = "nope" }))
	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times, want 0", len(engine.calls))
	}
	want := unknownTypeMessage(planningTestConfig([]string{"inline"}), "nope")
	found := false
	for _, f := range res.Findings {
		if f.Code == string(FCUnknownType) {
			found = true
			if f.Message != want {
				t.Errorf("message = %q, want %q", f.Message, want)
			}
		}
	}
	if !found {
		t.Errorf("missing unknown-type; got %v", res.Findings)
	}
}

func TestChangeGroomRefusesUnknownDeliverable(t *testing.T) {
	engine := &recordingEngine{}
	deps := PlanningDeps{Engine: engine, Reader: &fakeChangeReader{pin: mainModePin([]string{"inline"})}, Clock: testClock()}
	res := ChangeGroom(context.Background(), deps, "", fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.Deliverable = "research" }))
	if res.Result != ResultInvalidInput || len(engine.calls) != 0 {
		t.Fatalf("result = %q engine calls %d, want invalid-input and 0", res.Result, len(engine.calls))
	}
	want := `unknown deliverable "research"; valid values are pr, results-artifact`
	for _, f := range res.Findings {
		if f.Code == string(FCUnknownDeliverable) {
			if f.Message != want {
				t.Errorf("message = %q, want %q", f.Message, want)
			}
			return
		}
	}
	t.Errorf("missing unknown-deliverable; got %v", res.Findings)
}

func TestChangeGroomStackingRefusals(t *testing.T) {
	// (a) same-request results-artifact + stacked_on: shape refusal.
	sameReq := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.Deliverable = "results-artifact"; r.StackedOn = intPtrTest(3) })
	if findings := validateChangeGroomShape(sameReq); !hasFindingCode(findings, string(FCResultsArtifactNotStackable)) {
		t.Errorf("(a) missing shape refusal; got %v", findings)
	}

	assertRefused := func(t *testing.T, files map[string]string, req ChangeGroomRequest) {
		t.Helper()
		if findings := validateChangeGroomShape(req); len(findings) != 0 {
			t.Fatalf("shape refused (want a Plan refusal): %v", findings)
		}
		plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, req))
		if !opRes.Refused || len(plan.Files) != 0 {
			t.Fatalf("want a refusal writing nothing, got refused=%v files=%v", opRes.Refused, planPaths(plan))
		}
		for _, f := range opRes.Findings {
			if f.Code == string(FCResultsArtifactNotStackable) {
				return
			}
		}
		t.Errorf("missing results-artifact-not-stackable; got %v", opRes.Findings)
	}
	toResults := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.Deliverable = "results-artifact" })

	// (b) the record is already stacked; the request sets only the deliverable.
	stackedFiles := map[string]string{
		groomPath(2, "add-a-widget"): stackedChange(2, "add-a-widget", 3),
		groomPath(3, "base-thing"):   groomableChange(3, "base-thing"),
	}
	t.Run("b record already stacked", func(t *testing.T) { assertRefused(t, stackedFiles, toResults) })

	// (c) stacked_on names a results-artifact parent.
	t.Run("c results-artifact parent", func(t *testing.T) {
		files := map[string]string{
			groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
			groomPath(3, "base-thing"):   resultsArtifactChange(3, "base-thing"),
		}
		req := stubSectionReviseRequest()
		req.StackedOn = intPtrTest(3)
		assertRefused(t, files, req)
	})

	// (d) an active change stacks on the change being turned results-artifact.
	t.Run("d active child stacks on it", func(t *testing.T) {
		files := map[string]string{
			groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
			groomPath(5, "child-thing"):  stackedChange(5, "child-thing", 2),
		}
		assertRefused(t, files, toResults)
	})

	// A revise touching neither field still applies, even on a record already
	// in the refused state (a stacked results-artifact change, hand-written).
	t.Run("unrelated revise applies", func(t *testing.T) {
		legacy := map[string]string{
			groomPath(2, "add-a-widget"): strings.Replace(resultsArtifactChange(2, "add-a-widget"), "stacked_on:\n", "stacked_on: 3\n", 1),
			groomPath(3, "base-thing"):   groomableChange(3, "base-thing"),
		}
		plan, opRes := groomPlanFor(t, legacy, baseGroomOp([]string{}, stubSectionReviseRequest()))
		if opRes.Refused || len(plan.Files) == 0 {
			t.Fatalf("unrelated revise refused=%v files=%v findings=%v", opRes.Refused, planPaths(plan), opRes.Findings)
		}
	})
}

func TestChangeGroomStackingRemediesNameTypedPath(t *testing.T) {
	toResults := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.Deliverable = "results-artifact" })
	cases := []struct {
		name  string
		files map[string]string
		extra []string
	}{
		{"b record already stacked", map[string]string{
			groomPath(2, "add-a-widget"): stackedChange(2, "add-a-widget", 3),
			groomPath(3, "base-thing"):   groomableChange(3, "base-thing"),
		}, nil},
		{"d active child stacks on it", map[string]string{
			groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
			groomPath(5, "child-thing"):  stackedChange(5, "child-thing", 2),
		}, []string{"0005", "proposed"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, opRes := groomPlanFor(t, c.files, baseGroomOp([]string{}, toResults))
			var msg string
			found := false
			for _, f := range opRes.Findings {
				if f.Code == string(FCResultsArtifactNotStackable) {
					found, msg = true, f.Detail["message"]
				}
			}
			if !found {
				t.Fatalf("missing results-artifact-not-stackable; got %v", opRes.Findings)
			}
			if !strings.Contains(msg, "stacked_on: 0") || strings.Contains(msg, "ask a human") {
				t.Errorf("remedy does not name the typed path: %q", msg)
			}
			for _, want := range c.extra {
				if !strings.Contains(msg, want) {
					t.Errorf("remedy missing %q: %q", want, msg)
				}
			}
		})
	}
}

func TestChangeGroomTypeEditChangesNextClaimBranch(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		want string
	}{
		{"type", func(r *ChangeGroomRequest) { r.Type = "chore" }, "chore/add-a-widget"},
		{"branch_prefix", func(r *ChangeGroomRequest) { r.BranchPrefix = strPtr("spike") }, "spike/add-a-widget"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget")}
			plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, fieldOnlyReviseRequest(c.mut)))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			files[groomPath(2, "add-a-widget")] = string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
			after, err := newPlanningLoader(planningTestConfig(nil)).Load(context.Background(), newFakeTree(files))
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			ch, out := after.Snapshot.Change(domain.ChangeID(2))
			if out != domain.LookupFound {
				t.Fatalf("change 2 not found after groom")
			}
			res, fail := domain.Claim(ch, testClock().Now())
			if fail != nil {
				t.Fatalf("claim failed: %v", fail)
			}
			if got := res.Change.Branch().Value; got != c.want {
				t.Errorf("claimed branch = %q, want %q", got, c.want)
			}
		})
	}
}

// --- stacked_on clear and missing keys (change 0563) ---

// bareKeysChange is shaped like active record 0007: no auto_groomable:,
// stacked_on:, or discovered_from: key (it keeps the artifacts block, which
// groom's ReplaceBlock requires).
func bareKeysChange(id int, slug string) string {
	s := strings.Replace(groomableChange(id, slug), "stacked_on:\n", "", 1)
	return strings.Replace(s, "discovered_from: []\n", "", 1)
}

// noUpdatedChange lacks both updated: and auto_groomable:.
func noUpdatedChange(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "updated: 2026-08-02\n", "", 1)
}

// requireLacks fails the test when the fixture src carries any of keys, so a
// fixture drift cannot make a missing-key test vacuous.
func requireLacks(t *testing.T, src string, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if strings.Contains(src, "\n"+k+":") {
			t.Fatalf("fixture unexpectedly carries %s:\n%s", k, src)
		}
	}
}

// requireKeyOnce fails unless rec carries exactly one "key:" occurrence.
func requireKeyOnce(t *testing.T, rec string, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if n := strings.Count(rec, k+":"); n != 1 {
			t.Errorf("record carries %s: %d times, want 1:\n%s", k, n, rec)
		}
	}
}

func groomPlanRecord(t *testing.T, files map[string]string, req ChangeGroomRequest) (transaction.MutationPlan, string) {
	t.Helper()
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	return plan, string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
}

func TestChangeGroomPlanClearStackedOn(t *testing.T) {
	path := groomPath(2, "add-a-widget")
	files := map[string]string{
		path:                       stackedChange(2, "add-a-widget", 3),
		groomPath(3, "base-thing"): groomableChange(3, "base-thing"),
		"docs/changes/BOARD.md":    "# Backlog\n\nold\n",
	}
	plan, rec := groomPlanRecord(t, files, fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.StackedOn = intPtrTest(0) }))
	if !strings.Contains(rec, "\nstacked_on:\n") || strings.Contains(rec, "stacked_on: 3") {
		t.Fatalf("stacked_on not cleared to the valueless key:\n%s", rec)
	}
	if !strings.Contains(strings.Join(planPaths(plan), "\n"), "docs/changes/BOARD.md") {
		t.Errorf("board not planned; files: %v", planPaths(plan))
	}
	if n := strings.Count(rec, "docket:artifacts:start"); n != 1 {
		t.Errorf("artifacts block start markers = %d, want 1", n)
	}

	files[path] = rec
	after, err := newPlanningLoader(planningTestConfig(nil)).Load(context.Background(), newFakeTree(files))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	ch, out := after.Snapshot.Change(domain.ChangeID(2))
	if out != domain.LookupFound {
		t.Fatalf("change 2 absent after reload")
	}
	got := domain.ResolveEffectiveBase(after.Snapshot, ch, domain.NewBranchFacts(map[string]bool{}))
	want := domain.EffectiveBase{Kind: domain.BaseResolved, Branch: after.Snapshot.Policy().IntegrationBranch}
	if got != want {
		t.Errorf("effective base = %+v, want %+v", got, want)
	}
}

func TestChangeGroomPlanClearStackedOnInsertsMissingKey(t *testing.T) {
	src := bareKeysChange(2, "add-a-widget")
	requireLacks(t, src, "stacked_on")
	files := map[string]string{
		groomPath(2, "add-a-widget"): src,
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	_, rec := groomPlanRecord(t, files, fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.StackedOn = intPtrTest(0) }))
	if !strings.Contains(rec, "\nstacked_on:\n") {
		t.Errorf("valueless stacked_on: not inserted:\n%s", rec)
	}
	requireKeyOnce(t, rec, "stacked_on")
}

func TestChangeGroomPlanMissingKeysReviseInsertsRelationships(t *testing.T) {
	src := bareKeysChange(2, "add-a-widget")
	requireLacks(t, src, "stacked_on", "discovered_from")
	files := map[string]string{
		groomPath(2, "add-a-widget"):        src,
		groomPath(3, "base-thing"):          groomableChange(3, "base-thing"),
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	_, rec := groomPlanRecord(t, files, fieldOnlyReviseRequest(func(r *ChangeGroomRequest) {
		r.StackedOn = intPtrTest(3)
		r.DiscoveredFrom = []int{1}
	}))
	for _, want := range []string{"\nstacked_on: 3\n", "\ndiscovered_from: [1]\n"} {
		if !strings.Contains(rec, want) {
			t.Errorf("record lacks %q:\n%s", want, rec)
		}
	}
	requireKeyOnce(t, rec, "stacked_on", "discovered_from")
}

func TestChangeGroomPlanMissingKeyExplicitEmptyListInserted(t *testing.T) {
	src := bareKeysChange(2, "add-a-widget")
	requireLacks(t, src, "discovered_from")
	files := map[string]string{groomPath(2, "add-a-widget"): src}
	_, rec := groomPlanRecord(t, files, fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.DiscoveredFrom = []int{} }))
	if !strings.Contains(rec, "\ndiscovered_from: []\n") {
		t.Errorf("explicit empty discovered_from not inserted:\n%s", rec)
	}
	requireKeyOnce(t, rec, "discovered_from")
}

func TestChangeGroomPlanMissingKeysAbstainAndReEnable(t *testing.T) {
	records := []struct {
		name  string
		src   string
		lacks []string
	}{
		{"0007-shaped", bareKeysChange(2, "add-a-widget"), []string{"auto_groomable", "stacked_on", "discovered_from"}},
		{"no updated", noUpdatedChange(2, "add-a-widget"), []string{"auto_groomable", "updated"}},
	}
	requests := []struct {
		name string
		req  ChangeGroomRequest
		want string
	}{
		{"abstain", abstainRequest(), "auto_groomable: false"},
		{"re-enable", reEnableRequest(), "auto_groomable: true"},
	}
	for _, r := range records {
		for _, q := range requests {
			t.Run(r.name+"/"+q.name, func(t *testing.T) {
				requireLacks(t, r.src, r.lacks...)
				files := map[string]string{groomPath(2, "add-a-widget"): r.src}
				_, rec := groomPlanRecord(t, files, q.req)
				for _, want := range []string{q.want, "updated: '2026-08-16'"} {
					if !strings.Contains(rec, want) {
						t.Errorf("record lacks %q:\n%s", want, rec)
					}
				}
				requireKeyOnce(t, rec, "auto_groomable", "updated")
			})
		}
	}
}

func TestChangeGroomPlanSpecOutcomeOnBareRecordInsertsEveryField(t *testing.T) {
	src := bareKeysChange(2, "add-a-widget")
	requireLacks(t, src, "auto_groomable", "stacked_on", "discovered_from")
	files := map[string]string{
		groomPath(2, "add-a-widget"):        src,
		groomPath(3, "base-thing"):          groomableChange(3, "base-thing"),
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	req := validGroomSpecRequest()
	req.DependsOn = []int{1}
	req.StackedOn = intPtrTest(3)
	req.DiscoveredFrom = []int{1}
	_, rec := groomPlanRecord(t, files, req)
	for _, want := range []string{
		"spec: 'docs/superpowers/specs/2026-08-16-add-a-widget-design.md'",
		"\ndepends_on: [1]\n", "\nstacked_on: 3\n", "\ndiscovered_from: [1]\n",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("record lacks %q:\n%s", want, rec)
		}
	}
	requireKeyOnce(t, rec, "spec", "depends_on", "stacked_on", "discovered_from")
}

func TestChangeGroomPlanResultsArtifactWithStackClear(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): stackedChange(2, "add-a-widget", 3),
		groomPath(3, "base-thing"):   groomableChange(3, "base-thing"),
	}
	_, rec := groomPlanRecord(t, files, fieldOnlyReviseRequest(func(r *ChangeGroomRequest) {
		r.Deliverable = "results-artifact"
		r.StackedOn = intPtrTest(0)
	}))
	for _, want := range []string{"\ndeliverable: 'results-artifact'\n", "\nstacked_on:\n"} {
		if !strings.Contains(rec, want) {
			t.Errorf("record lacks %q:\n%s", want, rec)
		}
	}
}

// --- relationship-only revise and stacked_on shape (change 0563) ---------

func TestChangeGroomEachRelationshipFieldAloneIsAValidRevise(t *testing.T) {
	cases := map[string]func(*ChangeGroomRequest){
		"depends_on [1]":      func(r *ChangeGroomRequest) { r.DependsOn = []int{1} },
		"depends_on []":       func(r *ChangeGroomRequest) { r.DependsOn = []int{} },
		"related [1]":         func(r *ChangeGroomRequest) { r.Related = []int{1} },
		"discovered_from [1]": func(r *ChangeGroomRequest) { r.DiscoveredFrom = []int{1} },
		"adrs [1]":            func(r *ChangeGroomRequest) { r.ADRs = []int{1} },
		"stacked_on 3":        func(r *ChangeGroomRequest) { r.StackedOn = intPtrTest(3) },
		"stacked_on 0":        func(r *ChangeGroomRequest) { r.StackedOn = intPtrTest(0) },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := validateChangeGroomShape(fieldOnlyReviseRequest(mut)); len(findings) != 0 {
				t.Fatalf("relationship-only revise refused: %v", findings)
			}
		})
	}
	findings := validateChangeGroomShape(fieldOnlyReviseRequest(func(*ChangeGroomRequest) {}))
	var msg string
	for _, f := range findings {
		if f.Code == string(FCEmptyRevise) {
			msg = f.Message
		}
	}
	if msg == "" {
		t.Fatalf("empty revise not refused; findings %v", findings)
	}
	for _, want := range []string{"depends_on", "related", "discovered_from", "adrs", "stacked_on", "deliverable", "type", "branch_prefix"} {
		if !strings.Contains(msg, want) {
			t.Errorf("empty-revise message %q does not name %q", msg, want)
		}
	}
}

func TestChangeGroomStackedOnShape(t *testing.T) {
	const wantMsg = "stacked_on must be a positive change id, or 0 to clear"
	trivial := ChangeGroomRequest{
		ChangeID: 2, Path: groomPath(2, "add-a-widget"),
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Outcome: GroomTrivial,
		Sections: []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "Too small to design.\n"}},
	}
	negatives := map[string]ChangeGroomRequest{
		"spec":      validGroomSpecRequest(),
		"trivial":   trivial,
		"revise":    fieldOnlyReviseRequest(func(*ChangeGroomRequest) {}),
		"re-enable": reEnableRequest(),
	}
	for name, req := range negatives {
		t.Run("negative "+name, func(t *testing.T) {
			req.StackedOn = intPtrTest(-1)
			found := false
			for _, f := range validateChangeGroomShape(req) {
				if f.Code == string(FCInvalidStackedOn) && f.Message == wantMsg {
					found = true
				}
			}
			if !found {
				t.Errorf("missing invalid-stacked_on %q", wantMsg)
			}
		})
	}
	ab := abstainRequest()
	ab.StackedOn = intPtrTest(0)
	if f := validateChangeGroomShape(ab); !hasFindingCode(f, "invalid-stacked_on") {
		t.Errorf("abstain with stacked_on 0 not refused: %v", f)
	}
	clear := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) {
		r.Deliverable = "results-artifact"
		r.StackedOn = intPtrTest(0)
	})
	if f := validateChangeGroomShape(clear); len(f) != 0 {
		t.Errorf("results-artifact with stacked_on 0 refused: %v", f)
	}
	stack := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) {
		r.Deliverable = "results-artifact"
		r.StackedOn = intPtrTest(3)
	})
	if f := validateChangeGroomShape(stack); !hasFindingCode(f, "results-artifact-not-stackable") {
		t.Errorf("results-artifact with stacked_on 3 not refused: %v", f)
	}
}

func TestChangeGroomNegativeStackedOnRefusedWithoutEngineCall(t *testing.T) {
	run := func(n int) (ChangeGroomResult, *recordingEngine) {
		engine := &recordingEngine{}
		reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
		deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}
		req := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.StackedOn = intPtrTest(n) })
		return ChangeGroom(context.Background(), deps, "", req), engine
	}
	res, engine := run(-1)
	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if !hasFindingCode(res.Findings, "invalid-stacked_on") {
		t.Errorf("missing invalid-stacked_on; got %v", res.Findings)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on a shape failure, want 0", len(engine.calls))
	}
	res, _ = run(0)
	if hasFindingCode(res.Findings, "empty-revise") {
		t.Errorf("stacked_on 0 revise refused as empty: %v", res.Findings)
	}
}

// --- not-restackable (change 0563) ----------------------------------------

// TestChangeGroomPlanNotRestackable pins that a stacked_on edit refuses
// not-restackable when it would change the base of a change whose branch: is
// already cut, and that re-sending the current value (or clearing an unstacked
// record) is no change and passes. plan: alone never blocks a re-stack.
func TestChangeGroomPlanNotRestackable(t *testing.T) {
	const planPath = "docs/superpowers/plans/2026-08-10-add-a-widget.md"
	branched := func(rec string) string {
		return strings.Replace(rec, "status: proposed\n", "status: proposed\nbranch: 'feat/add-a-widget'\n", 1)
	}
	planned := strings.Replace(stackedChange(2, "add-a-widget", 3), "plan:\n", "plan: '"+planPath+"'\n", 1)
	// The Plan gate's code is not reached by the shape-validator registry
	// probe, so pin its registration here.
	registered := false
	for _, code := range AllFindingCodes {
		registered = registered || code == FCNotRestackable
	}
	if !registered {
		t.Errorf("%s is not registered in AllFindingCodes", FCNotRestackable)
	}
	cases := []struct {
		name    string
		rec     string
		stack   int
		refused bool
	}{
		{"re-parent with branch refuses", branched(stackedChange(2, "add-a-widget", 3)), 4, true},
		{"clear with branch refuses", branched(stackedChange(2, "add-a-widget", 3)), 0, true},
		{"first stack with branch refuses", branched(groomableChange(2, "add-a-widget")), 3, true},
		{"unchanged parent with branch passes", branched(stackedChange(2, "add-a-widget", 3)), 3, false},
		{"clear of unstacked with branch passes", branched(groomableChange(2, "add-a-widget")), 0, false},
		{"plan-only re-parent passes", planned, 4, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(c.rec, "branch: 'feat/add-a-widget'") && !strings.Contains(c.rec, "plan: '"+planPath+"'") {
				t.Fatalf("fixture lost its branch:/plan: artifact:\n%s", c.rec)
			}
			files := map[string]string{
				groomPath(2, "add-a-widget"): c.rec,
				groomPath(3, "base-thing"):   groomableChange(3, "base-thing"),
				groomPath(4, "other-base"):   groomableChange(4, "other-base"),
				planPath:                     "# Plan\n",
			}
			stack := c.stack
			req := fieldOnlyReviseRequest(func(r *ChangeGroomRequest) { r.StackedOn = intPtrTest(stack) })
			plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, req))
			if !c.refused {
				if opRes.Refused {
					t.Fatalf("refused: %v", opRes.Findings)
				}
				return
			}
			if !opRes.Refused {
				t.Fatalf("re-stack of a branched change planned: %v", planPaths(plan))
			}
			var msg string
			found := false
			for _, f := range opRes.Findings {
				if f.Code == string(FCNotRestackable) {
					found, msg = true, f.Detail["message"]
				}
			}
			if !found {
				t.Fatalf("missing not-restackable; got %v", opRes.Findings)
			}
			if !strings.Contains(msg, "feat/add-a-widget") {
				t.Errorf("message does not name the branch: %q", msg)
			}
			if strings.Contains(msg, "rebuild") {
				t.Errorf("message names a remedy that does not lift the refusal: %q", msg)
			}
			if !strings.Contains(msg, "branch: field") {
				t.Errorf("message does not tell the reader to clear the branch: field: %q", msg)
			}
			if len(plan.Files) != 0 {
				t.Errorf("refused plan still carries files: %v", planPaths(plan))
			}
		})
	}
}
