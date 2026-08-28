//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestIntegrationChangeADRRecordAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	receipt := mustMarshal(t, adrRecordReceipt{
		ID: 7, Op: OperationADRRecord, Path: adrPath("0007", "record-the-widget-decision"),
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRRecordOp(context.Background(), deps, repoDir, validADRRecordRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.ID != 7 || res.Path != adrPath("0007", "record-the-widget-decision") {
		t.Errorf("identity from receipt = (%d, %q)", res.ID, res.Path)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}
	if res.Replayed {
		t.Errorf("Replayed = true on a fresh apply")
	}
	if res.Operation != OperationADRRecord {
		t.Errorf("operation = %q, want %q", res.Operation, OperationADRRecord)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationADRRecord {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/docket" {
		t.Errorf("target ref = %q, want refs/heads/docket", req.TargetRef)
	}
	if req.Idempotency == nil || req.Idempotency.RequestID != "adr-00000001" {
		t.Errorf("idempotency key = %+v", req.Idempotency)
	}
	// No producing change ⇒ no entity expectation.
	if len(req.Expected) != 0 {
		t.Errorf("plain record must carry no entity expectation, got %+v", req.Expected)
	}
}

func TestIntegrationChangeADRRecordRefusedMapsInvalidInput(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionRefused}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRRecordOp(context.Background(), deps, repoDir, validADRRecordRequest())

	if res.Result != ResultInvalidInput {
		t.Fatalf("refused disposition mapped to %q, want invalid-input", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

func TestIntegrationChangeADRRecordReplayResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	receipt := mustMarshal(t, adrRecordReceipt{
		ID: 4, Op: OperationADRRecord, Path: adrPath("0004", "record-the-widget-decision"),
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionAlreadyApplied,
		AppliedCommit: "0000000000000000000000000000000000000abc",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRRecordOp(context.Background(), deps, repoDir, validADRRecordRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if !res.Replayed {
		t.Errorf("Replayed = false on an already-applied replay")
	}
	if res.ID != 4 {
		t.Errorf("id = %d, want 4 (from the original receipt)", res.ID)
	}
}

func TestIntegrationChangeADRRecordWithProducingChangeCarriesExpectation(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	req := validADRRecordRequest()
	req.Change = &ADRProducingChange{ID: 1, Path: "docs/changes/active/0001-first.md", Version: blobV}

	res := ADRRecordOp(context.Background(), deps, repoDir, req)

	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	call := engine.calls[0]
	if call.Idempotency == nil {
		t.Errorf("record is allocating; it must carry an idempotency key")
	}
	if len(call.Expected) != 1 {
		t.Fatalf("expected 1 entity expectation for the producing change, got %d", len(call.Expected))
	}
	exp := call.Expected[0]
	if string(exp.Path) != "docs/changes/active/0001-first.md" {
		t.Errorf("expectation path = %q", exp.Path)
	}
	if exp.Version.Kind != transaction.VersionBlob || string(exp.Version.ObjectID) != blobV {
		t.Errorf("expectation version = %+v", exp.Version)
	}
}

func TestIntegrationChangeADRReverseUsesReverseOperationKey(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRReverse(context.Background(), deps, repoDir, validADRReplaceRequest())
	if res.Operation != OperationADRReverse {
		t.Errorf("operation = %q, want %q", res.Operation, OperationADRReverse)
	}
	if engine.calls[0].Operation.Key() != OperationADRReverse {
		t.Errorf("operation key = %q", engine.calls[0].Operation.Key())
	}
}

func TestIntegrationChangeADRSupersedeAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	receipt := mustMarshal(t, adrRecordReceipt{
		ID: 7, Op: OperationADRSupersede, Path: adrPath("0007", "supersede-the-widget-decision"),
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRSupersede(context.Background(), deps, repoDir, validADRReplaceRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.ID != 7 || res.Path != adrPath("0007", "supersede-the-widget-decision") {
		t.Errorf("identity from receipt = (%d, %q)", res.ID, res.Path)
	}
	if res.Operation != OperationADRSupersede {
		t.Errorf("operation = %q, want %q", res.Operation, OperationADRSupersede)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	call := engine.calls[0]
	if call.Operation.Key() != OperationADRSupersede {
		t.Errorf("operation key = %q", call.Operation.Key())
	}
	if call.Idempotency == nil || call.Idempotency.RequestID != "adr-replace-0001" {
		t.Errorf("idempotency key = %+v", call.Idempotency)
	}
	// The target ADR is pinned by an exact-blob entity expectation.
	if len(call.Expected) != 1 {
		t.Fatalf("expected 1 entity expectation (the target), got %d", len(call.Expected))
	}
	exp := call.Expected[0]
	if string(exp.Path) != "docs/adrs/0001-one.md" {
		t.Errorf("target expectation path = %q", exp.Path)
	}
	if exp.Version.Kind != transaction.VersionBlob || string(exp.Version.ObjectID) != blobV {
		t.Errorf("target expectation version = %+v", exp.Version)
	}
}

// A blank successor RequestID must NOT fail the shape check — the outer key
// governs an ADR replacement, the inner one is ignored.
func TestIntegrationChangeADRSupersedeIgnoresSuccessorRequestID(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	req := validADRReplaceRequest()
	req.Successor.RequestID = "" // ignored

	res := ADRSupersede(context.Background(), deps, repoDir, req)
	if res.Result == ResultInvalidInput {
		t.Fatalf("blank successor request id wrongly failed shape validation: %v", res.Findings)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1 (shape passed)", len(engine.calls))
	}
}

func TestIntegrationChangeADRSupersedeRefusedNonAcceptedMapsInvalidState(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	// A refusal carrying the domain not-Accepted reason is state-shaped.
	engine := &recordingEngine{result: transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings:    []domain.Finding{{Code: adrNotAcceptedReason, Severity: domain.SeverityError}},
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRSupersede(context.Background(), deps, repoDir, validADRReplaceRequest())
	if res.Result != ResultInvalidState {
		t.Fatalf("not-Accepted refusal mapped to %q, want invalid-state", res.Result)
	}
}

func TestIntegrationChangeADRSupersedeRefusedRequestShapedMapsInvalidInput(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings:    []domain.Finding{{Code: "adr-dangling-reference", Severity: domain.SeverityError}},
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ADRSupersede(context.Background(), deps, repoDir, validADRReplaceRequest())
	if res.Result != ResultInvalidInput {
		t.Fatalf("request-shaped refusal mapped to %q, want invalid-input", res.Result)
	}
}

func TestIntegrationChangeADRSupersedeWithProducingChangeCarriesTwoExpectations(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	req := validADRReplaceRequest()
	req.Successor.Change = &ADRProducingChange{ID: 3, Path: "docs/changes/active/0003-third.md", Version: blobV}

	res := ADRSupersede(context.Background(), deps, repoDir, req)
	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	call := engine.calls[0]
	if len(call.Expected) != 2 {
		t.Fatalf("expected 2 entity expectations (target + producing change), got %d", len(call.Expected))
	}
	paths := map[string]bool{}
	for _, e := range call.Expected {
		paths[string(e.Path)] = true
	}
	if !paths["docs/adrs/0001-one.md"] || !paths["docs/changes/active/0003-third.md"] {
		t.Errorf("expectations do not cover both target and change: %v", paths)
	}
}

func TestIntegrationChangeBlockAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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
	if req.TargetRef != "refs/heads/docket" {
		t.Errorf("target ref = %q, want refs/heads/docket", req.TargetRef)
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

// TestChangeClaimApplies proves both halves of a successful claim: the app layer
// submits the exact expected version, an idempotency key, and the metadata
// target ref (recordingEngine); and the plan closure patches status/branch/
// claimed_at and names the record, board, and artifact surfaces.
func TestIntegrationChangeClaimApplies(t *testing.T) {
	const version = "1234123412341234123412341234123412341234"

	t.Run("submitted request", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
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
		if req.TargetRef != "refs/heads/docket" {
			t.Errorf("target ref = %q, want refs/heads/docket", req.TargetRef)
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
func TestIntegrationChangeClaimRefusals(t *testing.T) {
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
		repoDir := newWorkingRepo(t, nil).invocation
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
		repoDir := newWorkingRepo(t, nil).invocation
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

// TestChangeClaimRetryConvergence proves lost-response convergence: an idempotent
// replay of this exact request's own prior claim is `already-claimed`, while a
// foreign edit that moved the record is `contended`.
func TestIntegrationChangeClaimRetryConvergence(t *testing.T) {
	setup := func(t *testing.T, res transaction.Result) ChangeClaimResult {
		repoDir := newWorkingRepo(t, nil).invocation
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

func TestIntegrationChangeCreateAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	receipt := mustMarshal(t, changeCreateReceipt{
		ID: 7, Op: OperationChangeCreate, Path: "docs/changes/active/0007-add-a-widget.md", Slug: "add-a-widget",
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeCreate(context.Background(), deps, repoDir, validChangeCreateRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.ID != 7 || res.Slug != "add-a-widget" || res.Path != "docs/changes/active/0007-add-a-widget.md" {
		t.Errorf("identity from receipt = (%d, %q, %q)", res.ID, res.Slug, res.Path)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}
	if res.Replayed {
		t.Errorf("Replayed = true on a fresh apply")
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationChangeCreate {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/docket" {
		t.Errorf("target ref = %q, want refs/heads/docket", req.TargetRef)
	}
	if req.Remote != originRemote {
		t.Errorf("remote = %q", req.Remote)
	}
	if req.Idempotency == nil || req.Idempotency.RequestID != "req-00000001" {
		t.Errorf("idempotency key = %+v", req.Idempotency)
	}
	if req.Loader == nil {
		t.Errorf("loader is nil")
	}
}

func TestIntegrationChangeCreateContendedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeCreate(context.Background(), deps, repoDir, validChangeCreateRequest())

	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

func TestIntegrationChangeCreateRefusedMapsInvalidInput(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings: []domain.Finding{{
			Code: "dangling-reference", Severity: domain.SeverityError,
			Entity: domain.EntityRef{Kind: domain.EntityChange}, Field: "depends_on",
		}},
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeCreate(context.Background(), deps, repoDir, validChangeCreateRequest())

	if res.Result != ResultInvalidInput {
		t.Fatalf("refused disposition mapped to %q, want invalid-input", res.Result)
	}
	if !hasFindingCode(res.Findings, "dangling-reference") {
		t.Errorf("refusal findings not surfaced: %v", res.Findings)
	}
}

func TestIntegrationChangeCreateReplayResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	receipt := mustMarshal(t, changeCreateReceipt{
		ID: 4, Op: OperationChangeCreate, Path: "docs/changes/active/0004-add-a-widget.md", Slug: "add-a-widget",
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionAlreadyApplied,
		AppliedCommit: "0000000000000000000000000000000000000abc",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeCreate(context.Background(), deps, repoDir, validChangeCreateRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if !res.Replayed {
		t.Errorf("Replayed = false on an already-applied replay")
	}
	if res.ID != 4 {
		t.Errorf("id = %d, want 4 (from the original receipt)", res.ID)
	}
}

func TestIntegrationChangeDeferAppliedResultCarriesDeferStatus(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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

// TestEvidenceRecordFromPassedRun: a green terminal record plus a feature head
// matching the request produces an immutable record carrying the OBSERVED gate
// command (never a request field) and the exact head; the rendered block
// round-trips through evidence.Extract.
func TestIntegrationChangeEvidenceRecordFromPassedRun(t *testing.T) {
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(evidenceHead)},
	}
	deps, wdeps, repoDir := evidenceDeps(t, svc)
	runDir := passedRunDir(t)

	res := EvidenceRecord(context.Background(), deps, wdeps, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: evidenceHead})

	if res.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q msg=%q, want applied", res.Result, res.Reason, res.Message)
	}
	if res.Command != testGateCommand {
		t.Errorf("command=%q, want the observed gate command %q", res.Command, testGateCommand)
	}
	if res.Head != evidenceHead {
		t.Errorf("head=%q, want %q", res.Head, evidenceHead)
	}
	if res.Outcome != string(evidence.ResultGreen) {
		t.Errorf("outcome=%q, want green", res.Outcome)
	}
	if res.Block == "" {
		t.Fatal("no rendered evidence block")
	}
	rec, err := evidence.Extract([]byte(res.Block))
	if err != nil {
		t.Fatalf("block did not round-trip through Extract: %v", err)
	}
	if rec.Command != testGateCommand || rec.Head != evidenceHead || rec.Result != evidence.ResultGreen {
		t.Errorf("extracted record = %+v, want command/head to match and green", rec)
	}
}

// TestEvidenceRecordRefusals: failed, still-running, vanished, malformed, and
// head-mismatch runs each refuse with a DISTINCT stable reason and never render
// a block. A malformed or unreadable run dir is a probe error — its own typed
// failure, never folded into the clean "vanished" absence
// (probe-error-is-not-clean-absence). The whole table is the mutation guard:
// strip the passed-only gate and the non-passed rows would produce a block.
func TestIntegrationChangeEvidenceRecordRefusals(t *testing.T) {
	cases := []struct {
		name       string
		runDir     func(t *testing.T) string
		reqHead    string
		wantReason string
		reached    bool // whether workspace Inspect should have been reached
	}{
		{
			name:       "failed run",
			runDir:     func(t *testing.T) string { return runToTerminal(t, []string{"/bin/sh", "-c", "exit 3"}, "failed") },
			reqHead:    evidenceHead,
			wantReason: ReasonEvidenceGateFailed,
		},
		{
			name:       "still-running lock",
			runDir:     runningRunDir,
			reqHead:    evidenceHead,
			wantReason: ReasonEvidenceGateRunning,
		},
		{
			name: "vanished dir",
			runDir: func(t *testing.T) string {
				d := passedRunDir(t)
				if err := os.Remove(filepath.Join(d, "terminal.json")); err != nil {
					t.Fatalf("removing terminal record: %v", err)
				}
				return d
			},
			reqHead:    evidenceHead,
			wantReason: ReasonEvidenceGateVanished,
		},
		{
			name: "malformed terminal.json",
			runDir: func(t *testing.T) string {
				d := passedRunDir(t)
				if err := os.WriteFile(filepath.Join(d, "terminal.json"), []byte("{not json"), 0o600); err != nil {
					t.Fatalf("corrupting terminal record: %v", err)
				}
				return d
			},
			reqHead:    evidenceHead,
			wantReason: ReasonEvidenceProbeMalformed,
		},
		{
			name:       "head mismatch",
			runDir:     passedRunDir,
			reqHead:    evidenceOtherHead,
			wantReason: ReasonEvidenceHeadMismatch,
			reached:    true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &fakeWorkspaceService{
				inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(evidenceHead)},
			}
			deps, wdeps, repoDir := evidenceDeps(t, svc)
			runDir := c.runDir(t)

			res := EvidenceRecord(context.Background(), deps, wdeps, repoDir,
				EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: c.reqHead})

			if res.Result == ResultApplied {
				t.Fatalf("a %s run produced a record: %+v", c.name, res)
			}
			if res.Reason != c.wantReason {
				t.Fatalf("reason=%q, want %q (result %q)", res.Reason, c.wantReason, res.Result)
			}
			if res.Block != "" {
				t.Errorf("a refusal rendered a block: %q", res.Block)
			}
			if !c.reached && len(svc.inspectCalls) != 0 {
				t.Errorf("workspace Inspect reached on a pre-inspect refusal (%d calls)", len(svc.inspectCalls))
			}
		})
	}
}

// TestEvidenceRecordUnconfiguredGate: a passed run but no resolved
// finalize.test_command has no observed gate command to record, so the
// operation refuses (unsupported-config) rather than fabricate an empty command.
func TestIntegrationChangeEvidenceRecordUnconfiguredGate(t *testing.T) {
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(evidenceHead)},
	}
	reader := &fakeReader{
		pin:    mainPin(t), // built-in config: test_command resolves to unset ("")
		corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")},
	}
	deps := workspaceDepsFor(t, reader)
	repoDir := newWorkingRepo(t, nil).invocation
	runDir := passedRunDir(t)

	res := EvidenceRecord(context.Background(), deps, WorkspaceDeps{Service: svc}, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: evidenceHead})

	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result=%q reason=%q, want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if res.Block != "" {
		t.Errorf("refusal rendered a block: %q", res.Block)
	}
}

// TestEvidenceRecordUnreadableRunDir: a run dir the process cannot read is a
// probe error (its own external failure), never a silent "no evidence".
func TestIntegrationChangeEvidenceRecordUnreadableRunDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	svc := &fakeWorkspaceService{}
	deps, wdeps, repoDir := evidenceDeps(t, svc)
	runDir := passedRunDir(t)
	if err := os.Chmod(runDir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(runDir, 0o755) })

	res := EvidenceRecord(context.Background(), deps, wdeps, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: runDir, Head: evidenceHead})

	if res.Result == ResultApplied {
		t.Fatalf("an unreadable run dir produced a record: %+v", res)
	}
	if res.Reason != ReasonEvidenceProbeUnreadable {
		t.Fatalf("reason=%q, want %q", res.Reason, ReasonEvidenceProbeUnreadable)
	}
	if len(svc.inspectCalls) != 0 {
		t.Errorf("workspace Inspect reached on a probe error (%d calls)", len(svc.inspectCalls))
	}
}

func TestIntegrationChangeGateLaunchObserveEndToEnd(t *testing.T) {
	root := t.TempDir()
	res := GateLaunch(root, t.TempDir(), []string{"/bin/echo", "hello"})
	if res.Operation != "gate.launch" {
		t.Fatalf("operation %q", res.Operation)
	}
	if res.Result != ResultApplied {
		t.Fatalf("launch result %s (%s)", res.Result, res.Reason)
	}
	if res.RunDir == "" || res.RunID == "" {
		t.Fatalf("no handle: %+v", res)
	}
	// Poll observe to terminal; /bin/echo exits 0 fast.
	deadline := 300 // x100ms
	for i := 0; ; i++ {
		obs := GateObserve(res.RunDir)
		if obs.Result == ResultApplied && obs.State == "passed" {
			if obs.ExitCode == nil || *obs.ExitCode != 0 {
				t.Fatalf("exact code: %+v", obs.ExitCode)
			}
			break
		}
		if obs.State != "running" {
			t.Fatalf("unexpected: %+v", obs)
		}
		if i > deadline {
			t.Fatal("echo never became terminal")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestIntegrationChangeGatePruneRetentionWindow(t *testing.T) {
	repo := newGateRepo(t)
	root, err := gateRoot(repo)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
	}

	terminal := sampleGateRecord()
	terminal.Terminal = true
	nonterminal := sampleGateRecord()
	nonterminal.Terminal = false

	kOld, err := MintGateRecord(repo, terminal) // terminal, backdated past window -> pruned
	if err != nil {
		t.Fatal(err)
	}
	kFresh, err := MintGateRecord(repo, terminal) // terminal, inside window -> kept
	if err != nil {
		t.Fatal(err)
	}
	kLive, err := MintGateRecord(repo, nonterminal) // nonterminal, backdated past window -> kept
	if err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-8 * 24 * time.Hour)
	for _, k := range []string{kOld, kLive} {
		rp := filepath.Join(root, k, "record.json")
		if err := os.Chtimes(rp, past, past); err != nil {
			t.Fatal(err)
		}
	}

	PruneGateRecords(repo)

	if _, err := os.Stat(filepath.Join(root, kOld)); !os.IsNotExist(err) {
		t.Errorf("terminal record backdated past retention was not pruned (stat err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(root, kFresh)); err != nil {
		t.Errorf("terminal record inside the window was pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, kLive)); err != nil {
		t.Errorf("nonterminal record backdated past the window was age-pruned: %v", err)
	}
}

func TestIntegrationChangeGateRecordCorruptAndSchema(t *testing.T) {
	repo := newGateRepo(t)
	root, err := gateRoot(repo)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
	}

	// Corrupt JSON.
	kCorrupt, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, kCorrupt, "record.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadGateRecord(repo, kCorrupt)
	if gse, ok := AsGateStoreError(err); !ok || gse.Kind != ErrGateCorruptRecord {
		t.Errorf("corrupt JSON load = %v, want ErrGateCorruptRecord", err)
	}

	// Unsupported schema.
	kSchema, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	rec99 := `{"schema":99,"repo":"whatever","target":"docket-implement-next","retry":"unused"}`
	if err := os.WriteFile(filepath.Join(root, kSchema, "record.json"), []byte(rec99), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadGateRecord(repo, kSchema)
	if gse, ok := AsGateStoreError(err); !ok || gse.Kind != ErrGateCorruptRecord {
		t.Errorf("schema 99 load = %v, want ErrGateCorruptRecord", err)
	}
}

func TestIntegrationChangeGateRecordLinkedWorktreeSameRecord(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}

	wt := filepath.Join(t.TempDir(), "linked")
	runGit(t, repo, "worktree", "add", wt)

	// The linked worktree shares the same git common dir, so the record minted
	// through the main worktree resolves to the SAME record here.
	got, err := LoadGateRecord(wt, key)
	if err != nil {
		t.Fatalf("LoadGateRecord(linked worktree): %v", err)
	}
	if got.Target != "docket-implement-next" || !slices.Equal(got.BeforeIDs, []int{12, 34, 56}) {
		t.Errorf("linked-worktree load = %+v; want the same record", got)
	}
}

func TestIntegrationChangeGateRecordMalformedKey(t *testing.T) {
	repo := newGateRepo(t)
	for _, key := range []string{"../escape", "", "UPPER", repeat("a", 300)} {
		_, err := LoadGateRecord(repo, key)
		gse, ok := AsGateStoreError(err)
		if !ok || gse.Kind != ErrGateMalformedKey {
			t.Errorf("LoadGateRecord(%q) = %v, want ErrGateMalformedKey", key, err)
		}
	}

	// Key validation MUST precede any filesystem or git touch: a malformed key
	// against a path that is not even a git repo still returns malformed-key,
	// never a git/IO error.
	_, err := LoadGateRecord(filepath.Join(t.TempDir(), "not-a-repo"), "../escape")
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateMalformedKey {
		t.Errorf("malformed key against non-repo = %v, want ErrGateMalformedKey (validated before fs)", err)
	}
}

func TestIntegrationChangeGateRecordMintLoadRoundTrip(t *testing.T) {
	repo := newGateRepo(t)
	rec := sampleGateRecord()

	key, err := MintGateRecord(repo, rec)
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(key) {
		t.Fatalf("minted key %q does not match ^[a-z0-9-]+$", key)
	}
	if len(key) < 15 || len(key) > 128 {
		t.Fatalf("minted key %q length %d out of bounds", key, len(key))
	}
	if !regexp.MustCompile(`^implement-next-`).MatchString(key) {
		t.Fatalf("minted key %q is not prefixed implement-next-", key)
	}

	got, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if got.Schema != gateSchemaVersion {
		t.Errorf("Schema = %d, want %d", got.Schema, gateSchemaVersion)
	}
	if got.Repo == "" {
		t.Errorf("Repo is empty; want the canonical common-dir path")
	}
	if got.Target != rec.Target {
		t.Errorf("Target = %q, want %q", got.Target, rec.Target)
	}
	if got.CreatedAt != rec.CreatedAt {
		t.Errorf("CreatedAt = %d, want %d", got.CreatedAt, rec.CreatedAt)
	}
	if got.DispatchEpoch != rec.DispatchEpoch {
		t.Errorf("DispatchEpoch = %d, want %d", got.DispatchEpoch, rec.DispatchEpoch)
	}
	if !slices.Equal(got.BeforeIDs, rec.BeforeIDs) {
		t.Errorf("BeforeIDs = %v, want %v", got.BeforeIDs, rec.BeforeIDs)
	}
	if got.AttributedID != rec.AttributedID {
		t.Errorf("AttributedID = %d, want %d", got.AttributedID, rec.AttributedID)
	}
	if got.Retry != rec.Retry {
		t.Errorf("Retry = %q, want %q", got.Retry, rec.Retry)
	}
	if got.Disposition != rec.Disposition {
		t.Errorf("Disposition = %q, want %q", got.Disposition, rec.Disposition)
	}
	if got.Terminal != rec.Terminal {
		t.Errorf("Terminal = %v, want %v", got.Terminal, rec.Terminal)
	}
}

func TestIntegrationChangeGateRecordSaveDurableReload(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}

	loaded, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	loaded.Disposition = "gate-done run-complete"
	loaded.Terminal = true
	loaded.AttributedID = 42
	if err := SaveGateRecord(repo, key, loaded); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}

	// A fresh call reads only from disk — there is no shared in-memory state, so
	// this proves restart durability.
	again, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord (reload): %v", err)
	}
	if again.Disposition != "gate-done run-complete" || !again.Terminal || again.AttributedID != 42 {
		t.Errorf("reloaded record = %+v; save did not persist durably", again)
	}
}

func TestIntegrationChangeGateRecordWrongRepo(t *testing.T) {
	repoA := newGateRepo(t)
	repoB := newGateRepo(t)

	key, err := MintGateRecord(repoA, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord(repoA): %v", err)
	}

	// Simulate the record ending up under repo B's store (a stale copy, a moved
	// .git). The embedded Repo still names repo A, so a load from repo B must be
	// refused as wrong-repo rather than trusted.
	rootA, err := gateRoot(repoA)
	if err != nil {
		t.Fatalf("gateRoot(repoA): %v", err)
	}
	rootB, err := gateRoot(repoB)
	if err != nil {
		t.Fatalf("gateRoot(repoB): %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rootB, key), 0o755); err != nil {
		t.Fatal(err)
	}
	buf, err := os.ReadFile(filepath.Join(rootA, key, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, key, "record.json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = LoadGateRecord(repoB, key)
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateWrongRepo {
		t.Fatalf("LoadGateRecord(repoB, key) = %v, want ErrGateWrongRepo", err)
	}
}

func TestIntegrationChangeGateRetryConsumeOnceThenFalse(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}

	first, err := ConsumeGateRetry(repo, key)
	if err != nil {
		t.Fatalf("ConsumeGateRetry (first): %v", err)
	}
	if !first {
		t.Fatalf("first ConsumeGateRetry = false, want true")
	}

	// The JSON mirror is flipped to consumed (the marker is authority; the field
	// is the readable mirror).
	loaded, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord after consume: %v", err)
	}
	if loaded.Retry != RetryConsumed {
		t.Errorf("Retry = %q after consume, want %q", loaded.Retry, RetryConsumed)
	}

	second, err := ConsumeGateRetry(repo, key)
	if err != nil {
		t.Fatalf("ConsumeGateRetry (second): %v", err)
	}
	if second {
		t.Errorf("second ConsumeGateRetry = true, want false (permit already spent)")
	}
}

func TestIntegrationChangeGroomAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	specPath := "docs/superpowers/specs/2026-08-16-add-a-widget-design.md"
	receipt := mustMarshal(t, changeGroomReceipt{
		ID: 2, Op: OperationChangeGroom, Outcome: string(GroomSpec), SpecPath: specPath,
	})
	engine := &recordingEngine{result: transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       receipt,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, repoDir, validGroomSpecRequest())

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if res.ID != 2 || res.SpecPath != specPath {
		t.Errorf("identity from receipt = (%d, %q)", res.ID, res.SpecPath)
	}
	if res.Revision != "cafebabecafebabecafebabecafebabecafebabe" {
		t.Errorf("revision = %q", res.Revision)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Operation.Key() != OperationChangeGroom {
		t.Errorf("operation key = %q", req.Operation.Key())
	}
	if req.TargetRef != "refs/heads/docket" {
		t.Errorf("target ref = %q, want refs/heads/docket", req.TargetRef)
	}
	if req.Idempotency != nil {
		t.Errorf("groom is non-allocating; it must carry no idempotency key, got %+v", req.Idempotency)
	}
	if len(req.Expected) != 1 {
		t.Fatalf("expected %d entity expectations, want 1", len(req.Expected))
	}
	exp := req.Expected[0]
	if string(exp.Path) != groomPath(2, "add-a-widget") {
		t.Errorf("expectation path = %q", exp.Path)
	}
	if exp.Version.Kind != transaction.VersionBlob ||
		string(exp.Version.ObjectID) != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("expectation version = %+v", exp.Version)
	}
}

func TestIntegrationChangeGroomContendedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, repoDir, validGroomSpecRequest())

	if res.Result != ResultContended {
		t.Fatalf("result = %q, want contended", res.Result)
	}
	if res.Findings == nil {
		t.Errorf("Findings must marshal as [], not nil")
	}
}

func TestIntegrationChangeGroomRefusedMapsInvalidState(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{
		Disposition: transaction.DispositionRefused,
		Findings:    nil,
	}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeGroom(context.Background(), deps, repoDir, validGroomSpecRequest())

	if res.Result != ResultInvalidState {
		t.Fatalf("refused disposition mapped to %q, want invalid-state", res.Result)
	}
}

func TestIntegrationChangeKillAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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
	if req.TargetRef != "refs/heads/docket" {
		t.Errorf("target ref = %q, want refs/heads/docket", req.TargetRef)
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

func TestIntegrationChangeKillContendedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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

func TestIntegrationChangeKillRefusedMapsInvalidState(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionRefused}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeKill(context.Background(), deps, repoDir, validKillRequest())

	if res.Result != ResultInvalidState {
		t.Fatalf("refused disposition mapped to %q, want invalid-state", res.Result)
	}
}

func TestIntegrationChangeLearningRecordAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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
	if req.TargetRef != "refs/heads/docket" {
		t.Errorf("target ref = %q, want refs/heads/docket", req.TargetRef)
	}
	if req.Idempotency == nil || req.Idempotency.RequestID != "learn-00000001" {
		t.Errorf("idempotency key = %+v", req.Idempotency)
	}
	if len(req.Expected) != 0 {
		t.Errorf("record is allocating; it must carry no entity expectation, got %+v", req.Expected)
	}
}

func TestIntegrationChangeLearningRecordRefusedMapsInvalidInput(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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

func TestIntegrationChangeLearningRecordReplayResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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

func TestIntegrationChangeLearningUpdateAppliedResultCarriesExactVersion(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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

func TestIntegrationChangeLearningUpdateContendedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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

func TestIntegrationChangeLifecycleContendedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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

func TestIntegrationChangeLifecycleRefusedMapsInvalidState(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionRefused}}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

	res := ChangeBlock(context.Background(), deps, repoDir, validBlockRequest())

	if res.Result != ResultInvalidState {
		t.Fatalf("refused disposition mapped to %q, want invalid-state", res.Result)
	}
}

// TestMarkImplementedAppliesEndToEnd (real git): every conjunct holds, so the
// operation opens exactly one exact-version transaction and returns applied.
func TestIntegrationChangeMarkImplementedAppliesEndToEnd(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)
	pr := prRepo().Spec() + "#42"

	deps, wdeps, gdeps, inv, req, engine := buildMI(t, client, repo.invocation, miKit{
		reconciled: true, plan: miPlanPath(), version: miVersion, reqVersion: miVersion,
		reqHead: head, localHead: head, evidence: prEvidenceBytes(t, head),
		probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: pr,
	})

	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (findings %v)", res.Result, res.Findings)
	}
	if res.Status != "implemented" || res.Revision == "" {
		t.Errorf("applied result malformed: %+v", res)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want exactly 1", len(engine.calls))
	}
	exp := engine.calls[0].Expected
	if len(exp) != 1 || string(exp[0].Version.ObjectID) != miVersion {
		t.Errorf("transaction did not pin the exact version: %+v", exp)
	}
	if engine.calls[0].Operation.Key() != transaction.OperationKey(OperationChangeMarkImplemented) {
		t.Errorf("operation key = %q", engine.calls[0].Operation.Key())
	}
}

// TestMarkImplementedConjuncts is the mutation test for the five-conjunct
// reprobe: each row breaks exactly one conjunct and proves the operation refuses
// with that conjunct's stable reason WITHOUT ever calling the engine. The happy
// fixture (proven by TestMarkImplementedAppliesEndToEnd) satisfies all five.
func TestIntegrationChangeMarkImplementedConjuncts(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)
	pr := prRepo().Spec() + "#42"
	other := prOtherHead

	// happy returns the all-pass kit; each row mutates one field.
	happy := func() miKit {
		return miKit{
			reconciled: true, plan: miPlanPath(), version: miVersion, reqVersion: miVersion,
			reqHead: head, localHead: head, evidence: prEvidenceBytes(t, head),
			probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: pr,
		}
	}

	rows := []struct {
		name   string
		mutate func(k *miKit)
		reason string
	}{
		{ // conjunct 3
			name:   "evidence names another head",
			mutate: func(k *miKit) { k.evidence = prEvidenceBytes(t, other) },
			reason: ReasonImplementedEvidenceUnverified,
		},
		{ // conjunct 1
			name:   "record not reconciled",
			mutate: func(k *miKit) { k.reconciled = false },
			reason: ReasonImplementedNotReconciled,
		},
		{ // conjunct 1
			name:   "record not linked to a plan",
			mutate: func(k *miKit) { k.plan = "" },
			reason: ReasonImplementedPlanUnlinked,
		},
		{ // conjunct 1
			name:   "entity version moved",
			mutate: func(k *miKit) { k.reqVersion = "9999999999999999999999999999999999999999" },
			reason: ReasonImplementedVersionMismatch,
		},
		{ // conjunct 2a
			name: "local head differs from supplied head",
			mutate: func(k *miKit) {
				k.localHead = other
			},
			reason: ReasonImplementedLocalHeadMismatch,
		},
		{ // conjunct 2b
			name: "remote head differs from supplied head",
			mutate: func(k *miKit) {
				k.reqHead = other
				k.localHead = other
				k.evidence = prEvidenceBytes(t, other)
			},
			reason: ReasonImplementedRemoteHeadMismatch,
		},
		{ // conjunct 4
			name:   "no unique open PR for the feature branch",
			mutate: func(k *miKit) { k.probePRs = nil },
			reason: ReasonImplementedPRNotUnique,
		},
		{ // conjunct 4
			name: "open PR is not the supplied reference",
			mutate: func(k *miKit) {
				odd := happyPR(head)
				odd.Number = 99
				k.probePRs = []githubcli.PullRequest{odd}
			},
			reason: ReasonImplementedPRReferenceMismatch,
		},
		{ // conjunct 5
			name:   "attached results path no longer tracked at head",
			mutate: func(k *miKit) { k.results = "docs/changes/results/0003-widget-ghost.md" },
			reason: ReasonImplementedResultsIdentity,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			k := happy()
			row.mutate(&k)
			deps, wdeps, gdeps, inv, req, engine := buildMI(t, client, repo.invocation, k)

			res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
			if res.Result == ResultApplied {
				t.Fatalf("conjunct %q did not refuse (result applied)", row.name)
			}
			if len(engine.calls) != 0 {
				t.Fatalf("engine was called %d times on a refusal; a broken conjunct must never open a transaction", len(engine.calls))
			}
			code := firstStatusFindingCode(res.Findings)
			if code != row.reason {
				t.Fatalf("refusal reason = %q, want %q (findings %v)", code, row.reason, res.Findings)
			}
		})
	}
}

// TestMarkImplementedIdentityForms is the mutation test for the migrated identity
// conjunct (parsePRRef number vs the verified pr.Number): the transition applies
// when the supplied --pr names the verified PR in EITHER accepted form and
// refuses with pr-reference-mismatch when the number differs or the reference is
// unparseable. Number 42 is the verified PR (happyPR).
func TestIntegrationChangeMarkImplementedIdentityForms(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)

	cases := []struct {
		name        string
		reqPR       string
		wantApplied bool
	}{
		{"url form matches", miPRURL(), true},
		{"shorthand form matches", prRepo().Spec() + "#42", true},
		{"url form wrong number", "https://github.com/acme/widget/pull/99", false},
		{"shorthand wrong number", prRepo().Spec() + "#99", false},
		{"unparseable reference", "not-a-pr-ref", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, wdeps, gdeps, inv, req, engine := buildMI(t, client, repo.invocation, miKit{
				reconciled: true, plan: miPlanPath(), version: miVersion, reqVersion: miVersion,
				reqHead: head, localHead: head, evidence: prEvidenceBytes(t, head),
				probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: tc.reqPR,
			})
			res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
			if tc.wantApplied {
				if res.Result != ResultApplied {
					t.Fatalf("reqPR %q: result = %q, want applied (findings %v)", tc.reqPR, res.Result, res.Findings)
				}
				return
			}
			if res.Result == ResultApplied {
				t.Fatalf("reqPR %q: applied, want refusal", tc.reqPR)
			}
			if len(engine.calls) != 0 {
				t.Fatalf("reqPR %q: engine called %d times on a refusal", tc.reqPR, len(engine.calls))
			}
			if code := firstStatusFindingCode(res.Findings); code != ReasonImplementedPRReferenceMismatch {
				t.Fatalf("reqPR %q: reason = %q, want %q", tc.reqPR, code, ReasonImplementedPRReferenceMismatch)
			}
		})
	}
}

