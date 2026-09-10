package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/repository"
)

// These are the `docket run gate-verdict <key>` (attributed mode) tests (change
// 0334, Task 3). gate-verdict loads the durable gate record, attributes exactly
// one new in-progress claim through the three filters (id not in the before-set;
// claimed_at parses; claimed_at >= dispatch epoch), then delegates the run
// predicate to RunVerify and maps its verdict onto the attributed vocabulary —
// consuming the single retry permit BEFORE emitting so a lost retry is the safe
// failure, and failing closed to gate-unavailable on any load error or
// unrecognized verdict. Every outcome is a report line that exits 0.
//
// The store is rooted at a real temp git repo (newGateRepo / the run-verify
// fixture's invocation clone); attribution reads the fakeReader corpus; the
// RunVerify delegation is driven by the run_verify_test.go fixtures (rvFixture,
// rvRecord, rvInProgressRecord, rvPR, rvAgreeingReceipt).

const gateDefaultClaimedAt = "2026-08-02T00:00:00Z"

// gateClaimEpoch is the Unix epoch of gateDefaultClaimedAt — the claim instant
// lifecycleChange stamps on an in-progress record. Attribution filter (c)
// compares a candidate's claimed_at against the record's DispatchEpoch, so tests
// straddle this value.
func gateClaimEpoch(t *testing.T) int64 {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, gateDefaultClaimedAt)
	if err != nil {
		t.Fatalf("parse %q: %v", gateDefaultClaimedAt, err)
	}
	return tm.Unix()
}

// gateInProgressBlob builds an in-progress change blob whose claimed_at is the
// default stamp ("keep"), removed (""), or replaced with the given raw value.
func gateInProgressBlob(id int, slug, claimedAt string) StatusBlob {
	src := lifecycleChange(id, slug, "in-progress")
	const def = "claimed_at: " + gateDefaultClaimedAt
	switch claimedAt {
	case "keep":
		// leave the default stamp in place
	case "":
		src = strings.Replace(src, def+"\n", "", 1)
	default:
		src = strings.Replace(src, def, "claimed_at: "+claimedAt, 1)
	}
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(src),
	}
}

// gateIncompleteRecord renders an in-progress change 3 carrying a valid pr: and
// linkage, so the ONLY unmet postcondition RunVerify reports is not-implemented
// (the run is claimed but not yet marked implemented). It lets the retry-mapping
// tests assert an exact single-conjunct report line.
func gateIncompleteRecord() []byte {
	src := string(rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug))
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+rvRecordedPR()+"'\nblocked_by:\n", 1)
	return []byte(src)
}

// gateLightDeps wires a planning-only deps set over the given corpus (no git
// client, no workspace/github seams) for attribution paths that never reach
// RunVerify.
func gateLightDeps(t *testing.T, corpus []StatusBlob) PlanningDeps {
	t.Helper()
	return PlanningDeps{Reader: &fakeReader{pin: mainPin(t), corpus: corpus}, Clock: testClock()}
}

// gateMintArmed mints an armed record (Retry unused, no attribution yet) with the
// given before-set, dispatch epoch, and child-context hash, as gate-before would.
// Since change 0407 the before-set and dispatch epoch are diagnostics only (they
// no longer create attribution); hash is the record's ChildContextHash, the seam
// the verdict path's proof filter keys on.
func gateMintArmed(t *testing.T, repoDir string, beforeIDs []int, dispatchEpoch int64, hash string) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:           "docket-implement-next",
		CreatedAt:        1,
		DispatchEpoch:    dispatchEpoch,
		BeforeIDs:        beforeIDs,
		ChildContextHash: hash,
		Retry:            RetryUnused,
		Disposition:      "gate-armed",
		AttemptLimit:     2,
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// gateMintAttributed mints a record already attributed to id — the state a second
// gate-verdict call reads after a first call attributed the claim.
func gateMintAttributed(t *testing.T, repoDir string, id int) string {
	t.Helper()
	key := gateMintArmed(t, repoDir, nil, 1, "")
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.AttributedID = id
	if err := SaveGateRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}
	return key
}

// --- attribution filters (never reach RunVerify) ---------------------------

// --- RunVerify delegation (mapping table) ----------------------------------

// --- fail-closed: load errors and unknown verdicts -------------------------

// --- concurrency, isolation, restart durability ----------------------------

// --- unattributed (observe-only) mode --------------------------------------
//
// RunGateVerdictObserve is the `--unattributed` mode (change 0334, Task 4): NO
// key, NO record, NO writes. It re-syncs, verifies each supplied hint id (a hint
// is an id to verify, never attribution evidence) — or every current in-progress
// id when none are supplied — and renders one `gate-observe <verdict> <id>` line
// per id using RunVerify's verdict verbatim, through a SEPARATE render function
// that only knows the `gate-observe` prefix and is structurally unable to emit
// gate-retry-once.

// gateHaltedInProgressBlob builds an in-progress change carrying a durable
// "## Run halted" body section — status stays in-progress (so observeInProgress
// counts it) while RunVerify short-circuits it to run-halted with reader-only
// deps, letting the observe tests assert exact lines without the full git path.
func gateHaltedInProgressBlob(id int, slug string) StatusBlob {
	src := strings.TrimRight(lifecycleChange(id, slug, "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(src),
	}
}

// gateProposedBlob builds a proposed (never-claimed) change — RunVerify maps it
// to run-unclaimed with reader-only deps.
func gateProposedBlob(id int, slug string) StatusBlob {
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     []byte(lifecycleChange(id, slug, "proposed")),
	}
}

