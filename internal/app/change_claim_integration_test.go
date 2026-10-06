//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"fmt"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"path"
	"strings"
	"testing"
	"time"
)

// TestIntegrationRecordOpsClaimTransactionTargetsPinnedMetadataLayout pins the
// caller wiring (defaulted-param-hides-caller-wiring): the claim transaction's
// remote and target ref come from the PINNED layout, not a shared default. The
// pin carries a NON-default private layout, so a site that still hardcodes
// origin/docket reddens here even though every shared-mode test stays green.
func TestIntegrationRecordOpsClaimTransactionTargetsPinnedMetadataLayout(t *testing.T) {
	repoDir := newRunTrackerRepo(t)
	engine := &claimGateEngine{result: appliedGateResult(t, 3)}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})
	reader := deps.Reader.(*fakeReader)
	reader.pin.Layout = layout.PrivateLayout("/c", "/r", "/d", "o-r")

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Revision: gateClaimRevision})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (%v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Remote != "dckt" {
		t.Errorf("transaction remote = %q, want dckt (the pinned private metadata remote)", req.Remote)
	}
	if req.TargetRef != "refs/heads/dckt" {
		t.Errorf("transaction target ref = %q, want refs/heads/dckt (the pinned private metadata branch)", req.TargetRef)
	}
}

// TestIntegrationRecordOpsClaimRunContextInvalidRefusesBeforeTransaction: a supplied context that
// matches no started run-tracker record is a typed refusal that writes nothing and never
// degrades to an ungated claim (spec: "Never treat a supplied but invalid
// context as an ungated claim").
func TestIntegrationRecordOpsClaimRunContextInvalidRefusesBeforeTransaction(t *testing.T) {
	repoDir := newRunTrackerRepo(t) // no gate record started
	engine := &claimGateEngine{}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Revision: gateClaimRevision, RunContext: "tok"})

	if res.Result != ResultInvalidState {
		t.Fatalf("result = %q, want invalid-state (findings %v)", res.Result, res.Findings)
	}
	if res.Disposition != ClaimDispositionRunContextInvalid {
		t.Errorf("disposition = %q, want %q", res.Disposition, ClaimDispositionRunContextInvalid)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on an invalid run context, want 0", len(engine.calls))
	}
	for _, f := range res.Findings {
		if strings.Contains(f.Message, "tok") {
			t.Errorf("finding leaked the raw dispatch token: %q", f.Message)
		}
	}
}

// TestIntegrationRecordOpsClaimRunContextReservesAndConfirms: a valid context reserves before the
// transaction and confirms after the applied outcome; the digest payload and
// receipt carry the context hash, never the raw token.
func TestIntegrationRecordOpsClaimRunContextReservesAndConfirms(t *testing.T) {
	repoDir := newRunTrackerRepo(t)
	hash := runTrackerHashToken("tok")
	key := mintRunTrackerWithHash(t, repoDir, hash, false)

	var midOK, midConfirmed bool
	var midChangeID int
	engine := &claimGateEngine{
		result: appliedGateResult(t, 3),
		onExecute: func(_ transaction.Request) {
			// The reservation must exist, unconfirmed, at Execute time.
			b, ok, err := LoadRunTrackerClaimBinding(repoDir, key)
			if err != nil {
				t.Errorf("mid-transaction LoadRunTrackerClaimBinding: %v", err)
				return
			}
			midOK, midConfirmed, midChangeID = ok, b.Confirmed, b.ChangeID
		},
	}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Revision: gateClaimRevision, RunContext: "tok"})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (findings %v)", res.Result, res.Findings)
	}

	if !midOK || midConfirmed || midChangeID != 3 {
		t.Errorf("mid-transaction binding ok=%v confirmed=%v change=%d; want reserved-unconfirmed for change 3",
			midOK, midConfirmed, midChangeID)
	}

	b, ok, err := LoadRunTrackerClaimBinding(repoDir, key)
	if err != nil || !ok || !b.Confirmed || b.ChangeID != 3 || b.Revision != gateClaimCommit {
		t.Fatalf("post-claim binding = %+v ok=%v err=%v; want confirmed change 3 @ %s", b, ok, err, gateClaimCommit)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	gotDigest := engine.calls[0].Idempotency.Digest
	withHash, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Revision: gateClaimRevision, RunContextHash: hash})
	if err != nil {
		t.Fatalf("canonicalDigest (hash): %v", err)
	}
	ungated, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Revision: gateClaimRevision, RunContextHash: ""})
	if err != nil {
		t.Fatalf("canonicalDigest (ungated): %v", err)
	}
	if gotDigest != withHash {
		t.Errorf("digest = %q, want the hash-bearing digest %q", gotDigest, withHash)
	}
	if gotDigest == ungated {
		t.Errorf("digest equals the ungated digest %q; the context hash was not folded in", ungated)
	}

	op, okOp := engine.calls[0].Operation.(changeClaimOp)
	if !okOp {
		t.Fatalf("operation is %T, want changeClaimOp", engine.calls[0].Operation)
	}
	if op.runContextHash != hash {
		t.Errorf("op.runContextHash = %q, want %q", op.runContextHash, hash)
	}
	// The receipt bytes the operation hands the engine carry the hash, never the token.
	plan, opRes := claimPlanFor(t, map[string]string{groomPath(3, "widget"): claimableChange(3, "widget")}, op)
	if opRes.Refused {
		t.Fatalf("unexpected refusal building the receipt: %v", opRes.Findings)
	}
	if !strings.Contains(string(plan.Receipt), `"gate_context_hash":"`+hash+`"`) {
		t.Errorf("receipt missing gate_context_hash %q:\n%s", hash, plan.Receipt)
	}
	if strings.Contains(string(plan.Receipt), "tok") {
		t.Errorf("receipt leaked the raw dispatch token:\n%s", plan.Receipt)
	}
}