// TestMarkImplementedRecordsURL: the transition records the verified PR's
// canonical URL (pr.URL from the reprobe) as the manifest pr:, NOT the
// owner/repo#N shorthand the caller supplied. This is the board-safe form
// (boardPRCell mangles a shorthand to "#owner/repo#N"); the value is sourced from
// the snapshot, so it is the canonical URL even when --pr arrives as shorthand
// (change 0344).
func TestIntegrationChangeMarkImplementedRecordsURL(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)
	shorthand := prRepo().Spec() + "#42" // the caller may still assert the shorthand

	deps, wdeps, gdeps, inv, req, engine := buildMI(t, client, repo.invocation, miKit{
		reconciled: true, plan: miPlanPath(), version: miVersion, reqVersion: miVersion,
		reqHead: head, localHead: head, evidence: prEvidenceBytes(t, head),
		probePRs: []githubcli.PullRequest{happyPR(head)}, reqPR: shorthand,
	})

	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, inv, req)
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (findings %v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want exactly 1", len(engine.calls))
	}
	op, ok := engine.calls[0].Operation.(changeImplementedOp)
	if !ok {
		t.Fatalf("recorded operation is %T, want changeImplementedOp", engine.calls[0].Operation)
	}
	if op.pr != miPRURL() {
		t.Errorf("recorded pr: = %q, want the canonical URL %q (not the supplied shorthand %q)", op.pr, miPRURL(), shorthand)
	}
}

