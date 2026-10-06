package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// --- Plan-closure helper ---------------------------------------------------

func attachPlanFor(t *testing.T, files map[string]string, op changeAttachOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func baseAttachOp(surfaces []string, id int, kind, artifact string) changeAttachOp {
	return changeAttachOp{
		opKey:      attachOpKey(kind),
		kind:       kind,
		changeID:   id,
		artifact:   artifact,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

// attachTestPlanPath is the canonical plan path the plan-kind unit tests attach.
const attachTestPlanPath = "docs/superpowers/plans/2026-08-17-widget-plan.md"

// attachTestPlanMarkdown is a substantive plan body (no backlink block).
const attachTestPlanMarkdown = "# Plan\n\n## Task 1\n\nDo it.\n"

// planAttachOp is baseAttachOp for the plan kind carrying the stored artifact
// bytes the driver assembles: markdown behind the backlink rendered for change 3
// ("A change" at its active path), built by the production metadataArtifactBytes.
func planAttachOp(t *testing.T, surfaces []string, markdown string) changeAttachOp {
	t.Helper()
	op := baseAttachOp(surfaces, 3, attachKindPlan, attachTestPlanPath)
	artifact, err := metadataArtifactBytes([]byte(markdown), attachBacklinkBlock(3, "A change", groomPath(3, "widget")))
	if err != nil {
		t.Fatalf("metadataArtifactBytes: %v", err)
	}
	op.artifactBytes = artifact
	return op
}

// plannedFile returns the declared mutation for path, failing when absent.
func plannedFile(t *testing.T, plan transaction.MutationPlan, path string) transaction.FileMutation {
	t.Helper()
	for _, f := range plan.Files {
		if string(f.Path) == path {
			return f
		}
	}
	t.Fatalf("path %q not planned; files: %v", path, planPaths(plan))
	return transaction.FileMutation{}
}

// refusalCode returns the first refusal finding's code, failing when the plan
// did not refuse.
func refusalCode(t *testing.T, opRes transaction.OperationResult) string {
	t.Helper()
	if !opRes.Refused || len(opRes.Findings) == 0 {
		t.Fatalf("Plan did not refuse: %+v", opRes)
	}
	return opRes.Findings[0].Code
}

// --- attach-plan: the plan is written on the metadata branch ----------------

// TestAttachPlanWritesFileFieldAndBacklink proves one metadata transaction
// stores the plan path in the owned plan: field, re-renders the artifact block
// and board, AND creates the plan file whose bytes are the rendered backlink
// block prepended to the authored Markdown (the same prepend shape the spec file
// uses).
func TestAttachPlanWritesFileFieldAndBacklink(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{
		recPath:                 lifecycleChange(3, "widget", "in-progress"),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := attachPlanFor(t, files, planAttachOp(t, []string{"inline"}, attachTestPlanMarkdown))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:                 transaction.MutationReplace,
		"docs/changes/BOARD.md": transaction.MutationReplace,
		attachTestPlanPath:      transaction.MutationCreate,
	})
	rec := lifecycleRecordBytes(t, plan, recPath)
	for _, want := range []string{
		"plan: '" + attachTestPlanPath + "'",
		"updated: '2026-08-16'",
		"docket:artifacts:start",
		"status: in-progress", // untouched, still unquoted
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("attached record missing %q:\n%s", want, rec)
		}
	}
	if strings.Contains(rec, "results: '") {
		t.Errorf("attach-plan wrote a results field:\n%s", rec)
	}
	want := assembleSpecFile(attachBacklinkBlock(3, "A change", recPath), attachTestPlanMarkdown)
	if got := plannedFile(t, plan, attachTestPlanPath).Bytes; string(got) != string(want) {
		t.Errorf("plan file bytes =\n%q\nwant\n%q", got, want)
	}
}

// TestAttachPlanStripsSubmittedBacklink proves a body resubmitted exactly as read
// back (it still carries a docket:backlink block, any interior) is stored with
// exactly one backlink block, the freshly rendered one — and that re-assembling
// the stored bytes is a fixed point, so a read-back resubmission is no change.
func TestAttachPlanStripsSubmittedBacklink(t *testing.T) {
	fresh := attachBacklinkBlock(3, "A change", groomPath(3, "widget"))
	stale := attachBacklinkBlock(9, "Another change", "docs/changes/active/0009-other.md")
	got, err := metadataArtifactBytes([]byte(stale+"\n"+attachTestPlanMarkdown), fresh)
	if err != nil {
		t.Fatalf("metadataArtifactBytes: %v", err)
	}
	if n := strings.Count(string(got), "docket:backlink:start"); n != 1 {
		t.Fatalf("stored artifact carries %d backlink blocks, want 1:\n%s", n, got)
	}
	if !strings.HasPrefix(string(got), fresh) || strings.Contains(string(got), "Another change") {
		t.Errorf("stored backlink is not the freshly rendered one:\n%s", got)
	}
	if want := assembleSpecFile(fresh, attachTestPlanMarkdown); string(got) != string(want) {
		t.Errorf("stored artifact =\n%q\nwant\n%q", got, want)
	}
	again, err := metadataArtifactBytes(got, fresh)
	if err != nil {
		t.Fatalf("metadataArtifactBytes (resubmit): %v", err)
	}
	if string(again) != string(got) {
		t.Errorf("resubmitting the stored artifact changed it:\n%q\nwant\n%q", again, got)
	}
}

// TestAttachPlanRefusesMalformedBacklink proves a body whose backlink markers are
// malformed (a dangling start marker) refuses unbalanced-backlink before any pin
// or engine work — nothing is declared.
func TestAttachPlanRefusesMalformedBacklink(t *testing.T) {
	bad := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ dangling\n\n# Plan\n\nSteps.\n"
	if _, err := metadataArtifactBytes([]byte(bad), attachBacklinkBlock(3, "A change", groomPath(3, "widget"))); err == nil {
		t.Errorf("metadataArtifactBytes accepted malformed backlink markers")
	}
	res, engine, reader := runAttachPlanUnit(t, ChangeAttachRequest{ID: 3, Revision: blobV, Path: attachTestPlanPath, Markdown: []byte(bad)})
	if res.Reason != ReasonAttachUnbalancedBacklink {
		t.Fatalf("reason = %q, want %q (result %q msg %q)", res.Reason, ReasonAttachUnbalancedBacklink, res.Result, res.Message)
	}
	assertNothingAttempted(t, res, engine, reader)
}

// TestAttachPlanReplacesContentOnReplan proves a re-plan at the already linked
// path replaces the plan file's content in one MutationReplace and leaves the
// record undeclared when its bytes are unchanged (same day, field already set).
func TestAttachPlanReplacesContentOnReplan(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")}
	first, opRes := attachPlanFor(t, files, planAttachOp(t, nil, attachTestPlanMarkdown))
	if opRes.Refused {
		t.Fatalf("first attach refused: %v", opRes.Findings)
	}
	files[recPath] = lifecycleRecordBytes(t, first, recPath)
	files[attachTestPlanPath] = string(plannedFile(t, first, attachTestPlanPath).Bytes)

	revised := "# Plan\n\n## Task 1\n\nDo it differently.\n"
	second, opRes := attachPlanFor(t, files, planAttachOp(t, nil, revised))
	if opRes.Refused {
		t.Fatalf("re-plan refused: %v", opRes.Findings)
	}
	assertPlanPaths(t, second, map[string]transaction.MutationKind{
		attachTestPlanPath: transaction.MutationReplace,
	})
	if got := string(plannedFile(t, second, attachTestPlanPath).Bytes); !strings.Contains(got, "Do it differently.") {
		t.Errorf("re-plan did not write the revised content:\n%s", got)
	}
}

// TestAttachPlanRefusesDifferentPathWhenLinked proves a change whose plan: field
// already names path A refuses an attach of path B (artifact-path-mismatch)
// rather than silently re-pointing the link and orphaning A.
func TestAttachPlanRefusesDifferentPathWhenLinked(t *testing.T) {
	recPath := groomPath(3, "widget")
	linked := strings.Replace(lifecycleChange(3, "widget", "in-progress"),
		"plan:\n", "plan: 'docs/superpowers/plans/2026-08-01-widget-old.md'\n", 1)
	files := map[string]string{recPath: linked}
	_, opRes := attachPlanFor(t, files, planAttachOp(t, nil, attachTestPlanMarkdown))
	if code := refusalCode(t, opRes); code != ReasonAttachPathMismatch {
		t.Fatalf("refusal code = %q, want %q", code, ReasonAttachPathMismatch)
	}
}

// TestAttachPlanRefusesOccupiedPath proves an unlinked change refuses to write
// over a file already at the path whose backlink targets another change
// (artifact-path-occupied) — and that a file whose backlink targets THIS change
// (a prior attach's own file) is replaced.
func TestAttachPlanRefusesOccupiedPath(t *testing.T) {
	recPath := groomPath(3, "widget")
	t.Run("another change's file refuses", func(t *testing.T) {
		files := map[string]string{
			recPath:            lifecycleChange(3, "widget", "in-progress"),
			attachTestPlanPath: attachBacklinkBlock(9, "Another change", "docs/changes/active/0009-other.md") + "\n# Other plan\n",
		}
		_, opRes := attachPlanFor(t, files, planAttachOp(t, nil, attachTestPlanMarkdown))
		if code := refusalCode(t, opRes); code != ReasonAttachPathOccupied {
			t.Fatalf("refusal code = %q, want %q", code, ReasonAttachPathOccupied)
		}
	})
	t.Run("a file with no backlink refuses", func(t *testing.T) {
		files := map[string]string{
			recPath:            lifecycleChange(3, "widget", "in-progress"),
			attachTestPlanPath: "# Hand-written plan\n",
		}
		_, opRes := attachPlanFor(t, files, planAttachOp(t, nil, attachTestPlanMarkdown))
		if code := refusalCode(t, opRes); code != ReasonAttachPathOccupied {
			t.Fatalf("refusal code = %q, want %q", code, ReasonAttachPathOccupied)
		}
	})
	t.Run("this change's own file is replaced", func(t *testing.T) {
		files := map[string]string{
			recPath:            lifecycleChange(3, "widget", "in-progress"),
			attachTestPlanPath: attachBacklinkBlock(3, "A change", recPath) + "\n# Older plan\n",
		}
		plan, opRes := attachPlanFor(t, files, planAttachOp(t, nil, attachTestPlanMarkdown))
		if opRes.Refused {
			t.Fatalf("own file refused: %v", opRes.Findings)
		}
		if k := plannedFile(t, plan, attachTestPlanPath).Kind; k != transaction.MutationReplace {
			t.Errorf("own plan file kind = %q, want replace", k)
		}
	})
}

// TestAttachPlanIdenticalReattachIsNoOp pins the same-path same-day re-attach
// (change 0458): once the record stores the path with today's date and the plan
// file holds the same bytes, re-running the identical attach declares NO files —
// an empty plan the engine commits as a clean no-op, never an unchanged replace
// the engine's delta verifier (verifyActualDelta) refuses as invalid-state.
func TestAttachPlanIdenticalReattachIsNoOp(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{
		recPath:                 lifecycleChange(3, "widget", "in-progress"),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	op := planAttachOp(t, []string{"inline"}, attachTestPlanMarkdown)
	first, opRes := attachPlanFor(t, files, op)
	if opRes.Refused {
		t.Fatalf("first attach refused: %v", opRes.Findings)
	}
	for _, f := range first.Files {
		files[string(f.Path)] = string(f.Bytes)
	}
	second, opRes := attachPlanFor(t, files, op)
	if opRes.Refused {
		t.Fatalf("re-attach refused: %v", opRes.Findings)
	}
	if len(second.Files) != 0 {
		t.Errorf("identical re-attach declared files %v, want an empty (no-op) plan", planPaths(second))
	}
}

// TestAttachPlanPlaceholderStillRefuses proves a plan section whose entire body
// is a bare placeholder token refuses placeholder-token before any pin or engine
// work.
func TestAttachPlanPlaceholderStillRefuses(t *testing.T) {
	md := "# Plan\n\n## Error handling\n\n" + tok("tbd") + "\n"
	res, engine, reader := runAttachPlanUnit(t, ChangeAttachRequest{ID: 3, Revision: blobV, Path: attachTestPlanPath, Markdown: []byte(md)})
	if res.Reason != ReasonAttachPlaceholderToken {
		t.Fatalf("reason = %q, want %q (result %q msg %q)", res.Reason, ReasonAttachPlaceholderToken, res.Result, res.Message)
	}
	if !strings.Contains(res.Message, `section "Error handling"`) {
		t.Errorf("placeholder message does not name the slot: %q", res.Message)
	}
	assertNothingAttempted(t, res, engine, reader)
}

// TestAttachPlanRejectsBadShape proves the request-shape checks refuse before any
// pin or engine work: the id/path/revision scalars, an empty or oversized
// Markdown body. (The path-containment reasons need the resolved config and are
// pinned against real git in TestIntegrationWorkflowRepoChangeAttachPlanMetadataRefusals.)
func TestAttachPlanRejectsBadShape(t *testing.T) {
	valid := ChangeAttachRequest{ID: 3, Revision: blobV, Path: attachTestPlanPath, Markdown: []byte(attachTestPlanMarkdown)}
	t.Run("scalars", func(t *testing.T) {
		cases := []struct {
			name string
			mut  func(*ChangeAttachRequest)
			code string
		}{
			{"non-positive id", func(r *ChangeAttachRequest) { r.ID = 0 }, "invalid-id"},
			{"empty path", func(r *ChangeAttachRequest) { r.Path = "" }, "empty-path"},
			{"empty revision", func(r *ChangeAttachRequest) { r.Revision = "" }, "empty-revision"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				req := valid
				c.mut(&req)
				res, engine, reader := runAttachPlanUnit(t, req)
				if res.Result != ResultInvalidInput {
					t.Fatalf("result = %q, want invalid-input", res.Result)
				}
				if !hasFindingCode(res.Findings, c.code) {
					t.Errorf("missing finding %q; got %v", c.code, res.Findings)
				}
				assertNothingAttempted(t, res, engine, reader)
			})
		}
	})
	t.Run("markdown", func(t *testing.T) {
		cases := []struct {
			name     string
			markdown []byte
			reason   string
		}{
			{"empty", nil, ReasonAttachEmptyMarkdown},
			{"blank", []byte(" \n\n"), ReasonAttachEmptyMarkdown},
			{"too large", []byte("# Plan\n\n" + strings.Repeat("x", maxAuthoredMarkdownBytes) + "\n"), ReasonAttachMarkdownTooLarge},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				req := valid
				req.Markdown = c.markdown
				res, engine, reader := runAttachPlanUnit(t, req)
				if res.Result != ResultInvalidInput || res.Reason != c.reason {
					t.Fatalf("result/reason = %q/%q, want invalid-input/%q (msg %q)", res.Result, res.Reason, c.reason, res.Message)
				}
				assertNothingAttempted(t, res, engine, reader)
			})
		}
	})
}

