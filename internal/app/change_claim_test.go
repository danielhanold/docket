package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"slices"
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
// `go test -tags integration ./internal/app/ -run 'TestIntegrationRecordOpsChangeClaim.*Unrelated' -count=1`
// reddens on the progress row with the before-gate refusal the bug produced.

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

// assertAppliedSurfacesUnrelated proves an applied claim beside the unrelated
// broken record keeps its applied disposition AND lists A's grandfathered
// error finding (spec §1 step 5): the scoped gate accepted a corpus error, so
// the result must say so rather than read clean, and the surfaced error must
// not turn the applied result into a failure or change its exit code.
// Mutation check: drop the engine's kept findings from the applied result and
// this reddens.
func assertAppliedSurfacesUnrelated(t *testing.T, r ChangeClaimResult) {
	t.Helper()
	if r.Result != ResultApplied || r.Disposition != ClaimDispositionApplied {
		t.Fatalf("result = %q disposition = %q, want applied/applied", r.Result, r.Disposition)
	}
	if ExitCode(r.Result) != ExitCode(ResultApplied) {
		t.Errorf("exit code = %d, want the applied exit code %d", ExitCode(r.Result), ExitCode(ResultApplied))
	}
	for _, f := range r.Findings {
		if f.Path == unrelatedBrokenPath && f.Severity == "error" {
			return
		}
	}
	t.Errorf("applied claim findings %+v omit the unrelated record's error finding on %s", r.Findings, unrelatedBrokenPath)
}

// unrelatedProgressShape is one spec acceptance-1 unrelated-damage shape seeded
// beside B, with no record B names: files are the seeded records, and every one
// of them must stay byte-identical while a finding on one of them (by path, or
// by identity for a pathless corpus-level finding such as a cycle) is still
// reported.
type unrelatedProgressShape struct {
	name  string
	files map[string]string
	ids   []string
}

// unrelatedProgressShapes derives the production-loader progress shapes beyond
// the unparseable record: an A1/A2 duplicate-id pair nothing depends on, an
// A1↔A2 depends_on cycle, an active record whose status is done, and an ADR
// whose status is outside the four spellings.
func unrelatedProgressShapes(t *testing.T) []unrelatedProgressShape {
	t.Helper()
	dependsOn := func(src string, dep int) string {
		out := strings.Replace(src, "depends_on: []\n", "depends_on: ["+itoaTest(dep)+"]\n", 1)
		if out == src {
			t.Fatal("cycle fixture did not rewrite depends_on; the fixture shape changed")
		}
		return out
	}
	return []unrelatedProgressShape{
		{name: "duplicate-id pair", files: map[string]string{
			groomPath(98, "dup-one"): claimableChange(98, "dup-one"),
			groomPath(98, "dup-two"): claimableChange(98, "dup-two"),
		}, ids: []string{"0098"}},
		{name: "depends_on cycle", files: map[string]string{
			groomPath(96, "cyc-one"): dependsOn(claimableChange(96, "cyc-one"), 97),
			groomPath(97, "cyc-two"): dependsOn(claimableChange(97, "cyc-two"), 96),
		}, ids: []string{"0096", "0097"}},
		{name: "active record with status done", files: map[string]string{
			groomPath(95, "done-active"): lifecycleChange(95, "done-active", "done"),
		}, ids: []string{"0095"}},
		{name: "ADR with invalid status", files: map[string]string{
			"docs/adrs/0901-bogus.md": fixtureADRWithStatus(901, "bogus", "Bogus"),
		}},
	}
}

// assertUnrelatedShapeIntact proves every seeded record of the shape is
// byte-identical on the origin's branch and that a status read still reports a
// finding on one of them — the named write neither repaired nor hid the damage.
func assertUnrelatedShapeIntact(t *testing.T, repo *gitRepo, branch string, shape unrelatedProgressShape) {
	t.Helper()
	for p, want := range shape.files {
		if got, ok := originFile(t, repo.origin, branch, p); !ok || got != want {
			t.Errorf("%s: unrelated record %s on origin changed (present %v):\n%s", shape.name, p, ok, got)
		}
	}
	st := Status(context.Background(), NewGitStatusReader(newGitClient(t)), StatusOptions{RepoDir: cloneOrigin(t, repo.origin)})
	for _, f := range st.Findings {
		if _, ok := shape.files[f.Path]; ok {
			return
		}
		if f.Path == "" && slices.Contains(shape.ids, f.Identity) {
			return
		}
	}
	t.Errorf("%s: status no longer reports a finding on the unrelated records; findings %+v", shape.name, st.Findings)
}
