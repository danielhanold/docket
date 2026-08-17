package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// killArchivePath is the archive path a kill relocates a change to: the injected
// clock's UTC date, the zero-padded id, and the slug.
func killArchivePath(id int, slug string) string {
	return "docs/changes/archive/2026-08-16-" + padID(id) + "-" + slug + ".md"
}

func validKillRequest() ChangeKillRequest {
	return ChangeKillRequest{
		ChangeID: 3, Path: groomPath(3, "widget"), Version: blobV, WhyKilled: "Superseded by a better plan.\n",
	}
}

// --- request-shape validation (no engine call) ----------------------------

func TestChangeKillRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeKillRequest)
		code string
	}{
		{"non-positive change id", func(r *ChangeKillRequest) { r.ChangeID = 0 }, "invalid-change_id"},
		{"empty path", func(r *ChangeKillRequest) { r.Path = "" }, "empty-path"},
		{"empty version", func(r *ChangeKillRequest) { r.Version = "" }, "empty-version"},
		{"empty why_killed", func(r *ChangeKillRequest) { r.WhyKilled = "\n" }, "empty-why_killed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validKillRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeKill(context.Background(), deps, "", req)

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

func TestChangeKillFencesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline", "github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeKill(context.Background(), deps, "", validKillRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a fenced board surface")
	}
}

// --- outcome mapping (engine reached; Discover over a real temp repo) ------

func TestChangeKillAppliedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	archivePath := killArchivePath(3, "widget")
	receipt := mustMarshal(t, changeKillReceipt{
		ArchivePath: archivePath, ID: 3, Op: OperationChangeKill,
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeKill(context.Background(), deps, repoDir, validKillRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.ID != 3 || res.ArchivePath != archivePath {
		t.Errorf("identity from receipt = (%d, %q)", res.ID, res.ArchivePath)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}
	if res.Operation != OperationChangeKill {
		t.Errorf("operation = %q, want %q", res.Operation, OperationChangeKill)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationChangeKill {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/main" {
		t.Errorf("target ref = %q, want refs/heads/main", req.TargetRef)
	}
	if req.Idempotency != nil {
		t.Errorf("kill is non-allocating; it must carry no idempotency key, got %+v", req.Idempotency)
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

func TestChangeKillContendedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeKill(context.Background(), deps, repoDir, validKillRequest())

	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

func TestChangeKillRefusedMapsInvalidState(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionRefused}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeKill(context.Background(), deps, repoDir, validKillRequest())

	if res.Result != ResultInvalidState {
		t.Fatalf("refused disposition mapped to %q, want invalid-state", res.Result)
	}
}

// --- plan closure ----------------------------------------------------------

func killPlanFor(t *testing.T, files map[string]string, op changeKillOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func baseKillOp(surfaces []string, id int, recPath, why string) changeKillOp {
	return changeKillOp{
		changeID:   id,
		path:       recPath,
		whyKilled:  why,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

func killRecordBytes(t *testing.T, plan transaction.MutationPlan, path string) string {
	t.Helper()
	for _, f := range plan.Files {
		if string(f.Path) == path {
			return string(f.Bytes)
		}
	}
	t.Fatalf("record %q not planned; files: %v", path, planPaths(plan))
	return ""
}

func TestChangeKillPlanFileSet(t *testing.T) {
	recPath := groomPath(3, "widget")
	archivePath := killArchivePath(3, "widget")
	files := map[string]string{
		recPath:                 lifecycleChange(3, "widget", "in-progress"),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{"inline"}, 3, recPath, "Superseded.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// The archive move is one create at the archive path plus one delete of the
	// active path — leaving the active file would keep the change visibly alive.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		archivePath:             transaction.MutationCreate,
		recPath:                 transaction.MutationDelete,
		"docs/changes/BOARD.md": transaction.MutationReplace,
	})

	rec := killRecordBytes(t, plan, archivePath)
	if !strings.Contains(rec, "status: 'killed'") {
		t.Errorf("status not set to killed:\n%s", rec)
	}
	if !strings.Contains(rec, "## Why killed\n\nSuperseded.\n") {
		t.Errorf("## Why killed section not spliced:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	if !strings.Contains(rec, "docket:artifacts:start") {
		t.Errorf("artifact block missing:\n%s", rec)
	}
	// Kill clears the lease and the recorded branch.
	if strings.Contains(rec, "branch: feat/widget") {
		t.Errorf("branch not cleared:\n%s", rec)
	}
	if strings.Contains(rec, "claimed_at: 2026-08-02") {
		t.Errorf("claimed_at not cleared:\n%s", rec)
	}
}

func TestChangeKillPlanArchiveDateFromClock(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{recPath: lifecycleChange(3, "widget", "proposed")}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Dropped.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// A proposed kill: only the archive create and the active delete (no board).
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		killArchivePath(3, "widget"): transaction.MutationCreate,
		recPath:                      transaction.MutationDelete,
	})
}

func TestChangeKillPlanRetargetsSpecBacklink(t *testing.T) {
	recPath := groomPath(3, "widget")
	specPath := "docs/superpowers/specs/2026-08-01-widget-design.md"
	src := lifecycleChange(3, "widget", "in-progress")
	src = strings.Replace(src, "spec:\n", "spec: '"+specPath+"'\n", 1)

	specFile := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change 0003 — A change** — `" + recPath + "`\n" +
		"<!-- docket:backlink:end -->\n\n# Design\n\nBody.\n"

	files := map[string]string{
		recPath:  src,
		specPath: specFile,
	}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Superseded.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	archivePath := killArchivePath(3, "widget")
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		archivePath: transaction.MutationCreate,
		recPath:     transaction.MutationDelete,
		specPath:    transaction.MutationReplace,
	})

	spec := killRecordBytes(t, plan, specPath)
	if !strings.Contains(spec, archivePath) {
		t.Errorf("spec backlink not retargeted to the archive path:\n%s", spec)
	}
	if strings.Contains(spec, recPath) {
		t.Errorf("spec backlink still points at the active path:\n%s", spec)
	}
	// The authored spec body survives byte-identically.
	if !strings.Contains(spec, "# Design\n\nBody.\n") {
		t.Errorf("spec body not preserved:\n%s", spec)
	}
}

func TestChangeKillPlanNoSpecMutationWhenSpecAbsent(t *testing.T) {
	recPath := groomPath(3, "widget")
	specPath := "docs/superpowers/specs/2026-08-01-widget-design.md"
	// The change names a spec that does NOT exist in the tree: no spec mutation,
	// no failure.
	src := lifecycleChange(3, "widget", "in-progress")
	src = strings.Replace(src, "spec:\n", "spec: '"+specPath+"'\n", 1)
	files := map[string]string{recPath: src}

	plan, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Superseded.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		killArchivePath(3, "widget"): transaction.MutationCreate,
		recPath:                      transaction.MutationDelete,
	})
}

func TestChangeKillPlanNoSpecMutationWhenBacklinkBlockAbsent(t *testing.T) {
	recPath := groomPath(3, "widget")
	specPath := "docs/superpowers/specs/2026-08-01-widget-design.md"
	// The change names a spec that EXISTS but was hand-authored (or Bash-era) and
	// carries NO docket:backlink managed block: there is no block to retarget, so
	// the kill skips the spec mutation entirely — no spec mutation, no failure,
	// matching the absent-spec contract — rather than surfacing a bare
	// KindMissingPatchTarget as an internal-error.
	src := lifecycleChange(3, "widget", "in-progress")
	src = strings.Replace(src, "spec:\n", "spec: '"+specPath+"'\n", 1)
	specFile := "# Design\n\nHand-authored spec with no backlink block.\n"
	files := map[string]string{
		recPath:  src,
		specPath: specFile,
	}

	plan, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Superseded.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// The kill still archives the change and deletes the active path; the spec is
	// left untouched (no MutationReplace at specPath).
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		killArchivePath(3, "widget"): transaction.MutationCreate,
		recPath:                      transaction.MutationDelete,
	})
}

