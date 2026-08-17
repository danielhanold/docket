package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// reconcilableChange renders an in-progress change freshly claimed but not yet
// reconciled: the canonical in-progress record with reconciled: false and no
// ## Reconcile log section (the first reconcile creates it).
func reconcilableChange(id int, slug string) string {
	return strings.Replace(lifecycleChange(id, slug, "in-progress"), "reconciled: true\n", "reconciled: false\n", 1)
}

// reconcileFixture is an in-progress, not-yet-reconciled record carrying an
// unknown frontmatter key, an existing ## Reconcile log with a prior dated
// entry, and a trailing ## Custom notes section AFTER the log. The trailing
// section is the terminator witness: a reconcile-log slice that runs to EOF
// instead of stopping at the next heading would swallow it.
func reconcileFixture() string {
	s := reconcilableChange(3, "widget")
	s = strings.Replace(s, "trivial: false\n", "trivial: false\ncustom_field: 'survives'\n", 1)
	s = strings.TrimRight(s, "\n") +
		"\n\n## Reconcile log\n\n### 2026-08-01\n\nPrior entry.\n\n## Custom notes\n\nUnknown trailing section.\n"
	return s
}

// validReconcileRequest is a well-formed request against the reconcileFixture at
// id 3 / slug widget.
func validReconcileRequest() ChangeReconcileRequest {
	return ChangeReconcileRequest{
		ID:                3,
		Version:           blobV,
		Sections:          map[string]string{"## Why": "Refined why.\n"},
		Relations:         &DesiredRelations{DependsOn: []int{1}},
		ReconcileLogEntry: "Fresh reconcile.\n",
	}
}

// --- Plan-closure helper ---------------------------------------------------

func reconcilePlanFor(t *testing.T, files map[string]string, op changeReconcileOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func baseReconcileOp(surfaces []string, req ChangeReconcileRequest) changeReconcileOp {
	return changeReconcileOp{
		opKey:      OperationChangeReconcile,
		req:        req,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

// --- TestChangeReconcileAppliesPatch ---------------------------------------

// TestChangeReconcileAppliesPatch proves the structured edit: the owned proposal
// section is replaced, one dated log entry is appended under ## Reconcile log,
// reconciled flips true, claimed_at is restamped, relations are written as
// unquoted flow collections, and everything outside the patch — an unknown
// frontmatter key and the trailing ## Custom notes section — stays
// byte-identical.
func TestChangeReconcileAppliesPatch(t *testing.T) {
	recPath := groomPath(3, "widget")
	files := map[string]string{
		recPath:                 reconcileFixture(),
		"docs/changes/BOARD.md": "# Backlog\n\nold\n",
	}
	plan, opRes := reconcilePlanFor(t, files, baseReconcileOp([]string{"inline"}, validReconcileRequest()))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:                 transaction.MutationReplace,
		"docs/changes/BOARD.md": transaction.MutationReplace,
	})

	rec := lifecycleRecordBytes(t, plan, recPath)
	for _, want := range []string{
		"reconciled: true",
		"claimed_at: '2026-08-16T12:00:00Z'",
		"updated: '2026-08-16'",
		"depends_on: [1]",
		"Refined why.",
		"### 2026-08-16",
		"Fresh reconcile.",
		"### 2026-08-01",
		"Prior entry.",
		"docket:artifacts:start",
		"custom_field: 'survives'",
		"## Custom notes\n\nUnknown trailing section.\n",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("reconciled record missing %q:\n%s", want, rec)
		}
	}
	if strings.Contains(rec, "Original why.") {
		t.Errorf("## Why section not replaced:\n%s", rec)
	}
	// depends_on is a flow collection: never quoted (AGENTS.md YAML rule).
	if strings.Contains(rec, "depends_on: '[1]'") || strings.Contains(rec, "depends_on: \"[1]\"") {
		t.Errorf("relations must be an unquoted flow collection:\n%s", rec)
	}
	// Named-terminator witness: the reconcile-log slice must stop at ## Custom
	// notes, never run to EOF and fold the trailing section into the log body.
	// A dropped terminator duplicates the trailing section — count exactly one.
	if n := strings.Count(rec, "## Custom notes"); n != 1 {
		t.Errorf("## Custom notes appears %d times, want 1 (reconcile-log slice ran past its terminator):\n%s", n, rec)
	}
	if n := strings.Count(rec, "Unknown trailing section."); n != 1 {
		t.Errorf("trailing section text appears %d times, want 1:\n%s", n, rec)
	}
	// The new dated entry lands under ## Reconcile log, not before it.
	if strings.Index(rec, "## Reconcile log") > strings.Index(rec, "Fresh reconcile.") {
		t.Errorf("new entry appended before its section:\n%s", rec)
	}
}