// TestMarkImplementedRetry: a change already implemented whose recorded PR
// reference matches the request replays the prior applied outcome as a no-op —
// no duplicate transition, engine never called.
func TestIntegrationChangeMarkImplementedRetry(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)
	pr := prRepo().Spec() + "#42"

	// An implemented record carrying the matching PR reference.
	src := lifecycleChange(3, miSlug, "in-progress")
	src = strings.Replace(src, "status: in-progress", "status: implemented", 1)
	src = strings.Replace(src, "plan:\n", "plan: '"+miPlanPath()+"'\n", 1)
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+pr+"'\nblocked_by:\n", 1)
	blob := StatusBlob{Kind: repository.KindChange, Location: repository.LocationActive, Path: groomPath(3, miSlug), Version: miVersion, Data: []byte(src)}
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{blob}, facts: domain.NewBranchFacts(nil)}
	engine := &recordingEngine{}
	deps := PlanningDeps{Client: client, Engine: engine, Reader: reader, Clock: testClock()}
	wdeps := WorkspaceDeps{Service: &fakeWorkspaceService{inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)}}}
	gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: []githubcli.PullRequest{happyPR(head)}}}

	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, repo.invocation,
		MarkImplementedRequest{ID: 3, Version: miVersion, Head: head, PR: pr, EvidenceRecord: prEvidenceBytes(t, head)})

	if res.Result != ResultNoOp {
		t.Fatalf("retry result = %q, want no-op (findings %v)", res.Result, res.Findings)
	}
	if res.Status != "implemented" {
		t.Errorf("retry status = %q, want implemented", res.Status)
	}
	if len(engine.calls) != 0 {
		t.Errorf("retry called the engine %d times, want 0 (no duplicate transition)", len(engine.calls))
	}
}

