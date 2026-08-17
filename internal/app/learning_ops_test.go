package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// --- fixtures --------------------------------------------------------------

// learningsDisabledPin builds a main-mode pin whose resolved configuration has
// learnings disabled (every other leaf as planningTestConfig).
func learningsDisabledPin() StatusPin {
	eff := planningTestConfig([]string{})
	eff.Learnings.Enabled.Value = false
	return StatusPin{
		Mode:          metadataModeMain,
		DefaultBranch: "main",
		Config:        config.Snapshot{Effective: eff},
	}
}

// learningPath is the canonical learnings-dir record path for a slug.
func learningPath(slug string) string {
	return "docs/changes/learnings/" + slug + ".md"
}

// fixtureLearning renders a canonical learning finding carrying an unknown
// frontmatter field and an unknown authored body section so source-preservation
// is observable through an update.
func fixtureLearning(slug string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("hook: 'the original hook'\n")
	b.WriteString("topics: ['shell']\n")
	b.WriteString("changes: [3]\n")
	b.WriteString("created: 2026-08-01\n")
	b.WriteString("updated: 2026-08-01\n")
	b.WriteString("promotion_state: retained\n")
	b.WriteString("promoted_to:\n")
	b.WriteString("custom_field: 'unknown survives'\n")
	b.WriteString("---\n\n")
	b.WriteString("## Apply\n\nDo the thing.\n\n")
	b.WriteString("## War story\n\nIt broke once.\n\n")
	b.WriteString("## Custom notes\n\nUnknown section survives.\n")
	return b.String()
}

func validLearningRecordRequest() LearningRecordRequest {
	return LearningRecordRequest{
		RequestID: "learn-00000001",
		Slug:      "a-lesson",
		Hook:      "when the widget spins, check the bearing",
		Topics:    []string{"shell", "git"},
		Changes:   []int{3},
		Apply:     "Check the bearing first.\n",
		WarStory:  "We spent a day on it.\n",
	}
}

func validLearningUpdateRequest() LearningUpdateRequest {
	return LearningUpdateRequest{
		Path:    learningPath("a-lesson"),
		Version: blobV,
		Hook:    "a revised hook",
		Topics:  []string{"shell", "portability"},
		Changes: []int{3, 4},
		Sections: []SectionEditRequest{
			{Heading: "## War story", Intent: "replace", Markdown: "A longer war story.\n"},
		},
	}
}

// --- request-shape validation (no engine call) -----------------------------

func TestLearningRecordRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*LearningRecordRequest)
		code string
	}{
		{"short request id", func(r *LearningRecordRequest) { r.RequestID = "short" }, "invalid-request-id"},
		{"invalid slug", func(r *LearningRecordRequest) { r.Slug = "Bad Slug" }, "invalid-slug"},
		{"empty hook", func(r *LearningRecordRequest) { r.Hook = "  " }, "empty-hook"},
		{"empty apply", func(r *LearningRecordRequest) { r.Apply = "" }, "empty-apply"},
		{"empty war story", func(r *LearningRecordRequest) { r.WarStory = "" }, "empty-war_story"},
		{"duplicate change", func(r *LearningRecordRequest) { r.Changes = []int{3, 3} }, "duplicate-changes"},
		{"blank topic", func(r *LearningRecordRequest) { r.Topics = []string{"ok", "  "} }, "invalid-topics"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validLearningRecordRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := LearningRecordOp(context.Background(), deps, "", req)

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

func TestLearningUpdateRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*LearningUpdateRequest)
		code string
	}{
		{"empty path", func(r *LearningUpdateRequest) { r.Path = "" }, "empty-path"},
		{"empty version", func(r *LearningUpdateRequest) { r.Version = "" }, "empty-version"},
		{"unknown heading", func(r *LearningUpdateRequest) {
			r.Sections = []SectionEditRequest{{Heading: "## Why", Intent: "replace", Markdown: "x"}}
		}, "invalid-section-heading"},
		{"duplicate change", func(r *LearningUpdateRequest) { r.Changes = []int{4, 4} }, "duplicate-changes"},
		{"blank topic", func(r *LearningUpdateRequest) { r.Topics = []string{"  "} }, "invalid-topics"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validLearningUpdateRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := LearningUpdate(context.Background(), deps, "", req)

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

// --- learnings-disabled preflight fence ------------------------------------

func TestLearningRecordFencesWhenLearningsDisabled(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: learningsDisabledPin()}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := LearningRecordOp(context.Background(), deps, "", validLearningRecordRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite learnings disabled")
	}
	if !hasFindingCode(res.Findings, ReasonLearningsDisabled) {
		t.Errorf("missing %q finding; got %v", ReasonLearningsDisabled, res.Findings)
	}
}

func TestLearningUpdateFencesWhenLearningsDisabled(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: learningsDisabledPin()}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := LearningUpdate(context.Background(), deps, "", validLearningUpdateRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite learnings disabled")
	}
}

