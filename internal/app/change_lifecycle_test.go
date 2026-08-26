package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// blobV is a 40-char blob object id the pinned-entity requests carry.
const blobV = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// lifecycleChange renders a canonical change record in the given stored status,
// carrying every relationship field (present and empty), the empty
// docket:artifacts managed block, and a single ## Why authored section. Every
// post-claim status (in-progress, blocked, implemented, done, stacked-merged)
// additionally carries the branch/reconciled fields a claimed record holds — a
// claim records the branch once and it persists through the terminal statuses;
// a blocked record also carries blocked_by.
func lifecycleChange(id int, slug, status string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoaTest(id) + "\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A change'\n")
	b.WriteString("status: " + status + "\n")
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
	switch status {
	case "in-progress", "blocked", "implemented", "done", "stacked-merged":
		// A post-claim record carries the claim facts recorded once at claim time.
		b.WriteString("branch: feat/" + slug + "\n")
		b.WriteString("claimed_at: 2026-08-02T00:00:00Z\n")
		b.WriteString("reconciled: true\n")
	}
	// Canonical change records always carry blocked_by (empty unless blocked);
	// PatchSet.SetField patches an existing field rather than adding one.
	if status == "blocked" {
		b.WriteString("blocked_by: 'waiting on infra'\n")
	} else {
		b.WriteString("blocked_by:\n")
	}
	b.WriteString("---\n\n")
	b.WriteString("## Artifacts\n\n")
	b.WriteString("<!-- docket:artifacts:start (generated — do not hand-edit) -->\n")
	b.WriteString("<!-- docket:artifacts:end -->\n\n")
	b.WriteString("## Why\n\nOriginal why.\n")
	return b.String()
}

func validBlockRequest() ChangeBlockRequest {
	return ChangeBlockRequest{
		ChangeID: 3, Path: groomPath(3, "widget"), Version: blobV, Reason: "waiting on upstream",
	}
}

func validDeferRequest() ChangeDeferRequest {
	return ChangeDeferRequest{
		ChangeID: 3, Path: groomPath(3, "widget"), Version: blobV, WhyDeferred: "Parked pending a decision.\n",
	}
}

// --- request-shape validation (no engine call) ----------------------------

func TestChangeBlockRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeBlockRequest)
		code string
	}{
		{"non-positive change id", func(r *ChangeBlockRequest) { r.ChangeID = 0 }, "invalid-change_id"},
		{"empty path", func(r *ChangeBlockRequest) { r.Path = "" }, "empty-path"},
		{"empty version", func(r *ChangeBlockRequest) { r.Version = "" }, "empty-version"},
		{"empty reason", func(r *ChangeBlockRequest) { r.Reason = "  " }, "empty-reason"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validBlockRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeBlock(context.Background(), deps, "", req)

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

func TestChangeDeferRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeDeferRequest)
		code string
	}{
		{"non-positive change id", func(r *ChangeDeferRequest) { r.ChangeID = 0 }, "invalid-change_id"},
		{"empty path", func(r *ChangeDeferRequest) { r.Path = "" }, "empty-path"},
		{"empty version", func(r *ChangeDeferRequest) { r.Version = "" }, "empty-version"},
		{"empty why_deferred", func(r *ChangeDeferRequest) { r.WhyDeferred = "\n" }, "empty-why_deferred"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validDeferRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeDefer(context.Background(), deps, "", req)

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

func TestChangeBlockFencesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline", "github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeBlock(context.Background(), deps, "", validBlockRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a fenced board surface")
	}
}

func TestChangeDeferFencesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeDefer(context.Background(), deps, "", validDeferRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a fenced board surface")
	}
}

// --- outcome mapping (engine reached; Discover over a real temp repo) ------

func TestChangeBlockAppliedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	receipt := mustMarshal(t, changeLifecycleReceipt{
		ID: 3, Op: OperationChangeBlock, Status: "blocked",
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeBlock(context.Background(), deps, repoDir, validBlockRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.ID != 3 || res.Status != "blocked" {
		t.Errorf("identity from receipt = (%d, %q)", res.ID, res.Status)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}
	if res.Operation != OperationChangeBlock {
		t.Errorf("operation = %q, want %q", res.Operation, OperationChangeBlock)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationChangeBlock {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/main" {
		t.Errorf("target ref = %q, want refs/heads/main", req.TargetRef)
	}
	if req.Idempotency != nil {
		t.Errorf("lifecycle is non-allocating; it must carry no idempotency key, got %+v", req.Idempotency)
	}
	if len(req.Expected) != 1 {
		t.Fatalf("expected %d entity expectations, want 1", len(req.Expected))
	}
	exp := req.Expected[0]
	if string(exp.Path) != groomPath(3, "widget") {
		t.Errorf("expectation path = %q", exp.Path)
	}
	if exp.Version.Kind != transaction.VersionBlob || string(exp.Version.ObjectID) != blobV {
		t.Errorf("expectation version = %+v", exp.Version)
	}
}

func TestChangeDeferAppliedResultCarriesDeferStatus(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	receipt := mustMarshal(t, changeLifecycleReceipt{
		ID: 3, Op: OperationChangeDefer, Status: "deferred",
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeDefer(context.Background(), deps, repoDir, validDeferRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.Status != "deferred" || res.Operation != OperationChangeDefer {
		t.Errorf("result = (%q, %q)", res.Status, res.Operation)
	}
	if engine.calls[0].Operation.Key() != OperationChangeDefer {
		t.Errorf("operation key = %q", engine.calls[0].Operation.Key())
	}
}

func TestChangeLifecycleContendedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeBlock(context.Background(), deps, repoDir, validBlockRequest())

	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

func TestChangeLifecycleRefusedMapsInvalidState(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionRefused}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeBlock(context.Background(), deps, repoDir, validBlockRequest())

	if res.Result != ResultInvalidState {
		t.Fatalf("refused disposition mapped to %q, want invalid-state", res.Result)
	}
}

// --- plan closure ----------------------------------------------------------

func lifecyclePlanFor(t *testing.T, files map[string]string, op changeLifecycleOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func baseBlockOp(surfaces []string, id int, recPath, reason string) changeLifecycleOp {
	return baseLifecycleOp(OperationChangeBlock, surfaces, id, recPath,
		func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) { return domain.Block(c, reason) }, nil)
}

func baseDeferOp(surfaces []string, id int, recPath, why string) changeLifecycleOp {
	return baseLifecycleOp(OperationChangeDefer, surfaces, id, recPath,
		func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) { return domain.Defer(c) },
		[]render.SectionEdit{{Heading: "## Why deferred", Intent: render.SectionReplace, Markdown: why}})
}

func baseLifecycleOp(opKey string, surfaces []string, id int, recPath string,
	action func(domain.Change) (domain.ActionResult, *domain.PolicyFailure), sections []render.SectionEdit) changeLifecycleOp {
	return changeLifecycleOp{
		opKey:      opKey,
		changeID:   id,
		path:       recPath,
		action:     action,
		sections:   sections,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

func lifecycleRecordBytes(t *testing.T, plan transaction.MutationPlan, path string) string {
	t.Helper()
	for _, f := range plan.Files {
		if string(f.Path) == path {
			return string(f.Bytes)
		}
	}
	t.Fatalf("record %q not planned; files: %v", path, planPaths(plan))
	return ""
}

func TestChangeBlockPlanFileSet(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{
		recPath:                 lifecycleChange(3, "widget", "in-progress"),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := lifecyclePlanFor(t, files, baseBlockOp([]string{"inline"}, 3, recPath, "waiting on upstream"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:                 transaction.MutationReplace,
		"docs/changes/BOARD.md": transaction.MutationReplace,
	})

	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "status: 'blocked'") {
		t.Errorf("status not set to blocked:\n%s", rec)
	}
	if !strings.Contains(rec, "blocked_by: 'waiting on upstream'") {
		t.Errorf("blocked_by not written:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	// The artifact block is still present (re-rendered, empty here).
	if !strings.Contains(rec, "docket:artifacts:start") {
		t.Errorf("artifact block missing:\n%s", rec)
	}
	if strings.Contains(rec, "## Why deferred") {
		t.Errorf("block must not add a ## Why deferred section:\n%s", rec)
	}
}

func TestChangeDeferPlanFileSet(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{
		recPath: lifecycleChange(3, "widget", "proposed"),
	}
	plan, opRes := lifecyclePlanFor(t, files, baseDeferOp([]string{}, 3, recPath, "Parked pending a decision.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath: transaction.MutationReplace,
	})

	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "status: 'deferred'") {
		t.Errorf("status not set to deferred:\n%s", rec)
	}
	if !strings.Contains(rec, "## Why deferred\n\nParked pending a decision.\n") {
		t.Errorf("## Why deferred section not inserted:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped:\n%s", rec)
	}
}

func TestChangeBlockPlanSourceStatusMatrix(t *testing.T) {
	recPath := groomPath(3, "widget")
	cases := []struct {
		status  string
		refused bool
	}{
		{"in-progress", false},
		{"proposed", true},
		{"blocked", true},
		{"deferred", true},
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			files := map[string]string{recPath: lifecycleChange(3, "widget", c.status)}
			_, opRes := lifecyclePlanFor(t, files, baseBlockOp([]string{}, 3, recPath, "a reason"))
			if opRes.Refused != c.refused {
				t.Fatalf("block from %q: refused=%v, want %v (findings %v)", c.status, opRes.Refused, c.refused, opRes.Findings)
			}
			if c.refused && !hasDomainFindingCode(opRes.Findings, "illegal-source-status") {
				t.Errorf("block from %q: missing illegal-source-status finding; got %v", c.status, opRes.Findings)
			}
		})
	}
}

func TestChangeDeferPlanSourceStatusMatrix(t *testing.T) {
	recPath := groomPath(3, "widget")
	cases := []struct {
		status  string
		refused bool
	}{
		{"proposed", false},
		{"in-progress", false},
		{"blocked", true},
		{"deferred", true},
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			files := map[string]string{recPath: lifecycleChange(3, "widget", c.status)}
			_, opRes := lifecyclePlanFor(t, files, baseDeferOp([]string{}, 3, recPath, "Parked.\n"))
			if opRes.Refused != c.refused {
				t.Fatalf("defer from %q: refused=%v, want %v (findings %v)", c.status, opRes.Refused, c.refused, opRes.Findings)
			}
			if c.refused && !hasDomainFindingCode(opRes.Findings, "illegal-source-status") {
				t.Errorf("defer from %q: missing illegal-source-status finding; got %v", c.status, opRes.Findings)
			}
		})
	}
}

func TestChangeLifecyclePlanSourcePreservation(t *testing.T) {
	// An in-progress record carrying an unknown frontmatter field and an unknown
	// authored body section: both must survive byte-identically through a block.
	recPath := groomPath(3, "widget")
	src := lifecycleChange(3, "widget", "in-progress")
	src = strings.Replace(src, "trivial: false\n", "trivial: false\ncustom_field: 'unknown survives'\n", 1)
	src += "\n## Custom notes\n\nUnknown section survives.\n"

	files := map[string]string{recPath: src}
	plan, opRes := lifecyclePlanFor(t, files, baseBlockOp([]string{}, 3, recPath, "a reason"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "custom_field: 'unknown survives'") {
		t.Errorf("unknown frontmatter field did not survive:\n%s", rec)
	}
	if !strings.Contains(rec, "## Custom notes\n\nUnknown section survives.\n") {
		t.Errorf("unknown body section did not survive byte-identically:\n%s", rec)
	}
}

// TestChangeBlockPlanToleratesMissingUpdatedField pins that a block over a record
// lacking the updated: field inserts it rather than internal-erroring. A bare
// SetField("updated", …) returns KindMissingPatchTarget on an absent field —
// wrapped into a plan error — leaving these ops less tolerant of shape variance
// (e.g. Bash-era records) than the ADR ops, which upsert the same field.
func TestChangeBlockPlanToleratesMissingUpdatedField(t *testing.T) {
	recPath := groomPath(3, "widget")
	src := lifecycleChange(3, "widget", "in-progress")
	src = strings.Replace(src, "updated: 2026-08-02\n", "", 1)
	if strings.Contains(src, "updated:") {
		t.Fatalf("fixture still carries an updated field:\n%s", src)
	}

	files := map[string]string{recPath: src}
	plan, opRes := lifecyclePlanFor(t, files, baseBlockOp([]string{}, 3, recPath, "a reason"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not inserted from the clock on a record lacking it:\n%s", rec)
	}
}

// hasDomainFindingCode reports whether any domain finding carries code.
func hasDomainFindingCode(findings []domain.Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// TestLifecycleResultFromOutcomeFailedCarriesCause proves a lifecycle
// transaction that fails mid-flight surfaces its typed cause in the envelope's
// failure diagnosis instead of dropping it.
func TestLifecycleResultFromOutcomeFailedCarriesCause(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageLoadAfter,
		Kind:   transaction.KindInvalidState,
		Detail: "plan violates before/after tree rules",
	}
	out := lifecycleResultFromOutcome(OperationChangeMarkImplemented,
		transaction.Result{Disposition: transaction.DispositionFailed}, execErr)

	if out.Failure == nil {
		t.Fatal("failure diagnosis missing on a failed lifecycle transaction")
	}
	if out.Failure.Detail == "" {
		t.Error("failure.detail is empty")
	}
	if out.Failure.Stage != string(transaction.StageLoadAfter) {
		t.Errorf("failure.stage = %q, want %q", out.Failure.Stage, transaction.StageLoadAfter)
	}
}