// TestMarkImplementedRetryCrossForm: the response-loss replay guard is now by
// parsed number (samePRRef), so an already-implemented change recorded in the
// canonical URL form replays as a no-op when the retry asserts the same PR in the
// shorthand form, and still refuses as contended when the asserted number
// differs. This mutation-tests the migrated guard on the recorded-URL path 0344
// introduces.
func TestIntegrationChangeMarkImplementedRetryCrossForm(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	head := repo.writerAdvance(t, "feat/"+miSlug, map[string]string{"impl.go": "package impl\n"})
	client := newGitClient(t)

	// An implemented record carrying the canonical URL form (what 0344 records).
	src := lifecycleChange(3, miSlug, "in-progress")
	src = strings.Replace(src, "status: in-progress", "status: implemented", 1)
	src = strings.Replace(src, "plan:\n", "plan: '"+miPlanPath()+"'\n", 1)
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+miPRURL()+"'\nblocked_by:\n", 1)

	newDeps := func() (PlanningDeps, WorkspaceDeps, GitHubDeps, *recordingEngine) {
		blob := StatusBlob{Kind: repository.KindChange, Location: repository.LocationActive, Path: groomPath(3, miSlug), Version: miVersion, Data: []byte(src)}
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{blob}, facts: domain.NewBranchFacts(nil)}
		engine := &recordingEngine{}
		deps := PlanningDeps{Client: client, Engine: engine, Reader: reader, Clock: testClock()}
		wdeps := WorkspaceDeps{Service: &fakeWorkspaceService{inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)}}}
		gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: []githubcli.PullRequest{happyPR(head)}}}
		return deps, wdeps, gdeps, engine
	}

	// Same PR asserted in the shorthand form: response-loss replay ⇒ no-op.
	deps, wdeps, gdeps, engine := newDeps()
	res := ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, repo.invocation,
		MarkImplementedRequest{ID: 3, Version: miVersion, Head: head, PR: prRepo().Spec() + "#42", EvidenceRecord: prEvidenceBytes(t, head)})
	if res.Result != ResultNoOp {
		t.Fatalf("cross-form replay result = %q, want no-op (findings %v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 0 {
		t.Errorf("cross-form replay called the engine %d times, want 0", len(engine.calls))
	}

	// A different PR number: genuine conflict ⇒ contended.
	deps, wdeps, gdeps, engine = newDeps()
	res = ChangeMarkImplemented(context.Background(), deps, wdeps, gdeps, repo.invocation,
		MarkImplementedRequest{ID: 3, Version: miVersion, Head: head, PR: prRepo().Spec() + "#99", EvidenceRecord: prEvidenceBytes(t, head)})
	if res.Result != ResultContended {
		t.Fatalf("different-PR replay result = %q, want contended (findings %v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 0 {
		t.Errorf("different-PR replay called the engine %d times, want 0", len(engine.calls))
	}
}

func TestIntegrationChangePRPublishAgreementChecks(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation

	baseReq := func() PRPublishRequest {
		return PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "Authored prose.\n", EvidenceRecord: prEvidenceBytes(t, prHead)}
	}

	cases := []struct {
		name       string
		req        func() PRPublishRequest
		svc        *fakeWorkspaceService
		gh         *fakeGitHub
		wantResult Result
		wantReason string
	}{
		{
			name:       "control-all-agree",
			req:        baseReq,
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("b")}},
			wantResult: ResultApplied,
		},
		{
			name: "head-invalid",
			req: func() PRPublishRequest {
				r := baseReq()
				r.Head = "not-a-full-oid"
				return r
			},
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repo: prRepo()},
			wantResult: ResultInvalidInput,
			wantReason: ReasonPRHeadInvalid,
		},
		{
			name: "evidence-stale-head",
			req: func() PRPublishRequest {
				r := baseReq()
				r.EvidenceRecord = prEvidenceBytes(t, prOtherHead)
				return r
			},
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repo: prRepo()},
			wantResult: ResultInvalidState,
			wantReason: ReasonPREvidenceUnverified,
		},
		{
			name: "evidence-missing",
			req: func() PRPublishRequest {
				r := baseReq()
				r.EvidenceRecord = []byte("just prose, no block\n")
				return r
			},
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repo: prRepo()},
			wantResult: ResultInvalidState,
			wantReason: ReasonPREvidenceUnverified,
		},
		{
			name:       "local-head-mismatch",
			req:        baseReq,
			svc:        readyService(prOtherHead), // workspace head differs from requested
			gh:         &fakeGitHub{repo: prRepo()},
			wantResult: ResultInvalidState,
			wantReason: ReasonPRLocalHeadMismatch,
		},
		{
			name:       "repository-unresolved",
			req:        baseReq,
			svc:        readyService(prHead),
			gh:         &fakeGitHub{repoErr: errors.New("gh repo view failed")},
			wantResult: ResultExternalFailed,
			wantReason: ReasonPRRepositoryUnresolved,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := workspaceDepsFor(t, prReader(t))
			res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: tc.svc}, GitHubDeps{Service: tc.gh},
				repoDir, tc.req())

			if res.Result != tc.wantResult {
				t.Fatalf("result = %q, want %q (reason %q msg %q)", res.Result, tc.wantResult, res.Reason, res.Message)
			}
			if tc.wantReason != "" && res.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", res.Reason, tc.wantReason)
			}
			if tc.name == "control-all-agree" {
				if len(tc.gh.ensureCalls) != 1 {
					t.Fatalf("control: EnsurePullRequest called %d times, want 1", len(tc.gh.ensureCalls))
				}
				return
			}
			// Every broken conjunct must refuse BEFORE EnsurePullRequest.
			if len(tc.gh.ensureCalls) != 0 {
				t.Fatalf("%s: EnsurePullRequest invoked on a broken conjunct (%d calls)", tc.name, len(tc.gh.ensureCalls))
			}
		})
	}
}

func TestIntegrationChangePRPublishBodyAssembly(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	reader := prReader(t)

	// Authored prose already carrying a STALE build-evidence block. Publishing
	// must preserve the prose, replace the evidence block deterministically, and
	// insert the backlink exactly once.
	staleRec, _ := evidence.NewRecord("old command", prOtherHead, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	authored := "Authored prose before.\n\n" + evidence.Render(staleRec) + "\n\nAuthored prose after.\n"

	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	deps := workspaceDepsFor(t, reader)
	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: authored, EvidenceRecord: prEvidenceBytes(t, prHead)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (reason %q)", res.Result, res.Reason)
	}
	if len(gh.ensureCalls) != 1 {
		t.Fatalf("EnsurePullRequest called %d times, want 1", len(gh.ensureCalls))
	}
	gotBody := gh.ensureCalls[0].Body

	// Independent golden: parse authored, insert the backlink at the top, upsert
	// the fresh evidence — the exact deterministic composition the operation owns.
	change := prSnapshotChange(t, reader, 7)
	backlink, err := render.BacklinkContent(change, render.LinkContext{MetadataBranch: "main"})
	if err != nil {
		t.Fatalf("BacklinkContent: %v", err)
	}
	doc, err := document.Parse([]byte(authored))
	if err != nil {
		t.Fatalf("parse authored: %v", err)
	}
	var ps document.PatchSet
	ps.InsertBlock("backlink", "generated — do not hand-edit", backlinkInterior(backlink), document.AtDocumentStart)
	withBacklink, err := doc.Apply(ps)
	if err != nil {
		t.Fatalf("insert backlink: %v", err)
	}
	freshRec, _ := evidence.NewRecord("go test ./...", prHead, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	wantBody, err := evidence.Upsert(withBacklink, freshRec)
	if err != nil {
		t.Fatalf("upsert evidence: %v", err)
	}
	if gotBody != string(wantBody) {
		t.Fatalf("assembled body mismatch:\n got: %q\nwant: %q", gotBody, string(wantBody))
	}

	// Spot invariants that the full-byte golden also encodes.
	if !strings.Contains(gotBody, "Authored prose before.") || !strings.Contains(gotBody, "Authored prose after.") {
		t.Errorf("authored prose not preserved: %q", gotBody)
	}
	if strings.Count(gotBody, "<!-- docket:backlink:start") != 1 {
		t.Errorf("backlink block not inserted exactly once: %q", gotBody)
	}
	if strings.Contains(gotBody, prOtherHead) {
		t.Errorf("stale evidence head survived the replace: %q", gotBody)
	}
	if !strings.Contains(gotBody, prHead) {
		t.Errorf("fresh evidence head absent from body: %q", gotBody)
	}
}

func TestIntegrationChangePRPublishRedaction(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	const secret = "SECRET-PR-BODY-CONTENT-do-not-leak"

	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR(secret)}}
	deps := workspaceDepsFor(t, prReader(t))
	res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
		repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: secret + "\n", EvidenceRecord: prEvidenceBytes(t, prHead)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}

	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("result JSON leaked the PR body bytes: %s", raw)
	}
	if strings.Contains(res.HumanText(), secret) {
		t.Fatalf("HumanText leaked the PR body bytes: %q", res.HumanText())
	}
}

func TestIntegrationChangePRPublishThroughFakeGH(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation

	cases := []struct {
		name       string
		disp       githubcli.EnsureDisposition
		withPR     bool
		wantResult Result
	}{
		{"created", githubcli.EnsureCreated, true, ResultApplied},
		{"adopted", githubcli.EnsureAdopted, true, ResultNoOp},
		{"unknown", githubcli.EnsureUnknown, false, ResultExternalFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ens := githubcli.EnsureResult{Disposition: tc.disp}
			if tc.withPR {
				ens.PR = prMatchPR("body")
			}
			gh := &fakeGitHub{repo: prRepo(), ensureRes: ens}
			deps := workspaceDepsFor(t, prReader(t))
			res := PRPublish(context.Background(), deps, WorkspaceDeps{Service: readyService(prHead)}, GitHubDeps{Service: gh},
				repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "Add widget", Body: "prose\n", EvidenceRecord: prEvidenceBytes(t, prHead)})

			if res.Result != tc.wantResult {
				t.Fatalf("result = %q, want %q (reason %q)", res.Result, tc.wantResult, res.Reason)
			}
			if res.Disposition != string(tc.disp) {
				t.Errorf("disposition = %q, want %q (carried verbatim)", res.Disposition, tc.disp)
			}
			if tc.withPR {
				if res.Number != 42 || res.Head != prHead || res.Base != "main" {
					t.Errorf("PR snapshot did not round-trip: %+v", res)
				}
				if res.URL != "https://github.com/acme/widget/pull/42" {
					t.Errorf("PR url mismatch: %q", res.URL)
				}
				if res.Reference != "github.com/acme/widget#42" {
					t.Errorf("PR reference = %q, want github.com/acme/widget#42", res.Reference)
				}
			}
		})
	}
}