// --- TestChangeReconcileOwnedFieldFence ------------------------------------

// TestChangeReconcileOwnedFieldFence proves a request naming a non-owned section
// (a managed block heading, or a heading outside the proposal set) is a typed
// request-shape refusal reached before any engine call.
func TestChangeReconcileOwnedFieldFence(t *testing.T) {
	cases := []struct {
		name    string
		heading string
	}{
		{"managed artifacts block", "## Artifacts"},
		{"reconcile log is not a proposal section", "## Reconcile log"},
		{"unknown heading", "## Nope"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validReconcileRequest()
			req.Sections = map[string]string{c.heading: "x\n"}
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

			res := ChangeReconcile(context.Background(), deps, "", req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called %d times on a fenced section, want 0", len(engine.calls))
			}
			if !hasFindingCode(res.Findings, "invalid-section-heading") {
				t.Errorf("missing invalid-section-heading; got %v", res.Findings)
			}
		})
	}
}

// TestChangeReconcileRequiresLogEntry proves the dated reconcile-log entry is
// required — an empty entry is a typed request-shape refusal, no engine call.
func TestChangeReconcileRequiresLogEntry(t *testing.T) {
	req := validReconcileRequest()
	req.ReconcileLogEntry = "   \n"
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeReconcile(context.Background(), deps, "", req)
	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called despite a missing reconcile-log entry")
	}
	if !hasFindingCode(res.Findings, "empty-reconcile_log_entry") {
		t.Errorf("missing empty-reconcile_log_entry; got %v", res.Findings)
	}
}

// --- TestChangeReconcileContention -----------------------------------------

// TestChangeReconcileContention proves both contention paths write nothing: a
// stale version is the engine's CAS contention; a status that is no longer
// in-progress is an incompatible fresh state the plan closure refuses and the
// result maps to contended (never a text-merge).
func TestChangeReconcileContention(t *testing.T) {
	t.Run("stale version at the engine", func(t *testing.T) {
		repoDir := newMainModeRepo(t, nil).invocation
		engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
		reader := &fakeReader{pin: mainModePin([]string{"inline"}), corpus: []StatusBlob{changeBlob(3, "widget", "feat", "high", "")}}
		deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

		res := ChangeReconcile(context.Background(), deps, repoDir, validReconcileRequest())
		if res.Result != ResultContended || res.Disposition != ReconcileDispositionContended {
			t.Fatalf("result=%q disposition=%q, want contended/contended", res.Result, res.Disposition)
		}
	})

	t.Run("no longer in-progress refuses at the plan and maps contended", func(t *testing.T) {
		// The plan closure refuses a proposed record with the incompatible-state
		// reason.
		recPath := groomPath(3, "widget")
		files := map[string]string{recPath: claimableChange(3, "widget")} // proposed
		plan, opRes := reconcilePlanFor(t, files, baseReconcileOp(nil, validReconcileRequest()))
		if !opRes.Refused {
			t.Fatalf("reconcile of a proposed change must refuse")
		}
		if !hasDomainFindingCode(opRes.Findings, reasonReconcileNotInProgress) {
			t.Errorf("missing %q; got %v", reasonReconcileNotInProgress, opRes.Findings)
		}
		if len(plan.Files) != 0 {
			t.Errorf("a refusal planned %d files, want 0", len(plan.Files))
		}

		// The result mapping folds that reason onto contended.
		mapped := changeReconcileResultFromOutcome(transaction.Result{
			Disposition: transaction.DispositionRefused,
			Findings:    opRes.Findings,
		}, nil)
		if mapped.Result != ResultContended || mapped.Disposition != ReconcileDispositionContended {
			t.Errorf("mapped result=%q disposition=%q, want contended/contended", mapped.Result, mapped.Disposition)
		}
	})
}

