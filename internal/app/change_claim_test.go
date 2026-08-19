package app

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// claimableChange renders a proposed, build-ready change: the canonical proposed
// record with trivial: true (so it carries a design outcome) and no claim
// fields yet. Claiming it must INSERT branch/claimed_at/reconciled.
func claimableChange(id int, slug string) string {
	return strings.Replace(lifecycleChange(id, slug, "proposed"), "trivial: false\n", "trivial: true\n", 1)
}

// stackedOn rewrites a record's empty stacked_on edge to point at parent.
func stackedOn(src string, parent int) string {
	return strings.Replace(src, "stacked_on:\n", "stacked_on: "+itoaTest(parent)+"\n", 1)
}

// --- Plan-closure helper ---------------------------------------------------

func claimPlanFor(t *testing.T, files map[string]string, op changeClaimOp) (transaction.MutationPlan, transaction.OperationResult) {
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

func baseClaimOp(surfaces []string, id int, facts domain.BranchFacts) changeClaimOp {
	return changeClaimOp{
		opKey:      OperationChangeClaim,
		changeID:   id,
		facts:      facts,
		eff:        planningTestConfig(surfaces),
		clock:      testClock(),
		inline:     len(surfaces) > 0 && surfaces[0] == "inline",
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

func baseRefreshOp(surfaces []string, id int) changeClaimOp {
	op := baseClaimOp(surfaces, id, domain.BranchFacts{})
	op.opKey = OperationChangeRefreshClaim
	op.refresh = true
	return op
}

// --- TestChangeClaimApplies ------------------------------------------------

// TestChangeClaimApplies proves both halves of a successful claim: the app layer
// submits the exact expected version, an idempotency key, and the metadata
// target ref (recordingEngine); and the plan closure patches status/branch/
// claimed_at and names the record, board, and artifact surfaces.
func TestChangeClaimApplies(t *testing.T) {
	const version = "1234123412341234123412341234123412341234"

	t.Run("submitted request", func(t *testing.T) {
		repoDir := newMainModeRepo(t, nil).invocation
		receipt := mustMarshal(t, changeClaimReceipt{
			Branch: "feat/widget", ClaimedAt: "2026-08-16T12:00:00Z",
			ID: 3, Lease: "fresh", Op: OperationChangeClaim, Status: "in-progress",
		})
		engine := &recordingEngine{result: transaction.Result{
			Disposition:   transaction.DispositionApplied,
			AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
			Receipt:       receipt,
		}}
		reader := &fakeReader{pin: mainModePin([]string{"inline"}), corpus: []StatusBlob{changeBlob(3, "widget", "feat", "high", "")}}
		deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

		res := ChangeClaim(context.Background(), deps, repoDir, ChangeClaimRequest{ID: 3, Version: version})

		if res.Result != ResultApplied {
			t.Fatalf("result = %q, want applied (findings %v)", res.Result, res.Findings)
		}
		if res.Disposition != ClaimDispositionApplied {
			t.Errorf("disposition = %q, want applied", res.Disposition)
		}
		if res.Branch != "feat/widget" || res.Lease != "fresh" || res.Status != "in-progress" {
			t.Errorf("result echoes (branch=%q lease=%q status=%q); want the claim receipt", res.Branch, res.Lease, res.Status)
		}
		if res.ClaimedAt != "2026-08-16T12:00:00Z" {
			t.Errorf("claimed_at = %q", res.ClaimedAt)
		}
		if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
			t.Errorf("revision = %q", res.Revision)
		}

		if len(engine.calls) != 1 {
			t.Fatalf("engine calls = %d, want 1", len(engine.calls))
		}
		req := engine.calls[0]
		if req.Operation.Key() != OperationChangeClaim {
			t.Errorf("operation key = %q", req.Operation.Key())
		}
		if req.TargetRef != "refs/heads/main" {
			t.Errorf("target ref = %q, want refs/heads/main", req.TargetRef)
		}
		if len(req.Expected) != 1 {
			t.Fatalf("expected %d entity expectations, want 1", len(req.Expected))
		}
		exp := req.Expected[0]
		if string(exp.Path) != "docs/changes/active/0003-widget.md" {
			t.Errorf("expectation path = %q", exp.Path)
		}
		if exp.Version.Kind != transaction.VersionBlob || string(exp.Version.ObjectID) != version {
			t.Errorf("expectation version = %+v, want the request's exact version", exp.Version)
		}
		if req.Idempotency == nil || req.Idempotency.RequestID != "claim-3-"+version {
			t.Errorf("idempotency key = %+v, want a stable per-request id", req.Idempotency)
		}
		if req.Idempotency != nil && !strings.HasPrefix(string(req.Idempotency.Digest), "sha256:") {
			t.Errorf("idempotency digest = %q, want a sha256 digest", req.Idempotency.Digest)
		}
	})

	t.Run("plan patches claim fields and surfaces", func(t *testing.T) {
		recPath := groomPath(3, "widget")
		files := map[string]string{
			recPath:                 claimableChange(3, "widget"),
			"docs/changes/BOARD.md": "# Backlog\n\nold\n",
		}
		plan, opRes := claimPlanFor(t, files, baseClaimOp([]string{"inline"}, 3, domain.BranchFacts{}))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		assertPlanPaths(t, plan, map[string]transaction.MutationKind{
			recPath:                 transaction.MutationReplace,
			"docs/changes/BOARD.md": transaction.MutationReplace,
		})

		rec := lifecycleRecordBytes(t, plan, recPath)
		for _, want := range []string{
			"status: 'in-progress'",
			"branch: 'feat/widget'",
			"claimed_at: '2026-08-16T12:00:00Z'",
			"updated: '2026-08-16'",
			"docket:artifacts:start",
		} {
			if !strings.Contains(rec, want) {
				t.Errorf("claimed record missing %q:\n%s", want, rec)
			}
		}
	})
}

// --- TestChangeClaimRefusals -----------------------------------------------

// TestChangeClaimRefusals is the refusal table. The domain refusals are proven
// at the plan-closure seam (the engine sees a request whose in-transaction
// re-proof fails); a wrong version is proven at the app seam (the engine's CAS
// reports contended). No record is mutated on any refusal.
//
// Mutation check (run manually; noted in the commit): delete the
// `domain.ClaimEligibility` re-proof line in changeClaimOp.Plan and
// `go test ./internal/app/ -run TestChangeClaimRefusals -count=1` reddens on the
// not-build-ready and unresolved-base rows — the two the domain.Claim status
// gate alone cannot catch.
func TestChangeClaimRefusals(t *testing.T) {
	recPath := groomPath(3, "widget")
	parentPath := groomPath(2, "parent")

	planCases := []struct {
		name   string
		files  map[string]string
		facts  domain.BranchFacts
		reason string
	}{
		{
			// The re-proof catches not-proposed first, as not-ready-not-proposed;
			// stripping it drops the row to domain.Claim's illegal-source-status, so
			// this row reddens on the mutation too.
			name:   "not proposed",
			files:  map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")},
			reason: "not-ready-" + string(domain.ReadyNotProposed),
		},
		{
			name:   "not build-ready",
			files:  map[string]string{recPath: lifecycleChange(3, "widget", "proposed")}, // trivial:false, no spec
			reason: "not-ready-" + string(domain.ReadyNeedsBrainstorm),
		},
		{
			name: "unresolved base",
			files: map[string]string{
				recPath:    stackedOn(claimableChange(3, "widget"), 2),
				parentPath: lifecycleChange(2, "parent", "in-progress"), // branch feat/parent, absent from empty facts
			},
			reason: "not-ready-" + string(domain.ReadyStackBaseUnresolved),
		},
	}
	for _, c := range planCases {
		t.Run(c.name, func(t *testing.T) {
			plan, opRes := claimPlanFor(t, c.files, baseClaimOp(nil, 3, c.facts))
			if !opRes.Refused {
				t.Fatalf("%s: expected a refusal, got a plan", c.name)
			}
			if !hasDomainFindingCode(opRes.Findings, c.reason) {
				t.Errorf("%s: missing reason %q; got %v", c.name, c.reason, opRes.Findings)
			}
			if len(plan.Files) != 0 {
				t.Errorf("%s: a refusal planned %d files, want 0", c.name, len(plan.Files))
			}
		})
	}

	t.Run("wrong version", func(t *testing.T) {
		repoDir := newMainModeRepo(t, nil).invocation
		engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
		reader := &fakeReader{pin: mainModePin([]string{"inline"}), corpus: []StatusBlob{changeBlob(3, "widget", "feat", "high", "")}}
		deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

		res := ChangeClaim(context.Background(), deps, repoDir, ChangeClaimRequest{ID: 3, Version: "9999999999999999999999999999999999999999"})

		if res.Result != ResultContended || res.Disposition != ClaimDispositionContended {
			t.Fatalf("result=%q disposition=%q, want contended/contended", res.Result, res.Disposition)
		}
		if res.Findings == nil {
			t.Errorf("Findings must marshal as [], not nil")
		}
	})

	// A duplicate id cannot be attributed to one record: the pre-read refuses to
	// choose, before any engine call.
	t.Run("duplicate id", func(t *testing.T) {
		repoDir := newMainModeRepo(t, nil).invocation
		engine := &recordingEngine{}
		reader := &fakeReader{pin: mainModePin([]string{"inline"}), corpus: []StatusBlob{
			changeBlob(3, "widget", "feat", "high", ""),
			changeBlob(3, "dupe", "feat", "high", ""),
		}}
		deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

		res := ChangeClaim(context.Background(), deps, repoDir, ChangeClaimRequest{ID: 3, Version: "1234123412341234123412341234123412341234"})

		if res.Result != ResultInvalidState || res.Disposition != "ambiguous-change" {
			t.Fatalf("result=%q disposition=%q, want invalid-state/ambiguous-change", res.Result, res.Disposition)
		}
		if len(engine.calls) != 0 {
			t.Errorf("engine reached on an ambiguous id; want 0 calls, got %d", len(engine.calls))
		}
	})
}

// --- TestChangeClaimRetryConvergence ---------------------------------------

// TestChangeClaimRetryConvergence proves lost-response convergence: an idempotent
// replay of this exact request's own prior claim is `already-claimed`, while a
// foreign edit that moved the record is `contended`.
func TestChangeClaimRetryConvergence(t *testing.T) {
	setup := func(t *testing.T, res transaction.Result) ChangeClaimResult {
		repoDir := newMainModeRepo(t, nil).invocation
		engine := &recordingEngine{result: res}
		reader := &fakeReader{pin: mainModePin([]string{"inline"}), corpus: []StatusBlob{changeBlob(3, "widget", "feat", "high", "")}}
		deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}
		return ChangeClaim(context.Background(), deps, repoDir, ChangeClaimRequest{ID: 3, Version: "1234123412341234123412341234123412341234"})
	}

	t.Run("matching identity replays as already-claimed", func(t *testing.T) {
		receipt := mustMarshal(t, changeClaimReceipt{
			Branch: "feat/widget", ClaimedAt: "2026-08-16T12:00:00Z",
			ID: 3, Lease: "fresh", Op: OperationChangeClaim, Status: "in-progress",
		})
		res := setup(t, transaction.Result{
			Disposition:   transaction.DispositionAlreadyApplied,
			AppliedCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Receipt:       receipt,
		})
		if res.Result != ResultApplied {
			t.Fatalf("result = %q, want applied", res.Result)
		}
		if res.Disposition != ClaimDispositionAlreadyClaimed {
			t.Errorf("disposition = %q, want already-claimed", res.Disposition)
		}
		if res.Branch != "feat/widget" {
			t.Errorf("replay must echo the original receipt, branch = %q", res.Branch)
		}
	})

	t.Run("foreign claim is contended", func(t *testing.T) {
		res := setup(t, transaction.Result{Disposition: transaction.DispositionContended})
		if res.Result != ResultContended || res.Disposition != ClaimDispositionContended {
			t.Fatalf("result=%q disposition=%q, want contended/contended", res.Result, res.Disposition)
		}
	})
}

// --- TestChangeRefreshClaimStampsOnly --------------------------------------

// TestChangeRefreshClaimStampsOnly proves refresh re-stamps claimed_at (and the
// updated date) and nothing else, requires in-progress, and reports a version
// mismatch as contended — the stop-don't-overwrite instruction.
func TestChangeRefreshClaimStampsOnly(t *testing.T) {
	recPath := groomPath(3, "widget")

	t.Run("stamps claimed_at and updated only", func(t *testing.T) {
		// An in-progress record with an older claimed_at and an unknown frontmatter
		// key and unknown body section that must both survive untouched.
		src := lifecycleChange(3, "widget", "in-progress")
		src = strings.Replace(src, "trivial: false\n", "trivial: false\ncustom_field: 'survives'\n", 1)
		src += "\n## Custom notes\n\nUnknown section survives.\n"
		files := map[string]string{recPath: src}

		plan, opRes := claimPlanFor(t, files, baseRefreshOp(nil, 3))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		assertPlanPaths(t, plan, map[string]transaction.MutationKind{recPath: transaction.MutationReplace})

		rec := lifecycleRecordBytes(t, plan, recPath)
		if !strings.Contains(rec, "claimed_at: '2026-08-16T12:00:00Z'") {
			t.Errorf("claimed_at not re-stamped:\n%s", rec)
		}
		if !strings.Contains(rec, "updated: '2026-08-16'") {
			t.Errorf("updated not stamped:\n%s", rec)
		}
		// Untouched: status stays in-progress, branch/reconciled unchanged, and the
		// unknown key and section survive byte-identically.
		if !strings.Contains(rec, "status: in-progress") {
			t.Errorf("status must stay in-progress unquoted (untouched):\n%s", rec)
		}
		if !strings.Contains(rec, "branch: feat/widget") {
			t.Errorf("branch must be untouched:\n%s", rec)
		}
		if !strings.Contains(rec, "custom_field: 'survives'") {
			t.Errorf("unknown frontmatter key did not survive:\n%s", rec)
		}
		if !strings.Contains(rec, "## Custom notes\n\nUnknown section survives.\n") {
			t.Errorf("unknown body section did not survive:\n%s", rec)
		}
	})

	t.Run("requires in-progress", func(t *testing.T) {
		files := map[string]string{recPath: claimableChange(3, "widget")} // proposed
		plan, opRes := claimPlanFor(t, files, baseRefreshOp(nil, 3))
		if !opRes.Refused {
			t.Fatalf("refresh of a proposed change must refuse")
		}
		if !hasDomainFindingCode(opRes.Findings, "illegal-source-status") {
			t.Errorf("missing illegal-source-status; got %v", opRes.Findings)
		}
		if len(plan.Files) != 0 {
			t.Errorf("a refusal planned %d files, want 0", len(plan.Files))
		}
	})

	t.Run("version mismatch is contended", func(t *testing.T) {
		repoDir := newMainModeRepo(t, nil).invocation
		engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
		reader := &fakeReader{pin: mainModePin([]string{"inline"}), corpus: []StatusBlob{changeBlob(3, "widget", "feat", "high", "")}}
		deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

		res := ChangeRefreshClaim(context.Background(), deps, repoDir, ChangeClaimRequest{ID: 3, Version: "9999999999999999999999999999999999999999"})

		if res.Result != ResultContended || res.Disposition != ClaimDispositionContended {
			t.Fatalf("result=%q disposition=%q, want contended/contended", res.Result, res.Disposition)
		}
	})
}

func TestClaimResultFromOutcomeFailedCarriesCause(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageVerifyDelta,
		Kind:   transaction.KindInvalidState,
		Detail: "an undeclared path changed in the worktree",
	}
	res := transaction.Result{Disposition: transaction.DispositionFailed}

	out := claimResultFromOutcome(OperationChangeRefreshClaim, res, execErr)

	if out.Result != ResultInvalidState {
		t.Fatalf("result = %q, want %q", out.Result, ResultInvalidState)
	}
	if out.Disposition != ClaimDispositionFailed {
		t.Errorf("disposition = %q, want %q", out.Disposition, ClaimDispositionFailed)
	}
	if out.Disposition == string(out.Result) {
		t.Errorf("disposition %q merely restates the result — the tautology is back", out.Disposition)
	}
	if out.Failure == nil {
		t.Fatal("failure diagnosis missing on a failed disposition — the Failure was dropped again")
	}
	if out.Failure.Detail == "" {
		t.Error("failure.detail is empty")
	}
	if out.Failure.Stage != string(transaction.StageVerifyDelta) || out.Failure.Kind != string(transaction.KindInvalidState) {
		t.Errorf("failure = %+v, want stage %q kind %q", out.Failure, transaction.StageVerifyDelta, transaction.KindInvalidState)
	}
	if len(out.Findings) != 0 {
		t.Errorf("findings = %v, want empty — findings are the refusal channel, not the failure channel", out.Findings)
	}

	ok := claimResultFromOutcome(OperationChangeClaim, transaction.Result{Disposition: transaction.DispositionApplied}, nil)
	if ok.Failure != nil {
		t.Errorf("failure must be nil on an applied outcome, got %+v", ok.Failure)
	}
}