// TestReclaimIndependentOfAutoPolicy proves explicit reclaim applies even when
// reclaim.auto is false — the auto policy governs only maintenance sweep.
func TestIntegrationChangeReclaimIndependentOfAutoPolicy(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	repo := newWorkingRepo(t, map[string]string{
		".docket.yml": "reclaim:\n  auto: false\n",
		recPath:       lifecycleChange(3, "widget", "in-progress"),
	})
	node := planningDepsFor(t, repo.invocation)
	ver := blobVersionAt(t, repo.origin, "docket", recPath)
	res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
		ChangeReclaimRequest{ID: 3, Version: ver})
	if res.Result != ResultApplied || res.Disposition != ReclaimDispReclaimed {
		t.Fatalf("explicit reclaim under reclaim.auto:false = (%q, %q) findings=%v", res.Result, res.Disposition, res.Findings)
	}
}

// TestReclaimMalformedLeaseSkips proves a record whose claim stamp is malformed
// (an unevaluable lease, hence a corpus error) is refused end-to-end with no
// mutation — the destructive leg fails closed on an unreadable lease.
func TestIntegrationChangeReclaimMalformedLeaseSkips(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	repo := newWorkingRepo(t, map[string]string{
		recPath: reclaimRecordWithClaim(3, "widget", "not-a-timestamp"),
	})
	node := planningDepsFor(t, repo.invocation)
	ver := blobVersionAt(t, repo.origin, "docket", recPath)
	before, _ := originFile(t, repo.origin, "docket", recPath)
	res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
		ChangeReclaimRequest{ID: 3, Version: ver})
	if res.Result == ResultApplied {
		t.Fatalf("a malformed lease was reclaimed: %q", res.Result)
	}
	if res.Disposition != ReclaimDispSkipped {
		t.Errorf("disposition = %q, want skipped", res.Disposition)
	}
	assertOriginRecordUnchanged(t, repo.origin, "docket", recPath, before)
}

// TestReclaimRequiresProvenAbsence proves the destructive gate refuses and
// mutates NOTHING when a recorded/conventional branch is still present (locally
// or remotely), when an owned workspace is still live, or when any probe cannot
// be answered — unknown never shares the absent branch. Every refusal leaves the
// origin record byte-identical.
func TestIntegrationChangeReclaimRequiresProvenAbsence(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			// branch present locally.
			t.Run("local-branch-present", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				runGit(t, repo.invocation, "branch", "feat/widget")
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				assertReclaimSkipped(t, res, ReasonReclaimBranchPresent)
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})

			// the fresh-mint candidate is <type>/<slug>, not the fixed feat/
			// prefix: a type-fix change probes fix/<slug> alongside its recorded
			// name. Here the recorded feat/widget is absent, so the block can only
			// come from the minted fix/widget candidate.
			t.Run("mint-candidate-probes-type", func(t *testing.T) {
				rec := strings.Replace(lifecycleChange(3, "widget", "in-progress"), "type: feat", "type: fix", 1)
				repo := m.build(t, map[string]string{recPath: rec})
				node := planningDepsFor(t, repo.invocation)
				runGit(t, repo.invocation, "branch", "fix/widget")
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				assertReclaimSkipped(t, res, ReasonReclaimBranchPresent)
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})

			// branch present remotely (local absent).
			t.Run("remote-branch-present", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				runGit(t, repo.invocation, "push", "-q", "origin", "HEAD:refs/heads/feat/widget")
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				assertReclaimSkipped(t, res, ReasonReclaimBranchPresent)
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})

			// an owned workspace still holds live work.
			t.Run("workspace-active", func(t *testing.T) {
				for _, kind := range []workspace.StateKind{workspace.StateReady, workspace.StateDirty, workspace.StateResumable, workspace.StateMismatch} {
					repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
					node := planningDepsFor(t, repo.invocation)
					ver := blobVersionAt(t, repo.origin, m.branch, recPath)
					before, _ := originFile(t, repo.origin, m.branch, recPath)
					res := ChangeReclaim(context.Background(), node.deps,
						WorkspaceDeps{Service: fakeReclaimWorkspace{kind: kind}}, node.dir,
						ChangeReclaimRequest{ID: 3, Version: ver})
					assertReclaimSkipped(t, res, ReasonReclaimWorkspaceActive)
					assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
				}
			})

			// a workspace probe that cannot be answered fails closed.
			t.Run("workspace-probe-error", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps,
					WorkspaceDeps{Service: fakeReclaimWorkspace{err: errors.New("inspect boom")}}, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				assertReclaimSkipped(t, res, ReasonReclaimWorkspaceProbe)
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})
		})
	}
}

// TestReclaimTransaction proves the applied path end-to-end: the landed Reclaim
// action returns the record to proposed, clears branch/claim, sets
// reconciled:false, appends one dated ## Reclaim log entry, and rerenders the
// board — all in one atomic commit; and that an exact-version contention refuses.
func TestIntegrationChangeReclaimTransaction(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			t.Run("reclaims", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				if res.Result != ResultApplied || res.Disposition != ReclaimDispReclaimed {
					t.Fatalf("reclaim = (%q, %q) reason=%q findings=%v", res.Result, res.Disposition, res.Reason, res.Findings)
				}
				if res.Revision == "" {
					t.Fatalf("applied reclaim carried no committed revision")
				}
				rec, ok := originFile(t, repo.origin, m.branch, recPath)
				if !ok {
					t.Fatalf("record vanished after reclaim")
				}
				for _, want := range []string{"status: 'proposed'", "reconciled: false", "## Reclaim log", "2026-08-02T00:00:00Z", "lease strictly expired"} {
					if !strings.Contains(rec, want) {
						t.Errorf("reclaimed origin record missing %q:\n%s", want, rec)
					}
				}
				if strings.Contains(rec, "branch: feat/widget") {
					t.Errorf("branch not cleared on origin:\n%s", rec)
				}
				if strings.Contains(rec, "claimed_at: 2026-08-02") {
					t.Errorf("claim stamp not cleared on origin:\n%s", rec)
				}
				// The board is refreshed to a fresh render of the committed corpus.
				assertBoardMatchesCommitted(t, repo.origin, m.branch, repo.invocation)
			})

			t.Run("version-drift-contends", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: strings.Repeat("b", 40)})
				if res.Result != ResultContended {
					t.Fatalf("version drift = %q, want contended", res.Result)
				}
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})
		})
	}
}

// TestReclaimUnreachableRemoteFailsClosed proves the destructive leg fails closed
// when the remote is unreachable: nothing is reclaimed and the origin record is
// untouched. An unreachable origin is caught while pinning authoritative context
// (which fetches origin before any probe), so a reclaim can never proceed on
// state it could not authoritatively read — unknown never shares the clean-read
// branch on a destructive operation.
func TestIntegrationChangeReclaimUnreachableRemoteFailsClosed(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	repo := newWorkingRepo(t, map[string]string{
		recPath: lifecycleChange(3, "widget", "in-progress"),
	})
	node := planningDepsFor(t, repo.invocation)
	ver := blobVersionAt(t, repo.origin, "docket", recPath)
	before, _ := originFile(t, repo.origin, "docket", recPath)
	// Break the remote so no origin state can be authoritatively read or probed.
	runGit(t, repo.invocation, "remote", "set-url", "origin", t.TempDir()+"/nonexistent.git")
	res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
		ChangeReclaimRequest{ID: 3, Version: ver})
	if res.Result == ResultApplied {
		t.Fatalf("an unreachable remote was reclaimed: %q", res.Result)
	}
	if res.Disposition != ReclaimDispSkipped {
		t.Errorf("disposition = %q, want skipped", res.Disposition)
	}
	assertOriginRecordUnchanged(t, repo.origin, "docket", recPath, before)
}

