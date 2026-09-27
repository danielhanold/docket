package app

import (
	"context"
	"encoding/json"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"slices"
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
// it is the spec_version that matches the linked spec on a fake tree.
const fakeTreeBlobID = "a"

// validReviseRequest is a well-formed revise request (sections + spec body)
// against the revisable fixture at id 2 / slug add-a-widget.
func validReviseRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID:     2,
		Path:         groomPath(2, "add-a-widget"),
		Version:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:      GroomRevise,
		SpecMarkdown: "# Design\n\nThe revised design body.\n",
		SpecVersion:  fakeTreeBlobID,
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
		{"sections-only revise passes", func(r *ChangeGroomRequest) { r.SpecMarkdown, r.SpecVersion = "", "" }, ""},
		{"spec-only revise passes", func(r *ChangeGroomRequest) { r.Sections = nil }, ""},
		{"empty revise refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.Sections = nil
		}, "empty-revise"},
		{"all-preserve revise refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "preserve"}}
		}, "empty-revise"},
		{"relationships-only revise refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.Sections = nil
			r.DependsOn = []int{1}
		}, "empty-revise"},
		{"unparseable revise spec_markdown refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = "---\nid: 1\n"
		}, "invalid-spec_markdown"},
		// An author resubmitting the spec file as read keeps its backlink block;
		// the revise re-renders that block itself, so the resubmission is refused
		// rather than committed with a duplicate marker pair.
		{"revise spec_markdown carrying a backlink block refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = groomBacklinkedSpecMarkdown
		}, "invalid-spec_markdown"},
		// A spec-body revise overwrites the spec file, so it must pin its version.
		{"spec revise without spec_version refused", func(r *ChangeGroomRequest) {
			r.SpecVersion = ""
		}, "empty-spec_version"},
		{"sections-only revise without spec_version passes", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecVersion = "", ""
		}, ""},
		// spec_version pins only a spec-body revise; anywhere else it would be
		// silently unchecked, so it is refused.
		{"sections-only revise with spec_version refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
		}, "invalid-spec_version"},
		{"spec outcome with spec_version refused", func(r *ChangeGroomRequest) {
			r.Outcome = GroomSpec
		}, "invalid-spec_version"},
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

// TestChangeGroomSpecReviseWithoutSpecVersionRefusedWithoutEngineCall pins the
// review finding: a spec-body revise that does not pin the spec file's blob
// version is a shape refusal, never an unpinned whole-body replace.
func TestChangeGroomSpecReviseWithoutSpecVersionRefusedWithoutEngineCall(t *testing.T) {
	req := validReviseRequest()
	req.SpecVersion = ""
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
	if !hasFindingCode(res.Findings, "empty-spec_version") {
		t.Errorf("missing finding empty-spec_version; got %v", res.Findings)
	}
}

// TestChangeGroomSpecVersionMismatchMapsToContended pins the result mapping: a
// plan refused with spec-version-mismatch is the spec analogue of a stale record
// pin, so the caller sees contended (re-read and retry), not invalid-state.
func TestChangeGroomSpecVersionMismatchMapsToContended(t *testing.T) {
	_, opRes := groomPlanFor(t, reviseFixtureFiles(), baseGroomOp([]string{}, validReviseRequest()))
	if opRes.Refused {
		t.Fatalf("precondition: a matching spec_version must plan; got %v", opRes.Findings)
	}
	res := changeGroomResultFromOutcome(transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings: []domain.Finding{{
			Code: "spec-version-mismatch", Severity: domain.SeverityError,
			Entity: domain.EntityRef{Kind: domain.EntityChange},
		}},
	}, nil)
	if res.Result != ResultContended {
		t.Errorf("spec-version-mismatch refusal = %q, want contended", res.Result)
	}
	other := changeGroomResultFromOutcome(transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings:    []domain.Finding{{Code: "not-revisable", Severity: domain.SeverityError}},
	}, nil)
	if other.Result != ResultInvalidState {
		t.Errorf("not-revisable refusal = %q, want invalid-state", other.Result)
	}
}