// --- gate-continue: nonterminal continuation + outer takeover (change 0359) ---
//
// VerdictRunWaiting no longer maps to a terminal gate-stop: the outer gate emits
// a nonterminal gate-continue that keeps the SAME key and spends NO retry, and a
// run-incomplete that still owns a tracked drive under this dispatch's recovery
// scope is taken over (event-authorized) and continued BEFORE the retry permit is
// ever reached. The tests below fake the ContinuationSeam (the drive-layer surface
// the verdict path needs) and, for the takeover path, a stateful waiting reader
// that only starts reporting a handoff after the takeover synthesizes one — so the
// re-run of the UNCHANGED RunVerify predicate validates the continuation.

// fakeContinuationSeam fakes the drive-layer surface RunGateVerdict needs on the
// continuation path: outer-drive candidate resolution, the event-authorized
// takeover+handoff synthesis, and the existing-handoff read.
type fakeContinuationSeam struct {
	candidates    []string
	locateErr     error
	handoffToken  string
	existingErr   error
	takeoverHalt  bool
	takeoverCause string
	takeoverErr   error
	onTakeover    func()
	// bind* record the fresh-run defense-in-depth scope binding: the verdict path
	// binds the outer scope to the one attributed claim when attribution first
	// resolves it (spec §3), and never re-binds on a continuation.
	bindCalls    int
	bindScopeID  string
	bindChangeID int
	bindErr      error
}

func (s *fakeContinuationSeam) LocateOuterDrive(changeID int, childContextHash string) ([]string, error) {
	return s.candidates, s.locateErr
}

func (s *fakeContinuationSeam) TakeoverAndHandoff(scopeID, parentCap, driveID string) (string, bool, string, error) {
	if s.takeoverErr != nil {
		return "", false, "", s.takeoverErr
	}
	if s.takeoverHalt {
		return "", true, s.takeoverCause, nil
	}
	if s.onTakeover != nil {
		s.onTakeover()
	}
	return s.handoffToken, false, "", nil
}

func (s *fakeContinuationSeam) ExistingHandoffToken(driveID string) (string, error) {
	return s.handoffToken, s.existingErr
}

func (s *fakeContinuationSeam) BindScopeChange(scopeID string, changeID int) error {
	s.bindCalls++
	s.bindScopeID = scopeID
	s.bindChangeID = changeID
	return s.bindErr
}

// gatedWaitingReader is a stateful WaitingReceiptReader: it reports no waiting
// receipt until ready flips true (after an outer takeover synthesizes a handoff),
// then returns the fully-agreeing receipt. It models the production store, whose
// state changes between the first RunVerify (run-incomplete) and the re-run after
// the synthesized handoff (run-waiting).
type gatedWaitingReader struct {
	receipt WaitingReceipt
	ready   *bool
}

func (r gatedWaitingReader) Read(_ context.Context, _ string, _ int) (WaitingReceipt, bool, error) {
	if r.ready == nil || !*r.ready {
		return WaitingReceipt{}, false, nil
	}
	return r.receipt, true, nil
}

// gateMintArmedScoped mints an armed record carrying the outer recovery-scope
// binding (ScopeID/ParentCap/ChildContextHash) gate-before stamps for a dispatched
// implement-next run, so the verdict path's outer-takeover branch is reachable.
func gateMintArmedScoped(t *testing.T, repoDir, scopeID, parentCap, childContextHash string) string {
	t.Helper()
	key := gateMintArmed(t, repoDir, nil, 1, "")
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.ScopeID = scopeID
	rec.ParentCap = parentCap
	rec.ChildContextHash = childContextHash
	if err := SaveGateRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}
	return key
}

// gateMintAttributedScoped mints a scoped record already attributed to id — the
// resume-verified shape (AttributedID set, BoundRequestID empty) a continuation of
// a scoped dispatch reads. resolveGateOwnership returns immediately for this shape
// (change 0407: continuity for a resume-bound id is RunVerify's job), so it reaches
// the unchanged continuation and retry paths exactly as a first verdict's
// attribution used to.
func gateMintAttributedScoped(t *testing.T, repoDir, scopeID, parentCap, childContextHash string, id int) string {
	t.Helper()
	key := gateMintArmedScoped(t, repoDir, scopeID, parentCap, childContextHash)
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.AttributedID = id
	if err := SaveGateRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}
	return key
}

// gateMintAttributedLimit mints an UNSCOPED attributed record (AttributedID set,
// no ScopeID) whose AttemptLimit is limit — the shape a quiescent run-incomplete
// verdict reaches straight through to the retry CAS (ScopeID == "" skips the
// continuation check). limit must be >= 1 (SaveGateRecord's v4 floor). It lets the
// counted-budget tests drive successive eligible incompletes at a chosen limit.
func gateMintAttributedLimit(t *testing.T, repoDir string, id, limit int) string {
	t.Helper()
	key := gateMintArmed(t, repoDir, nil, 1, "")
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.AttributedID = id
	rec.AttemptLimit = limit
	if err := SaveGateRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}
	return key
}

