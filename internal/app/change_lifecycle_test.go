package app

import (
	"context"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
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

// TestLifecycleShapeFindingNamesRealKey proves the id-shape finding carries the
// JSON key the request actually decodes — "id" for reconcile-family requests,
// "change_id" for block/defer/kill/groom — in both code and message.
func TestLifecycleShapeFindingNamesRealKey(t *testing.T) {
	got := validateLifecycleShape("id", 0, "p", "v")
	if len(got) != 1 || got[0].Code != "invalid-id" || !strings.Contains(got[0].Message, "id must be") {
		t.Errorf("id-keyed shape finding = %+v, want code invalid-id naming key id", got)
	}
	got = validateLifecycleShape("change_id", 0, "p", "v")
	if len(got) != 1 || got[0].Code != "invalid-change_id" {
		t.Errorf("change_id-keyed shape finding = %+v, want code invalid-change_id", got)
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

// --- change unblock / change revive ------------------------------------------

func validUnblockRequest() ChangeUnblockRequest {
	return ChangeUnblockRequest{ChangeID: 3, Path: groomPath(3, "widget"), Version: blobV}
}

func validReviveRequest() ChangeReviveRequest {
	return ChangeReviveRequest{ChangeID: 3, Path: groomPath(3, "widget"), Version: blobV}
}

func unblockOp(surfaces []string, id int, recPath string) changeLifecycleOp {
	return baseLifecycleOp(OperationChangeUnblock, surfaces, id, recPath,
		func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) { return domain.Unblock(c) }, nil)
}

func reviveOp(surfaces []string, id int, recPath string) changeLifecycleOp {
	return baseLifecycleOp(OperationChangeRevive, surfaces, id, recPath,
		func(c domain.Change) (domain.ActionResult, *domain.PolicyFailure) { return domain.Revive(c) }, nil)
}

// pinnedShapeCases are the request-shape failures common to every pinned-entity
// lifecycle request without an authored payload (unblock, revive): each
// mutates the valid (id, path, version) triple and names the expected finding.
var pinnedShapeCases = []struct {
	name string
	mut  func(id *int, path, version *string)
	code string
}{
	{"non-positive change id", func(id *int, _, _ *string) { *id = 0 }, "invalid-change_id"},
	{"empty path", func(_ *int, p, _ *string) { *p = "" }, "empty-path"},
	{"empty version", func(_ *int, _, v *string) { *v = " " }, "empty-version"},
}

func TestChangeUnblockRejectsBadShapeWithoutEngineCall(t *testing.T) {
	for _, c := range pinnedShapeCases {
		t.Run(c.name, func(t *testing.T) {
			req := validUnblockRequest()
			c.mut(&req.ChangeID, &req.Path, &req.Version)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeUnblock(context.Background(), deps, "", req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if res.Operation != OperationChangeUnblock {
				t.Errorf("operation = %q, want %q", res.Operation, OperationChangeUnblock)
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

func TestChangeReviveRejectsBadShapeWithoutEngineCall(t *testing.T) {
	for _, c := range pinnedShapeCases {
		t.Run(c.name, func(t *testing.T) {
			req := validReviveRequest()
			c.mut(&req.ChangeID, &req.Path, &req.Version)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeRevive(context.Background(), deps, "", req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if res.Operation != OperationChangeRevive {
				t.Errorf("operation = %q, want %q", res.Operation, OperationChangeRevive)
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

func TestChangeUnblockFencesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline", "github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeUnblock(context.Background(), deps, "", validUnblockRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a fenced board surface")
	}
}

func TestChangeReviveFencesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeRevive(context.Background(), deps, "", validReviveRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a fenced board surface")
	}
}

func TestChangeUnblockPlanFileSet(t *testing.T) {
	recPath := groomPath(3, "widget")
	src := strings.Replace(lifecycleChange(3, "widget", "blocked"),
		"blocked_by: 'waiting on infra'\n", "blocked_by: 'waiting on 0446'\n", 1)
	files := map[string]string{
		recPath:                 src,
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := lifecyclePlanFor(t, files, unblockOp([]string{"inline"}, 3, recPath))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:                 transaction.MutationReplace,
		"docs/changes/BOARD.md": transaction.MutationReplace,
	})

	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "status: 'in-progress'") {
		t.Errorf("status not set to in-progress:\n%s", rec)
	}
	if !strings.Contains(rec, "\nblocked_by:\n") || strings.Contains(rec, "waiting on 0446") {
		t.Errorf("blocked_by not cleared to the bare null form:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	if !strings.Contains(rec, "docket:artifacts:start") {
		t.Errorf("artifact block missing:\n%s", rec)
	}
	if !strings.Contains(rec, "branch: feat/widget\n") || !strings.Contains(rec, "claimed_at: 2026-08-02T00:00:00Z\n") {
		t.Errorf("unblock must leave the claim facts intact:\n%s", rec)
	}
	if plan.CommitSubject != "change 0003 → in-progress" {
		t.Errorf("commit subject = %q, want %q", plan.CommitSubject, "change 0003 → in-progress")
	}
	rc, ok := decodeChangeLifecycleReceipt(plan.Receipt)
	if !ok || rc != (changeLifecycleReceipt{ID: 3, Op: "change.unblock", Status: "in-progress"}) {
		t.Errorf("receipt = %+v (ok=%v), want {3 change.unblock in-progress}", rc, ok)
	}
}

// deferredWithClaim renders a deferred record that still carries the branch and
// claim stamp a deferral of an in-progress change leaves behind.
func deferredWithClaim() string {
	return strings.Replace(lifecycleChange(3, "widget", "deferred"), "trivial: false\n",
		"trivial: false\nbranch: 'feat/widget'\nclaimed_at: 2026-08-02T00:00:00Z\n", 1)
}

func TestChangeRevivePlanFileSet(t *testing.T) {
	// A deferred record WITHOUT a ## Why deferred section (deferred by an old
	// tool or a hand edit): revive passes no section edits, so it still applies.
	recPath := groomPath(3, "widget")
	src := deferredWithClaim()
	if strings.Contains(src, "## Why deferred") {
		t.Fatalf("fixture unexpectedly carries ## Why deferred:\n%s", src)
	}
	files := map[string]string{recPath: src}
	plan, opRes := lifecyclePlanFor(t, files, reviveOp([]string{}, 3, recPath))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath: transaction.MutationReplace,
	})

	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "status: 'proposed'") {
		t.Errorf("status not set to proposed:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped:\n%s", rec)
	}
	if !strings.Contains(rec, "branch: 'feat/widget'\n") || !strings.Contains(rec, "claimed_at: 2026-08-02T00:00:00Z\n") {
		t.Errorf("revive must leave branch and claimed_at byte-intact:\n%s", rec)
	}
	if strings.Contains(rec, "## Why deferred") {
		t.Errorf("revive must not add a ## Why deferred section:\n%s", rec)
	}
	if plan.CommitSubject != "change 0003 → proposed" {
		t.Errorf("commit subject = %q, want %q", plan.CommitSubject, "change 0003 → proposed")
	}
	rc, ok := decodeChangeLifecycleReceipt(plan.Receipt)
	if !ok || rc != (changeLifecycleReceipt{ID: 3, Op: "change.revive", Status: "proposed"}) {
		t.Errorf("receipt = %+v (ok=%v), want {3 change.revive proposed}", rc, ok)
	}
}

func TestChangeRevivePlanPreservesWhyDeferredAndClaim(t *testing.T) {
	recPath := groomPath(3, "widget")
	src := deferredWithClaim() + "\n## Why deferred\n\nParked for X.\n"
	files := map[string]string{recPath: src}
	plan, opRes := lifecyclePlanFor(t, files, reviveOp([]string{}, 3, recPath))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "status: 'proposed'") {
		t.Errorf("status not set to proposed:\n%s", rec)
	}
	if !strings.Contains(rec, "\n## Why deferred\n\nParked for X.\n") {
		t.Errorf("## Why deferred section did not survive byte-identically:\n%s", rec)
	}
	if !strings.Contains(rec, "branch: 'feat/widget'\n") || !strings.Contains(rec, "claimed_at: 2026-08-02T00:00:00Z\n") {
		t.Errorf("revive must leave branch and claimed_at byte-intact:\n%s", rec)
	}
}

func TestChangeUnblockPlanSourceStatusMatrix(t *testing.T) {
	recPath := groomPath(3, "widget")
	cases := []struct {
		status  string
		refused bool
	}{
		{"blocked", false},
		{"proposed", true},
		{"in-progress", true},
		{"deferred", true},
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			files := map[string]string{recPath: lifecycleChange(3, "widget", c.status)}
			plan, opRes := lifecyclePlanFor(t, files, unblockOp([]string{}, 3, recPath))
			if opRes.Refused != c.refused {
				t.Fatalf("unblock from %q: refused=%v, want %v (findings %v)", c.status, opRes.Refused, c.refused, opRes.Findings)
			}
			if c.refused {
				if !hasDomainFindingCode(opRes.Findings, "illegal-source-status") {
					t.Errorf("unblock from %q: missing illegal-source-status finding; got %v", c.status, opRes.Findings)
				}
				if len(plan.Files) != 0 {
					t.Errorf("unblock from %q: refused plan still carries files %v", c.status, planPaths(plan))
				}
			}
		})
	}
}

func TestChangeRevivePlanSourceStatusMatrix(t *testing.T) {
	recPath := groomPath(3, "widget")
	cases := []struct {
		status  string
		refused bool
	}{
		{"deferred", false},
		{"proposed", true},
		{"in-progress", true},
		{"blocked", true},
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			files := map[string]string{recPath: lifecycleChange(3, "widget", c.status)}
			plan, opRes := lifecyclePlanFor(t, files, reviveOp([]string{}, 3, recPath))
			if opRes.Refused != c.refused {
				t.Fatalf("revive from %q: refused=%v, want %v (findings %v)", c.status, opRes.Refused, c.refused, opRes.Findings)
			}
			if c.refused {
				if !hasDomainFindingCode(opRes.Findings, "illegal-source-status") {
					t.Errorf("revive from %q: missing illegal-source-status finding; got %v", c.status, opRes.Findings)
				}
				if len(plan.Files) != 0 {
					t.Errorf("revive from %q: refused plan still carries files %v", c.status, planPaths(plan))
				}
			}
		})
	}
}

// TestChangeUnblockPlanToleratesMissingUpdatedField pins that an unblock over a
// Bash-era record lacking updated: inserts the field rather than internal-erroring.
func TestChangeUnblockPlanToleratesMissingUpdatedField(t *testing.T) {
	recPath := groomPath(3, "widget")
	src := lifecycleChange(3, "widget", "blocked")
	src = strings.Replace(src, "updated: 2026-08-02\n", "", 1)
	if strings.Contains(src, "updated:") {
		t.Fatalf("fixture still carries an updated field:\n%s", src)
	}

	files := map[string]string{recPath: src}
	plan, opRes := lifecyclePlanFor(t, files, unblockOp([]string{}, 3, recPath))
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

// --- 0449: unrelated invalid records never block a named block/defer --------
// Shares the unrelated-broken-record fixtures with change_claim_test.go. Both
// transitions compose executeChangeLifecycle, whose one transaction.Request is
// the scoped site.
//
// Mutation check (run manually; noted in the commit): delete the `Scope:` field
// from executeChangeLifecycle's transaction.Request and
// `go test ./internal/app/ -run 'TestChangeLifecycleUnrelated' -count=1` reddens
// on the progress rows with the before-gate refusal the bug produced.

func TestChangeLifecycleUnrelatedInvalidRecordProgress(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	rows := []struct {
		name, from, want string
		run              func(node realNode, version string) ChangeLifecycleResult
	}{
		{name: "block", from: "in-progress", want: "blocked", run: func(node realNode, version string) ChangeLifecycleResult {
			return ChangeBlock(context.Background(), node.deps, node.dir,
				ChangeBlockRequest{ChangeID: id, Path: recPath, Version: version, Reason: "waiting on upstream"})
		}},
		{name: "defer", from: "proposed", want: "deferred", run: func(node realNode, version string) ChangeLifecycleResult {
			return ChangeDefer(context.Background(), node.deps, node.dir,
				ChangeDeferRequest{ChangeID: id, Path: recPath, Version: version, WhyDeferred: "Parked pending a decision.\n"})
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			repo := newWorkingRepo(t, map[string]string{
				recPath:             lifecycleChange(id, "widget", r.from),
				unrelatedBrokenPath: unrelatedBrokenBytes,
			})
			node := planningDepsFor(t, repo.invocation)
			res := r.run(node, blobVersionAt(t, repo.origin, "docket", recPath))
			if res.Result != ResultApplied || res.Status != r.want {
				t.Fatalf("%s beside an unrelated unparseable record = %q status %q (findings %v), want applied %q",
					r.name, res.Result, res.Status, res.Findings, r.want)
			}
			assertUnrelatedBrokenIntact(t, repo)
		})
	}
}

func TestChangeLifecycleUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	for _, c := range unrelatedRefusalCases(t, id, recPath, lifecycleChange(id, "widget", "in-progress"), lifecycleChange(id, "dupe", "in-progress")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			node := planningDepsFor(t, repo.invocation)
			tip := originTip(t, repo.origin, "docket")

			res := ChangeBlock(context.Background(), node.deps, node.dir, ChangeBlockRequest{
				ChangeID: id, Path: recPath, Version: blobVersionAt(t, repo.origin, "docket", recPath), Reason: "waiting on upstream",
			})
			if res.Result == ResultApplied {
				t.Fatalf("block applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, "", res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused block moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}