// TestChangeGroomReviseSpecVersionContendsRealGit drives the finding's exact
// race through a real engine and a bare origin: two revises pinned to the SAME
// record version (a same-day spec-only revise leaves the record bytes
// unchanged) and the same spec version. The first applies; the second's spec
// pin is stale, so it contends and writes nothing instead of clobbering the
// first revise's spec body.
func TestChangeGroomReviseSpecVersionContendsRealGit(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(2, "add-a-widget")
	repo := newWorkingRepo(t, reviseFixtureFiles())
	node := planningDepsFor(t, repo.invocation)

	revise := func(body, recV, specV string) ChangeGroomResult {
		req := validReviseRequest()
		req.Sections = nil
		req.SpecMarkdown = "# Design\n\n" + body + "\n"
		req.Version, req.SpecVersion = recV, specV
		return ChangeGroom(context.Background(), node.deps, node.dir, req)
	}

	// Settle the record (updated: today, artifacts rendered) with a matching
	// pin — the matching-version apply path.
	if res := revise("Settling body.", blobVersionAt(t, repo.origin, "docket", recPath),
		blobVersionAt(t, repo.origin, "docket", reviseSpecPath)); res.Result != ResultApplied {
		t.Fatalf("settling revise = %q (findings %v), want applied", res.Result, res.Findings)
	}
	recV := blobVersionAt(t, repo.origin, "docket", recPath)
	specV := blobVersionAt(t, repo.origin, "docket", reviseSpecPath)

	if res := revise("Body A.", recV, specV); res.Result != ResultApplied {
		t.Fatalf("revise A = %q (findings %v), want applied", res.Result, res.Findings)
	}
	if got := blobVersionAt(t, repo.origin, "docket", recPath); got != recV {
		t.Fatalf("precondition: a same-day spec-only revise must leave the record version unchanged (%s -> %s)", recV, got)
	}
	tip := originTip(t, repo.origin, "docket")

	res := revise("Body B.", recV, specV) // stale spec pin, current record pin
	if res.Result != ResultContended {
		t.Fatalf("revise B over a stale spec_version = %q (findings %v), want contended", res.Result, res.Findings)
	}
	if !hasFindingCode(res.Findings, "spec-version-mismatch") {
		t.Errorf("missing finding spec-version-mismatch; got %v", res.Findings)
	}
	if got := originTip(t, repo.origin, "docket"); got != tip {
		t.Errorf("a contended revise moved the metadata branch %s -> %s", tip, got)
	}
	spec, _ := originFile(t, repo.origin, "docket", reviseSpecPath)
	if !strings.Contains(spec, "Body A.") || strings.Contains(spec, "Body B.") {
		t.Errorf("revise A's spec body was clobbered:\n%s", spec)
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
// proposed to status at recPath (a terminal status lives under archive/), extra
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
	req.SpecMarkdown, req.SpecVersion = "", ""
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
		// Spec item 6: a needs-brainstorm change is groom's target, not revise's.
		{"not-revisable needs-brainstorm", map[string]string{
			groomPath(2, "add-a-widget"): groomableChange(2, "add-a-widget"),
		}, func(r *ChangeGroomRequest) {}, "not-revisable"},
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
		// spec_version refuses rather than overwriting a newer spec body.
		{"spec-version-mismatch", reviseFixtureFiles(), func(r *ChangeGroomRequest) {
			r.SpecVersion = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}, "spec-version-mismatch"},
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
		Version:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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
		{"abstain with spec_version", func(r *ChangeGroomRequest) { r.SpecVersion = "a" }, "invalid-spec_version"},
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

// rearmRequest is a well-formed rearm request against the fixture at id 2.
func rearmRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID: 2,
		Path:     groomPath(2, "add-a-widget"),
		Version:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:  GroomRearm,
	}
}