// gateMintAttributedScopedLimit is gateMintAttributedScoped with the AttemptLimit
// overridden to limit (>= 1) — a scoped, attributed record used by the
// continuation-vs-budget tests.
func gateMintAttributedScopedLimit(t *testing.T, repoDir, scopeID, parentCap, childContextHash string, id, limit int) string {
	t.Helper()
	key := gateMintAttributedScoped(t, repoDir, scopeID, parentCap, childContextHash, id)
	rec, err := LoadGateRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.AttemptLimit = limit
	if err := SaveGateRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}
	return key
}

// TestVerdictIncompleteRespectsAttemptLimit is the counted-budget heart (change
// 0421): a quiescent run-incomplete grants at most AttemptLimit-1 gate-retry-once,
// each on a distinct attempt transition, then a terminal gate-stop. limit 1 grants
// none; limit 2 grants one; limit 4 grants exactly three. The report TOKENS are
// unchanged (gate-retry-once / gate-stop … run-incomplete <id> <unmet>); the
// used/limit surface is the additive AttemptsUsed/AttemptLimit result fields.
func TestVerdictIncompleteRespectsAttemptLimit(t *testing.T) {
	cases := []struct {
		limit       int
		wantRetries int
	}{
		{limit: 1, wantRetries: 0},
		{limit: 2, wantRetries: 1},
		{limit: 4, wantRetries: 3},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("limit-%d", tc.limit), func(t *testing.T) {
			f := newRunVerifyFixture(t, true)
			deps, wdeps, gdeps := f.deps(
				gateIncompleteRecord(),
				rvPR(f.head, string(prEvidenceBytes(t, f.head))),
			)
			key := gateMintAttributedLimit(t, f.repo.invocation, 3, tc.limit)

			for i := 1; i <= tc.wantRetries; i++ {
				res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
				if res.Decision != GateDecisionRetryOnce || res.Terminal {
					t.Fatalf("call %d: Decision=%q Terminal=%v, want gate-retry-once/false", i, res.Decision, res.Terminal)
				}
				if got, want := res.HumanText(), "gate-retry-once "+key+" run-incomplete 3 not-implemented"; got != want {
					t.Fatalf("call %d: HumanText = %q, want %q", i, got, want)
				}
				if res.AttemptsUsed != i || res.AttemptLimit != tc.limit {
					t.Errorf("call %d: attempts %d/%d, want %d/%d", i, res.AttemptsUsed, res.AttemptLimit, i, tc.limit)
				}
			}

			// The next eligible incomplete is the terminal gate-stop: budget exhausted.
			res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
			if res.Decision != GateDecisionStop || !res.Terminal {
				t.Fatalf("terminal: Decision=%q Terminal=%v, want gate-stop/true", res.Decision, res.Terminal)
			}
			if got, want := res.HumanText(), "gate-stop "+key+" run-incomplete 3 not-implemented"; got != want {
				t.Fatalf("terminal HumanText = %q, want %q", got, want)
			}
			if res.AttemptsUsed != tc.limit || res.AttemptLimit != tc.limit {
				t.Errorf("terminal attempts %d/%d, want %d/%d (exhausted)", res.AttemptsUsed, res.AttemptLimit, tc.limit, tc.limit)
			}
			if used, err := GateRetryUsage(f.repo.invocation, key); err != nil || used != tc.wantRetries {
				t.Errorf("GateRetryUsage = %d,%v; want %d,nil", used, err, tc.wantRetries)
			}
		})
	}
}

// TestVerdictIncompleteRepeatObservationDoesNotDoubleGrant: after a gate-retry-once
// for attempt 1 (default limit 2), a second verdict call WITHOUT a new attempt
// completing is the terminal gate-stop — the budget is spent — and GateRetryUsage
// stays 1 (no second marker).
func TestVerdictIncompleteRepeatObservationDoesNotDoubleGrant(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintAttributedLimit(t, f.repo.invocation, 3, 2)

	res1 := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res1.Decision != GateDecisionRetryOnce {
		t.Fatalf("first: Decision = %q, want gate-retry-once", res1.Decision)
	}
	res2 := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res2.Decision != GateDecisionStop || !res2.Terminal {
		t.Fatalf("repeat: Decision=%q Terminal=%v, want gate-stop/true", res2.Decision, res2.Terminal)
	}
	if used, err := GateRetryUsage(f.repo.invocation, key); err != nil || used != 1 {
		t.Errorf("GateRetryUsage = %d,%v; want 1,nil (no double grant)", used, err)
	}
}

// TestVerdictIncompleteNoGrantLeavesRetryMirrorUnused: an immediate-exhaustion
// (limit 1) run-incomplete stops terminally with no retry granted and no marker
// created, so the persisted GateRecord's readable Retry mirror must stay
// RetryUnused — nothing was consumed. LoadGateRecord only upgrades the mirror from
// markers and never downgrades, so a mirror set to RetryConsumed on a no-grant stop
// would permanently misreport "consumed" though GateRetryUsage == 0. This reddens
// if the mirror is set before branching on `granted`.
func TestVerdictIncompleteNoGrantLeavesRetryMirrorUnused(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintAttributedLimit(t, f.repo.invocation, 3, 1)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != GateDecisionStop || !res.Terminal {
		t.Fatalf("Decision=%q Terminal=%v, want gate-stop/true (limit 1 grants no retry)", res.Decision, res.Terminal)
	}
	if used, err := GateRetryUsage(f.repo.invocation, key); err != nil || used != 0 {
		t.Fatalf("GateRetryUsage = %d,%v; want 0,nil (no marker on a no-grant stop)", used, err)
	}
	rec, err := LoadGateRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.Retry != RetryUnused {
		t.Errorf("persisted Retry mirror = %v, want RetryUnused (nothing was consumed)", rec.Retry)
	}
}

