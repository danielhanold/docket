package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
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

// mainModePin builds a StatusPin carrying the resolved configuration with the
// given board surfaces. (Named for the historical main topology; there is now a
// single docket topology, so it is an ordinary pin fixture.)
func mainModePin(surfaces []string) StatusPin {
	return StatusPin{
		DefaultBranch: "main",
		Config:        config.Snapshot{Effective: planningTestConfig(surfaces)},
		Layout:        testSharedLayout(),
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
		{"short request id", func(r *ChangeCreateRequest) { r.RequestID = "short" }, "invalid-request_id"},
		{"empty title", func(r *ChangeCreateRequest) { r.Title = "" }, "empty-title"},
		{"multi-line title", func(r *ChangeCreateRequest) { r.Title = "Add\na widget" }, "invalid-title"},
		{"control-character title", func(r *ChangeCreateRequest) { r.Title = "Add\x07a widget" }, "invalid-title"},
		{"line-separator title", func(r *ChangeCreateRequest) { r.Title = "Add\u2028a widget" }, "invalid-title"},
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

func TestChangeCreateRefusesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline", "github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeCreate(context.Background(), deps, "", validChangeCreateRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a refused github board surface")
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
// archive-placed record must carry a final status.
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
	if !strings.Contains(body, "| ADRs | [ADR-0001](../../adrs/0001-a-decision.md) |") {
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

// TestChangeCreateRecordHasNoRepairFindings is change 0447's end-to-end guard:
// a record written by change.create — with an adversarial title carrying a
// leading "-", ": ", an apostrophe, " #", and the word "yes" — is reported
// clean by the repair planner that feeds `docket repository check` and the
// `repository migrate` preview. Before the fix every writer-quoted string
// field (slug, title, type, …) produced a frontmatter-manual-review finding.
func TestChangeCreateRecordHasNoRepairFindings(t *testing.T) {
	const title = "-lead: it's a #tag, yes"
	files := map[string]string{
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	op := baseOp([]string{})
	op.req.Title = title
	plan, opRes := planFor(t, files, op)
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	var recPath string
	var rec []byte
	for _, f := range plan.Files {
		if strings.HasSuffix(string(f.Path), "0002-add-a-widget.md") {
			recPath, rec = string(f.Path), f.Bytes
		}
	}
	if rec == nil {
		t.Fatal("new record not planned")
	}

	// The adversarial title really landed (the guard is not vacuous).
	doc, err := document.Parse(rec)
	if err != nil {
		t.Fatalf("written record does not parse: %v\n%s", err, rec)
	}
	var fm struct {
		Title string `yaml:"title"`
	}
	if err := doc.DecodeFrontmatter(&fm); err != nil || fm.Title != title {
		t.Fatalf("title = %q (err %v), want %q", fm.Title, err, title)
	}

	fs, err := reposetup.PlanRepairs(recPath, rec, false)
	if err != nil {
		t.Fatalf("PlanRepairs: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("change.create output must have zero repair findings, got %+v\n%s", fs, rec)
	}
}

func TestChangeCreateRejectsInvalidBranchPrefixWithoutEngineCall(t *testing.T) {
	for _, raw := range []string{"team/hotfix", "refs/heads/x", "hotfix//", "-x", "x.lock", "hot fix"} {
		t.Run(raw, func(t *testing.T) {
			req := validChangeCreateRequest()
			req.BranchPrefix = raw
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeCreate(context.Background(), deps, "", req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called %d times on an invalid prefix, want 0", len(engine.calls))
			}
			var msg string
			for _, f := range res.Findings {
				if f.Code == "invalid-branch_prefix" {
					msg = f.Message
				}
			}
			if msg == "" {
				t.Fatalf("missing invalid-branch_prefix; got %v", res.Findings)
			}
			if !strings.Contains(msg, fmt.Sprintf("%q", raw)) {
				t.Errorf("message %q does not name the rejected value %q", msg, raw)
			}
		})
	}
}

func TestChangeCreatePlanWritesDraftScalars(t *testing.T) {
	yes, no := true, false
	recPath := "docs/changes/active/0002-add-a-widget.md"
	cases := []struct {
		name       string
		auto       *bool
		prefix     string
		wantAuto   string
		wantPrefix string
		wantAG     domain.OptionalBool
		wantBranch string
	}{
		{"absent", nil, "", "\nauto_groomable:\n", "\nbranch_prefix:\n",
			domain.OptionalBool{State: domain.FieldEmpty}, "feat/add-a-widget"},
		{"true with messy prefix", &yes, " Hotfix/ ", "\nauto_groomable: true\n", "\nbranch_prefix: 'hotfix'\n",
			domain.OptionalBool{State: domain.FieldPresent, Value: true, Raw: "true"}, "hotfix/add-a-widget"},
		{"explicit false", &no, "", "\nauto_groomable: false\n", "\nbranch_prefix:\n",
			domain.OptionalBool{State: domain.FieldPresent, Value: false, Raw: "false"}, "feat/add-a-widget"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			op := baseOp([]string{})
			op.req.AutoGroomable = c.auto
			op.req.BranchPrefix = c.prefix
			plan, opRes := planFor(t, map[string]string{
				"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
			}, op)
			if opRes.Refused {
				t.Fatalf("unexpected refusal: %v", opRes.Findings)
			}
			rec := groomedRecordBytes(t, plan, recPath)
			if s := string(rec); !strings.Contains(s, c.wantAuto) || !strings.Contains(s, c.wantPrefix) {
				t.Errorf("record missing %q / %q:\n%s", c.wantAuto, c.wantPrefix, s)
			}
			// Round trip through the real decoder, then mint exactly as claim does.
			snap, err := buildCandidateSnapshot(op.eff, nil, rec, recPath)
			if err != nil {
				t.Fatalf("buildCandidateSnapshot: %v", err)
			}
			ch, out := snap.Change(2)
			if out != domain.LookupFound {
				t.Fatalf("created record not decodable as change 2")
			}
			if got := ch.AutoGroomable(); got != c.wantAG {
				t.Errorf("decoded AutoGroomable = %+v, want %+v", got, c.wantAG)
			}
			if got := domain.MintBranch(ch.Type(), ch.BranchPrefix(), ch.Slug()); got != c.wantBranch {
				t.Errorf("minted branch = %q, want %q", got, c.wantBranch)
			}
		})
	}
}

func TestChangeCreateDigestBindsNormalizedDraftScalars(t *testing.T) {
	digest := func(mut func(*ChangeCreateRequest)) transaction.RequestDigest {
		t.Helper()
		req := validChangeCreateRequest()
		mut(&req)
		d, err := canonicalDigest(OperationChangeCreate, changeCreateSemanticPayload(req))
		if err != nil {
			t.Fatalf("canonicalDigest: %v", err)
		}
		return d
	}
	yes, no := true, false
	none := digest(func(*ChangeCreateRequest) {})
	if digest(func(r *ChangeCreateRequest) { r.BranchPrefix = "Hotfix/" }) != digest(func(r *ChangeCreateRequest) { r.BranchPrefix = "hotfix" }) {
		t.Error("Hotfix/ and hotfix must digest identically (the normalized prefix is bound), so a retry replays")
	}
	if digest(func(r *ChangeCreateRequest) { r.BranchPrefix = "hotfix" }) == none {
		t.Error("a set prefix must change the digest")
	}
	if digest(func(r *ChangeCreateRequest) { r.AutoGroomable = &yes }) == digest(func(r *ChangeCreateRequest) { r.AutoGroomable = &no }) {
		t.Error("differing auto_groomable under one request_id must conflict, not replay")
	}
	if digest(func(r *ChangeCreateRequest) { r.AutoGroomable = &no }) == none {
		t.Error("an explicit false must differ from unset (inherit)")
	}
}

// TestChangeCreatePayloadOmitsUnsetDraftScalars — Review Focus 1: a request
// carrying neither new field must digest exactly as it did before change 0382,
// so re-running a pre-0382 request under its request_id still replays.
func TestChangeCreatePayloadOmitsUnsetDraftScalars(t *testing.T) {
	b, err := json.Marshal(changeCreateSemanticPayload(validChangeCreateRequest()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, k := range []string{`"auto_groomable"`, `"branch_prefix"`} {
		if strings.Contains(string(b), k) {
			t.Errorf("unset %s appears in the digest payload %s; it would change every pre-0382 digest", k, b)
		}
	}
}

// TestChangeCreateWhitespaceTitleReportsEmptyTitleOnce pins Review Focus 2: the
// shared validateTitle runs only on a non-blank title, so a blank one yields the
// existing empty-title finding exactly once, never a duplicate.
func TestChangeCreateWhitespaceTitleReportsEmptyTitleOnce(t *testing.T) {
	req := validChangeCreateRequest()
	req.Title = "   "
	n := 0
	for _, f := range validateChangeCreateShape(req) {
		if f.Code == string(FCEmptyTitle) {
			n++
		}
		if f.Code == string(FCInvalidTitle) {
			t.Errorf("blank title also reported %q: %v", FCInvalidTitle, f)
		}
	}
	if n != 1 {
		t.Errorf("empty-title reported %d times, want exactly 1", n)
	}
}