func TestChangeKillPlanSourceStatusMatrix(t *testing.T) {
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
			_, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Dropped.\n"))
			if opRes.Refused != c.refused {
				t.Fatalf("kill from %q: refused=%v, want %v (findings %v)", c.status, opRes.Refused, c.refused, opRes.Findings)
			}
			if c.refused && !hasDomainFindingCode(opRes.Findings, "illegal-source-status") {
				t.Errorf("kill from %q: missing illegal-source-status finding; got %v", c.status, opRes.Findings)
			}
		})
	}
}

func TestChangeKillPlanSourcePreservation(t *testing.T) {
	// An in-progress record carrying an unknown frontmatter field and an unknown
	// authored body section: both must survive byte-identically through a kill.
	recPath := groomPath(3, "widget")
	src := lifecycleChange(3, "widget", "in-progress")
	src = strings.Replace(src, "trivial: false\n", "trivial: false\ncustom_field: 'unknown survives'\n", 1)
	src += "\n## Custom notes\n\nUnknown section survives.\n"

	files := map[string]string{recPath: src}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Dropped.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := killRecordBytes(t, plan, killArchivePath(3, "widget"))
	if !strings.Contains(rec, "custom_field: 'unknown survives'") {
		t.Errorf("unknown frontmatter field did not survive:\n%s", rec)
	}
	if !strings.Contains(rec, "## Custom notes\n\nUnknown section survives.\n") {
		t.Errorf("unknown body section did not survive byte-identically:\n%s", rec)
	}
}

