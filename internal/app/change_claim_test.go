package app

import (
	"context"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"strings"
	"testing"
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

// --- TestChangeClaimRefusals -----------------------------------------------

// --- TestChangeClaimRetryConvergence ---------------------------------------

// --- TestChangeRefreshClaimStampsOnly --------------------------------------

// --- TestChangeRefreshClaimSkipsUnchangedBoard ------------------------------

// TestChangeRefreshClaimSkipsUnchangedBoard: a refresh re-stamps only
// claimed_at and updated — neither is board-visible — so its board re-render
// can be byte-identical to the committed BOARD.md. Declaring an unchanged
// path trips the engine's verify-delta guard ("a declared path is not an
// actual change") and fails the whole refresh (change 0335), so the plan must
// declare the board only when it truly changes the tree: absent -> create,
// differing -> replace, byte-identical -> not declared at all.
func TestChangeRefreshClaimSkipsUnchangedBoard(t *testing.T) {
	recPath := groomPath(3, "widget")
	src := lifecycleChange(3, "widget", "in-progress")
	const boardPath = "docs/changes/BOARD.md"

	// Pass 1: no committed board. The refresh must still declare the board as
	// a create — and its declared bytes are the canonical render for this
	// corpus at the fixed test clock, which seeds the byte-identical case.
	plan, opRes := claimPlanFor(t, map[string]string{recPath: src}, baseRefreshOp([]string{"inline"}, 3))
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	assertPlanPaths(t, plan, map[string]transaction.MutationKind{
		recPath:   transaction.MutationReplace,
		boardPath: transaction.MutationCreate,
	})
	var boardBytes []byte
	for _, f := range plan.Files {
		if string(f.Path) == boardPath {
			boardBytes = f.Bytes
		}
	}
	if len(boardBytes) == 0 {
		t.Fatal("absent-board refresh declared no board bytes")
	}

	t.Run("byte-identical committed board is not declared", func(t *testing.T) {
		files := map[string]string{recPath: src, boardPath: string(boardBytes)}
		plan, opRes := claimPlanFor(t, files, baseRefreshOp([]string{"inline"}, 3))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		// Exactly the record — a declared-but-unchanged board is the 0335 bug.
		assertPlanPaths(t, plan, map[string]transaction.MutationKind{
			recPath: transaction.MutationReplace,
		})
	})

	t.Run("stale committed board is still declared", func(t *testing.T) {
		files := map[string]string{recPath: src, boardPath: "# Backlog\n\nstale\n"}
		plan, opRes := claimPlanFor(t, files, baseRefreshOp([]string{"inline"}, 3))
		if opRes.Refused {
			t.Fatalf("unexpected refusal: %v", opRes.Findings)
		}
		assertPlanPaths(t, plan, map[string]transaction.MutationKind{
			recPath:   transaction.MutationReplace,
			boardPath: transaction.MutationReplace,
		})
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

// --- gate-context binding (change 0407) ------------------------------------
//
// These drive ChangeClaim end-to-end over a real gate store (newGateRepo, whose
// git common dir roots the rungate records the store primitives read/write) and
// a real Discover client, with the metadata transaction faked by claimGateEngine.
// The gate seam sits between resolveClaimTarget and the engine call, so a fake
// engine is enough to prove validate/reserve/digest/receipt/confirm without a
// real metadata commit; the receipt bytes are proven by driving the captured
// operation's Plan closure (claimPlanFor).

const gateClaimVersion = "1234123412341234123412341234123412341234"
const gateClaimCommit = "cafebabecafebabecafebabecafebabecafebabe"

// claimGateEngine records every Execute call, optionally runs onExecute during
// the call (to observe store state mid-transaction, before the post-apply
// confirm), and returns a scripted outcome.
type claimGateEngine struct {
	result    transaction.Result
	err       error
	calls     []transaction.Request
	onExecute func(req transaction.Request)
}

func (e *claimGateEngine) Execute(_ context.Context, req transaction.Request) (transaction.Result, error) {
	e.calls = append(e.calls, req)
	if e.onExecute != nil {
		e.onExecute(req)
	}
	return e.result, e.err
}

// appliedGateResult is a scripted applied claim outcome carrying the commit that
// becomes the confirmed binding's revision plus a canonical claim receipt.
func appliedGateResult(t *testing.T, id int) transaction.Result {
	t.Helper()
	return transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: gateClaimCommit,
		Receipt: mustMarshal(t, changeClaimReceipt{
			Branch: "feat/widget", ClaimedAt: "2026-08-16T12:00:00Z",
			ID: id, Lease: "fresh", Op: OperationChangeClaim, Status: "in-progress",
		}),
	}
}

func gateClaimDeps(t *testing.T, engine *claimGateEngine, corpus []StatusBlob) PlanningDeps {
	t.Helper()
	return PlanningDeps{
		Client: newGitClient(t),
		Engine: engine,
		Reader: &fakeReader{pin: mainModePin([]string{"inline"}), corpus: corpus},
		Clock:  testClock(),
	}
}

// TestClaimGateContextInvalidRefusesBeforeTransaction: a supplied context that
// matches no armed gate record is a typed refusal that writes nothing and never
// degrades to an ungated claim (spec: "Never treat a supplied but invalid
// context as an ungated claim").
func TestClaimGateContextInvalidRefusesBeforeTransaction(t *testing.T) {
	repoDir := newGateRepo(t) // no gate record armed
	engine := &claimGateEngine{}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Version: gateClaimVersion, GateContext: "tok"})

	if res.Result != ResultInvalidState {
		t.Fatalf("result = %q, want invalid-state (findings %v)", res.Result, res.Findings)
	}
	if res.Disposition != ClaimDispositionGateContextInvalid {
		t.Errorf("disposition = %q, want %q", res.Disposition, ClaimDispositionGateContextInvalid)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called %d times on an invalid gate context, want 0", len(engine.calls))
	}
	for _, f := range res.Findings {
		if strings.Contains(f.Message, "tok") {
			t.Errorf("finding leaked the raw dispatch token: %q", f.Message)
		}
	}
}

// TestClaimGateContextReservesAndConfirms: a valid context reserves before the
// transaction and confirms after the applied outcome; the digest payload and
// receipt carry the context hash, never the raw token.
func TestClaimGateContextReservesAndConfirms(t *testing.T) {
	repoDir := newGateRepo(t)
	hash := gateHashToken("tok")
	key := mintGateWithHash(t, repoDir, hash, false)

	var midOK, midConfirmed bool
	var midChangeID int
	engine := &claimGateEngine{
		result: appliedGateResult(t, 3),
		onExecute: func(_ transaction.Request) {
			// The reservation must exist, unconfirmed, at Execute time.
			b, ok, err := LoadGateClaimBinding(repoDir, key)
			if err != nil {
				t.Errorf("mid-transaction LoadGateClaimBinding: %v", err)
				return
			}
			midOK, midConfirmed, midChangeID = ok, b.Confirmed, b.ChangeID
		},
	}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Version: gateClaimVersion, GateContext: "tok"})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (findings %v)", res.Result, res.Findings)
	}

	if !midOK || midConfirmed || midChangeID != 3 {
		t.Errorf("mid-transaction binding ok=%v confirmed=%v change=%d; want reserved-unconfirmed for change 3",
			midOK, midConfirmed, midChangeID)
	}

	b, ok, err := LoadGateClaimBinding(repoDir, key)
	if err != nil || !ok || !b.Confirmed || b.ChangeID != 3 || b.Revision != gateClaimCommit {
		t.Fatalf("post-claim binding = %+v ok=%v err=%v; want confirmed change 3 @ %s", b, ok, err, gateClaimCommit)
	}

	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	gotDigest := engine.calls[0].Idempotency.Digest
	withHash, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Version: gateClaimVersion, GateContextHash: hash})
	if err != nil {
		t.Fatalf("canonicalDigest (hash): %v", err)
	}
	ungated, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Version: gateClaimVersion, GateContextHash: ""})
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
	if op.gateContextHash != hash {
		t.Errorf("op.gateContextHash = %q, want %q", op.gateContextHash, hash)
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

// TestClaimGateContextConflictRefused: a second claim for a DIFFERENT change id
// under the same context is refused gate-context-conflict before its
// transaction (criterion 3: one context cannot claim two changes).
func TestClaimGateContextConflictRefused(t *testing.T) {
	repoDir := newGateRepo(t)
	mintGateWithHash(t, repoDir, gateHashToken("tok"), false)
	corpus := []StatusBlob{
		changeBlob(3, "widget", "feat", "high", ""),
		changeBlob(4, "gadget", "feat", "high", ""),
	}

	engine1 := &claimGateEngine{result: appliedGateResult(t, 3)}
	first := ChangeClaim(context.Background(), gateClaimDeps(t, engine1, corpus), repoDir,
		ChangeClaimRequest{ID: 3, Version: gateClaimVersion, GateContext: "tok"})
	if first.Result != ResultApplied {
		t.Fatalf("first claim result = %q, want applied (%v)", first.Result, first.Findings)
	}

	engine2 := &claimGateEngine{result: appliedGateResult(t, 4)}
	second := ChangeClaim(context.Background(), gateClaimDeps(t, engine2, corpus), repoDir,
		ChangeClaimRequest{ID: 4, Version: gateClaimVersion, GateContext: "tok"})

	if second.Result != ResultInvalidState {
		t.Errorf("second result = %q, want invalid-state", second.Result)
	}
	if second.Disposition != ClaimDispositionGateContextConflict {
		t.Errorf("second disposition = %q, want %q", second.Disposition, ClaimDispositionGateContextConflict)
	}
	if len(engine2.calls) != 0 {
		t.Errorf("second claim reached the engine %d times, want 0", len(engine2.calls))
	}
}

// TestClaimSameIDDifferentContextDigestDiffers: two dispatches submitting the
// SAME (id, version) under different contexts must not share the idempotency
// path — their digests differ while their request ids match, so the engine's
// replay scan refuses the second as id-reuse rather than replaying the first's
// receipt (criterion 3).
func TestClaimSameIDDifferentContextDigestDiffers(t *testing.T) {
	h1 := gateHashToken("tokA")
	h2 := gateHashToken("tokB")
	d1, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Version: gateClaimVersion, GateContextHash: h1})
	if err != nil {
		t.Fatalf("digest 1: %v", err)
	}
	d2, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Version: gateClaimVersion, GateContextHash: h2})
	if err != nil {
		t.Fatalf("digest 2: %v", err)
	}
	if d1 == d2 {
		t.Errorf("digests match across differing contexts (%q); the same (id,version) would share the idempotency path", d1)
	}
	reqA := claimRequestID(ChangeClaimRequest{ID: 3, Version: gateClaimVersion, GateContext: "tokA"})
	reqB := claimRequestID(ChangeClaimRequest{ID: 3, Version: gateClaimVersion, GateContext: "tokB"})
	if reqA != reqB {
		t.Errorf("request ids differ (%q vs %q); they must match so the engine's replay scan sees id-reuse", reqA, reqB)
	}
}