// TestIntegrationRecordOpsClaimRunContextConflictRefused: a second claim for a DIFFERENT change id
// under the same context is refused run-context-conflict before its
// transaction (criterion 3: one context cannot claim two changes).
func TestIntegrationRecordOpsClaimRunContextConflictRefused(t *testing.T) {
	repoDir := newRunTrackerRepo(t)
	mintRunTrackerWithHash(t, repoDir, runTrackerHashToken("tok"), false)
	corpus := []StatusBlob{
		changeBlob(3, "widget", "feat", "high", ""),
		changeBlob(4, "gadget", "feat", "high", ""),
	}

	engine1 := &claimGateEngine{result: appliedGateResult(t, 3)}
	first := ChangeClaim(context.Background(), gateClaimDeps(t, engine1, corpus), repoDir,
		ChangeClaimRequest{ID: 3, Revision: gateClaimRevision, RunContext: "tok"})
	if first.Result != ResultApplied {
		t.Fatalf("first claim result = %q, want applied (%v)", first.Result, first.Findings)
	}

	engine2 := &claimGateEngine{result: appliedGateResult(t, 4)}
	second := ChangeClaim(context.Background(), gateClaimDeps(t, engine2, corpus), repoDir,
		ChangeClaimRequest{ID: 4, Revision: gateClaimRevision, RunContext: "tok"})

	if second.Result != ResultInvalidState {
		t.Errorf("second result = %q, want invalid-state", second.Result)
	}
	if second.Disposition != ClaimDispositionRunContextConflict {
		t.Errorf("second disposition = %q, want %q", second.Disposition, ClaimDispositionRunContextConflict)
	}
	if len(engine2.calls) != 0 {
		t.Errorf("second claim reached the engine %d times, want 0", len(engine2.calls))
	}
}

// TestIntegrationRecordOpsClaimUngatedUnchanged: no RunContext → no gate lookup, no reservation,
// digest equals the empty-hash payload, receipt carries gate_context_hash "".
func TestIntegrationRecordOpsClaimUngatedUnchanged(t *testing.T) {
	repoDir := newRunTrackerRepo(t)
	engine := &claimGateEngine{result: appliedGateResult(t, 3)}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Revision: gateClaimRevision}) // no RunContext
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (%v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	want, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Revision: gateClaimRevision, RunContextHash: ""})
	if err != nil {
		t.Fatalf("canonicalDigest: %v", err)
	}
	if engine.calls[0].Idempotency.Digest != want {
		t.Errorf("ungated digest = %q, want %q", engine.calls[0].Idempotency.Digest, want)
	}
	op := engine.calls[0].Operation.(changeClaimOp)
	if op.runContextHash != "" {
		t.Errorf("ungated op.runContextHash = %q, want empty", op.runContextHash)
	}
	plan, opRes := claimPlanFor(t, map[string]string{groomPath(3, "widget"): claimableChange(3, "widget")}, op)
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	if !strings.Contains(string(plan.Receipt), `"gate_context_hash":""`) {
		t.Errorf("ungated receipt missing empty gate_context_hash:\n%s", plan.Receipt)
	}
}