// TestChangeReconcileAppliedResult proves the app layer submits the exact
// expected version and metadata target ref, carries NO idempotency key (a
// non-allocating edit of an existing record), and decodes the applied receipt.
func TestIntegrationChangeReconcileAppliedResult(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
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
	if req.TargetRef != "refs/heads/docket" {
		t.Errorf("target ref = %q, want refs/heads/docket", req.TargetRef)
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

// TestChangeReconcileContention proves both contention paths write nothing: a
// stale version is the engine's CAS contention; a status that is no longer
// in-progress is an incompatible fresh state the plan closure refuses and the
// result maps to contended (never a text-merge).
func TestIntegrationChangeReconcileContention(t *testing.T) {
	t.Run("stale version at the engine", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
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

// TestChangeRefreshClaimStampsOnly proves refresh re-stamps claimed_at (and the
// updated date) and nothing else, requires in-progress, and reports a version
// mismatch as contended — the stop-don't-overwrite instruction.
func TestIntegrationChangeRefreshClaimStampsOnly(t *testing.T) {
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
		repoDir := newWorkingRepo(t, nil).invocation
		engine := &recordingEngine{result: transaction.Result{Disposition: transaction.DispositionContended}}
		reader := &fakeReader{pin: mainModePin([]string{"inline"}), corpus: []StatusBlob{changeBlob(3, "widget", "feat", "high", "")}}
		deps := PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}

		res := ChangeRefreshClaim(context.Background(), deps, repoDir, ChangeClaimRequest{ID: 3, Version: "9999999999999999999999999999999999999999"})

		if res.Result != ResultContended || res.Disposition != ClaimDispositionContended {
			t.Fatalf("result=%q disposition=%q, want contended/contended", res.Result, res.Disposition)
		}
	})
}

// TestRepairAdoptPRHeadPinsExactVersion proves the transaction pins the approved
// version exactly, keying the repair op on the exact record blob.
func TestIntegrationChangeRepairAdoptPRHeadPinsExactVersion(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	repo.writerAdvance(t, "feat/renamed", map[string]string{"impl.go": "package impl\n"})
	ws := &fakeRepairWorkspace{inspection: workspace.Inspection{Kind: workspace.StateForeign}}
	deps, engine := repairRealDeps(t, repo.invocation, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/renamed"), ws)
	engine.result = transaction.Result{Disposition: transaction.DispositionApplied, AppliedCommit: gitcli.ObjectID(strings.Repeat("c", 40))}

	res := RepairIdentity(context.Background(), deps, repo.invocation, RepairIdentityRequest{
		ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/renamed",
	})
	if res.Result != ResultApplied || res.Reason != RepairRepairedBranch {
		t.Fatalf("repair = (%q, %q)", res.Result, res.Reason)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want exactly 1", len(engine.calls))
	}
	exp := engine.calls[0].Expected
	if len(exp) != 1 || string(exp[0].Version.ObjectID) != repairVersion {
		t.Errorf("transaction did not pin the exact approved version: %+v", exp)
	}
	if engine.calls[0].Operation.Key() != transaction.OperationKey(OperationChangeRepairIdentity) {
		t.Errorf("operation key = %q", engine.calls[0].Operation.Key())
	}
}

// TestRepairAdoptPRHeadWritesBranch proves the applied path end-to-end: every
// conjunct holds, so the repair opens one exact-version transaction that adopts
// the PR's reported head as branch:, refreshes updated, and commits only that.
func TestIntegrationChangeRepairAdoptPRHeadWritesBranch(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	repo := newWorkingRepo(t, map[string]string{recPath: repairRecord(3, "widget", "")})
	repo.writerAdvance(t, "feat/renamed", map[string]string{"impl.go": "package impl\n"})
	node := planningDepsFor(t, repo.invocation)
	ver := blobVersionAt(t, repo.origin, "docket", recPath)

	gh := repairGitHub("feat/renamed")
	ws := &fakeRepairWorkspace{inspection: workspace.Inspection{Kind: workspace.StateForeign}}
	deps := FinalizeDeps{Planning: node.deps, GitHub: gh, Workspace: ws}

	res := RepairIdentity(context.Background(), deps, node.dir, RepairIdentityRequest{
		ID: 3, ExpectVersion: ver, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/renamed",
	})
	if res.Result != ResultApplied || res.Reason != RepairRepairedBranch {
		t.Fatalf("repair = (%q, %q) msg=%q findings=%v", res.Result, res.Reason, res.Message, res.Findings)
	}
	if res.Branch != "feat/renamed" || res.Revision == "" {
		t.Errorf("applied result malformed: %+v", res)
	}
	rec, ok := originFile(t, repo.origin, "docket", recPath)
	if !ok {
		t.Fatalf("record vanished after repair")
	}
	for _, want := range []string{"branch: 'feat/renamed'", "updated: '2026-08-16'"} {
		if !strings.Contains(rec, want) {
			t.Errorf("repaired origin record missing %q:\n%s", want, rec)
		}
	}
	if strings.Contains(rec, "branch: feat/widget") {
		t.Errorf("the stale recorded branch survived the repair:\n%s", rec)
	}
}

// TestRepairCandidateBranchAbsent proves clause 2's candidate-branch proof: the
// branch the record would carry must be present on the remote; an absent branch
// is candidate-branch-absent with no write.
func TestIntegrationChangeRepairCandidateBranchAbsent(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil) // origin carries no feat/renamed branch
	ws := &fakeRepairWorkspace{inspection: workspace.Inspection{Kind: workspace.StateForeign}}
	deps, engine := repairRealDeps(t, repo.invocation, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/renamed"), ws)
	res := RepairIdentity(context.Background(), deps, repo.invocation, RepairIdentityRequest{
		ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/renamed",
	})
	assertRepairRefused(t, res, ResultInvalidState, RepairCandidateBranchAbsent, engine)
	if len(ws.inspectCalls) != 0 {
		t.Errorf("the workspace was inspected before the candidate-branch proof failed")
	}
}

// TestRepairInspectErrorIsConflict proves the fail-closed reading: an inspection
// that cannot be answered is ambiguity and takes the workspace-conflict path,
// never a pass (probe-error-is-not-clean-absence).
func TestIntegrationChangeRepairInspectErrorIsConflict(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	repo.writerAdvance(t, "feat/renamed", map[string]string{"impl.go": "package impl\n"})
	ws := &fakeRepairWorkspace{inspectErr: errors.New("inspect boom")}
	deps, engine := repairRealDeps(t, repo.invocation, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/renamed"), ws)
	res := RepairIdentity(context.Background(), deps, repo.invocation, RepairIdentityRequest{
		ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/renamed",
	})
	assertRepairRefused(t, res, ResultInvalidState, RepairWorkspaceConflict, engine)
}

// TestRepairWorkspaceConflictBlocks proves clause 4: an owned workspace that
// targets a branch other than the one the record will carry blocks the repair.
// The fixture's recorded branch (feat/widget) is owned and live while the repair
// proposes feat/renamed; the fake Inspect's recorded call is the sentinel that
// the conflicting-workspace check actually executed. Deleting the branch
// comparison in repairProveWorkspaceClear lets the repair proceed to a write,
// reddening this assertion.
func TestIntegrationChangeRepairWorkspaceConflictBlocks(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	// The candidate branch must be present on the remote so the probe passes and
	// control reaches the workspace gate.
	repo.writerAdvance(t, "feat/renamed", map[string]string{"impl.go": "package impl\n"})
	ws := &fakeRepairWorkspace{inspection: workspace.Inspection{Kind: workspace.StateReady}}
	deps, engine := repairRealDeps(t, repo.invocation, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/renamed"), ws)
	res := RepairIdentity(context.Background(), deps, repo.invocation, RepairIdentityRequest{
		ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/renamed",
	})
	assertRepairRefused(t, res, ResultInvalidState, RepairWorkspaceConflict, engine)
	if len(ws.inspectCalls) != 1 {
		t.Fatalf("conflicting-workspace check ran %d times, want exactly 1 (sentinel)", len(ws.inspectCalls))
	}
	if got := ws.inspectCalls[0].Target.FeatureBranch(); got != "feat/widget" {
		t.Errorf("inspected target branch = %q, want the recorded feat/widget", got)
	}
}

// TestChangeResumeHalted proves the full recovery: a live-writer reprobe refuses
// and leaves the marker; a version drift is contended; a quiescent reprobe
// refreshes the claim, removes exactly the marker section, and preserves every
// other byte.
func TestIntegrationChangeResumeHalted(t *testing.T) {
	for _, m := range planRepoModes() {
		t.Run(m.name, func(t *testing.T) {
			// A live writer (allocating workspace) refuses; the marker stays.
			t.Run("live-writer-refuses", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateResumable, head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: f.version, AcknowledgeQuiescent: true})
				if got.Reason != ReasonResumeWorkspaceActive {
					t.Fatalf("reason=%q, want %q", got.Reason, ReasonResumeWorkspaceActive)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if !strings.Contains(rec, "## Run halted") {
					t.Errorf("marker removed on a refused resume:\n%s", rec)
				}
			})

			// A version drift is a lost race: contended, marker retained.
			t.Run("version-drift-contended", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateReady, head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: strings.Repeat("b", 40), AcknowledgeQuiescent: true})
				if got.Result != ResultContended {
					t.Fatalf("result=%q disp=%q, want contended", got.Result, got.Disposition)
				}
			})

			// A quiescent reprobe recovers: claim refreshed, marker removed, other
			// bytes preserved.
			t.Run("quiescent-resumes", func(t *testing.T) {
				f := setupHaltedFixture(t, m)
				got := ChangeResumeHalted(context.Background(), f.deps,
					WorkspaceDeps{Service: fakeResumeWorkspace{kind: workspace.StateReady, head: f.head}}, f.repo.invocation,
					ResumeRequest{ID: f.id, Version: f.version, AcknowledgeQuiescent: true})
				if got.Result != ResultApplied || got.Disposition != HaltDispResumed {
					t.Fatalf("result=%q disp=%q reason=%q", got.Result, got.Disposition, got.Reason)
				}
				rec, _ := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
				if strings.Contains(rec, "## Run halted") {
					t.Errorf("marker not removed on resume:\n%s", rec)
				}
				if !strings.Contains(rec, "claimed_at: '2026-08-16T12:00:00Z'") {
					t.Errorf("claim lease not refreshed:\n%s", rec)
				}
				// Preserved: branch and the authored ## Why section byte-identical.
				for _, want := range []string{"branch: feat/widget", "## Why\n\nOriginal why."} {
					if !strings.Contains(rec, want) {
						t.Errorf("resume altered a preserved byte; missing %q:\n%s", want, rec)
					}
				}
			})
		})
	}
}

// TestRunGateBeforeArmsWithLoadableKey: a successful arm prints `gate-armed
// <key>`, the record loads, and its BeforeIDs are exactly the fixture's
// in-progress ids with the store-owned target and an unused retry permit.
func TestIntegrationChangeRunGateBeforeArmsWithLoadableKey(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want %q (report lines exit 0)", res.Result, ResultApplied)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("gate-armed exit code = %d, want 0", code)
	}
	if !res.Armed || res.Key == "" {
		t.Fatalf("Armed=%v Key=%q, want armed with a non-empty key", res.Armed, res.Key)
	}
	if got, want := res.HumanText(), "gate-armed "+res.Key; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}

	got, err := LoadGateRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadGateRecord(%q): %v", res.Key, err)
	}
	if want := []int{3, 7}; !slices.Equal(got.BeforeIDs, want) {
		t.Errorf("BeforeIDs = %v, want %v", got.BeforeIDs, want)
	}
	if got.Target != "docket-implement-next" {
		t.Errorf("Target = %q, want %q", got.Target, "docket-implement-next")
	}
	if got.Retry != RetryUnused {
		t.Errorf("Retry = %q, want %q", got.Retry, RetryUnused)
	}
	if got.AttributedID != 0 {
		t.Errorf("AttributedID = %d, want 0 (not yet attributed)", got.AttributedID)
	}
	if got.Terminal {
		t.Errorf("Terminal = true, want false on a fresh arm")
	}
}

// TestRunGateBeforeDispatchEpochAfterBeforeRead: DispatchEpoch is captured after
// the before-read, so it is at or after the record's CreatedAt and is a real
// (non-zero) wall-clock stamp.
func TestIntegrationChangeRunGateBeforeDispatchEpochAfterBeforeRead(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if !res.Armed {
		t.Fatalf("gate did not arm: %q", res.HumanText())
	}
	rec, err := LoadGateRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.CreatedAt <= 0 || rec.DispatchEpoch <= 0 {
		t.Fatalf("CreatedAt=%d DispatchEpoch=%d, want real wall-clock stamps", rec.CreatedAt, rec.DispatchEpoch)
	}
	if rec.DispatchEpoch < rec.CreatedAt {
		t.Errorf("DispatchEpoch %d < CreatedAt %d, want captured at or after the before-read", rec.DispatchEpoch, rec.CreatedAt)
	}
}

// TestRunGateBeforeEmptyBacklogArms: no in-progress claims still arms with an
// empty before-set — an empty set is a valid observation, not a failure.
func TestIntegrationChangeRunGateBeforeEmptyBacklogArms(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, []StatusBlob{}, nil, nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if !res.Armed {
		t.Fatalf("empty backlog did not arm: %q", res.HumanText())
	}
	rec, err := LoadGateRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if len(rec.BeforeIDs) != 0 {
		t.Errorf("BeforeIDs = %v, want empty", rec.BeforeIDs)
	}
}

// TestRunGateBeforeInvalidTarget: any target other than `implement-next` is a
// usage error — a non-zero exit with no gate-armed / gate-unarmed report line.
func TestIntegrationChangeRunGateBeforeInvalidTarget(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "bogus-target")
	if res.Armed {
		t.Fatalf("armed for an invalid target")
	}
	if code := ExitCode(res.Env().Result); code == 0 {
		t.Errorf("invalid-target exit code = 0, want non-zero (result %q)", res.Env().Result)
	}
	if res.Key != "" {
		t.Errorf("invalid target minted a key %q, want none", res.Key)
	}
}

// TestRunGateBeforeSyncFailure: a fresh-origin re-sync failure (PinContext) is
// reported as gate-unarmed with the sync reason token, exit 0.
func TestIntegrationChangeRunGateBeforeSyncFailure(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, nil, errors.New("fetch failed"), nil), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if res.Armed {
		t.Fatalf("armed despite a sync failure, want gate-unarmed")
	}
	if res.Reason != ReasonGateSyncFailed {
		t.Errorf("Reason = %q, want %q", res.Reason, ReasonGateSyncFailed)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("gate-unarmed exit code = %d, want 0", code)
	}
}

// TestRunGateBeforeUnreadableChangesDir: an unreadable corpus prints
// `gate-unarmed <reason>` with a stable token and still exits 0 (the report line
// is the contract), and mints no record.
func TestIntegrationChangeRunGateBeforeUnreadableChangesDir(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, nil, nil, errors.New("changes dir unreadable")), Clock: testClock()}

	res := RunGateBefore(context.Background(), deps, repo, "implement-next")
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want %q (gate-unarmed is a report line)", res.Result, ResultApplied)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("gate-unarmed exit code = %d, want 0", code)
	}
	if res.Armed {
		t.Fatalf("armed on an unreadable corpus, want gate-unarmed")
	}
	if res.Reason != ReasonGateChangesUnreadable {
		t.Errorf("Reason = %q, want %q", res.Reason, ReasonGateChangesUnreadable)
	}
	if got, want := res.HumanText(), "gate-unarmed "+ReasonGateChangesUnreadable; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}

// TestRunGateVerdictAmbiguousClaims: two survivors cannot be told apart, so the
// gate refuses with a terminal ambiguous-claims listing every survivor id.
func TestIntegrationChangeRunGateVerdictAmbiguousClaims(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{
		gateInProgressBlob(7, "bravo", "keep"),
		gateInProgressBlob(3, "alpha", "keep"),
	})
	key := gateMintArmed(t, repo, nil, 1)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-stop "+key+" ambiguous-claims 3 7"; got != want {
		t.Fatalf("HumanText = %q, want %q (survivors sorted)", got, want)
	}
	if !res.Terminal {
		t.Errorf("ambiguous-claims is terminal, got Terminal=false")
	}
}

// TestRunGateVerdictClaimAtDispatchEpochAttributes: the filter is >= (claimed_at
// AT the dispatch epoch is attributable, not before it). A claim exactly at the
// epoch survives — proving the boundary is inclusive — and, being the sole
// survivor over a not-implemented run, yields gate-retry-once.
func TestIntegrationChangeRunGateVerdictClaimAtDispatchEpochAttributes(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	// DispatchEpoch equal to the claim instant: the >= filter admits it.
	key := gateMintArmed(t, f.repo.invocation, nil, gateClaimEpoch(t))

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.AttributedID != 3 {
		t.Fatalf("AttributedID = %d, want 3 (claim at the epoch is attributable)", res.AttributedID)
	}
	if res.Decision != GateDecisionRetryOnce {
		t.Fatalf("Decision = %q, want %q", res.Decision, GateDecisionRetryOnce)
	}
}