// TestChangeKillEvolutionAcceptsRelocation proves the archive move is not read as
// identity reuse: the record vanishes from the active path and reappears at the
// archive path, which repository.ValidateEvolution models as a move, not a reuse.
func TestChangeKillEvolutionAcceptsRelocation(t *testing.T) {
	recPath := groomPath(3, "widget")
	archivePath := killArchivePath(3, "widget")
	beforeFiles := map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")}

	op := baseKillOp([]string{}, 3, recPath, "Superseded.\n")
	beforeTree := newFakeTree(beforeFiles)
	loader := newPlanningLoader(op.eff)
	before, err := loader.Load(context.Background(), beforeTree)
	if err != nil {
		t.Fatalf("loader.Load(before): %v", err)
	}

	plan, opRes, err := op.Plan(context.Background(), transaction.AttemptState{
		Base: beforeTree.Revision(), State: before, Tree: beforeTree,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}

	// Apply the plan's file set to produce the after tree the engine would load.
	afterFiles := make(map[string]string, len(beforeFiles))
	for p, b := range beforeFiles {
		afterFiles[p] = b
	}
	for _, f := range plan.Files {
		switch f.Kind {
		case transaction.MutationDelete:
			delete(afterFiles, string(f.Path))
		default:
			afterFiles[string(f.Path)] = string(f.Bytes)
		}
	}
	afterTree := newFakeTree(afterFiles)
	after, err := loader.Load(context.Background(), afterTree)
	if err != nil {
		t.Fatalf("loader.Load(after): %v", err)
	}
	if after.Report.HasErrors() {
		t.Fatalf("after-state has errors: %v", after.Report.Findings())
	}

	findings := loader.ValidateEvolution(before, after)
	for _, f := range findings {
		if f.Code == repository.CodeIdentityReused || f.Code == repository.CodeIdentityMutated {
			t.Errorf("relocation flagged as identity reuse/mutation: %+v", f)
		}
	}
	if _, ok := afterFiles[recPath]; ok {
		t.Errorf("active path %q still present after kill", recPath)
	}
	if _, ok := afterFiles[archivePath]; !ok {
		t.Errorf("archive path %q missing after kill", archivePath)
	}
}

// TestChangeKillPlanToleratesMissingUpdatedField pins that a kill over a record
// lacking the updated: field inserts it rather than internal-erroring, matching
// the ADR ops' upsert of the same field (a bare SetField returns
// KindMissingPatchTarget on an absent target).
func TestChangeKillPlanToleratesMissingUpdatedField(t *testing.T) {
	recPath := groomPath(3, "widget")
	src := lifecycleChange(3, "widget", "in-progress")
	src = strings.Replace(src, "updated: 2026-08-02\n", "", 1)
	if strings.Contains(src, "updated:") {
		t.Fatalf("fixture still carries an updated field:\n%s", src)
	}

	files := map[string]string{recPath: src}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Dropped.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec := killRecordBytes(t, plan, killArchivePath(3, "widget"))
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not inserted from the clock on a record lacking it:\n%s", rec)
	}
}