func TestChangeGroomRearmShapeValidation(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		code string
	}{
		{"bare rearm passes", func(r *ChangeGroomRequest) {}, ""},
		{"rearm with owned-section edits passes", func(r *ChangeGroomRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Open questions", Intent: "replace", Markdown: "Resolved.\n"}}
		}, ""},
		{"rearm with blocked_note", func(r *ChangeGroomRequest) { r.BlockedNote = "x\n" }, "invalid-blocked_note"},
		{"rearm with spec_markdown", func(r *ChangeGroomRequest) { r.SpecMarkdown = "# Design\n" }, "invalid-spec_markdown"},
		{"rearm with spec_version", func(r *ChangeGroomRequest) { r.SpecVersion = "a" }, "invalid-spec_version"},
		// Review Focus 5: the op removes this section itself.
		{"rearm editing ## Auto-groom blocked", func(r *ChangeGroomRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Auto-groom blocked", Intent: "replace", Markdown: "x\n"}}
		}, "invalid-section-heading"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := rearmRequest()
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

func TestChangeGroomPlanRearmClearsSectionSetsFlagAndBoard(t *testing.T) {
	files := map[string]string{
		groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget"),
		"docs/changes/BOARD.md":      "# Backlog\n\nold\n",
	}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{"inline"}, rearmRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if strings.Contains(rec, "## Auto-groom blocked") || strings.Contains(rec, "First note.") {
		t.Errorf("re-arm left the presence-encoded section behind:\n%s", rec)
	}
	if !strings.Contains(rec, "\nauto_groomable: true\n") || strings.Contains(rec, "auto_groomable: false") {
		t.Errorf("re-arm did not set auto_groomable: true:\n%s", rec)
	}
	board := string(groomedRecordBytes(t, plan, "docs/changes/BOARD.md"))
	if strings.Contains(board, "auto-groom blocked — needs you") || !strings.Contains(board, "needs-brainstorm") {
		t.Errorf("board row did not return to needs-brainstorm in the same plan:\n%s", board)
	}
	var receipt changeGroomReceipt
	if err := json.Unmarshal(plan.Receipt, &receipt); err != nil || receipt.Outcome != "rearm" {
		t.Errorf("receipt = %s (%v), want outcome rearm", plan.Receipt, err)
	}
}

func TestChangeGroomPlanRearmAppliesSectionEditsInOneRecord(t *testing.T) {
	req := rearmRequest()
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

func TestChangeGroomPlanRearmArmsWithoutABlockedSection(t *testing.T) {
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
			plan, opRes := groomPlanFor(t, map[string]string{groomPath(2, "add-a-widget"): c.rec}, baseGroomOp([]string{}, rearmRequest()))
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			if rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget"))); !strings.Contains(rec, "\nauto_groomable: true\n") {
				t.Errorf("auto_groomable: true not written:\n%s", rec)
			}
		})
	}
}