// --- TestChangeReconcileGuardsRedden ---------------------------------------

// TestChangeReconcileGuardsRedden is the mutation-style guard table (spec
// Testing bullet 4). Each row corrupts the fixture so a guard MUST refuse
// without planning any file:
//   - a duplicate ## Reconcile log heading has no single named terminator for
//     the slice (learning section-slice-needs-a-named-terminator);
//   - an authored entry that injects a managed-block marker line would corrupt
//     the artifact block, so the reparse guard refuses.
//
// Mutation evidence (noted in the commit): the terminator/uniqueness refusal in
// appendReconcileLog is what reddens the duplicate-heading row — strip it and a
// second ## Reconcile log is silently appended to.
func TestChangeReconcileGuardsRedden(t *testing.T) {
	recPath := groomPath(3, "widget")

	cases := []struct {
		name  string
		src   string
		entry string
	}{
		{
			name: "duplicate reconcile-log heading is ambiguous",
			src: strings.TrimRight(reconcilableChange(3, "widget"), "\n") +
				"\n\n## Reconcile log\n\nfirst.\n\n## Reconcile log\n\nsecond.\n",
			entry: "Fresh reconcile.\n",
		},
		{
			name:  "authored entry injecting a managed marker is rejected",
			src:   reconcileFixture(),
			entry: "Sneaky.\n<!-- docket:artifacts:start (generated) -->\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validReconcileRequest()
			req.ReconcileLogEntry = c.entry
			files := map[string]string{recPath: c.src}
			plan, opRes := reconcilePlanFor(t, files, baseReconcileOp(nil, req))
			if !opRes.Refused {
				t.Fatalf("%s: expected a refusal, got a plan", c.name)
			}
			if len(plan.Files) != 0 {
				t.Errorf("%s: a refusal planned %d files, want 0", c.name, len(plan.Files))
			}
		})
	}
}

// --- TestChangeReconcileBoundsInput ----------------------------------------

// TestChangeReconcileBoundsInput proves an oversized authored input is a typed
// request-shape refusal reached before any engine call.
func TestChangeReconcileBoundsInput(t *testing.T) {
	req := validReconcileRequest()
	req.ReconcileLogEntry = strings.Repeat("x", maxAuthoredMarkdownBytes+1)
	engine := &recordingEngine{}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeReconcile(context.Background(), deps, "", req)
	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want invalid-input", res.Result)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called on an oversized authored input")
	}
	if !hasFindingCode(res.Findings, "authored-input-too-large") {
		t.Errorf("missing authored-input-too-large; got %v", res.Findings)
	}
}

// --- TestChangeReconcilePatchesLinkedSpec ----------------------------------

// TestChangeReconcilePatchesLinkedSpec proves the still-mutable linked spec is
// patched in the same transaction: a named spec section is replaced while the
// change record is reconciled.
func TestChangeReconcilePatchesLinkedSpec(t *testing.T) {
	recPath := groomPath(3, "widget")
	specPath := "docs/superpowers/specs/2026-08-01-widget-design.md"
	rec := strings.Replace(reconcileFixture(), "spec:\n", "spec: '"+specPath+"'\n", 1)
	spec := "# Design\n\n## Goal\n\nOld goal.\n\n## Approach\n\nUnchanged approach.\n"

	req := validReconcileRequest()
	req.SpecSections = map[string]string{"## Goal": "New goal.\n"}

	files := map[string]string{
		recPath:  rec,
		specPath: spec,
	}
	plan, opRes := reconcilePlanFor(t, files, baseReconcileOp(nil, req))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:  transaction.MutationReplace,
		specPath: transaction.MutationReplace,
	})
	specOut := string(groomedRecordBytes(t, plan, specPath))
	if !strings.Contains(specOut, "New goal.") || strings.Contains(specOut, "Old goal.") {
		t.Errorf("## Goal not replaced in the linked spec:\n%s", specOut)
	}
	if !strings.Contains(specOut, "## Approach\n\nUnchanged approach.\n") {
		t.Errorf("untouched spec section not byte-identical:\n%s", specOut)
	}
}

