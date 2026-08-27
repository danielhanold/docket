package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
	"time"
)

// --- fakes ----------------------------------------------------------------

// fakeChangeReader is a StatusReader whose PinContext returns a canned pin; the
// change-create operation calls no other reader method.
type fakeChangeReader struct {
	pin    StatusPin
	pinErr error
	pinned int
}

func (r *fakeChangeReader) PinContext(_ context.Context, _ string) (StatusPin, error) {
	r.pinned++
	return r.pin, r.pinErr
}
func (r *fakeChangeReader) ReadCorpus(context.Context, StatusPin) ([]StatusBlob, error) {
	return nil, nil
}
func (r *fakeChangeReader) BranchFacts(context.Context, StatusPin, []string) (domain.BranchFacts, error) {
	return domain.BranchFacts{}, nil
}
func (r *fakeChangeReader) ArtifactExists(context.Context, StatusPin, string, string) (bool, error) {
	return false, nil
}
func (r *fakeChangeReader) ReadArtifact(context.Context, StatusPin, string, string) (StatusArtifact, error) {
	return StatusArtifact{}, nil
}

// recordingEngine records every Execute call and returns a scripted outcome.
type recordingEngine struct {
	calls  []transaction.Request
	result transaction.Result
	err    error
}

func (e *recordingEngine) Execute(_ context.Context, req transaction.Request) (transaction.Result, error) {
	e.calls = append(e.calls, req)
	return e.result, e.err
}

// fixedClock is a transaction.Clock pinned to one instant.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func testClock() fixedClock {
	return fixedClock{t: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)}
}

// mainModePin builds a main-mode StatusPin carrying the resolved configuration
// with the given board surfaces.
func mainModePin(surfaces []string) StatusPin {
	return StatusPin{
		Mode:          metadataModeMain,
		DefaultBranch: "main",
		Config:        config.Snapshot{Effective: planningTestConfig(surfaces)},
	}
}

// validChangeCreateRequest is a well-formed request the shape/config checks pass.
func validChangeCreateRequest() ChangeCreateRequest {
	return ChangeCreateRequest{
		RequestID:   "req-00000001",
		Title:       "Add a widget",
		Type:        "feat",
		Priority:    "high",
		Why:         "Because we need it.\n",
		WhatChanges: "Adds the widget.\n",
		OutOfScope:  "Everything else.\n",
	}
}

// --- request-shape validation (no engine call) ----------------------------

func TestChangeCreateRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeCreateRequest)
		code string
	}{
		{"short request id", func(r *ChangeCreateRequest) { r.RequestID = "short" }, "invalid-request-id"},
		{"empty title", func(r *ChangeCreateRequest) { r.Title = "" }, "empty-title"},
		{"blank why", func(r *ChangeCreateRequest) { r.Why = "   " }, "empty-why"},
		{"empty what", func(r *ChangeCreateRequest) { r.WhatChanges = "" }, "empty-what_changes"},
		{"empty out of scope", func(r *ChangeCreateRequest) { r.OutOfScope = "" }, "empty-out_of_scope"},
		{"duplicate depends_on", func(r *ChangeCreateRequest) { r.DependsOn = []int{3, 3} }, "duplicate-depends_on"},
		{"non-positive related", func(r *ChangeCreateRequest) { r.Related = []int{0} }, "invalid-related"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validChangeCreateRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeCreate(context.Background(), deps, "", req)

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

func TestChangeCreateRejectsUnknownTypeAndPriority(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeCreateRequest)
		code string
	}{
		{"unknown type", func(r *ChangeCreateRequest) { r.Type = "nope" }, "unknown-type"},
		{"unknown priority", func(r *ChangeCreateRequest) { r.Priority = "urgent" }, "unknown-priority"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validChangeCreateRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeCreate(context.Background(), deps, "", req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called on a config failure, want 0")
			}
			if !hasFindingCode(res.Findings, c.code) {
				t.Errorf("missing finding %q; got %v", c.code, res.Findings)
			}
		})
	}
}

func TestChangeCreateFencesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline", "github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeCreate(context.Background(), deps, "", validChangeCreateRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a fenced board surface")
	}
}

// --- outcome mapping (engine reached; Discover over a real temp repo) ------

// --- plan closure ----------------------------------------------------------

// planFor drives the change-create SemanticOperation directly over an in-memory
// before-state, returning the plan and its outcome.
func planFor(t *testing.T, files map[string]string, op changeCreateOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func baseOp(surfaces []string) changeCreateOp {
	return changeCreateOp{
		req:        validChangeCreateRequest(),
		eff:        planningTestConfig(surfaces),
		slug:       "add-a-widget",
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

func TestChangeCreatePlanFileSetInlineReplacesExistingBoard(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
		"docs/changes/BOARD.md":             "# Backlog\n\nold\n",
	}
	plan, opRes := planFor(t, files, baseOp([]string{"inline"}))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		"docs/changes/active/0002-add-a-widget.md": transaction.MutationCreate,
		"docs/changes/BOARD.md":                    transaction.MutationReplace,
	})
	if plan.CommitSubject == "" {
		t.Error("empty commit subject")
	}
	assertCanonicalReceipt(t, plan.Receipt, 2, "add-a-widget", "docs/changes/active/0002-add-a-widget.md")
}

func TestChangeCreatePlanCreatesBoardWhenAbsent(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	plan, _ := planFor(t, files, baseOp([]string{"inline"}))
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		"docs/changes/active/0002-add-a-widget.md": transaction.MutationCreate,
		"docs/changes/BOARD.md":                    transaction.MutationCreate,
	})
}