// TestRunGateVerdictLoadErrorsFailClosed: every store load fault maps to a
// terminal gate-stop gate-unavailable carrying the store's typed reason token —
// never a retry.
func TestIntegrationChangeRunGateVerdictLoadErrorsFailClosed(t *testing.T) {
	t.Run("malformed key", func(t *testing.T) {
		repo := newGateRepo(t)
		res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{}, GitHubDeps{}, repo, "../escape")
		if got, want := res.HumanText(), "gate-stop ../escape gate-unavailable malformed-key"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		if !res.Terminal {
			t.Errorf("gate-unavailable is terminal")
		}
	})

	t.Run("record not found", func(t *testing.T) {
		repo := newGateRepo(t)
		key := "implement-next-nope"
		res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{}, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "gate-stop "+key+" gate-unavailable not-found"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
	})

	t.Run("corrupt record", func(t *testing.T) {
		repo := newGateRepo(t)
		key := gateMintArmed(t, repo, nil, 1)
		root, err := gateRoot(repo)
		if err != nil {
			t.Fatalf("gateRoot: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, key, "record.json"), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{}, GitHubDeps{}, repo, key)
		if got, want := res.HumanText(), "gate-stop "+key+" gate-unavailable corrupt-record"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
	})

	t.Run("wrong repo", func(t *testing.T) {
		repoA := newGateRepo(t)
		repoB := newGateRepo(t)
		key := gateMintArmed(t, repoA, nil, 1)
		rootA, _ := gateRoot(repoA)
		rootB, _ := gateRoot(repoB)
		if err := os.MkdirAll(filepath.Join(rootB, key), 0o755); err != nil {
			t.Fatal(err)
		}
		buf, err := os.ReadFile(filepath.Join(rootA, key, "record.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootB, key, "record.json"), buf, 0o644); err != nil {
			t.Fatal(err)
		}
		res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{}, GitHubDeps{}, repoB, key)
		if got, want := res.HumanText(), "gate-stop "+key+" gate-unavailable wrong-repo"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
	})
}

// TestRunGateVerdictNoAttributableClaim: with every in-progress id already in the
// before-set, zero claims are attributable and the gate reports a terminal
// gate-done no-attributable-claim.
func TestIntegrationChangeRunGateVerdictNoAttributableClaim(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{gateInProgressBlob(3, "alpha", "keep")})
	key := gateMintArmed(t, repo, []int{3}, 1)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)

	if got, want := res.HumanText(), "gate-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("no-attributable-claim is terminal, got Terminal=false")
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("exit code = %d, want 0 (report line)", code)
	}
	// The durable record records the terminal disposition.
	rec, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if !rec.Terminal {
		t.Errorf("record Terminal = false, want true after a terminal outcome")
	}
}

// TestRunGateVerdictObserveEmptyBacklogNoCurrentRun: no in-progress ids and no
// hints → a single terminal-shaped `gate-observe no-current-run` line.
func TestIntegrationChangeRunGateVerdictObserveEmptyBacklogNoCurrentRun(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{gateProposedBlob(9, "charlie")})

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, nil)
	if got, want := res.HumanText(), "gate-observe no-current-run"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

// TestRunGateVerdictObserveHintsMixedVerdicts: supplied hint ids are each verified
// and rendered as one `gate-observe <verdict> <id>` line, in the INPUT order given
// (never re-sorted), using RunVerify's verdict verbatim.
func TestIntegrationChangeRunGateVerdictObserveHintsMixedVerdicts(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{
		gateProposedBlob(3, "alpha"),
		gateHaltedInProgressBlob(7, "bravo"),
	})

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, []string{"7", "3"})
	if got, want := res.HumanText(), "gate-observe run-halted 7\ngate-observe run-unclaimed 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (one line per hint, input order)", got, want)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("exit code = %d, want 0 (report lines)", code)
	}
}

// TestRunGateVerdictObserveIncompleteWritesNothing: an incomplete run observed
// unattributed emits `gate-observe run-incomplete <id> <unmet...>` and writes NO
// record and consumes NOTHING — the rungate root is never created (no mint, no
// save, no retry consumption on the observe path).
func TestIntegrationChangeRunGateVerdictObserveIncompleteWritesNothing(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)

	res := RunGateVerdictObserve(context.Background(), deps, wdeps, gdeps, f.repo.invocation, []string{"3"})
	if got, want := res.HumanText(), "gate-observe run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if strings.HasPrefix(res.HumanText(), GateDecisionRetryOnce) {
		t.Fatalf("observe must never emit %q", GateDecisionRetryOnce)
	}

	// NO record, NO writes: the rungate root is never created by the observe path.
	root, err := gateRoot(f.repo.invocation)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("rungate root %q exists (stat err %v); observe must write nothing", root, statErr)
	}
}

// TestRunGateVerdictObserveKeyIsUsageError: `--unattributed` combined with a key
// (a non-integer positional) is a usage error — a non-zero exit, never a report
// line. Hints are change ids; a key can never be one.
func TestIntegrationChangeRunGateVerdictObserveKeyIsUsageError(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{gateHaltedInProgressBlob(3, "alpha")})

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, []string{"implement-next-20260826T000000Z-1-abcd"})
	if res.Env().Result != ResultInvalidInput {
		t.Fatalf("Result = %q, want ResultInvalidInput (a key is not a hint)", res.Env().Result)
	}
	if code := ExitCode(res.Env().Result); code == 0 {
		t.Fatalf("exit code = %d, want non-zero (usage error)", code)
	}
}

// TestRunGateVerdictObserveNoHintsAllInProgress: with no hints, every current
// in-progress id is verified (sorted), one line each.
func TestIntegrationChangeRunGateVerdictObserveNoHintsAllInProgress(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{
		gateHaltedInProgressBlob(7, "bravo"),
		gateHaltedInProgressBlob(3, "alpha"),
		gateProposedBlob(9, "charlie"), // proposed: not in-progress, never observed
	})

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, nil)
	if got, want := res.HumanText(), "gate-observe run-halted 3\ngate-observe run-halted 7"; got != want {
		t.Fatalf("HumanText = %q, want %q (all in-progress ids, sorted)", got, want)
	}
}

// TestRunGateVerdictObserveSyncFailureUnavailable: a re-sync/read fault fails
// closed to a single `gate-observe gate-unavailable <reason>` line.
func TestIntegrationChangeRunGateVerdictObserveSyncFailureUnavailable(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: &fakeReader{pinErr: errors.New("boom")}, Clock: testClock()}

	res := RunGateVerdictObserve(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, nil)
	if got, want := res.HumanText(), "gate-observe gate-unavailable "+ReasonGateSyncFailed; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
}

// TestRunGateVerdictRestartDurability: gate-before and gate-verdict share nothing
// but the repository directory and the key. Minting the record through the store
// (as a separate gate-before process would) and then reading a verdict through a
// fresh RunGateVerdict call — no record value passed between them — still resolves
// the attributed run.
func TestIntegrationChangeRunGateVerdictRestartDurability(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	// Simulate the arming process: mint and forget (nothing carried in memory).
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

	// A fresh call sharing only repoDir + key attributes and reports.
	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.AttributedID != 3 {
		t.Fatalf("AttributedID = %d, want 3 (attributed from the durable record alone)", res.AttributedID)
	}
	if res.Decision != GateDecisionRetryOnce {
		t.Fatalf("Decision = %q, want gate-retry-once", res.Decision)
	}
}

// TestRunGateVerdictRunComplete: the attributed-id short-circuit verifies the
// stored id directly. An implemented change 3 (which fresh attribution could
// never pick, being no longer in-progress) yields gate-done run-complete —
// proving the short-circuit bypassed attribution.
func TestIntegrationChangeRunGateVerdictRunComplete(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintAttributed(t, f.repo.invocation, 3)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("run-complete is terminal")
	}
	if res.AttributedID != 3 {
		t.Errorf("AttributedID = %d, want 3 (short-circuit kept the stored id)", res.AttributedID)
	}
}

// TestRunGateVerdictRunHalted: a halted in-progress change is attributed, then
// RunVerify's run-halted maps to a terminal gate-stop (a human is needed).
func TestIntegrationChangeRunGateVerdictRunHalted(t *testing.T) {
	repo := newGateRepo(t)
	src := strings.TrimRight(lifecycleChange(3, "widget", "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"
	corpus := []StatusBlob{{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(3, "widget"),
		Version:  miVersion,
		Data:     []byte(src),
	}}
	deps := gateLightDeps(t, corpus)
	key := gateMintArmed(t, repo, nil, 1)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-stop "+key+" run-halted 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("run-halted is terminal")
	}
}

// TestRunGateVerdictRunIncompleteRetryThenStop drives the one-retry accounting:
// a not-implemented run yields gate-retry-once on the first call (retry permit
// consumed, non-terminal), and gate-stop run-incomplete on the second (permit
// spent, terminal). The post-pass durable state — not merely the emitted line —
// records the consumed marker and the attributed id.
func TestIntegrationChangeRunGateVerdictRunIncompleteRetryThenStop(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

	res1 := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res1.HumanText(), "gate-retry-once "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("first call HumanText = %q, want %q", got, want)
	}
	if res1.Terminal {
		t.Errorf("gate-retry-once must be non-terminal")
	}

	// POST-PASS DURABLE STATE: reload the record and prove the retry marker is
	// present and the claim is attributed — a fresh load, not the emitted line.
	rec, err := LoadGateRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.Retry != RetryConsumed {
		t.Errorf("reloaded Retry = %q, want %q (marker present)", rec.Retry, RetryConsumed)
	}
	if rec.AttributedID != 3 {
		t.Errorf("reloaded AttributedID = %d, want 3 (durably attributed)", rec.AttributedID)
	}

	res2 := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res2.HumanText(), "gate-stop "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("second call HumanText = %q, want %q", got, want)
	}
	if !res2.Terminal {
		t.Errorf("second gate-stop run-incomplete must be terminal")
	}
}

// TestRunGateVerdictRunUnclaimed: the attributed-id short-circuit over a change
// that is now proposed (never-claimed) maps RunVerify's run-unclaimed to a
// terminal gate-done.
func TestIntegrationChangeRunGateVerdictRunUnclaimed(t *testing.T) {
	repo := newGateRepo(t)
	deps := rvProposedDeps(t) // corpus: change 3 proposed
	key := gateMintAttributed(t, repo, 3)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-done "+key+" run-unclaimed 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("run-unclaimed is terminal")
	}
}

// TestRunGateVerdictRunWaiting: a fully-agreeing local waiting receipt over an
// in-progress change yields a terminal gate-stop run-waiting that passes the
// opaque handoff id and phase through verbatim.
func TestIntegrationChangeRunGateVerdictRunWaiting(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true})
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "gate-stop "+key+" run-waiting 3 d0opaque build"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if res.HandoffID != "d0opaque" || res.Phase != "build" {
		t.Errorf("handoff/phase = %q/%q, want d0opaque/build (verbatim pass-through)", res.HandoffID, res.Phase)
	}
	if !res.Terminal {
		t.Errorf("run-waiting is terminal")
	}
}

// TestRunGateVerdictThreeFilters: each attribution filter rejects its candidate
// independently, collapsing to no-attributable-claim.
func TestIntegrationChangeRunGateVerdictThreeFilters(t *testing.T) {
	claimEpoch := gateClaimEpoch(t)
	rows := []struct {
		name          string
		claimedAt     string
		beforeIDs     []int
		dispatchEpoch int64
	}{
		{name: "id present in before-set", claimedAt: "keep", beforeIDs: []int{3}, dispatchEpoch: 1},
		{name: "claimed_at missing", claimedAt: "", beforeIDs: nil, dispatchEpoch: 1},
		{name: "claimed_at malformed", claimedAt: "not-a-timestamp", beforeIDs: nil, dispatchEpoch: 1},
		{name: "claimed_at before dispatch", claimedAt: "keep", beforeIDs: nil, dispatchEpoch: claimEpoch + 1},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			repo := newGateRepo(t)
			deps := gateLightDeps(t, []StatusBlob{gateInProgressBlob(3, "alpha", row.claimedAt)})
			key := gateMintArmed(t, repo, row.beforeIDs, row.dispatchEpoch)

			res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
			if got, want := res.HumanText(), "gate-done "+key+" no-attributable-claim"; got != want {
				t.Fatalf("HumanText = %q, want %q", got, want)
			}
			if res.AttributedID != 0 {
				t.Errorf("AttributedID = %d, want 0 (nothing attributed)", res.AttributedID)
			}
		})
	}
}