// TestVerdictHaltPrecedenceOverBudget: a run-halted verdict against a fresh,
// unspent limit-4 record stops terminally (gate-stop run-halted) and spends NO
// attempt — run-halted keeps absolute precedence ahead of any counting, and the
// attempts surface stays absent on the halt path.
func TestVerdictHaltPrecedenceOverBudget(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{gateHaltedInProgressBlob(3, rvSlug)})
	key := gateMintAttributedLimit(t, repo, 3, 4)

	res := RunGateVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if res.Decision != GateDecisionStop || res.Outcome != VerdictRunHalted {
		t.Fatalf("Decision/Outcome = %q/%q, want gate-stop/run-halted", res.Decision, res.Outcome)
	}
	if !res.Terminal {
		t.Errorf("run-halted stop must be terminal")
	}
	if used, err := GateRetryUsage(repo, key); err != nil || used != 0 {
		t.Errorf("GateRetryUsage = %d,%v; want 0,nil (halt precedes counting)", used, err)
	}
	if res.AttemptsUsed != 0 || res.AttemptLimit != 0 {
		t.Errorf("halt path surfaced attempts %d/%d, want 0/0 (no counting)", res.AttemptsUsed, res.AttemptLimit)
	}
}

// TestVerdictContinuationConsumesNoAttempt: a scope-bound run-incomplete taken over
// as a live continuation reaches gate-continue WITHOUT touching the retry CAS —
// GateRetryUsage stays 0 even with a fresh limit-4 budget. This reddens if the CAS
// is ever moved above the outer-takeover check.
func TestVerdictContinuationConsumesNoAttempt(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	tookOver := false
	reader := gatedWaitingReader{receipt: rvAgreeingReceipt(f.head), ready: &tookOver}
	deps, wdeps, gdeps := rvWaitingDeps(t, f, reader)
	wdeps.Continuation = &fakeContinuationSeam{
		candidates:   []string{"d0opaque"},
		handoffToken: "h0token",
		onTakeover:   func() { tookOver = true },
	}
	key := gateMintAttributedScopedLimit(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3, 4)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != GateDecisionContinue {
		t.Fatalf("Decision = %q, want %q", res.Decision, GateDecisionContinue)
	}
	if used, err := GateRetryUsage(f.repo.invocation, key); err != nil || used != 0 {
		t.Errorf("GateRetryUsage = %d,%v; want 0,nil (a continuation spends no attempt)", used, err)
	}
}

// fakeProofScanner is the injected ClaimProofScanner for the verdict path's
// ownership resolution (change 0407): it returns canned proofs newest-first, or an
// error. A nil scanner (not this fake) is the fail-closed proof-unavailable case.
type fakeProofScanner struct {
	proofs []ClaimProof
	err    error
}

func (f *fakeProofScanner) ScanClaimProofs(context.Context, string) ([]ClaimProof, error) {
	return f.proofs, f.err
}

// gateRetryMarkerExists reports whether the O_EXCL retry marker for key exists on
// disk. It reads the FILESYSTEM (never a mock), so a continuation that must never
// reach the retry CAS is a real, provable property.
func gateRetryMarkerExists(t *testing.T, repoDir, key string) bool {
	t.Helper()
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	_, serr := os.Stat(filepath.Join(common, "docket", "rungate", key, gateRetryMarkerName))
	if serr != nil && !os.IsNotExist(serr) {
		t.Fatalf("stat retry marker: %v", serr)
	}
	return serr == nil
}

// TestVerdictWaitingIsNonterminalContinue: a RunVerify run-waiting (a worker
// cooperatively handed off) maps to a NONTERMINAL gate-continue that keeps the key
// and spends no retry, minting a continuation id and persisting the triple.
func TestVerdictWaitingIsNonterminalContinue(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true})
	wdeps.Continuation = &fakeContinuationSeam{handoffToken: "h0token"}
	// Resume-verified shape (AttributedID set, no binding): ownership resolution
	// returns immediately and the run-waiting continuation maps unchanged.
	key := gateMintAttributed(t, f.repo.invocation, 3)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != GateDecisionContinue {
		t.Fatalf("Decision = %q, want %q", res.Decision, GateDecisionContinue)
	}
	if res.Terminal {
		t.Errorf("gate-continue must be nonterminal")
	}
	if res.ContinuationID == "" {
		t.Fatalf("continuation id must be minted")
	}
	want := "gate-continue " + key + " run-waiting 3 " + res.ContinuationID + " build"
	if got := res.HumanText(); got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if gateRetryMarkerExists(t, f.repo.invocation, key) {
		t.Errorf("retry marker present — a continuation must not spend the retry")
	}
	rec, err := LoadGateRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.ContinuationID != res.ContinuationID || rec.ContinuationDrive != "d0opaque" || rec.ContinuationHandoff != "h0token" {
		t.Errorf("triple = {%q,%q,%q}, want {%q,d0opaque,h0token}",
			rec.ContinuationID, rec.ContinuationDrive, rec.ContinuationHandoff, res.ContinuationID)
	}
	if rec.Retry != RetryUnused {
		t.Errorf("Retry = %q, want unused (continue preserves the retry)", rec.Retry)
	}
}

