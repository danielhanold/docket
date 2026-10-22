package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
)

// killArchivePath is the archive path a kill relocates a change to: the injected
// clock's UTC date, the zero-padded id, and the slug.
func killArchivePath(id int, slug string) string {
	return "docs/changes/archive/2026-08-16-" + padID(id) + "-" + slug + ".md"
}

func validKillRequest() ChangeKillRequest {
	return ChangeKillRequest{
		ChangeID: 3, Path: groomPath(3, "widget"), Revision: blobV, WhyKilled: "Superseded by a better plan.\n",
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
		{"empty revision", func(r *ChangeKillRequest) { r.Revision = "" }, "empty-revision"},
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

func TestChangeKillRefusesGithubBoardSurface(t *testing.T) {
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline", "github"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeKill(context.Background(), deps, "", validKillRequest())

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want unsupported-config", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a refused github board surface")
	}
}

// --- outcome mapping (engine reached; Discover over a real temp repo) ------

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
	// Kill clears the recorded branch; an older record's claimed_at: line is
	// compatibility data and survives byte-identical.
	if strings.Contains(rec, "branch: feat/widget") {
		t.Errorf("branch not cleared:\n%s", rec)
	}
	if !strings.Contains(rec, "\nclaimed_at: 2026-08-02T00:00:00Z\n") {
		t.Errorf("kill rewrote the claimed_at line:\n%s", rec)
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
	if want := "> ↩ **[Change 0003 — A change](../../changes/archive/2026-08-16-0003-widget.md)**"; !strings.Contains(spec, want) {
		t.Errorf("spec backlink not retargeted to the archive path (want %q):\n%s", want, spec)
	}
	if strings.Contains(spec, "0003-widget.md`") || strings.Contains(spec, "active/") {
		t.Errorf("spec backlink still points at the active path:\n%s", spec)
	}
	// The authored spec body survives byte-identically.
	if !strings.Contains(spec, "# Design\n\nBody.\n") {
		t.Errorf("spec body not preserved:\n%s", spec)
	}
}