// TestRunGateVerdictTwoKeysIsolated: two distinct keys in one repository hold
// independent retry permits — consuming one never touches the other.
func TestIntegrationChangeRunGateVerdictTwoKeysIsolated(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	keyA := gateMintArmed(t, f.repo.invocation, nil, 1)
	keyB := gateMintArmed(t, f.repo.invocation, nil, 1)

	resA := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, keyA)
	resB := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, keyB)
	if resA.Decision != GateDecisionRetryOnce {
		t.Errorf("key A decision = %q, want gate-retry-once", resA.Decision)
	}
	if resB.Decision != GateDecisionRetryOnce {
		t.Errorf("key B decision = %q, want gate-retry-once (independent permit)", resB.Decision)
	}
}

// TestRunGateVerdictUnknownVerdictFailsClosed: a RunVerify outcome with no
// recognized verdict (here an operational unknown-change error over an
// attributed id absent from the corpus) fails closed to gate-unavailable
// unknown-verdict — never a retry, never a silent pass.
func TestIntegrationChangeRunGateVerdictUnknownVerdictFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{}) // empty corpus: id 999 is unknown
	key := gateMintAttributed(t, repo, 999)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-stop "+key+" gate-unavailable unknown-verdict"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("gate-unavailable is terminal")
	}
}

// TestRunVerifyComplete: an implemented change satisfying every postcondition ⇒
// run-complete with no unmet conjuncts.
func TestIntegrationChangeRunVerifyComplete(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunComplete {
		t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunComplete, unmetReasons(res))
	}
	if len(res.Unmet) != 0 {
		t.Errorf("run-complete carried unmet conjuncts: %v", unmetReasons(res))
	}
	if res.Head != f.head {
		t.Errorf("head = %q, want %q", res.Head, f.head)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("run-complete exit code = %d, want 0", code)
	}
}

// TestRunVerifyCompletePrecedesStaleHandoff: when every completed-run
// postcondition holds, run-complete wins even though a fully-agreeing local
// handoff receipt is present.
func TestIntegrationChangeRunVerifyCompletePrecedesStaleHandoff(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Waiting = fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true}

	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunComplete {
		t.Fatalf("verdict = %q, want %q (a completed run must outrank a stale handoff)", res.Verdict, VerdictRunComplete)
	}
}

// TestRunVerifyIncompleteEnumeratesConjuncts is the spec's own testing rule: each
// row mutates or removes exactly one promised postcondition and expects
// run-incomplete carrying that conjunct's stable reason — asserted as the FULL
// unmet list, not merely non-empty. The happy fixture (TestRunVerifyComplete)
// satisfies all of them.
func TestIntegrationChangeRunVerifyIncompleteEnumeratesConjuncts(t *testing.T) {
	pub := newRunVerifyFixture(t, true)
	ev := string(prEvidenceBytes(t, pub.head))
	recordedPR := rvRecordedPR()
	ghostPlan := "docs/superpowers/plans/2026-08-17-ghost.md"
	ghostResults := "docs/changes/results/0003-ghost.md"

	rows := []struct {
		name   string
		record []byte
		pr     githubcli.PullRequest
		want   string
	}{
		{
			name:   "missing plan link",
			record: rvRecord("", rvResultsPath, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(pub.head, ev),
			want:   ReasonRunPlanUnlinked,
		},
		{
			name:   "plan file gone at recorded path",
			record: rvRecord(ghostPlan, rvResultsPath, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(pub.head, ev),
			want:   ReasonRunPlanMissing,
		},
		{
			name:   "stale evidence names another head",
			record: rvRecord(rvPlanPath, rvResultsPath, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(pub.head, string(prEvidenceBytes(t, prOtherHead))),
			want:   ReasonRunEvidenceUnverified,
		},
		{
			name:   "PR names another head",
			record: rvRecord(rvPlanPath, rvResultsPath, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(prOtherHead, ev),
			want:   ReasonRunPRUnverified,
		},
		{
			name:   "results identity broken",
			record: rvRecord(rvPlanPath, ghostResults, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(pub.head, ev),
			want:   ReasonRunResultsIdentity,
		},
		{
			// The recorded branch is honored end-to-end: run verify inspects and
			// probes feat/other (the record's branch), never a reconstructed
			// feat/<slug>. Since only feat/<slug> was published, the recorded
			// branch's remote head is absent — caught as remote-head-mismatch. Were
			// the branch reconstructed from the slug, the remote probe would find the
			// published head and this conjunct would wrongly pass.
			name:   "recorded branch honored — its remote head is absent",
			record: rvRecord(rvPlanPath, rvResultsPath, recordedPR, "feat/other"),
			pr:     rvPR(pub.head, ev),
			want:   ReasonRunRemoteHeadMismatch,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			deps, wdeps, gdeps := pub.deps(row.record, row.pr)
			res := RunVerify(context.Background(), deps, wdeps, gdeps, pub.repo.invocation, RunVerifyRequest{ID: 3})
			if res.Verdict != VerdictRunIncomplete {
				t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunIncomplete, unmetReasons(res))
			}
			if got := unmetReasons(res); len(got) != 1 || got[0] != row.want {
				t.Fatalf("unmet = %v, want exactly [%s]", got, row.want)
			}
			if code := ExitCode(res.Env().Result); code != 0 {
				t.Errorf("run-incomplete exit code = %d, want 0", code)
			}
		})
	}

	// The remote-head postcondition needs an unpublished feature head: the local
	// head exists but the remote never received it, so the remote is absent.
	t.Run("feature head differs from remote", func(t *testing.T) {
		unpub := newRunVerifyFixture(t, false)
		deps, wdeps, gdeps := unpub.deps(
			rvRecord(rvPlanPath, rvResultsPath, recordedPR, "feat/"+rvSlug),
			rvPR(unpub.head, string(prEvidenceBytes(t, unpub.head))),
		)
		res := RunVerify(context.Background(), deps, wdeps, gdeps, unpub.repo.invocation, RunVerifyRequest{ID: 3})
		if res.Verdict != VerdictRunIncomplete {
			t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunIncomplete, unmetReasons(res))
		}
		if got := unmetReasons(res); len(got) != 1 || got[0] != ReasonRunRemoteHeadMismatch {
			t.Fatalf("unmet = %v, want exactly [%s]", got, ReasonRunRemoteHeadMismatch)
		}
	})
}

// TestRunVerifyOperationalError: an absent id is an operational error, not a
// verdict — it carries no verdict and exits non-zero.
func TestIntegrationChangeRunVerifyOperationalError(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug), rvPR(f.head, string(prEvidenceBytes(t, f.head))))
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 999})
	if res.Verdict != "" {
		t.Errorf("operational error carried a verdict %q", res.Verdict)
	}
	if code := ExitCode(res.Env().Result); code == 0 {
		t.Errorf("operational error exit code = 0, want non-zero (result %q)", res.Env().Result)
	}
}

// TestRunVerifyPRIdentityForms is the mutation test for the migrated PR-identity
// conjunct: run verify accepts a recorded pr: in EITHER form (canonical URL or
// legacy owner/repo#N shorthand) when its parsed number equals the verified PR's
// number, and flags pr-unverified when the number differs or the recorded value
// is unparseable. The verified PR is number 42 (rvPR).
func TestIntegrationChangeRunVerifyPRIdentityForms(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	ev := string(prEvidenceBytes(t, f.head))

	cases := []struct {
		name       string
		recorded   string
		wantVerify bool
	}{
		{"url form matches", rvRecordedPRURL(), true},
		{"shorthand form matches", rvRecordedPR(), true},
		{"url form wrong number", "https://github.com/acme/widget/pull/99", false},
		{"shorthand wrong number", prRepo().Spec() + "#99", false},
		{"unparseable recorded pr", "not-a-pr-ref", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, wdeps, gdeps := f.deps(
				rvRecord(rvPlanPath, rvResultsPath, tc.recorded, "feat/"+rvSlug),
				rvPR(f.head, ev),
			)
			res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
			reasons := unmetReasons(res)
			hasPRUnverified := false
			for _, r := range reasons {
				if r == ReasonRunPRUnverified {
					hasPRUnverified = true
				}
			}
			if tc.wantVerify {
				if res.Verdict != VerdictRunComplete {
					t.Fatalf("recorded %q: verdict = %q, want run-complete (unmet %v)", tc.recorded, res.Verdict, reasons)
				}
			} else if !hasPRUnverified {
				t.Fatalf("recorded %q: expected a pr-unverified conjunct, got unmet %v (verdict %q)", tc.recorded, reasons, res.Verdict)
			}
		})
	}
}

// TestRunVerifyWaitingAgreeingChain: a fully-agreeing local receipt chain over an
// in-progress change yields run-waiting, exposing the opaque handoff id and phase
// (never an owner credential), as a success-shaped, exit-0 verdict.
func TestIntegrationChangeRunVerifyWaitingAgreeingChain(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	reader := fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true}
	deps, wdeps, gdeps := rvWaitingDeps(t, f, reader)

	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunWaiting {
		t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunWaiting, unmetReasons(res))
	}
	if res.HandoffID != "d0opaque" {
		t.Errorf("handoff id = %q, want %q", res.HandoffID, "d0opaque")
	}
	if res.Phase != "build" {
		t.Errorf("phase = %q, want %q", res.Phase, "build")
	}
	if len(res.Unmet) != 0 {
		t.Errorf("run-waiting carried unmet conjuncts: %v", unmetReasons(res))
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("run-waiting exit code = %d, want 0", code)
	}
}

// TestRunVerifyWaitingMutationsDisappear is the spec's mutation rule: flip exactly
// one receipt dimension of the agreeing chain and prove waiting disappears —
// falling through to the ordinary run-incomplete verdict. A found=false / errored
// reader (missing local state, e.g. another machine) also never invents waiting.
func TestIntegrationChangeRunVerifyWaitingMutationsDisappear(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	base := rvAgreeingReceipt(f.head)

	rows := []struct {
		name    string
		mutate  func(*WaitingReceipt)
		found   bool
		readErr error
	}{
		{name: "head drift (drive vs workspace)", mutate: func(r *WaitingReceipt) { r.DriveHead = "0000000000000000000000000000000000000000" }, found: true},
		{name: "head drift (live vs drive)", mutate: func(r *WaitingReceipt) { r.LiveFingerprint.Head = "0000000000000000000000000000000000000000" }, found: true},
		{name: "fingerprint drift", mutate: func(r *WaitingReceipt) { r.LiveFingerprint.Worktree = "drifted" }, found: true},
		{name: "claimed handoff", mutate: func(r *WaitingReceipt) { r.HasUnclaimedHandoff = false }, found: true},
		{name: "expired deadline without terminal", mutate: func(r *WaitingReceipt) { r.DeadlineLive = false; r.TerminalWaiting = false }, found: true},
		{name: "mismatched raw run", mutate: func(r *WaitingReceipt) { r.RawRunMatches = false }, found: true},
		{name: "broken chain: change id mismatch", mutate: func(r *WaitingReceipt) { r.ChangeID = "99" }, found: true},
		{name: "broken chain: empty drive id", mutate: func(r *WaitingReceipt) { r.DriveID = "" }, found: true},
		{name: "broken chain: empty phase", mutate: func(r *WaitingReceipt) { r.Phase = "" }, found: true},
		{name: "worktree missing", mutate: func(r *WaitingReceipt) { r.WorktreeExists = false }, found: true},
		{name: "recorded branch mismatch", mutate: func(r *WaitingReceipt) { r.Branch = "feat/other" }, found: true},
		{name: "missing local state (not found)", mutate: func(r *WaitingReceipt) {}, found: false},
		{name: "reader error", mutate: func(r *WaitingReceipt) {}, found: true, readErr: errors.New("store unreadable")},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			rcpt := base
			row.mutate(&rcpt)
			deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rcpt, found: row.found, err: row.readErr})
			res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
			if res.Verdict == VerdictRunWaiting {
				t.Fatalf("mutation %q still reported run-waiting", row.name)
			}
			if res.Verdict != VerdictRunIncomplete {
				t.Fatalf("mutation %q verdict = %q, want %q", row.name, res.Verdict, VerdictRunIncomplete)
			}
		})
	}
}

// TestRunVerifyWaitingTerminalOverridesDeadline: an expired deadline still yields
// run-waiting WHEN a durable terminal result is waiting to be consumed — the one
// admitted exception to the live-deadline condition.
func TestIntegrationChangeRunVerifyWaitingTerminalOverridesDeadline(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	rcpt := rvAgreeingReceipt(f.head)
	rcpt.DeadlineLive = false
	rcpt.TerminalWaiting = true
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rcpt, found: true})

	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunWaiting {
		t.Fatalf("verdict = %q, want %q", res.Verdict, VerdictRunWaiting)
	}
}