// --- outcome mapping (engine reached; Discover over a real temp repo) -------

func TestLearningRecordAppliedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	receipt := mustMarshal(t, learningReceipt{
		Op: OperationLearningRecord, Path: learningPath("a-lesson"), Slug: "a-lesson",
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := LearningRecordOp(context.Background(), deps, repoDir, validLearningRecordRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.Slug != "a-lesson" || res.Path != learningPath("a-lesson") {
		t.Errorf("identity from receipt = (%q, %q)", res.Slug, res.Path)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}
	if res.Replayed {
		t.Errorf("Replayed = true on a fresh apply")
	}
	if res.Operation != OperationLearningRecord {
		t.Errorf("operation = %q, want %q", res.Operation, OperationLearningRecord)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationLearningRecord {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/main" {
		t.Errorf("target ref = %q, want refs/heads/main", req.TargetRef)
	}
	if req.Idempotency == nil || req.Idempotency.RequestID != "learn-00000001" {
		t.Errorf("idempotency key = %+v", req.Idempotency)
	}
	if len(req.Expected) != 0 {
		t.Errorf("record is allocating; it must carry no entity expectation, got %+v", req.Expected)
	}
}

func TestLearningRecordReplayResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	receipt := mustMarshal(t, learningReceipt{
		Op: OperationLearningRecord, Path: learningPath("a-lesson"), Slug: "a-lesson",
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionAlreadyApplied,
		AppliedCommit: "0000000000000000000000000000000000000abc",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := LearningRecordOp(context.Background(), deps, repoDir, validLearningRecordRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if !res.Replayed {
		t.Errorf("Replayed = false on an already-applied replay")
	}
	if res.Slug != "a-lesson" {
		t.Errorf("slug = %q, want a-lesson (from the original receipt)", res.Slug)
	}
}

func TestLearningRecordRefusedMapsInvalidInput(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionRefused}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := LearningRecordOp(context.Background(), deps, repoDir, validLearningRecordRequest())

	if res.Result != ResultInvalidInput {
		t.Fatalf("refused disposition mapped to %q, want invalid-input", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

func TestLearningUpdateContendedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := LearningUpdate(context.Background(), deps, repoDir, validLearningUpdateRequest())

	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

func TestLearningUpdateAppliedResultCarriesExactVersion(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	receipt := mustMarshal(t, learningReceipt{
		Op: OperationLearningUpdate, Path: learningPath("a-lesson"), Slug: "a-lesson",
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := LearningUpdate(context.Background(), deps, repoDir, validLearningUpdateRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.Slug != "a-lesson" || res.Operation != OperationLearningUpdate {
		t.Errorf("result = (%q, %q)", res.Slug, res.Operation)
	}
	req := engine.calls[0]
	if req.Idempotency != nil {
		t.Errorf("update is non-allocating; it must carry no idempotency key, got %+v", req.Idempotency)
	}
	if len(req.Expected) != 1 {
		t.Fatalf("expected 1 entity expectation, got %d", len(req.Expected))
	}
	exp := req.Expected[0]
	if string(exp.Path) != learningPath("a-lesson") {
		t.Errorf("expectation path = %q", exp.Path)
	}
	if exp.Version.Kind != transaction.VersionBlob || string(exp.Version.ObjectID) != blobV {
		t.Errorf("expectation version = %+v", exp.Version)
	}
}

// --- record plan closure ---------------------------------------------------

func baseLearningRecordOp() learningRecordOp {
	return learningRecordOp{
		req:          validLearningRecordRequest(),
		clock:        testClock(),
		learningsDir: "docs/changes/learnings",
	}
}

func learningRecordPlanFor(t *testing.T, files map[string]string, op learningRecordOp) (transaction.MutationPlan, transaction.OperationResult) {
	t.Helper()
	op.eff = planningTestConfig([]string{})
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

func TestLearningRecordPlanFileSetExcludesIndex(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0003-three.md": fixtureChange(3, "three"),
		// A pre-existing index and a sibling learning must be left untouched.
		"docs/changes/learnings/LEARNINGS.md": "# ledger\n",
		"docs/changes/learnings/other.md":     fixtureLearning("other"),
	}
	plan, opRes := learningRecordPlanFor(t, files, baseLearningRecordOp())
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// The plan touches EXACTLY the new learning record — never the index.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		learningPath("a-lesson"): transaction.MutationCreate,
	})
	if plan.CommitSubject == "" {
		t.Error("empty commit subject")
	}
}

func TestLearningRecordPlanCanonicalContent(t *testing.T) {
	files := map[string]string{
		"docs/changes/active/0003-three.md": fixtureChange(3, "three"),
	}
	plan, _ := learningRecordPlanFor(t, files, baseLearningRecordOp())
	rec := planFileBytes(t, plan, learningPath("a-lesson"))
	for _, want := range []string{
		"slug: 'a-lesson'",
		"hook: 'when the widget spins, check the bearing'",
		"promotion_state: 'retained'",
		"created: '2026-08-16'",
		"updated: '2026-08-16'",
		"## Apply\n\nCheck the bearing first.",
		"## War story\n\nWe spent a day on it.",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
}

func TestLearningRecordPlanRefusesDuplicateSlug(t *testing.T) {
	files := map[string]string{
		"docs/changes/learnings/a-lesson.md": fixtureLearning("a-lesson"),
	}
	plan, opRes := learningRecordPlanFor(t, files, baseLearningRecordOp())
	if !opRes.Refused {
		t.Fatalf("duplicate slug was not refused")
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", plan.Files)
	}
	if !hasDomainFindingCode(opRes.Findings, "duplicate-slug") {
		t.Errorf("missing duplicate-slug finding: %v", opRes.Findings)
	}
}

// --- update plan closure ---------------------------------------------------

func baseLearningUpdateOp(req LearningUpdateRequest) learningUpdateOp {
	return learningUpdateOp{
		req:   req,
		eff:   planningTestConfig([]string{}),
		clock: testClock(),
	}
}

func learningUpdatePlanFor(t *testing.T, files map[string]string, op learningUpdateOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func TestLearningUpdatePlanFileSetAndPreservation(t *testing.T) {
	files := map[string]string{
		learningPath("a-lesson"):              fixtureLearning("a-lesson"),
		"docs/changes/learnings/LEARNINGS.md": "# ledger\n",
	}
	plan, opRes := learningUpdatePlanFor(t, files, baseLearningUpdateOp(validLearningUpdateRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	// The plan touches EXACTLY the learning record — never the index.
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		learningPath("a-lesson"): transaction.MutationReplace,
	})

	rec := planFileBytes(t, plan, learningPath("a-lesson"))
	if !strings.Contains(rec, "hook: 'a revised hook'") {
		t.Errorf("hook not updated:\n%s", rec)
	}
	if !strings.Contains(rec, "## War story\n\nA longer war story.\n") {
		t.Errorf("war story section not replaced:\n%s", rec)
	}
	if !strings.Contains(rec, "updated: '2026-08-16'") {
		t.Errorf("updated not stamped from the clock:\n%s", rec)
	}
	// Identity, created, promotion, unknown field, and unknown section survive.
	for _, want := range []string{
		"created: 2026-08-01",
		"promotion_state: retained",
		"custom_field: 'unknown survives'",
		"## Custom notes\n\nUnknown section survives.\n",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("expected preserved bytes %q missing:\n%s", want, rec)
		}
	}
}

func TestLearningUpdatePlanNoOpWhenNothingChanges(t *testing.T) {
	files := map[string]string{
		learningPath("a-lesson"): fixtureLearning("a-lesson"),
	}
	// A request that re-sends the record's exact current hook/topics/changes and
	// makes no section change: the planned bytes equal the source, so the plan is
	// empty (the engine's no-op) and `updated` is NOT bumped.
	req := LearningUpdateRequest{
		Path:    learningPath("a-lesson"),
		Version: blobV,
		Hook:    "the original hook",
		Topics:  []string{"shell"},
		Changes: []int{3},
	}
	plan, opRes := learningUpdatePlanFor(t, files, baseLearningUpdateOp(req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	if len(plan.Files) != 0 {
		t.Errorf("no-op update still planned files: %v", planPaths(plan))
	}
}

func TestLearningUpdatePlanRefusesUnknownPath(t *testing.T) {
	files := map[string]string{
		learningPath("a-lesson"): fixtureLearning("a-lesson"),
	}
	req := validLearningUpdateRequest()
	req.Path = learningPath("nonexistent")
	plan, opRes := learningUpdatePlanFor(t, files, baseLearningUpdateOp(req))
	if !opRes.Refused {
		t.Fatalf("update of an absent record was not refused")
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused plan still carries files: %v", plan.Files)
	}
}

// planFileBytes returns the planned bytes for path, failing if absent.
func planFileBytes(t *testing.T, plan transaction.MutationPlan, path string) string {
	t.Helper()
	for _, f := range plan.Files {
		if string(f.Path) == path {
			return string(f.Bytes)
		}
	}
	t.Fatalf("path %q not planned; files: %v", path, planPaths(plan))
	return ""
}