// TestChangeKillPlanRetargetsPlanAndResultsBacklinks: a killed change's plan and
// results live on the metadata branch beside its spec, so the kill retargets
// their backlinks to the archive path in the same plan, each relative to its own
// file's directory; an artifact whose block is already current is not declared.
func TestChangeKillPlanRetargetsPlanAndResultsBacklinks(t *testing.T) {
	recPath := groomPath(3, "widget")
	planPath := "docs/superpowers/plans/2026-08-01-widget.md"
	resultsPath := "docs/results/2026-08-01-widget-results.md"
	src := lifecycleChange(3, "widget", "in-progress")
	src = strings.Replace(src, "plan:\n", "plan: '"+planPath+"'\n", 1)
	src = strings.Replace(src, "results:\n", "results: '"+resultsPath+"'\n", 1)

	activeBlock := func(rel string) string {
		return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
			"> ↩ **[Change 0003 — A change](" + rel + ")**\n" +
			"<!-- docket:backlink:end -->\n"
	}
	files := map[string]string{
		recPath:     src,
		planPath:    activeBlock("../../changes/active/0003-widget.md") + "\n# Plan\n\nSteps.\n",
		resultsPath: activeBlock("../changes/active/0003-widget.md") + "\n# Results\n\nOutcome.\n",
	}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Superseded.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	archivePath := killArchivePath(3, "widget")
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		archivePath: transaction.MutationCreate,
		recPath:     transaction.MutationDelete,
		planPath:    transaction.MutationReplace,
		resultsPath: transaction.MutationReplace,
	})
	for p, want := range map[string]string{
		planPath:    activeBlock("../../changes/archive/2026-08-16-0003-widget.md") + "\n# Plan\n\nSteps.\n",
		resultsPath: activeBlock("../changes/archive/2026-08-16-0003-widget.md") + "\n# Results\n\nOutcome.\n",
	} {
		if got := killRecordBytes(t, plan, p); got != want {
			t.Errorf("%s after kill =\n%q\nwant\n%q", p, got, want)
		}
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

// --- the branch a kill leaves behind ---------------------------------------

// TestChangeKillPlanReceiptRecordsLeftBranch: the pre-kill branch: rides the
// commit receipt (never the archived record); a branchless record's receipt
// carries no branch key at all.
func TestChangeKillPlanReceiptRecordsLeftBranch(t *testing.T) {
	recPath := groomPath(3, "widget")

	files := map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Dropped.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	rec, ok := decodeChangeKillReceipt(plan.Receipt)
	if !ok || rec.Branch != "feat/widget" {
		t.Fatalf("receipt = %s (ok=%v), want branch feat/widget", plan.Receipt, ok)
	}
	if archived := killRecordBytes(t, plan, killArchivePath(3, "widget")); strings.Contains(archived, "feat/widget") {
		t.Errorf("archived record still carries the branch:\n%s", archived)
	}

	files = map[string]string{recPath: lifecycleChange(3, "widget", "proposed")}
	plan, opRes = killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Dropped.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	if rec, ok := decodeChangeKillReceipt(plan.Receipt); !ok || rec.Branch != "" {
		t.Fatalf("receipt = %s (ok=%v), want no branch", plan.Receipt, ok)
	}
	if strings.Contains(string(plan.Receipt), `"branch"`) {
		t.Errorf("branchless receipt carries a branch key: %s", plan.Receipt)
	}
}

// TestKillReceiptCanonicalWithHTMLCharacters: a branch carrying &, <, or >
// (legal in git refs) must still produce a receipt the engine accepts as
// canonical — re-encoding with HTML escaping off yields the same bytes.
func TestKillReceiptCanonicalWithHTMLCharacters(t *testing.T) {
	b, err := marshalKillReceipt(changeKillReceipt{
		ArchivePath: killArchivePath(3, "widget"), Branch: "feat/a&b<c>", ID: 3, Op: OperationChangeKill,
	})
	if err != nil {
		t.Fatalf("marshalKillReceipt: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(decoded); err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if got := bytes.TrimSuffix(buf.Bytes(), []byte("\n")); !bytes.Equal(got, b) {
		t.Errorf("receipt not canonical:\n got %s\nwant %s", b, got)
	}
	if !bytes.Contains(b, []byte("a&b<c>")) {
		t.Errorf("receipt escaped the branch: %s", b)
	}
}

func killOutcome(t *testing.T, branch string) transaction.Result {
	t.Helper()
	receipt, err := marshalKillReceipt(changeKillReceipt{
		ArchivePath: killArchivePath(3, "widget"), Branch: branch, ID: 3, Op: OperationChangeKill,
	})
	if err != nil {
		t.Fatalf("marshalKillReceipt: %v", err)
	}
	return transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: gitcli.ObjectID(strings.Repeat("cafe", 10)),
		Receipt:       receipt,
	}
}

func TestChangeKillResultWarnsBranchLeftBehind(t *testing.T) {
	res := changeKillResultFromOutcome(killOutcome(t, "feat/widget"), nil)
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	var hits []StatusFinding
	for _, f := range res.Findings {
		if f.Code == "branch-left-behind" {
			hits = append(hits, f)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("branch-left-behind findings = %v, want exactly one", res.Findings)
	}
	f := hits[0]
	if f.Severity != "warning" || f.Entity != "change" || f.Identity != "0003" {
		t.Errorf("finding = %+v, want warning on change 0003", f)
	}
	for _, want := range []string{"feat/widget", "local and remote", "worktree", "delete them by hand"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message %q missing %q", f.Message, want)
		}
	}
	rev := strings.Repeat("cafe", 10)
	want := "change 0003 killed (archived " + killArchivePath(3, "widget") + ") — " + rev + "\n  " + f.Message
	if got := res.HumanText(); got != want {
		t.Errorf("HumanText =\n%q\nwant\n%q", got, want)
	}
}

func TestChangeKillResultNoBranchNoWarning(t *testing.T) {
	res := changeKillResultFromOutcome(killOutcome(t, ""), nil)
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if hasFindingCode(res.Findings, "branch-left-behind") {
		t.Errorf("branchless kill warned: %v", res.Findings)
	}
	rev := strings.Repeat("cafe", 10)
	want := "change 0003 killed (archived " + killArchivePath(3, "widget") + ") — " + rev
	if got := res.HumanText(); got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}

// --- built stacked descendants ---------------------------------------------

// builtChild is change 4 ("child") at status, stacked on 3, carrying the PR
// reference and plan a built record's state guarantees.
func builtChild(status string) string {
	rec := stackedOn(lifecycleChange(4, "child", status), 3)
	rec = strings.Replace(rec, "blocked_by:\n", "pr: 'https://github.com/o/r/pull/40'\nblocked_by:\n", 1)
	return strings.Replace(rec, "plan:\n", "plan: docs/superpowers/plans/2026-08-16-child-plan.md\n", 1)
}

// builtDescendantRefusal plans killing change 3 (parentRec) over a stacked
// change 4 at childStatus and asserts the built-stack-descendants refusal.
func builtDescendantRefusal(t *testing.T, parentRec, childStatus string) transaction.OperationResult {
	t.Helper()
	recPath := groomPath(3, "widget")
	files := map[string]string{
		recPath:                 parentRec,
		groomPath(4, "child"):   builtChild(childStatus),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{"inline"}, 3, recPath, "Dropped.\n"))
	if !opRes.Refused {
		t.Fatalf("kill over a %s stacked child: want refusal, got plan %v", childStatus, planPaths(plan))
	}
	if len(plan.Files) != 0 {
		t.Errorf("refused kill planned files: %v", planPaths(plan))
	}
	if !hasDomainFindingCode(opRes.Findings, "built-stack-descendants") {
		t.Fatalf("missing built-stack-descendants finding; got %v", opRes.Findings)
	}
	return opRes
}