// TestVerdictIncompleteWithTrackedDriveContinuesWithoutRetry: a run-incomplete
// whose recovery scope still binds a tracked drive is TAKEN OVER and continued,
// and the O_EXCL retry marker is NEVER created — asserted on the filesystem, so
// the "cannot reach the retry CAS" ordering property is real. This is the mutation
// target for Task 6 Step 3 (moving ConsumeGateRetry above the tracked-drive check
// creates the marker and reddens this test).
func TestVerdictIncompleteWithTrackedDriveContinuesWithoutRetry(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	tookOver := false
	reader := gatedWaitingReader{receipt: rvAgreeingReceipt(f.head), ready: &tookOver}
	deps, wdeps, gdeps := rvWaitingDeps(t, f, reader)
	wdeps.Continuation = &fakeContinuationSeam{
		candidates:   []string{"d0opaque"},
		handoffToken: "h0token",
		onTakeover:   func() { tookOver = true },
	}
	key := gateMintAttributedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != GateDecisionContinue {
		t.Fatalf("Decision = %q, want %q", res.Decision, GateDecisionContinue)
	}
	if res.Terminal {
		t.Errorf("gate-continue must be nonterminal")
	}
	if gateRetryMarkerExists(t, f.repo.invocation, key) {
		t.Fatalf("retry marker present — a takeover-continue must not reach the retry CAS")
	}
	rec, err := LoadGateRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.ContinuationID == "" || rec.ContinuationDrive != "d0opaque" || rec.ContinuationHandoff != "h0token" {
		t.Errorf("triple = {%q,%q,%q}, want {non-empty,d0opaque,h0token}",
			rec.ContinuationID, rec.ContinuationDrive, rec.ContinuationHandoff)
	}
	if rec.Retry != RetryUnused {
		t.Errorf("Retry = %q, want unused (takeover-continue preserves the retry)", rec.Retry)
	}
}

// TestVerdictIncompleteQuiescentStillRetriesOnce: a run-incomplete with a scope
// but ZERO tracked-drive candidates is genuinely quiescent — it falls through to
// the unchanged retry path (gate-retry-once, then terminal gate-stop).
func TestVerdictIncompleteQuiescentStillRetriesOnce(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Continuation = &fakeContinuationSeam{candidates: nil} // zero candidates
	key := gateMintAttributedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3)

	res1 := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res1.HumanText(), "gate-retry-once "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("first call HumanText = %q, want %q", got, want)
	}
	if res1.Terminal {
		t.Errorf("gate-retry-once must be nonterminal")
	}

	res2 := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res2.HumanText(), "gate-stop "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("second call HumanText = %q, want %q", got, want)
	}
	if !res2.Terminal {
		t.Errorf("exhausted gate-stop must be terminal")
	}
}

// TestVerdictAmbiguousDrivesStops: more than one candidate tracked drive is
// unsafe ownership — it earns neither retry nor continuation, stopping terminally
// with gate-unavailable takeover-ambiguous and never touching the retry marker.
func TestVerdictAmbiguousDrivesStops(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Continuation = &fakeContinuationSeam{candidates: []string{"a", "b"}}
	key := gateMintAttributedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != GateDecisionStop || res.Outcome != GateOutcomeUnavailable {
		t.Fatalf("decision/outcome = %q/%q, want gate-stop/gate-unavailable", res.Decision, res.Outcome)
	}
	if res.Reason != "takeover-ambiguous" {
		t.Errorf("reason = %q, want takeover-ambiguous", res.Reason)
	}
	if !res.Terminal {
		t.Errorf("ambiguous-drives stop must be terminal")
	}
	if gateRetryMarkerExists(t, f.repo.invocation, key) {
		t.Errorf("ambiguous drives must never spend the retry")
	}
}

// TestVerdictTakeoverHaltStops: a takeover that HALTs (unsafe ownership) stops
// terminally with the driver's cause and never spends the retry.
func TestVerdictTakeoverHaltStops(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Continuation = &fakeContinuationSeam{
		candidates:    []string{"d0opaque"},
		takeoverHalt:  true,
		takeoverCause: "identity-mismatch",
	}
	key := gateMintAttributedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != GateDecisionStop || res.Outcome != GateOutcomeUnavailable {
		t.Fatalf("decision/outcome = %q/%q, want gate-stop/gate-unavailable", res.Decision, res.Outcome)
	}
	if res.Reason != "identity-mismatch" {
		t.Errorf("reason = %q, want identity-mismatch (driver cause passed through)", res.Reason)
	}
	if !res.Terminal {
		t.Errorf("halted takeover must be terminal")
	}
	if gateRetryMarkerExists(t, f.repo.invocation, key) {
		t.Errorf("halted takeover must never spend the retry")
	}
}