// TestIntegrationRecordOpsClaimTerminalGateRefused: a context whose only record is Terminal is
// run-context-invalid (the dispatch it named is already decided).
func TestIntegrationRecordOpsClaimTerminalGateRefused(t *testing.T) {
	repoDir := newRunTrackerRepo(t)
	mintRunTrackerWithHash(t, repoDir, runTrackerHashToken("tok"), true) // terminal
	engine := &claimGateEngine{}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Revision: gateClaimRevision, RunContext: "tok"})
	if res.Disposition != ClaimDispositionRunContextInvalid {
		t.Errorf("disposition = %q, want %q", res.Disposition, ClaimDispositionRunContextInvalid)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called on a terminal run context, want 0")
	}
}

func TestIntegrationRecordOpsChangeClaimUnrelatedInvalidRecordProgress(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	repo := newWorkingRepo(t, map[string]string{
		recPath:             claimableChange(id, "widget"),
		unrelatedBrokenPath: unrelatedBrokenBytes,
	})
	node := planningDepsFor(t, repo.invocation)
	later := planningDepsForClock(t, repo.invocation, fixedClock{t: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)})
	ctx := context.Background()

	claim := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Revision: blobRevisionAt(t, repo.origin, "docket", recPath)})
	if claim.Result != ResultApplied {
		t.Fatalf("claim beside an unrelated unparseable record = %q (disposition %q findings %v), want applied",
			claim.Result, claim.Disposition, claim.Findings)
	}
	assertAppliedSurfacesUnrelated(t, claim)
	rec, _ := originFile(t, repo.origin, "docket", recPath)
	if !strings.Contains(rec, "status: 'in-progress'") {
		t.Errorf("claimed record on origin is not in-progress:\n%s", rec)
	}
	assertUnrelatedBrokenIntact(t, repo)
	// The board landed in the same applied commit (change 0449 Task 8): B's
	// row in its new section AND the repair notice naming the unparseable A,
	// which the snapshot cannot see and the board used to drop silently.
	board, ok := originFile(t, repo.origin, "docket", "docs/changes/BOARD.md")
	if !ok {
		t.Fatal("claim beside an unrelated unparseable record published no board")
	}
	if !strings.Contains(board, "## 🟢 In progress (1)") || !strings.Contains(board, "(active/"+path.Base(recPath)+")") {
		t.Errorf("board lacks B's in-progress row:\n%s", board)
	}
	if !strings.Contains(board, "| `"+unrelatedBrokenPath+"` | unclosed-frontmatter |") {
		t.Errorf("board lacks the repair notice naming the unrelated record:\n%s", board)
	}

	refresh := ChangeRefreshClaim(ctx, later.deps, later.dir, ChangeClaimRequest{ID: id, Revision: blobRevisionAt(t, repo.origin, "docket", recPath)})
	if refresh.Result != ResultApplied {
		t.Fatalf("refresh-claim beside an unrelated unparseable record = %q (disposition %q findings %v), want applied",
			refresh.Result, refresh.Disposition, refresh.Findings)
	}
	assertAppliedSurfacesUnrelated(t, refresh)
	assertUnrelatedBrokenIntact(t, repo)
}