// runAttachPlanUnit drives ChangeAttachPlan over a recording engine and a reader
// that records whether it was pinned, with no git client: every refusal it pins
// must land before the operation reads anything.
func runAttachPlanUnit(t *testing.T, req ChangeAttachRequest) (ChangeAttachResult, *recordingEngine, *fakeChangeReader) {
	t.Helper()
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}
	return ChangeAttachPlan(context.Background(), deps, "", req), engine, reader
}

// assertNothingAttempted proves a refusal read nothing and opened no transaction.
func assertNothingAttempted(t *testing.T, res ChangeAttachResult, engine *recordingEngine, reader *fakeChangeReader) {
	t.Helper()
	if res.Result == ResultApplied {
		t.Fatalf("attach applied, want a refusal")
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on a pre-transaction refusal, want 0", len(engine.calls))
	}
	if reader.pinned != 0 {
		t.Errorf("reader pinned %d times on a request-shape refusal, want 0", reader.pinned)
	}
}

// --- TestChangeAttachResultsPatch ------------------------------------------

// TestChangeAttachResultsPatch proves the results transaction stores the results
// path in the owned results: field, leaving plan: untouched, and (until results
// move to the metadata branch) declares no artifact file.
func TestChangeAttachResultsPatch(t *testing.T) {
	recPath := groomPath(3, "widget")
	resultsPath := "docs/results/2026-08-17-widget-results.md"
	files := map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")}
	plan, opRes := attachPlanFor(t, files, baseAttachOp(nil, 3, attachKindResults, resultsPath))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{recPath: transaction.MutationReplace})
	rec := lifecycleRecordBytes(t, plan, recPath)
	if !strings.Contains(rec, "results: '"+resultsPath+"'") {
		t.Errorf("attached record missing results field:\n%s", rec)
	}
	if strings.Contains(rec, "plan: '") {
		t.Errorf("attach-results wrote a plan field:\n%s", rec)
	}
}