func builtDescendantMessage(t *testing.T, opRes transaction.OperationResult) string {
	t.Helper()
	for _, f := range opRes.Findings {
		if f.Code == "built-stack-descendants" {
			return f.Detail["message"]
		}
	}
	t.Fatalf("no built-stack-descendants finding in %v", opRes.Findings)
	return ""
}

func TestChangeKillPlanRefusesBuiltStackedChild(t *testing.T) {
	opRes := builtDescendantRefusal(t, lifecycleChange(3, "widget", "in-progress"), "implemented")
	msg := builtDescendantMessage(t, opRes)
	for _, want := range []string{"#0004 implemented", "change.block", "change.unblock"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q: %s", want, msg)
		}
	}
	if strings.Contains(msg, "change.defer") {
		t.Errorf("in-progress parent's remedy names change.defer: %s", msg)
	}
}

func TestChangeKillPlanBuiltStackRefusalDetail(t *testing.T) {
	opRes := builtDescendantRefusal(t, lifecycleChange(3, "widget", "in-progress"), "implemented")
	for _, f := range opRes.Findings {
		if f.Code != "built-stack-descendants" {
			continue
		}
		for k, want := range map[string]string{"reason": "built-stack-descendants", "from": "in-progress", "action": "kill"} {
			if got := f.Detail[k]; got != want {
				t.Errorf("detail[%q] = %q, want %q", k, got, want)
			}
		}
	}
}

func TestChangeKillPlanRefusesStackedMergedChild(t *testing.T) {
	opRes := builtDescendantRefusal(t, lifecycleChange(3, "widget", "in-progress"), "stacked-merged")
	if msg := builtDescendantMessage(t, opRes); !strings.Contains(msg, "#0004 stacked-merged") {
		t.Errorf("message missing #0004 stacked-merged: %s", msg)
	}
}

func TestChangeKillPlanLegacyProposedParentRemedy(t *testing.T) {
	parent := strings.Replace(lifecycleChange(3, "widget", "proposed"), "blocked_by:\n", "branch: feat/widget\nblocked_by:\n", 1)
	opRes := builtDescendantRefusal(t, parent, "implemented")
	msg := builtDescendantMessage(t, opRes)
	if !strings.Contains(msg, "change.defer") {
		t.Errorf("proposed parent's remedy missing change.defer: %s", msg)
	}
	if strings.Contains(msg, "change.block") {
		t.Errorf("proposed parent's remedy names change.block: %s", msg)
	}
}

func TestChangeKillPlanLeavesProposedChildUntouched(t *testing.T) {
	recPath := groomPath(3, "widget")
	childPath := groomPath(4, "child")
	files := map[string]string{
		recPath:                 lifecycleChange(3, "widget", "in-progress"),
		childPath:               stackedOn(lifecycleChange(4, "child", "proposed"), 3),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := killPlanFor(t, files, baseKillOp([]string{"inline"}, 3, recPath, "Dropped.\n"))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	for _, f := range plan.Files {
		if string(f.Path) == childPath {
			t.Errorf("kill planned a mutation of the proposed child: %v", f.Kind)
		}
	}
}

func TestChangeKillBuiltDescendantRefusalPrintsPlainText(t *testing.T) {
	opRes := builtDescendantRefusal(t, lifecycleChange(3, "widget", "in-progress"), "implemented")
	r := changeKillResultFromOutcome(transaction.Result{Disposition: transaction.DispositionRefused, Findings: opRes.Findings}, nil)
	if r.Result != ResultInvalidState {
		t.Fatalf("result = %s, want invalid-state", r.Result)
	}
	text := r.HumanText()
	lines := strings.Split(text, "\n")
	if len(lines) != 2 {
		t.Fatalf("HumanText has %d lines, want 2:\n%s", len(lines), text)
	}
	if lines[0] != "change kill: invalid-state" {
		t.Errorf("first line = %q", lines[0])
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[1]), "change 0003 has built stacked changes") {
		t.Errorf("second line = %q", lines[1])
	}
	if strings.Contains(text, "message=") {
		t.Errorf("HumanText leaks the code-plus-detail fallback:\n%s", text)
	}
}

func TestChangeKillIllegalSourceRefusalPrintsPlainText(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{recPath: lifecycleChange(3, "widget", "blocked")}
	_, opRes := killPlanFor(t, files, baseKillOp([]string{}, 3, recPath, "Dropped.\n"))
	r := changeKillResultFromOutcome(transaction.Result{Disposition: transaction.DispositionRefused, Findings: opRes.Findings}, nil)
	if text := r.HumanText(); !strings.Contains(text, "change 0003 is blocked, so it cannot be killed.") {
		t.Errorf("HumanText = %q", text)
	}
}