// TestIntegrationRecordOpsChangeClaimUnrelatedDependentsOfBrokenProgress is the canonical real-world
// shape of the 0449 bug one step removed: the unparseable A (id 99) has
// unrelated dependents — C depends on 99 and E is stacked on 99 — so the corpus
// also carries error-severity dangling references whose target id no parsed
// record carries. Those ids name no record, so they cannot name B's subjects;
// B's claim applies and C and E are left byte-identical. The refusal rows keep
// the converse: B itself depending on the unparseable A still refuses.
//
// Mutation check (run manually; noted in the commit): make
// transaction.resolveRef treat an absent lookup as unresolvable again and this
// test reddens with the claim refused on C's and E's dangling references.
func TestIntegrationRecordOpsChangeClaimUnrelatedDependentsOfBrokenProgress(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	consumerPath, stackedPath := groomPath(4, "consumer"), groomPath(5, "stacked")
	consumer := strings.Replace(claimableChange(4, "consumer"), "depends_on: []\n", "depends_on: [99]\n", 1)
	stacked := stackedOn(claimableChange(5, "stacked"), 99)
	if !strings.Contains(consumer, "depends_on: [99]") || !strings.Contains(stacked, "stacked_on: 99") {
		t.Fatal("dependent fixtures did not rewrite their records; the fixture shape changed")
	}
	repo := newWorkingRepo(t, map[string]string{
		recPath:             claimableChange(id, "widget"),
		consumerPath:        consumer,
		stackedPath:         stacked,
		unrelatedBrokenPath: unrelatedBrokenBytes,
	})
	node := planningDepsFor(t, repo.invocation)

	claim := ChangeClaim(context.Background(), node.deps, node.dir, ChangeClaimRequest{ID: id, Revision: blobRevisionAt(t, repo.origin, "docket", recPath)})
	if claim.Result != ResultApplied {
		t.Fatalf("claim beside unrelated dependents of an unparseable record = %q (disposition %q findings %v), want applied",
			claim.Result, claim.Disposition, claim.Findings)
	}
	if rec, _ := originFile(t, repo.origin, "docket", recPath); !strings.Contains(rec, "status: 'in-progress'") {
		t.Errorf("claimed record on origin is not in-progress:\n%s", rec)
	}
	for p, want := range map[string]string{consumerPath: consumer, stackedPath: stacked} {
		if got, ok := originFile(t, repo.origin, "docket", p); !ok || got != want {
			t.Errorf("unrelated dependent %s on origin changed (present %v):\n%s", p, ok, got)
		}
	}
	assertUnrelatedBrokenIntact(t, repo)
}

// TestIntegrationRecordOpsChangeClaimUnrelatedShapesProgress drives B's claim through the
// production loader and engine beside each unrelated-damage shape: B applies,
// the damaged records are byte-identical, and the finding is still reported.
func TestIntegrationRecordOpsChangeClaimUnrelatedShapesProgress(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	for _, shape := range unrelatedProgressShapes(t) {
		t.Run(shape.name, func(t *testing.T) {
			files := map[string]string{recPath: claimableChange(id, "widget")}
			for p, b := range shape.files {
				files[p] = b
			}
			repo := newWorkingRepo(t, files)
			node := planningDepsFor(t, repo.invocation)
			res := ChangeClaim(context.Background(), node.deps, node.dir,
				ChangeClaimRequest{ID: id, Revision: blobRevisionAt(t, repo.origin, "docket", recPath)})
			if res.Result != ResultApplied {
				t.Fatalf("claim beside %s = %q (disposition %q findings %v), want applied", shape.name, res.Result, res.Disposition, res.Findings)
			}
			if rec, _ := originFile(t, repo.origin, "docket", recPath); !strings.Contains(rec, "status: 'in-progress'") {
				t.Errorf("claimed record on origin is not in-progress:\n%s", rec)
			}
			assertUnrelatedShapeIntact(t, repo, "docket", shape)
		})
	}
}

func TestIntegrationRecordOpsChangeClaimUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	for _, c := range unrelatedRefusalCases(t, id, recPath, claimableChange(id, "widget"), claimableChange(id, "dupe")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			node := planningDepsFor(t, repo.invocation)
			tip := originTip(t, repo.origin, "docket")

			res := ChangeClaim(context.Background(), node.deps, node.dir,
				ChangeClaimRequest{ID: id, Revision: blobRevisionAt(t, repo.origin, "docket", recPath)})
			if res.Result == ResultApplied {
				t.Fatalf("claim applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, res.Disposition, res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused claim moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}

func TestIntegrationRecordOpsChangeRefreshClaimUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	src := lifecycleChange(id, "widget", "in-progress")
	for _, c := range unrelatedRefusalCases(t, id, recPath, src, lifecycleChange(id, "dupe", "in-progress")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			node := planningDepsFor(t, repo.invocation)
			tip := originTip(t, repo.origin, "docket")

			res := ChangeRefreshClaim(context.Background(), node.deps, node.dir,
				ChangeClaimRequest{ID: id, Revision: blobRevisionAt(t, repo.origin, "docket", recPath)})
			if res.Result == ResultApplied {
				t.Fatalf("refresh-claim applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, res.Disposition, res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused refresh-claim moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}

// TestIntegrationRecordOpsClaimResumeContextRefusedBeforeReserve: a `run start --resume` start pre-binds
// the resumed change as AttributedID and never gets a claim binding (change 0463).
// A claim under that context, for the resumed change itself or for any other change,
// is refused run-context-conflict BEFORE ReserveRunTrackerClaim writes a binding file: a
// stray unconfirmed reservation would make run.cancel refuse claim-unconfirmed, and a
// confirmed claim of a different change would make it refuse claim-mismatch, leaving
// the resume run uncancellable either way.
func TestIntegrationRecordOpsClaimResumeContextRefusedBeforeReserve(t *testing.T) {
	for _, id := range []int{3, 4} {
		t.Run(fmt.Sprintf("claim-%d", id), func(t *testing.T) {
			repoDir := newRunTrackerRepo(t)
			key, err := MintRunTrackerRecord(repoDir, RunTrackerRecord{
				Target: "docket-implement-next", Retry: RetryUnused, AttemptLimit: 2,
				ChildContextHash: runTrackerHashToken("tok"), AttributedID: 3,
			})
			if err != nil {
				t.Fatalf("MintRunTrackerRecord: %v", err)
			}
			corpus := []StatusBlob{
				changeBlob(3, "widget", "feat", "high", ""),
				changeBlob(4, "gadget", "feat", "high", ""),
			}
			engine := &claimGateEngine{result: appliedGateResult(t, id)}

			res := ChangeClaim(context.Background(), gateClaimDeps(t, engine, corpus), repoDir,
				ChangeClaimRequest{ID: id, Revision: gateClaimRevision, RunContext: "tok"})

			if res.Result != ResultInvalidState || res.Disposition != ClaimDispositionRunContextConflict {
				t.Fatalf("result = %q disposition = %q, want invalid-state %q (findings %v)",
					res.Result, res.Disposition, ClaimDispositionRunContextConflict, res.Findings)
			}
			if len(engine.calls) != 0 {
				t.Errorf("engine called %d times under a resume context, want 0", len(engine.calls))
			}
			if _, ok, berr := LoadRunTrackerClaimBinding(repoDir, key); berr != nil || ok {
				t.Errorf("claim binding present=%v err=%v after refusal; want none written", ok, berr)
			}
		})
	}
}

// TestIntegrationRecordOpsClaimRunContextRetryAfterConfirmAdmitted: the resume-context refusal keys on
// the resume-verified shape only. A fresh start's record gains AttributedID at confirm
// time together with BoundRequestID, so an idempotent retry of the same confirmed
// claim must still reach the engine rather than being refused as a resume context.
func TestIntegrationRecordOpsClaimRunContextRetryAfterConfirmAdmitted(t *testing.T) {
	repoDir := newRunTrackerRepo(t)
	mintRunTrackerWithHash(t, repoDir, runTrackerHashToken("tok"), false)
	corpus := []StatusBlob{changeBlob(3, "widget", "feat", "high", "")}

	for i := 0; i < 2; i++ {
		engine := &claimGateEngine{result: appliedGateResult(t, 3)}
		res := ChangeClaim(context.Background(), gateClaimDeps(t, engine, corpus), repoDir,
			ChangeClaimRequest{ID: 3, Revision: gateClaimRevision, RunContext: "tok"})
		if res.Result != ResultApplied {
			t.Fatalf("attempt %d result = %q disposition = %q, want applied (%v)", i+1, res.Result, res.Disposition, res.Findings)
		}
		if len(engine.calls) != 1 {
			t.Fatalf("attempt %d engine calls = %d, want 1", i+1, len(engine.calls))
		}
	}
}