// TestVerdictContinueNeverAuthorizesNewClaim: the continuation path leaves
// attribution untouched — an already-attributed record's AttributedID is unchanged.
func TestVerdictContinueNeverAuthorizesNewClaim(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true})
	wdeps.Continuation = &fakeContinuationSeam{handoffToken: "h0token"}
	key := gateMintAttributed(t, f.repo.invocation, 3)

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != GateDecisionContinue {
		t.Fatalf("Decision = %q, want %q", res.Decision, GateDecisionContinue)
	}
	if res.AttributedID != 3 {
		t.Errorf("AttributedID = %d, want 3 (attribution untouched on continue)", res.AttributedID)
	}
	rec, err := LoadGateRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.AttributedID != 3 {
		t.Errorf("record AttributedID = %d, want 3", rec.AttributedID)
	}
}

// TestVerdictFreshRunBindsScopeChange: on a FRESH run (no pre-attributed id), when
// ownership resolution adopts the sole matching claim proof the verdict path binds
// that change id into the outer recovery scope (spec §3 defense-in-depth) so a later
// outer takeover's scopeIdentityMatch pins the change rather than skipping it on an
// empty scope field. Mutation target: dropping the BindScopeChange call at the
// adoption point reddens the bindCalls assertion below.
func TestVerdictFreshRunBindsScopeChange(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	seam := &fakeContinuationSeam{} // no candidates: ownership adopts the sole proof,
	// then the run-incomplete path finds zero tracked drives and takes the ordinary
	// retry route — but the bind already fired at the adoption point.
	wdeps.Continuation = seam
	// One committed proof matches the record's context hash, so the no-binding branch
	// adopts it and mirrors the change id onto the still-lagging record (AttributedID
	// 0 → 3), firing the scope bind.
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ctxhash-1", Revision: "r1"},
	}}
	// A scoped, unattributed record: ScopeID present, AttributedID == 0 (fresh run).
	key := gateMintArmedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1")

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.AttributedID != 3 {
		t.Fatalf("AttributedID = %d, want 3 (fresh attribution resolved the sole claim)", res.AttributedID)
	}
	if seam.bindCalls != 1 {
		t.Fatalf("BindScopeChange called %d times, want exactly 1 on a fresh-run attribution", seam.bindCalls)
	}
	if seam.bindScopeID != "scope-1" || seam.bindChangeID != 3 {
		t.Errorf("bound (%q, %d), want (scope-1, 3)", seam.bindScopeID, seam.bindChangeID)
	}
}

// TestVerdictContinuationDoesNotRebindScope: a continuation (an already-attributed
// record — the state a second gate-verdict call reads) skips attribution entirely,
// so it MUST NOT re-bind the outer scope's change id. This is the bind-once guard's
// other half: the fresh run binds, a continuation never touches it.
func TestVerdictContinuationDoesNotRebindScope(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true})
	seam := &fakeContinuationSeam{handoffToken: "h0token"}
	wdeps.Continuation = seam
	// Already attributed AND scoped: a continuation of the same attempt.
	key := gateMintArmedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1")
	rec, err := LoadGateRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	rec.AttributedID = 3
	if err := SaveGateRecord(f.repo.invocation, key, rec); err != nil {
		t.Fatalf("SaveGateRecord: %v", err)
	}

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != GateDecisionContinue {
		t.Fatalf("Decision = %q, want %q (a live continuation)", res.Decision, GateDecisionContinue)
	}
	if seam.bindCalls != 0 {
		t.Fatalf("BindScopeChange called %d times on a continuation, want 0 (bind-once: never re-bind)", seam.bindCalls)
	}
}

// TestVerdictObservePathStillCannotContinue: the observe (unattributed) render
// path is structurally unable to emit a retry OR a continuation — extending the
// existing observe-cannot-retry guarantee to gate-continue.
func TestVerdictObservePathStillCannotContinue(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		gateIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	res := RunGateVerdictObserve(context.Background(), deps, wdeps, gdeps, f.repo.invocation, []string{"3"})
	line := res.HumanText()
	if strings.HasPrefix(line, GateDecisionRetryOnce) {
		t.Errorf("observe must never emit %q; line = %q", GateDecisionRetryOnce, line)
	}
	if strings.Contains(line, GateDecisionContinue) {
		t.Errorf("observe must never emit %q; line = %q", GateDecisionContinue, line)
	}
	if !strings.HasPrefix(line, GateDecisionObserve) {
		t.Errorf("observe line must start with %q; got %q", GateDecisionObserve, line)
	}
}

// --- ownership from proof, never inference (change 0407) --------------------
//
// resolveGateOwnership replaces the before-set/cardinality attribution with the
// verified dispatch-to-claim binding: a confirmed binding's continuity is checked
// against committed proofs, an unconfirmed reservation recovers only from its exact
// receipt, an absent binding adopts the sole matching proof, and every missing /
// conflicting / unprovable case fails CLOSED to a non-authorizing report. The
// before-set and dispatch epoch are diagnostics only and can never grant a retry.

// TestVerdictConfirmedBindingResolvesBoundChange: a confirmed binding whose newest
// proof for the id matches the bound request id resolves the bound change and
// delegates unchanged to RunVerify — a completed run reports run-complete even
// though the claim left the change in-progress.
func TestVerdictConfirmedBindingResolvesBoundChange(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintArmed(t, f.repo.invocation, nil, 1, "ha")
	if err := ReserveGateClaim(f.repo.invocation, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmGateClaim(f.repo.invocation, key, 3, "claim-3-v", "r1"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
	}}

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if res.AttributedID != 3 {
		t.Errorf("AttributedID = %d, want 3 (bound change resolved)", res.AttributedID)
	}
}