// TestClaimUngatedUnchanged: no GateContext → no gate lookup, no reservation,
// digest equals the empty-hash payload, receipt carries gate_context_hash "".
func TestClaimUngatedUnchanged(t *testing.T) {
	repoDir := newGateRepo(t)
	engine := &claimGateEngine{result: appliedGateResult(t, 3)}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Version: gateClaimVersion}) // no GateContext
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (%v)", res.Result, res.Findings)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	want, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 3, Version: gateClaimVersion, GateContextHash: ""})
	if err != nil {
		t.Fatalf("canonicalDigest: %v", err)
	}
	if engine.calls[0].Idempotency.Digest != want {
		t.Errorf("ungated digest = %q, want %q", engine.calls[0].Idempotency.Digest, want)
	}
	op := engine.calls[0].Operation.(changeClaimOp)
	if op.gateContextHash != "" {
		t.Errorf("ungated op.gateContextHash = %q, want empty", op.gateContextHash)
	}
	plan, opRes := claimPlanFor(t, map[string]string{groomPath(3, "widget"): claimableChange(3, "widget")}, op)
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	if !strings.Contains(string(plan.Receipt), `"gate_context_hash":""`) {
		t.Errorf("ungated receipt missing empty gate_context_hash:\n%s", plan.Receipt)
	}
}

// TestClaimTerminalGateRefused: a context whose only record is Terminal is
// gate-context-invalid (the dispatch it named is already decided).
func TestClaimTerminalGateRefused(t *testing.T) {
	repoDir := newGateRepo(t)
	mintGateWithHash(t, repoDir, gateHashToken("tok"), true) // terminal
	engine := &claimGateEngine{}
	deps := gateClaimDeps(t, engine, []StatusBlob{changeBlob(3, "widget", "feat", "high", "")})

	res := ChangeClaim(context.Background(), deps, repoDir,
		ChangeClaimRequest{ID: 3, Version: gateClaimVersion, GateContext: "tok"})
	if res.Disposition != ClaimDispositionGateContextInvalid {
		t.Errorf("disposition = %q, want %q", res.Disposition, ClaimDispositionGateContextInvalid)
	}
	if len(engine.calls) != 0 {
		t.Errorf("engine called on a terminal gate context, want 0")
	}
}
