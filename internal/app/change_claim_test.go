package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"path"
	"strings"
	"testing"
	"time"
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

// claimEarlyErrOp is a minimal valid-keyed semantic operation for driving the
// real engine's call-shape validation; Execute fails on the malformed
// expectation before Plan can ever run.
type claimEarlyErrOp struct{}

func (claimEarlyErrOp) Key() transaction.OperationKey { return "change.claim" }

func (claimEarlyErrOp) Plan(context.Context, transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	return transaction.MutationPlan{}, transaction.OperationResult{}, errors.New("unreachable: call-shape validation fails first")
}

// claimEngineClock pins the engine's clock; the validation path never reads it.
type claimEngineClock struct{}

func (claimEngineClock) Now() time.Time { return time.Unix(1758400000, 0).UTC() }

// TestClaimResultRealEngineMalformedVersion is change 0350's end-to-end
// regression: a REAL transaction.Engine given a malformed (shortened)
// expected-version object id returns its base result — empty disposition —
// with a typed *Failure from StageValidateRequest, and claimResultFromOutcome
// must surface that as invalid-input with a populated failure diagnosis, not
// a bare internal-error.
func TestClaimResultRealEngineMalformedVersion(t *testing.T) {
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	eng, err := transaction.NewEngine(client, claimEngineClock{})
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	res, execErr := eng.Execute(context.Background(), transaction.Request{
		TargetRef: "refs/heads/docket",
		Expected: []transaction.EntityExpectation{{
			Path: "docs/changes/active/0350-surface.md",
			// Shortened object id — the confirmed early-validation trigger
			// (a full-length well-formed wrong id follows the contended
			// path instead; see the change file's "## Why").
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: "abc123"},
		}},
		Operation: claimEarlyErrOp{},
		// Loader deliberately nil: expectations are validated before the
		// loader, so Execute must return before touching it or any git state.
	})
	if res.Disposition != "" {
		t.Fatalf("Disposition = %q, want empty (early validation return)", res.Disposition)
	}
	if execErr == nil {
		t.Fatal("Execute error = nil, want a typed *Failure")
	}
	out := claimResultFromOutcome(OperationChangeClaim, res, execErr)
	if out.Result != ResultInvalidInput {
		t.Fatalf("Result = %q, want %q", out.Result, ResultInvalidInput)
	}
	if out.Failure == nil {
		t.Fatal("Failure = nil, want the typed diagnosis")
	}
	if out.Failure.Stage != string(transaction.StageValidateRequest) {
		t.Errorf("Failure.Stage = %q, want %q", out.Failure.Stage, transaction.StageValidateRequest)
	}
	if out.Failure.Kind != string(transaction.KindInvalidInput) {
		t.Errorf("Failure.Kind = %q, want %q", out.Failure.Kind, transaction.KindInvalidInput)
	}
	if !strings.Contains(out.Failure.Detail, "invalid expectations") {
		t.Errorf("Failure.Detail = %q, want it to name the invalid expectations", out.Failure.Detail)
	}
}

// --- 0449: unrelated invalid records never block a named claim --------------
//
// These real-git tests drive the production operations through a real
// transaction.Engine and the production planning loader over a corpus that also
// carries an UNRELATED unparseable change record (A, id 99). The progress rows
// are the reproduction of the 0449 bug: under the strict whole-corpus gate A's
// parse finding refused every named write. The refusal rows prove the scope is
// bounded — a defect on B, B depending on the unparseable A, and a duplicate of
// B's id still refuse, and nothing moves on the origin.
//
// Mutation check (run manually; noted in the commit): delete the `Scope:` field
// from ChangeClaim's transaction.Request and
// `go test ./internal/app/ -run 'TestChangeClaim.*Unrelated' -count=1` reddens on
// the progress row with the before-gate refusal the bug produced.

// unrelatedBrokenPath is the unrelated change record A every 0449 row seeds; its
// bytes open a frontmatter block and never close it, so document.Parse rejects
// it and the loader reports a parse finding on this path.
const (
	unrelatedBrokenPath  = "docs/changes/active/0099-broken.md"
	unrelatedBrokenBytes = "---\nid: 99\nslug: broken\n"
)

// unrelatedRefusalCase is one bounded-scope refusal row: the metadata files
// seeded beside B (which always include the unrelated broken A).
type unrelatedRefusalCase struct {
	name  string
	files map[string]string
}

