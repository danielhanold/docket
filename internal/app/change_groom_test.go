package app

import (
	"context"
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

// validReviseRequest is a well-formed revise request (sections + spec body)
// against the revisable fixture at id 2 / slug add-a-widget.
func validReviseRequest() ChangeGroomRequest {
	return ChangeGroomRequest{
		ChangeID:     2,
		Path:         groomPath(2, "add-a-widget"),
		Version:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Outcome:      GroomRevise,
		SpecMarkdown: "# Design\n\nThe revised design body.\n",
		SpecPath:     reviseSpecPath,
		SpecVersion:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
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
		{"sections-only revise passes", func(r *ChangeGroomRequest) { r.SpecMarkdown = "" }, ""},
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
		// A spec-body revise overwrites the spec file, so it must pin it.
		{"spec revise without spec_version refused", func(r *ChangeGroomRequest) {
			r.SpecVersion = ""
		}, "empty-spec_version"},
		{"spec revise without spec_path refused", func(r *ChangeGroomRequest) {
			r.SpecPath = ""
		}, "empty-spec_path"},
		{"spec revise without any spec pin refused", func(r *ChangeGroomRequest) {
			r.SpecPath, r.SpecVersion = "", ""
		}, "empty-spec_version"},
		{"sections-only revise without a spec pin passes", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecPath, r.SpecVersion = "", "", ""
		}, ""},
		// The pin fields travel together on any outcome: a half pin is refused.
		{"sections-only revise half pin refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecVersion = "", ""
		}, "empty-spec_version"},
		{"sections-only revise path-less pin refused", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecPath = "", ""
		}, "empty-spec_path"},
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

// TestChangeGroomRevisePinsSpecExpectation proves the spec pin reaches the
// engine: a spec-body revise submits the record expectation AND a second
// exact-blob expectation on the linked spec path, while a revise with no spec
// pin submits the record expectation alone.
func TestChangeGroomRevisePinsSpecExpectation(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeGroomRequest)
		want map[string]string // path -> pinned blob id
	}{
		{"spec-body revise pins record and spec", func(r *ChangeGroomRequest) {}, map[string]string{
			groomPath(2, "add-a-widget"): validReviseRequest().Version,
			reviseSpecPath:               validReviseRequest().SpecVersion,
		}},
		{"sections-only revise without a pin pins only the record", func(r *ChangeGroomRequest) {
			r.SpecMarkdown, r.SpecPath, r.SpecVersion = "", "", ""
		}, map[string]string{
			groomPath(2, "add-a-widget"): validReviseRequest().Version,
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repoDir := newWorkingRepo(t, nil).invocation
			engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}
			req := validReviseRequest()
			c.mut(&req)

			ChangeGroom(context.Background(), deps, repoDir, req)

			if len(engine.calls) != 1 {
				t.Fatalf("engine calls = %d, want 1", len(engine.calls))
			}
			got := map[string]string{}
			for _, e := range engine.calls[0].Expected {
				if e.Version.Kind != transaction.VersionBlob {
					t.Errorf("expectation %q kind = %q, want blob", e.Path, e.Version.Kind)
				}
				got[string(e.Path)] = string(e.Version.ObjectID)
			}
			if len(got) != len(c.want) {
				t.Fatalf("expectations = %v, want %v", got, c.want)
			}
			for p, v := range c.want {
				if got[p] != v {
					t.Errorf("expectation on %q = %q, want %q (all: %v)", p, got[p], v, got)
				}
			}
		})
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
	req.SpecMarkdown, req.SpecPath, req.SpecVersion = "", "", ""
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
		// Review Focus 3: dangling spec link — spec: names a path absent from
		// the tree; never silently mint a file.
		{"spec-file-missing", map[string]string{
			groomPath(2, "add-a-widget"): revisableChange(2, "add-a-widget", reviseSpecPath),
		}, func(r *ChangeGroomRequest) {}, "spec-file-missing"},
		// The spec pin must name the change's linked spec: a pin on any other
		// path would leave the file actually overwritten unpinned.
		{"spec-path-mismatch", reviseFixtureFiles(), func(r *ChangeGroomRequest) {
			r.SpecPath = "docs/superpowers/specs/2026-08-01-other-design.md"
		}, "spec-path-mismatch"},
		{"spec-path-mismatch sections-only pin", reviseFixtureFiles(), func(r *ChangeGroomRequest) {
			r.SpecMarkdown = ""
			r.SpecPath = "docs/superpowers/specs/2026-08-01-other-design.md"
		}, "spec-path-mismatch"},
		{"spec-path-mismatch pin on a trivial change", map[string]string{
			groomPath(2, "add-a-widget"): trivialChange(2, "add-a-widget"),
		}, func(r *ChangeGroomRequest) { r.SpecMarkdown = "" }, "spec-path-mismatch"},
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