// TestVerdictNoBindingNoProofIsNoAttributableClaim: a dispatch that claims nothing
// resolves to no-attributable-claim and never acquires a sibling's claim — a proof
// under a DIFFERENT context hash is filtered out, the retry is never spent, and the
// sibling id never appears in the report.
func TestVerdictNoBindingNoProofIsNoAttributableClaim(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1, "ha")
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "x", ChangeID: 9, GateContextHash: "OTHER"},
	}}}

	res := RunGateVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if !res.Terminal {
		t.Errorf("no-attributable-claim is terminal")
	}
	if res.AttributedID != 0 {
		t.Errorf("AttributedID = %d, want 0 (nothing adopted)", res.AttributedID)
	}
	if len(res.AmbiguousIDs) != 0 {
		t.Errorf("no id may be named on a no-attributable-claim; got AmbiguousIDs=%v", res.AmbiguousIDs)
	}
	if gateRetryMarkerExists(t, repo, key) {
		t.Errorf("no-attributable-claim must never spend the retry")
	}
}

// TestVerdictUnconfirmedReservationRecoversFromExactReceipt: a reservation whose
// confirm was interrupted recovers from the exact committed receipt (same request
// id, same context hash), confirms the binding, and delegates to the bound id.
func TestVerdictUnconfirmedReservationRecoversFromExactReceipt(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintArmed(t, f.repo.invocation, nil, 1, "ha")
	if err := ReserveGateClaim(f.repo.invocation, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
	}}

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (recovered from the exact receipt)", got, want)
	}
	b, ok, err := LoadGateClaimBinding(f.repo.invocation, key)
	if err != nil || !ok || !b.Confirmed || b.ChangeID != 3 {
		t.Fatalf("binding = %+v ok=%v err=%v, want confirmed change 3", b, ok, err)
	}
}

// TestVerdictUnconfirmedReservationWithoutReceiptStops: an unconfirmed reservation
// with no matching committed receipt never became a real claim — the verdict is
// no-attributable-claim and the reservation is left refused (never released so a
// different claim can take it, never confirmed).
func TestVerdictUnconfirmedReservationWithoutReceiptStops(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1, "ha")
	if err := ReserveGateClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: nil}}

	res := RunGateVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	b, ok, err := LoadGateClaimBinding(repo, key)
	if err != nil || !ok || b.Confirmed || b.ChangeID != 3 || b.RequestID != "claim-3-v" {
		t.Fatalf("binding = %+v ok=%v err=%v, want the unconfirmed reservation intact", b, ok, err)
	}
}

// TestVerdictUnconfirmedReservationSiblingContextHashIsNoAttributableClaim: an
// unconfirmed reservation whose only committed proof shares the request id but
// carries a DIFFERENT context hash is a sibling collision, not this dispatch's
// receipt. gateProofForClaim's `&& p.GateContextHash == contextHash` clause must
// reject it, so the verdict is no-attributable-claim and nothing is adopted.
// Dropping that clause reddens this test (mutation-load-bearing) — neither
// RecoversFromExactReceipt (matching hash) nor WithoutReceiptStops (no proofs)
// exercises the same-request-id sibling.
func TestVerdictUnconfirmedReservationSiblingContextHashIsNoAttributableClaim(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1, "ha")
	if err := ReserveGateClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "hb"},
	}}}

	res := RunGateVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q (sibling context hash must not recover)", got, want)
	}
	if res.AttributedID != 0 {
		t.Errorf("AttributedID = %d, want 0 (the sibling proof must never be attributed)", res.AttributedID)
	}
}

// TestVerdictAbsentBindingAdoptsSoleProof: with no binding file at all, a single
// committed proof matching the record's context hash is adopted (reserved,
// confirmed, mirrored) and delegation proceeds; two matching proofs are unsafe
// ownership and fail closed to binding-conflict.
func TestVerdictAbsentBindingAdoptsSoleProof(t *testing.T) {
	t.Run("sole proof adopted", func(t *testing.T) {
		f := newRunVerifyFixture(t, true)
		deps, wdeps, gdeps := f.deps(
			rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
			rvPR(f.head, string(prEvidenceBytes(t, f.head))),
		)
		key := gateMintArmed(t, f.repo.invocation, nil, 1, "ha")
		wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
			{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
		}}

		res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
		if got, want := res.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		b, ok, err := LoadGateClaimBinding(f.repo.invocation, key)
		if err != nil || !ok || !b.Confirmed || b.ChangeID != 3 {
			t.Fatalf("binding = %+v ok=%v err=%v, want confirmed change 3 after adoption", b, ok, err)
		}
	})

	t.Run("two proofs is binding-conflict", func(t *testing.T) {
		repo := newGateRepo(t)
		key := gateMintArmed(t, repo, nil, 1, "ha")
		wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
			{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
			{RequestID: "claim-4-v", ChangeID: 4, GateContextHash: "ha", Revision: "r2"},
		}}}

		res := RunGateVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
		if res.Decision != GateDecisionStop || res.Outcome != GateOutcomeUnavailable || res.Reason != ReasonGateBindingConflict {
			t.Fatalf("got %q/%q/%q, want gate-stop/gate-unavailable/%s", res.Decision, res.Outcome, res.Reason, ReasonGateBindingConflict)
		}
		if !res.Terminal {
			t.Errorf("binding-conflict is terminal")
		}
	})
}