// TestChangeReconcileSpecSectionRequiresLinkedSpec proves SpecSections against a
// change with no linked spec is a typed plan refusal.
func TestChangeReconcileSpecSectionRequiresLinkedSpec(t *testing.T) {
	recPath := groomPath(3, "widget")
	req := validReconcileRequest()
	req.SpecSections = map[string]string{"## Goal": "New goal.\n"}
	files := map[string]string{recPath: reconcileFixture()} // spec: empty
	plan, opRes := reconcilePlanFor(t, files, baseReconcileOp(nil, req))
	if !opRes.Refused {
		t.Fatalf("expected a refusal patching a spec on a change with none")
	}
	if len(plan.Files) != 0 {
		t.Errorf("a refusal planned %d files, want 0", len(plan.Files))
	}
}

// --- app seam (engine reached) ---------------------------------------------

// TestChangeReconcileAppliedResult proves the app layer submits the exact
// expected version and metadata target ref, carries NO idempotency key (a
// non-allocating edit of an existing record), and decodes the applied receipt.
func TestChangeReconcileAppliedResult(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	receipt := mustMarshal(t, changeReconcileReceipt{ID: 3, Op: OperationChangeReconcile})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeReader{pin: mainModePin([]string{"inline"}), corpus: []StatusBlob{changeBlob(3, "widget", "feat", "high", "")}}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeReconcile(context.Background(), deps, repoDir, validReconcileRequest())
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (findings %v)", res.Result, res.Findings)
	}
	if res.ID != 3 || res.Disposition != ReconcileDispositionApplied {
		t.Errorf("identity/disposition = (%d, %q)", res.ID, res.Disposition)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationChangeReconcile {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/main" {
		t.Errorf("target ref = %q, want refs/heads/main", req.TargetRef)
	}
	if req.Idempotency != nil {
		t.Errorf("reconcile is non-allocating; it must carry no idempotency key, got %+v", req.Idempotency)
	}
	if len(req.Expected) != 1 {
		t.Fatalf("expected %d entity expectations, want 1", len(req.Expected))
	}
	exp := req.Expected[0]
	if string(exp.Path) != groomPath(3, "widget") {
		t.Errorf("expectation path = %q", exp.Path)
	}
	if exp.Version.Kind != transaction.VersionBlob || string(exp.Version.ObjectID) != blobV {
		t.Errorf("expectation version = %+v, want the request's exact version", exp.Version)
	}
}

// TestChangeReconcileRejectsBadShapeWithoutEngineCall covers the pinned-entity
// request checks.
func TestChangeReconcileRejectsBadShapeWithoutEngineCall(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*ChangeReconcileRequest)
		code string
	}{
		{"non-positive id", func(r *ChangeReconcileRequest) { r.ID = 0 }, "invalid-change_id"},
		{"empty version", func(r *ChangeReconcileRequest) { r.Version = "" }, "empty-version"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := validReconcileRequest()
			c.mut(&req)
			engine := &recordingEngine{}
			reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
			deps := PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()}
			res := ChangeReconcile(context.Background(), deps, "", req)
			if res.Result != ResultInvalidInput {
				t.Fatalf("result = %q, want invalid-input", res.Result)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called on a shape failure")
			}
			if !hasFindingCode(res.Findings, c.code) {
				t.Errorf("missing %q; got %v", c.code, res.Findings)
			}
		})
	}
}