// --- TestChangeAttachContention --------------------------------------------

// TestChangeAttachContention proves a transaction CAS miss folds to a contended
// result carrying no revision, with Findings marshalling as [] never nil.
func TestChangeAttachContention(t *testing.T) {
	res := attachResultFromOutcome(OperationChangeAttachPlan, attachKindPlan,
		"docs/superpowers/plans/x.md", transaction.Result{Disposition: transaction.DispositionContended}, nil)
	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	if res.Revision != "" {
		t.Errorf("contended result carried a revision %q", res.Revision)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

// --- TestChangeAttachResultsRejectsBadShape --------------------------------

// TestChangeAttachResultsRejectsBadShape proves the results request-shape check
// refuses a malformed request before any pin/engine work, for every missing
// scalar (results still verify a feature commit, so commit is required).
func TestChangeAttachResultsRejectsBadShape(t *testing.T) {
	valid := ChangeAttachRequest{ID: 3, Revision: blobV, Path: "docs/results/x-results.md", Commit: blobV}
	cases := []struct {
		name string
		mut  func(*ChangeAttachRequest)
		code string
	}{
		{"non-positive id", func(r *ChangeAttachRequest) { r.ID = 0 }, "invalid-id"},
		{"empty path", func(r *ChangeAttachRequest) { r.Path = "" }, "empty-path"},
		{"empty revision", func(r *ChangeAttachRequest) { r.Revision = "" }, "empty-revision"},
		{"empty commit", func(r *ChangeAttachRequest) { r.Commit = " " }, "empty-commit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := valid
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeAttachResults(context.Background(), deps, WorkspaceDeps{}, "", req)

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

// --- TestChangeAttachResultsIdenticalReattachIsNoOp ------------------------

// TestChangeAttachResultsIdenticalReattachIsNoOp pins the same-path same-day
// results re-attach (change 0458): an identical re-attach declares NO files.
func TestChangeAttachResultsIdenticalReattachIsNoOp(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{
		recPath:                 lifecycleChange(3, "widget", "in-progress"),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	op := baseAttachOp([]string{"inline"}, 3, attachKindResults, "docs/results/2026-09-25-widget-results.md")

	first, opRes := attachPlanFor(t, files, op)
	if opRes.Refused {
		t.Fatalf("first attach refused: %v", opRes.Findings)
	}
	files[recPath] = lifecycleRecordBytes(t, first, recPath)
	files["docs/changes/BOARD.md"] = lifecycleRecordBytes(t, first, "docs/changes/BOARD.md")

	second, opRes := attachPlanFor(t, files, op)
	if opRes.Refused {
		t.Fatalf("re-attach refused: %v", opRes.Findings)
	}
	if len(second.Files) != 0 {
		t.Errorf("identical re-attach declared files %v, want an empty (no-op) plan", planPaths(second))
	}
}