// TestVerdictClaimReplacedStops: a confirmed binding whose bound change carries a
// NEWER proof under a different request id means the change was reclaimed and
// re-claimed by another run — the old gate must neither retry nor take over the
// replacement. The binding is left unchanged and the retry is never spent.
func TestVerdictClaimReplacedStops(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1, "ha")
	if err := ReserveGateClaim(repo, key, 3, "claim-3-v1"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-v1", "r1"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v2", ChangeID: 3, GateContextHash: ""},
		{RequestID: "claim-3-v1", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
	}}}

	res := RunGateVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if res.Decision != GateDecisionStop || res.Outcome != GateOutcomeUnavailable || res.Reason != ReasonGateClaimReplaced {
		t.Fatalf("got %q/%q/%q, want gate-stop/gate-unavailable/%s", res.Decision, res.Outcome, res.Reason, ReasonGateClaimReplaced)
	}
	if gateRetryMarkerExists(t, repo, key) {
		t.Errorf("a replaced claim must never spend the retry")
	}
	b, _, err := LoadGateClaimBinding(repo, key)
	if err != nil {
		t.Fatalf("load binding: %v", err)
	}
	if b.RequestID != "claim-3-v1" || !b.Confirmed || b.Revision != "r1" {
		t.Errorf("binding = %+v, want the original confirmed claim-3-v1@r1 unchanged", b)
	}
}

// TestVerdictNilProofScannerFailsClosed: a claim-bound gate with no proof access
// cannot verify continuity — unlike the continuation seam, ownership can never
// proceed without proofs, so it fails closed to proof-unavailable.
func TestVerdictNilProofScannerFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1, "ha")
	if err := ReserveGateClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-v", "r1"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{ClaimProofs: nil}, GitHubDeps{}, repo, key)
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateProofUnavailable {
		t.Fatalf("got %q/%q, want gate-stop/%s", res.Decision, res.Reason, ReasonGateProofUnavailable)
	}
	if gateRetryMarkerExists(t, repo, key) {
		t.Errorf("proof-unavailable must never spend the retry")
	}
}

// TestVerdictProofScanErrorFailsClosed: a proof-scan error fails closed to
// proof-unavailable and never consumes a retry.
func TestVerdictProofScanErrorFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1, "ha")
	if err := ReserveGateClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-v", "r1"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{err: errors.New("boom")}}
	res := RunGateVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateProofUnavailable {
		t.Fatalf("got %q/%q, want gate-stop/%s", res.Decision, res.Reason, ReasonGateProofUnavailable)
	}
	if gateRetryMarkerExists(t, repo, key) {
		t.Errorf("a scan error must never spend the retry")
	}
}

// TestVerdictCorruptBindingFailsClosed: an unparseable binding file is a typed
// binding-unreadable stop — never a silent (ok=false) fall-through to attribution.
func TestVerdictCorruptBindingFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1, "ha")
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(common, "docket", "rungate", key, gateClaimBindingName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res := RunGateVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{ClaimProofs: &fakeProofScanner{}}, GitHubDeps{}, repo, key)
	if res.Decision != GateDecisionStop || res.Reason != ReasonGateBindingUnreadable {
		t.Fatalf("got %q/%q, want gate-stop/%s", res.Decision, res.Reason, ReasonGateBindingUnreadable)
	}
}

// TestVerdictResumeBindingSkipsContinuity: a gate-before --resume record
// (AttributedID set, BoundRequestID empty) is pre-bound by verified identity — the
// continuity check never runs, so a scanner that WOULD report a replacement still
// delegates to RunVerify (preserved verified-resume behavior).
func TestVerdictResumeBindingSkipsContinuity(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := gateMintAttributed(t, f.repo.invocation, 3)
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v2", ChangeID: 3, GateContextHash: ""},
	}}

	res := RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (resume-bound id delegates, never claim-replaced)", got, want)
	}
	if res.Reason == ReasonGateClaimReplaced {
		t.Errorf("a resume-bound verdict must never stop on continuity")
	}
}

// TestVerdictOwnershipIgnoresBeforeSetAndEpoch pins the 0407 defect: a sibling
// in-progress claim the OLD before-set/epoch filters would have attributed sits in
// the corpus, but ownership reads only committed proofs. With no proof carrying the
// record's context hash, the verdict is no-attributable-claim — never the sibling.
// Mutation partner: re-introducing epoch/before-set inference reddens exactly this.
func TestVerdictOwnershipIgnoresBeforeSetAndEpoch(t *testing.T) {
	repo := newGateRepo(t)
	deps := gateLightDeps(t, []StatusBlob{gateInProgressBlob(9, "sibling", "keep")})
	key := gateMintArmed(t, repo, nil, 1, "ha")
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "sib", ChangeID: 9, GateContextHash: "OTHER"},
	}}}

	res := RunGateVerdict(context.Background(), deps, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "gate-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q (never the sibling)", got, want)
	}
	if res.AttributedID != 0 {
		t.Errorf("AttributedID = %d, want 0 (the sibling id 9 must never be attributed)", res.AttributedID)
	}
	if len(res.AmbiguousIDs) != 0 {
		t.Errorf("no id may be named; got AmbiguousIDs=%v", res.AmbiguousIDs)
	}
}