// unrelatedRefusalCases derives the three refusal rows from B's healthy record
// src at recPath: a validation defect on B itself (an unknown type), B
// depending on the unparseable A (a dangling depends_on error on B), and a
// second record carrying B's id.
func unrelatedRefusalCases(t *testing.T, id int, recPath, src, dupeSrc string) []unrelatedRefusalCase {
	t.Helper()
	// An ill-shaped type token is an error finding on B that still leaves B
	// decodable, so B stays in the snapshot and the defect is B's own.
	defect := strings.Replace(src, "type: feat\n", "type: 'Not A Token'\n", 1)
	dependent := strings.Replace(src, "depends_on: []\n", "depends_on: [99]\n", 1)
	if defect == src || dependent == src {
		t.Fatal("refusal fixtures did not rewrite B's record; the fixture shape changed")
	}
	return []unrelatedRefusalCase{
		{name: "defect on B", files: map[string]string{recPath: defect, unrelatedBrokenPath: unrelatedBrokenBytes}},
		{name: "B depends on unparseable A", files: map[string]string{recPath: dependent, unrelatedBrokenPath: unrelatedBrokenBytes}},
		{name: "duplicate of B's id", files: map[string]string{
			recPath: src, groomPath(id, "dupe"): dupeSrc, unrelatedBrokenPath: unrelatedBrokenBytes,
		}},
	}
}

// assertUnrelatedBrokenIntact proves the unrelated broken record is
// byte-identical on the origin's metadata branch and that a subsequent status
// read still reports its parse finding — the named write neither repaired nor
// hid it.
func assertUnrelatedBrokenIntact(t *testing.T, repo *gitRepo) {
	t.Helper()
	got, ok := originFile(t, repo.origin, "docket", unrelatedBrokenPath)
	if !ok || got != unrelatedBrokenBytes {
		t.Errorf("unrelated broken record on origin = %q (present %v), want its exact seeded bytes", got, ok)
	}
	st := Status(context.Background(), NewGitStatusReader(newGitClient(t)), StatusOptions{RepoDir: cloneOrigin(t, repo.origin)})
	for _, f := range st.Findings {
		if f.Path == unrelatedBrokenPath && f.Severity == "error" {
			return
		}
	}
	t.Errorf("status no longer reports the unrelated parse finding on %s; findings %+v", unrelatedBrokenPath, st.Findings)
}

// assertRefusalBeyondUnrelated proves a refusal is attributed to B's own
// bounded scope — not merely to the unrelated broken record — by requiring a
// finding on some other path (or a pathless typed refusal such as an ambiguous
// id, or a typed refusal reason). A refusal carrying only A's parse finding is
// the strict-gate bug shape.
func assertRefusalBeyondUnrelated(t *testing.T, reason string, findings []StatusFinding) {
	t.Helper()
	if reason != "" && reason != "unclosed-frontmatter" {
		return
	}
	for _, f := range findings {
		if f.Path != unrelatedBrokenPath {
			return
		}
	}
	t.Errorf("refusal carries only the unrelated record's findings %+v; want a finding naming B's own defect", findings)
}

func TestChangeClaimUnrelatedInvalidRecordProgress(t *testing.T) {
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

	claim := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath)})
	if claim.Result != ResultApplied {
		t.Fatalf("claim beside an unrelated unparseable record = %q (disposition %q findings %v), want applied",
			claim.Result, claim.Disposition, claim.Findings)
	}
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

	refresh := ChangeRefreshClaim(ctx, later.deps, later.dir, ChangeClaimRequest{ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath)})
	if refresh.Result != ResultApplied {
		t.Fatalf("refresh-claim beside an unrelated unparseable record = %q (disposition %q findings %v), want applied",
			refresh.Result, refresh.Disposition, refresh.Findings)
	}
	assertUnrelatedBrokenIntact(t, repo)
}

func TestChangeClaimUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	for _, c := range unrelatedRefusalCases(t, id, recPath, claimableChange(id, "widget"), claimableChange(id, "dupe")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			node := planningDepsFor(t, repo.invocation)
			tip := originTip(t, repo.origin, "docket")

			res := ChangeClaim(context.Background(), node.deps, node.dir,
				ChangeClaimRequest{ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath)})
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

func TestChangeRefreshClaimUnrelatedInvalidRecordRefusals(t *testing.T) {
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
				ChangeClaimRequest{ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath)})
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