func TestChangeGroomPlanRearmRefusals(t *testing.T) {
	armed := strings.Replace(groomableChange(2, "add-a-widget"), "trivial: false\n", "trivial: false\nauto_groomable: true\n", 1)
	cases := []struct {
		name  string
		files map[string]string
		code  string
	}{
		{"nothing to re-arm", map[string]string{groomPath(2, "add-a-widget"): armed}, "nothing-to-rearm"},
		{"trivial", map[string]string{groomPath(2, "add-a-widget"): trivialChange(2, "add-a-widget")}, "not-groomable"},
		{"spec'd", reviseFixtureFiles(), "not-groomable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, opRes := groomPlanFor(t, c.files, baseGroomOp([]string{}, rearmRequest()))
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

// TestChangeGroomRearmFencedMarkerAgreesWithBoard — a heading-shaped
// "## Auto-groom blocked" line inside fenced code is not the abstain section:
// the decoded record (which the board's cell keys on) and rearm's section scan
// must agree, so the board never shows "needs you" on a record rearm reports
// as having nothing to re-arm.
func TestChangeGroomRearmFencedMarkerAgreesWithBoard(t *testing.T) {
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
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, rearmRequest()))
	found := false
	for _, f := range opRes.Findings {
		found = found || f.Code == "nothing-to-rearm"
	}
	if !opRes.Refused || len(plan.Files) != 0 || !found {
		t.Errorf("rearm over a fenced marker: refused=%v files=%v findings=%v, want nothing-to-rearm", opRes.Refused, planPaths(plan), opRes.Findings)
	}
}

func TestChangeGroomResultHumanTextRearm(t *testing.T) {
	r := newChangeGroomResult(ResultApplied, ChangeGroomResult{ID: 7, Outcome: string(GroomRearm), Revision: "cafe"})
	if got, want := r.HumanText(), "change 0007 re-armed for auto-groom — cafe"; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}

// TestChangeGroomAbstainThenRearmRealGit drives both outcomes through the real
// engine and a bare origin: the abstain lands the record and BOARD.md in ONE
// commit; a re-arm pinned to the pre-abstain version contends and writes
// nothing; a re-arm at the current version restores needs-brainstorm.
func TestChangeGroomAbstainThenRearmRealGit(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(2, "add-a-widget")
	repo := newWorkingRepo(t, map[string]string{recPath: groomableChange(2, "add-a-widget")})
	node := planningDepsFor(t, repo.invocation)

	ab := abstainRequest()
	ab.Version = blobVersionAt(t, repo.origin, "docket", recPath)
	if res := ChangeGroom(context.Background(), node.deps, node.dir, ab); res.Result != ResultApplied {
		t.Fatalf("abstain = %q (findings %v), want applied", res.Result, res.Findings)
	}
	tip := originTip(t, repo.origin, "docket")
	paths := originCommitPaths(t, repo.origin, tip)
	if !slices.Contains(paths, recPath) || !slices.Contains(paths, "docs/changes/BOARD.md") {
		t.Fatalf("abstain commit paths = %v, want the record and BOARD.md in one commit", paths)
	}
	if board, _ := originFile(t, repo.origin, "docket", "docs/changes/BOARD.md"); !strings.Contains(board, "auto-groom blocked — needs you") {
		t.Errorf("committed board does not show the abstain:\n%s", board)
	}

	stale := rearmRequest()
	stale.Version = ab.Version // pre-abstain pin
	if res := ChangeGroom(context.Background(), node.deps, node.dir, stale); res.Result != ResultContended {
		t.Fatalf("stale re-arm = %q (findings %v), want contended", res.Result, res.Findings)
	}
	if got := originTip(t, repo.origin, "docket"); got != tip {
		t.Fatalf("a contended re-arm moved the metadata branch %s -> %s", tip, got)
	}

	fresh := rearmRequest()
	fresh.Version = blobVersionAt(t, repo.origin, "docket", recPath)
	if res := ChangeGroom(context.Background(), node.deps, node.dir, fresh); res.Result != ResultApplied {
		t.Fatalf("re-arm = %q (findings %v), want applied", res.Result, res.Findings)
	}
	rec, _ := originFile(t, repo.origin, "docket", recPath)
	board, _ := originFile(t, repo.origin, "docket", "docs/changes/BOARD.md")
	if strings.Contains(rec, "## Auto-groom blocked") || !strings.Contains(rec, "\nauto_groomable: true\n") {
		t.Errorf("re-armed record:\n%s", rec)
	}
	if strings.Contains(board, "auto-groom blocked — needs you") {
		t.Errorf("committed board still shows the abstain after re-arm:\n%s", board)
	}
}

// followedSection is a non-final section placed after ## Auto-groom blocked so
// abstain-append and rearm-removal are proven not to consume what follows.
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

func TestChangeGroomPlanRearmRemovesSectionBeforeFollowingSection(t *testing.T) {
	files := map[string]string{groomPath(2, "add-a-widget"): abstainedChange(2, "add-a-widget") + "\n" + followedSection}
	plan, opRes := groomPlanFor(t, files, baseGroomOp([]string{}, rearmRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := string(groomedRecordBytes(t, plan, groomPath(2, "add-a-widget")))
	if strings.Contains(rec, "## Auto-groom blocked") || strings.Contains(rec, "First note.") {
		t.Errorf("re-arm left the blocked section behind:\n%s", rec)
	}
	if strings.Count(rec, followedSection) != 1 || !strings.HasSuffix(rec, followedSection) {
		t.Errorf("following section not preserved byte-identically:\n%s", rec)
	}
}