func TestChangeCreatePlanNoBoardWhenSurfaceEmpty(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	plan, _ := planFor(t, files, baseOp([]string{}))
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		"docs/changes/active/0002-add-a-widget.md": transaction.MutationCreate,
	})
}

// fixtureArchivedDone renders a minimal well-formed archived (done) change; an
// archive-placed record must carry a terminal status.
func fixtureArchivedDone(id int, slug string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %d\n", id))
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A change'\n")
	b.WriteString("status: done\n")
	b.WriteString("priority: medium\n")
	b.WriteString("type: feat\n")
	b.WriteString("created: 2026-08-01\n")
	b.WriteString("updated: 2026-08-02\n")
	b.WriteString("---\n\n## Why\n\nBody.\n")
	return b.String()
}

func TestChangeCreatePlanAllocatesMaxPlusOneAcrossGaps(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0002-two.md":              fixtureChange(2, "two"),
		"docs/changes/archive/2026-01-01-0005-five.md": fixtureArchivedDone(5, "five"),
	}
	plan, _ := planFor(t, files, baseOp([]string{}))
	// max(2, 5) + 1 = 6; the gap at 1, 3, 4 is never backfilled.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		"docs/changes/active/0006-add-a-widget.md": transaction.MutationCreate,
	})
}

func TestChangeCreatePlanRefusesDanglingReference(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	op := baseOp([]string{})
	op.req.DependsOn = []int{999} // no such change
	plan, opRes := planFor(t, files, op)
	if !opRes.Refused {
		t.Fatalf("dangling depends_on was not refused")
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", plan.Files)
	}
	sawDangling := false
	for _, f := range opRes.Findings {
		if f.Code == "dangling-reference" && f.Field == "depends_on" {
			sawDangling = true
		}
	}
	if !sawDangling {
		t.Errorf("missing dangling-reference finding: %v", opRes.Findings)
	}
}

func TestChangeCreatePlanFillsArtifactBlockForADRs(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
		"docs/adrs/0001-a-decision.md":      fixtureADR(1, "a-decision"),
	}
	op := baseOp([]string{})
	op.req.ADRs = []int{1}
	plan, opRes := planFor(t, files, op)
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	var recordBytes []byte
	for _, f := range plan.Files {
		if strings.HasSuffix(string(f.Path), "0002-add-a-widget.md") {
			recordBytes = f.Bytes
		}
	}
	if recordBytes == nil {
		t.Fatal("new record not planned")
	}
	body := string(recordBytes)
	if !strings.Contains(body, "| ADRs |") {
		t.Errorf("artifact block missing ADRs row:\n%s", body)
	}
	if !strings.Contains(body, "docs/adrs/0001-a-decision.md") {
		t.Errorf("ADR reference not resolved into the artifact block:\n%s", body)
	}
}

func TestChangeCreatePlanFreshIDAcrossMovedBase(t *testing.T) {
	first, _ := planFor(t, map[string]string{
		"docs/changes/active/0003-three.md": fixtureChange(3, "three"),
	}, baseOp([]string{}))
	second, _ := planFor(t, map[string]string{
		"docs/changes/active/0007-seven.md": fixtureChange(7, "seven"),
	}, baseOp([]string{}))

	if got := planPathSuffix(first); got != "0004-add-a-widget.md" {
		t.Errorf("first attempt id = %q, want 0004", got)
	}
	if got := planPathSuffix(second); got != "0008-add-a-widget.md" {
		t.Errorf("second attempt id = %q, want 0008", got)
	}
}

// --- helpers ---------------------------------------------------------------

func hasFindingCode(findings []StatusFinding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func assertPlanPaths(t *testing.T, plan transaction.MutationPlan, want map[string]transaction.MutationKind) {
	t.Helper()
	got := make(map[string]transaction.MutationKind, len(plan.Files))
	for _, f := range plan.Files {
		got[string(f.Path)] = f.Kind
	}
	if len(got) != len(want) {
		t.Fatalf("plan paths = %v, want %v", got, want)
	}
	for p, k := range want {
		if got[p] != k {
			t.Errorf("path %q kind = %q, want %q", p, got[p], k)
		}
	}
}

func planPathSuffix(plan transaction.MutationPlan) string {
	for _, f := range plan.Files {
		s := string(f.Path)
		if strings.Contains(s, "/active/") {
			return s[strings.LastIndex(s, "/")+1:]
		}
	}
	return ""
}

func assertCanonicalReceipt(t *testing.T, receipt []byte, id int, slug, path string) {
	t.Helper()
	var rec changeCreateReceipt
	if err := json.Unmarshal(receipt, &rec); err != nil {
		t.Fatalf("receipt does not decode: %v", err)
	}
	if rec.ID != id || rec.Slug != slug || rec.Path != path || rec.Op != OperationChangeCreate {
		t.Errorf("receipt = %+v", rec)
	}
	// Canonical: compact, sorted keys — re-marshalling the decoded generic value
	// reproduces the exact bytes.
	var generic any
	if err := json.Unmarshal(receipt, &generic); err != nil {
		t.Fatalf("receipt generic decode: %v", err)
	}
	remar, err := json.Marshal(generic)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(remar) != string(receipt) {
		t.Errorf("receipt is not canonical:\n got %s\nwant %s", receipt, remar)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
