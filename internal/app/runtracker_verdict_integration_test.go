//go:build integration

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the `docket run verdict <key>` (attributed mode) tests (change
// 0334, Task 3). run verdict loads the durable gate record, attributes exactly
// one new in-progress claim through the three filters (id not in the before-set;
// claimed_at parses; claimed_at >= dispatch time), then delegates the run
// predicate to RunVerify and maps its verdict onto the attributed vocabulary —
// consuming the single retry permit BEFORE emitting so a lost retry is the safe
// failure, and failing closed to run-tracker-unavailable on any load error or
// unrecognized verdict. Every outcome is a report line that exits 0.
//
// The store is rooted at a real temp git repo (newRunTrackerRepo / the run-verify
// fixture's invocation clone); attribution reads the fakeReader corpus; the
// RunVerify delegation is driven by the run_verify_test.go fixtures (rvFixture,
// rvRecord, rvInProgressRecord, rvPR, rvAgreeingReceipt).

// runTrackerClaimUnix is the Unix epoch of runTrackerDefaultClaimedAt — the claim instant
// lifecycleChange stamps on an in-progress record. Attribution filter (c)
// compares a candidate's claimed_at against the record's DispatchedAt, so tests
// straddle this value.
func runTrackerClaimUnix(t *testing.T) int64 {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, runTrackerDefaultClaimedAt)
	if err != nil {
		t.Fatalf("parse %q: %v", runTrackerDefaultClaimedAt, err)
	}
	return tm.Unix()
}

// runTrackerLightDeps wires a planning-only deps set over the given corpus (no git
// client, no workspace/github seams) for attribution paths that never reach
// RunVerify.
func runTrackerLightDeps(t *testing.T, corpus []StatusBlob) PlanningDeps {
	t.Helper()
	return PlanningDeps{Reader: &fakeReader{pin: mainPin(t), corpus: corpus}, Clock: testClock()}
}

// runTrackerMintAttributed mints a record already attributed to id — the state a second
// run verdict call reads after a first call attributed the claim.
func runTrackerMintAttributed(t *testing.T, repoDir string, id int) string {
	t.Helper()
	key := runTrackerMintStarted(t, repoDir, nil, 1, "")
	rec, err := LoadRunTrackerRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	rec.AttributedID = id
	if err := SaveRunTrackerRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveRunTrackerRecord: %v", err)
	}
	return key
}

// --- attribution filters (never reach RunVerify) ---------------------------

// --- RunVerify delegation (mapping table) ----------------------------------

// --- fail-closed: load errors and unknown verdicts -------------------------

// --- concurrency, isolation, restart durability ----------------------------

// --- unattributed (observe-only) mode --------------------------------------
//
// RunVerdictObserve is the `--unattributed` mode (change 0334, Task 4): NO
// key, NO record, NO writes. It re-syncs, verifies each supplied hint id (a hint
// is an id to verify, never attribution evidence) — or every current in-progress
// id when none are supplied — and renders one `run-observe <verdict> <id>` line
// per id using RunVerify's verdict verbatim, through a SEPARATE render function
// that only knows the `run-observe` prefix and is structurally unable to emit
// run-retry-once.

// runTrackerHaltedInProgressBlob builds an in-progress change carrying a durable
// "## Run halted" body section — status stays in-progress (so observeInProgress
// counts it) while RunVerify short-circuits it to run-halted with reader-only
// deps, letting the observe tests assert exact lines without the full git path.
func runTrackerHaltedInProgressBlob(id int, slug string) StatusBlob {
	src := strings.TrimRight(lifecycleChange(id, slug, "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Revision: miRevision,
		Data:     []byte(src),
	}
}

// runTrackerProposedBlob builds a proposed (never-claimed) change — RunVerify maps it
// to run-unclaimed with reader-only deps.
func runTrackerProposedBlob(id int, slug string) StatusBlob {
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Revision: miRevision,
		Data:     []byte(lifecycleChange(id, slug, "proposed")),
	}
}

// --- run-continue: nonterminal continuation + outer takeover (change 0359) ---
//
// VerdictRunWaiting no longer maps to a terminal run-stop: the run tracker emits
// a nonterminal run-continue that keeps the SAME key and spends NO retry, and a
// run-incomplete that still owns a tracked drive under this dispatch's recovery
// scope is taken over (event-authorized) and continued BEFORE the retry permit is
// ever reached. The tests below fake the ContinuationSeam (the drive-layer surface
// the verdict path needs) and, for the takeover path, a stateful waiting reader
// that only starts reporting a handoff after the takeover synthesizes one — so the
// re-run of the UNCHANGED RunVerify predicate validates the continuation.

// fakeContinuationSeam fakes the drive-layer surface RunVerdict needs on the
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

// runTrackerMintStartedScoped mints a started record carrying the outer recovery-scope
// binding (ScopeID/ParentCap/ChildContextHash) run start stamps for a dispatched
// implement-next run, so the verdict path's outer-takeover branch is reachable.
func runTrackerMintStartedScoped(t *testing.T, repoDir, scopeID, parentCap, childContextHash string) string {
	t.Helper()
	key := runTrackerMintStarted(t, repoDir, nil, 1, "")
	rec, err := LoadRunTrackerRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	rec.ScopeID = scopeID
	rec.ParentCap = parentCap
	rec.ChildContextHash = childContextHash
	if err := SaveRunTrackerRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveRunTrackerRecord: %v", err)
	}
	return key
}

// runTrackerMintAttributedScoped mints a scoped record already attributed to id — the
// resume-verified shape (AttributedID set, BoundRequestID empty) a continuation of
// a scoped dispatch reads. resolveRunTrackerOwnership returns immediately for this shape
// (change 0407: continuity for a resume-bound id is RunVerify's job), so it reaches
// the unchanged continuation and retry paths exactly as a first verdict's
// attribution used to.
func runTrackerMintAttributedScoped(t *testing.T, repoDir, scopeID, parentCap, childContextHash string, id int) string {
	t.Helper()
	key := runTrackerMintStartedScoped(t, repoDir, scopeID, parentCap, childContextHash)
	rec, err := LoadRunTrackerRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	rec.AttributedID = id
	if err := SaveRunTrackerRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveRunTrackerRecord: %v", err)
	}
	return key
}

// runTrackerMintAttributedLimit mints an UNSCOPED attributed record (AttributedID set,
// no ScopeID) whose AttemptLimit is limit — the shape a quiescent run-incomplete
// verdict reaches straight through to the retry CAS (ScopeID == "" skips the
// continuation check). limit must be >= 1 (SaveRunTrackerRecord's v4 floor). It lets the
// counted-budget tests drive successive eligible incompletes at a chosen limit.
func runTrackerMintAttributedLimit(t *testing.T, repoDir string, id, limit int) string {
	t.Helper()
	key := runTrackerMintStarted(t, repoDir, nil, 1, "")
	rec, err := LoadRunTrackerRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	rec.AttributedID = id
	rec.AttemptLimit = limit
	if err := SaveRunTrackerRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveRunTrackerRecord: %v", err)
	}
	return key
}

// runTrackerMintAttributedScopedLimit is runTrackerMintAttributedScoped with the AttemptLimit
// overridden to limit (>= 1) — a scoped, attributed record used by the
// continuation-vs-budget tests.
func runTrackerMintAttributedScopedLimit(t *testing.T, repoDir, scopeID, parentCap, childContextHash string, id, limit int) string {
	t.Helper()
	key := runTrackerMintAttributedScoped(t, repoDir, scopeID, parentCap, childContextHash, id)
	rec, err := LoadRunTrackerRecord(repoDir, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	rec.AttemptLimit = limit
	if err := SaveRunTrackerRecord(repoDir, key, rec); err != nil {
		t.Fatalf("SaveRunTrackerRecord: %v", err)
	}
	return key
}

// TestIntegrationRunVerdictVerdictIncompleteRespectsAttemptLimit is the counted-budget heart (change
// 0421): a quiescent run-incomplete grants at most AttemptLimit-1 run-retry-once,
// each on a distinct attempt transition, then a terminal run-stop. limit 1 grants
// none; limit 2 grants one; limit 4 grants exactly three. The report TOKENS are
// unchanged (run-retry-once / run-stop … run-incomplete <id> <unmet>); the
// used/limit surface is the additive AttemptsUsed/AttemptLimit result fields.
func TestIntegrationRunVerdictVerdictIncompleteRespectsAttemptLimit(t *testing.T) {
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
				runTrackerIncompleteRecord(),
				rvPR(f.head, string(prEvidenceBytes(t, f.head))),
			)
			key := runTrackerMintAttributedLimit(t, f.repo.invocation, 3, tc.limit)

			for i := 1; i <= tc.wantRetries; i++ {
				res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
				if res.Decision != RunDecisionRetryOnce || res.Terminal {
					t.Fatalf("call %d: Decision=%q Terminal=%v, want run-retry-once/false", i, res.Decision, res.Terminal)
				}
				if got, want := res.HumanText(), "run-retry-once "+key+" run-incomplete 3 not-implemented"; got != want {
					t.Fatalf("call %d: HumanText = %q, want %q", i, got, want)
				}
				if res.AttemptsUsed != i || res.AttemptLimit != tc.limit {
					t.Errorf("call %d: attempts %d/%d, want %d/%d", i, res.AttemptsUsed, res.AttemptLimit, i, tc.limit)
				}
			}

			// The next eligible incomplete is the terminal run-stop: budget exhausted.
			res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
			if res.Decision != RunDecisionStop || !res.Terminal {
				t.Fatalf("terminal: Decision=%q Terminal=%v, want run-stop/true", res.Decision, res.Terminal)
			}
			if got, want := res.HumanText(), "run-stop "+key+" run-incomplete 3 not-implemented"; got != want {
				t.Fatalf("terminal HumanText = %q, want %q", got, want)
			}
			if res.AttemptsUsed != tc.limit || res.AttemptLimit != tc.limit {
				t.Errorf("terminal attempts %d/%d, want %d/%d (exhausted)", res.AttemptsUsed, res.AttemptLimit, tc.limit, tc.limit)
			}
			if used, err := RunTrackerRetryUsage(f.repo.invocation, key); err != nil || used != tc.wantRetries {
				t.Errorf("RunTrackerRetryUsage = %d,%v; want %d,nil", used, err, tc.wantRetries)
			}
		})
	}
}

// TestIntegrationRunVerdictVerdictIncompleteRepeatObservationDoesNotDoubleGrant: after a run-retry-once
// for attempt 1 (default limit 2), a second verdict call WITHOUT a new attempt
// completing is the terminal run-stop — the budget is spent — and RunTrackerRetryUsage
// stays 1 (no second marker).
func TestIntegrationRunVerdictVerdictIncompleteRepeatObservationDoesNotDoubleGrant(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		runTrackerIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := runTrackerMintAttributedLimit(t, f.repo.invocation, 3, 2)

	res1 := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res1.Decision != RunDecisionRetryOnce {
		t.Fatalf("first: Decision = %q, want run-retry-once", res1.Decision)
	}
	res2 := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res2.Decision != RunDecisionStop || !res2.Terminal {
		t.Fatalf("repeat: Decision=%q Terminal=%v, want run-stop/true", res2.Decision, res2.Terminal)
	}
	if used, err := RunTrackerRetryUsage(f.repo.invocation, key); err != nil || used != 1 {
		t.Errorf("RunTrackerRetryUsage = %d,%v; want 1,nil (no double grant)", used, err)
	}
}

// TestIntegrationRunVerdictVerdictIncompleteNoGrantLeavesRetryMirrorUnused: an immediate-exhaustion
// (limit 1) run-incomplete stops terminally with no retry granted and no marker
// created, so the persisted RunTrackerRecord's readable Retry mirror must stay
// RetryUnused — nothing was consumed. LoadRunTrackerRecord only upgrades the mirror from
// markers and never downgrades, so a mirror set to RetryConsumed on a no-grant stop
// would permanently misreport "consumed" though RunTrackerRetryUsage == 0. This reddens
// if the mirror is set before branching on `granted`.
func TestIntegrationRunVerdictVerdictIncompleteNoGrantLeavesRetryMirrorUnused(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		runTrackerIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := runTrackerMintAttributedLimit(t, f.repo.invocation, 3, 1)

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != RunDecisionStop || !res.Terminal {
		t.Fatalf("Decision=%q Terminal=%v, want run-stop/true (limit 1 grants no retry)", res.Decision, res.Terminal)
	}
	if used, err := RunTrackerRetryUsage(f.repo.invocation, key); err != nil || used != 0 {
		t.Fatalf("RunTrackerRetryUsage = %d,%v; want 0,nil (no marker on a no-grant stop)", used, err)
	}
	rec, err := LoadRunTrackerRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	if rec.Retry != RetryUnused {
		t.Errorf("persisted Retry mirror = %v, want RetryUnused (nothing was consumed)", rec.Retry)
	}
}

// TestIntegrationRunVerdictVerdictHaltPrecedenceOverBudget: a run-halted verdict against a fresh,
// unspent limit-4 record stops terminally (run-stop run-halted) and spends NO
// attempt — run-halted keeps absolute precedence ahead of any counting, and the
// attempts surface stays absent on the halt path.
func TestIntegrationRunVerdictVerdictHaltPrecedenceOverBudget(t *testing.T) {
	repo := newRunTrackerRepo(t)
	deps := runTrackerLightDeps(t, []StatusBlob{runTrackerHaltedInProgressBlob(3, rvSlug)})
	key := runTrackerMintAttributedLimit(t, repo, 3, 4)

	res := RunVerdict(context.Background(), deps, WorkspaceDeps{}, GitHubDeps{}, repo, key)
	if res.Decision != RunDecisionStop || res.Outcome != VerdictRunHalted {
		t.Fatalf("Decision/Outcome = %q/%q, want run-stop/run-halted", res.Decision, res.Outcome)
	}
	if !res.Terminal {
		t.Errorf("run-halted stop must be terminal")
	}
	if used, err := RunTrackerRetryUsage(repo, key); err != nil || used != 0 {
		t.Errorf("RunTrackerRetryUsage = %d,%v; want 0,nil (halt precedes counting)", used, err)
	}
	if res.AttemptsUsed != 0 || res.AttemptLimit != 0 {
		t.Errorf("halt path surfaced attempts %d/%d, want 0/0 (no counting)", res.AttemptsUsed, res.AttemptLimit)
	}
}

// TestIntegrationRunVerdictVerdictContinuationConsumesNoAttempt: a scope-bound run-incomplete taken over
// as a live continuation reaches run-continue WITHOUT touching the retry CAS —
// RunTrackerRetryUsage stays 0 even with a fresh limit-4 budget. This reddens if the CAS
// is ever moved above the outer-takeover check.
func TestIntegrationRunVerdictVerdictContinuationConsumesNoAttempt(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	tookOver := false
	reader := gatedWaitingReader{receipt: rvAgreeingReceipt(f.head), ready: &tookOver}
	deps, wdeps, gdeps := rvWaitingDeps(t, f, reader)
	wdeps.Continuation = &fakeContinuationSeam{
		candidates:   []string{"d0opaque"},
		handoffToken: "h0token",
		onTakeover:   func() { tookOver = true },
	}
	key := runTrackerMintAttributedScopedLimit(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3, 4)

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != RunDecisionContinue {
		t.Fatalf("Decision = %q, want %q", res.Decision, RunDecisionContinue)
	}
	if used, err := RunTrackerRetryUsage(f.repo.invocation, key); err != nil || used != 0 {
		t.Errorf("RunTrackerRetryUsage = %d,%v; want 0,nil (a continuation spends no attempt)", used, err)
	}
}

// TestIntegrationRunVerdictVerdictWaitingIsNonterminalContinue: a RunVerify run-waiting (a worker
// cooperatively handed off) maps to a NONTERMINAL run-continue that keeps the key
// and spends no retry, minting a continuation id and persisting the triple.
func TestIntegrationRunVerdictVerdictWaitingIsNonterminalContinue(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true})
	wdeps.Continuation = &fakeContinuationSeam{handoffToken: "h0token"}
	// Resume-verified shape (AttributedID set, no binding): ownership resolution
	// returns immediately and the run-waiting continuation maps unchanged.
	key := runTrackerMintAttributed(t, f.repo.invocation, 3)

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != RunDecisionContinue {
		t.Fatalf("Decision = %q, want %q", res.Decision, RunDecisionContinue)
	}
	if res.Terminal {
		t.Errorf("run-continue must be nonterminal")
	}
	if res.ContinuationID == "" {
		t.Fatalf("continuation id must be minted")
	}
	want := "run-continue " + key + " run-waiting 3 " + res.ContinuationID + " build"
	if got := res.HumanText(); got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if runTrackerRetryMarkerExists(t, f.repo.invocation, key) {
		t.Errorf("retry marker present — a continuation must not spend the retry")
	}
	rec, err := LoadRunTrackerRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	if rec.ContinuationID != res.ContinuationID || rec.ContinuationDrive != "d0opaque" || rec.ContinuationHandoff != "h0token" {
		t.Errorf("triple = {%q,%q,%q}, want {%q,d0opaque,h0token}",
			rec.ContinuationID, rec.ContinuationDrive, rec.ContinuationHandoff, res.ContinuationID)
	}
	if rec.Retry != RetryUnused {
		t.Errorf("Retry = %q, want unused (continue preserves the retry)", rec.Retry)
	}
}

// TestIntegrationRunVerdictVerdictIncompleteWithTrackedDriveContinuesWithoutRetry: a run-incomplete
// whose recovery scope still binds a tracked drive is TAKEN OVER and continued,
// and the O_EXCL retry marker is NEVER created — asserted on the filesystem, so
// the "cannot reach the retry CAS" ordering property is real. This is the mutation
// target for Task 6 Step 3 (moving ConsumeRunTrackerRetry above the tracked-drive check
// creates the marker and reddens this test).
func TestIntegrationRunVerdictVerdictIncompleteWithTrackedDriveContinuesWithoutRetry(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	tookOver := false
	reader := gatedWaitingReader{receipt: rvAgreeingReceipt(f.head), ready: &tookOver}
	deps, wdeps, gdeps := rvWaitingDeps(t, f, reader)
	wdeps.Continuation = &fakeContinuationSeam{
		candidates:   []string{"d0opaque"},
		handoffToken: "h0token",
		onTakeover:   func() { tookOver = true },
	}
	key := runTrackerMintAttributedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3)

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != RunDecisionContinue {
		t.Fatalf("Decision = %q, want %q", res.Decision, RunDecisionContinue)
	}
	if res.Terminal {
		t.Errorf("run-continue must be nonterminal")
	}
	if runTrackerRetryMarkerExists(t, f.repo.invocation, key) {
		t.Fatalf("retry marker present — a takeover-continue must not reach the retry CAS")
	}
	rec, err := LoadRunTrackerRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	if rec.ContinuationID == "" || rec.ContinuationDrive != "d0opaque" || rec.ContinuationHandoff != "h0token" {
		t.Errorf("triple = {%q,%q,%q}, want {non-empty,d0opaque,h0token}",
			rec.ContinuationID, rec.ContinuationDrive, rec.ContinuationHandoff)
	}
	if rec.Retry != RetryUnused {
		t.Errorf("Retry = %q, want unused (takeover-continue preserves the retry)", rec.Retry)
	}
}

// TestIntegrationRunVerdictVerdictIncompleteQuiescentStillRetriesOnce: a run-incomplete with a scope
// but ZERO tracked-drive candidates is genuinely quiescent — it falls through to
// the unchanged retry path (run-retry-once, then terminal run-stop).
func TestIntegrationRunVerdictVerdictIncompleteQuiescentStillRetriesOnce(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		runTrackerIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Continuation = &fakeContinuationSeam{candidates: nil} // zero candidates
	key := runTrackerMintAttributedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3)

	res1 := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res1.HumanText(), "run-retry-once "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("first call HumanText = %q, want %q", got, want)
	}
	if res1.Terminal {
		t.Errorf("run-retry-once must be nonterminal")
	}

	res2 := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res2.HumanText(), "run-stop "+key+" run-incomplete 3 not-implemented"; got != want {
		t.Fatalf("second call HumanText = %q, want %q", got, want)
	}
	if !res2.Terminal {
		t.Errorf("exhausted run-stop must be terminal")
	}
}

// TestIntegrationRunVerdictVerdictAmbiguousDrivesStops: more than one candidate tracked drive is
// unsafe ownership — it earns neither retry nor continuation, stopping terminally
// with run-tracker-unavailable takeover-ambiguous and never touching the retry marker.
func TestIntegrationRunVerdictVerdictAmbiguousDrivesStops(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		runTrackerIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Continuation = &fakeContinuationSeam{candidates: []string{"a", "b"}}
	key := runTrackerMintAttributedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3)

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != RunDecisionStop || res.Outcome != RunOutcomeUnavailable {
		t.Fatalf("decision/outcome = %q/%q, want run-stop/run-tracker-unavailable", res.Decision, res.Outcome)
	}
	if res.Reason != "takeover-ambiguous" {
		t.Errorf("reason = %q, want takeover-ambiguous", res.Reason)
	}
	if !res.Terminal {
		t.Errorf("ambiguous-drives stop must be terminal")
	}
	if runTrackerRetryMarkerExists(t, f.repo.invocation, key) {
		t.Errorf("ambiguous drives must never spend the retry")
	}
}

// TestIntegrationRunVerdictVerdictTakeoverHaltStops: a takeover that HALTs (unsafe ownership) stops
// terminally with the driver's cause and never spends the retry.
func TestIntegrationRunVerdictVerdictTakeoverHaltStops(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		runTrackerIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Continuation = &fakeContinuationSeam{
		candidates:    []string{"d0opaque"},
		takeoverHalt:  true,
		takeoverCause: "scope-identity-mismatch",
	}
	key := runTrackerMintAttributedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1", 3)

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != RunDecisionStop || res.Outcome != RunOutcomeUnavailable {
		t.Fatalf("decision/outcome = %q/%q, want run-stop/run-tracker-unavailable", res.Decision, res.Outcome)
	}
	if res.Reason != "scope-identity-mismatch" {
		t.Errorf("reason = %q, want scope-identity-mismatch (driver cause passed through)", res.Reason)
	}
	if !res.Terminal {
		t.Errorf("halted takeover must be terminal")
	}
	if runTrackerRetryMarkerExists(t, f.repo.invocation, key) {
		t.Errorf("halted takeover must never spend the retry")
	}
}

// TestIntegrationRunVerdictVerdictContinueNeverAuthorizesNewClaim: the continuation path leaves
// attribution untouched — an already-attributed record's AttributedID is unchanged.
func TestIntegrationRunVerdictVerdictContinueNeverAuthorizesNewClaim(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true})
	wdeps.Continuation = &fakeContinuationSeam{handoffToken: "h0token"}
	key := runTrackerMintAttributed(t, f.repo.invocation, 3)

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != RunDecisionContinue {
		t.Fatalf("Decision = %q, want %q", res.Decision, RunDecisionContinue)
	}
	if res.AttributedID != 3 {
		t.Errorf("AttributedID = %d, want 3 (attribution untouched on continue)", res.AttributedID)
	}
	rec, err := LoadRunTrackerRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	if rec.AttributedID != 3 {
		t.Errorf("record AttributedID = %d, want 3", rec.AttributedID)
	}
}

// TestIntegrationRunVerdictVerdictFreshRunBindsScopeChange: on a FRESH run (no pre-attributed id), when
// ownership resolution adopts the sole matching claim proof the verdict path binds
// that change id into the outer recovery scope (spec §3 defense-in-depth) so a later
// outer takeover's scopeIdentityMatch pins the change rather than skipping it on an
// empty scope field. Mutation target: dropping the BindScopeChange call at the
// adoption point reddens the bindCalls assertion below.
func TestIntegrationRunVerdictVerdictFreshRunBindsScopeChange(t *testing.T) {
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
		{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "ctxhash-1", Revision: "r1"},
	}}
	// A scoped, unattributed record: ScopeID present, AttributedID == 0 (fresh run).
	key := runTrackerMintStartedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1")

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
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

// TestIntegrationRunVerdictVerdictContinuationDoesNotRebindScope: a continuation (an already-attributed
// record — the state a second run verdict call reads) skips attribution entirely,
// so it MUST NOT re-bind the outer scope's change id. This is the bind-once guard's
// other half: the fresh run binds, a continuation never touches it.
func TestIntegrationRunVerdictVerdictContinuationDoesNotRebindScope(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true})
	seam := &fakeContinuationSeam{handoffToken: "h0token"}
	wdeps.Continuation = seam
	// Already attributed AND scoped: a continuation of the same attempt.
	key := runTrackerMintStartedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1")
	rec, err := LoadRunTrackerRecord(f.repo.invocation, key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	rec.AttributedID = 3
	if err := SaveRunTrackerRecord(f.repo.invocation, key, rec); err != nil {
		t.Fatalf("SaveRunTrackerRecord: %v", err)
	}

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if res.Decision != RunDecisionContinue {
		t.Fatalf("Decision = %q, want %q (a live continuation)", res.Decision, RunDecisionContinue)
	}
	if seam.bindCalls != 0 {
		t.Fatalf("BindScopeChange called %d times on a continuation, want 0 (bind-once: never re-bind)", seam.bindCalls)
	}
}

// TestIntegrationRunVerdictVerdictObservePathStillCannotContinue: the observe (unattributed) render
// path is structurally unable to emit a retry OR a continuation — extending the
// existing observe-cannot-retry guarantee to run-continue.
func TestIntegrationRunVerdictVerdictObservePathStillCannotContinue(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		runTrackerIncompleteRecord(),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	res := RunVerdictObserve(context.Background(), deps, wdeps, gdeps, f.repo.invocation, []string{"3"})
	line := res.HumanText()
	if strings.HasPrefix(line, RunDecisionRetryOnce) {
		t.Errorf("observe must never emit %q; line = %q", RunDecisionRetryOnce, line)
	}
	if strings.Contains(line, RunDecisionContinue) {
		t.Errorf("observe must never emit %q; line = %q", RunDecisionContinue, line)
	}
	if !strings.HasPrefix(line, RunDecisionObserve) {
		t.Errorf("observe line must start with %q; got %q", RunDecisionObserve, line)
	}
}

// --- ownership from proof, never inference (change 0407) --------------------
//
// resolveRunTrackerOwnership replaces the before-set/cardinality attribution with the
// verified dispatch-to-claim binding: a confirmed binding's continuity is checked
// against committed proofs, an unconfirmed reservation recovers only from its exact
// receipt, an absent binding adopts the sole matching proof, and every missing /
// conflicting / unprovable case fails CLOSED to a non-authorizing report. The
// before-set and dispatch time are diagnostics only and can never grant a retry.

// TestIntegrationRunVerdictVerdictConfirmedBindingResolvesBoundChange: a confirmed binding whose newest
// proof for the id matches the bound request id resolves the bound change and
// delegates unchanged to RunVerify — a completed run reports run-complete even
// though the claim left the change in-progress.
func TestIntegrationRunVerdictVerdictConfirmedBindingResolvesBoundChange(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := runTrackerMintStarted(t, f.repo.invocation, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(f.repo.invocation, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmRunTrackerClaim(f.repo.invocation, key, 3, "claim-3-v", "r1", ""); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "ha", Revision: "r1"},
	}}

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "run-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if res.AttributedID != 3 {
		t.Errorf("AttributedID = %d, want 3 (bound change resolved)", res.AttributedID)
	}
}

// TestIntegrationRunVerdictVerdictNoBindingNoProofIsNoAttributableClaim: a dispatch that claims nothing
// resolves to no-attributable-claim and never acquires a sibling's claim — a proof
// under a DIFFERENT context hash is filtered out, the retry is never spent, and the
// sibling id never appears in the report.
func TestIntegrationRunVerdictVerdictNoBindingNoProofIsNoAttributableClaim(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "x", ChangeID: 9, RunContextHash: "OTHER"},
	}}}

	res := RunVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "run-done "+key+" no-attributable-claim"; got != want {
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
	if runTrackerRetryMarkerExists(t, repo, key) {
		t.Errorf("no-attributable-claim must never spend the retry")
	}
}

// TestIntegrationRunVerdictVerdictUnconfirmedReservationRecoversFromExactReceipt: a reservation whose
// confirm was interrupted recovers from the exact committed receipt (same request
// id, same context hash), confirms the binding, and delegates to the bound id.
func TestIntegrationRunVerdictVerdictUnconfirmedReservationRecoversFromExactReceipt(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := runTrackerMintStarted(t, f.repo.invocation, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(f.repo.invocation, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "ha", Revision: "r1"},
	}}

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "run-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (recovered from the exact receipt)", got, want)
	}
	b, ok, err := LoadRunTrackerClaimBinding(f.repo.invocation, key)
	if err != nil || !ok || !b.Confirmed || b.ChangeID != 3 {
		t.Fatalf("binding = %+v ok=%v err=%v, want confirmed change 3", b, ok, err)
	}
}

// TestIntegrationRunVerdictVerdictUnconfirmedReservationWithoutReceiptStops: an unconfirmed reservation
// with no matching committed receipt never became a real claim — the verdict is
// no-attributable-claim and the reservation is left refused (never released so a
// different claim can take it, never confirmed).
func TestIntegrationRunVerdictVerdictUnconfirmedReservationWithoutReceiptStops(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: nil}}

	res := RunVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "run-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	b, ok, err := LoadRunTrackerClaimBinding(repo, key)
	if err != nil || !ok || b.Confirmed || b.ChangeID != 3 || b.RequestID != "claim-3-v" {
		t.Fatalf("binding = %+v ok=%v err=%v, want the unconfirmed reservation intact", b, ok, err)
	}
}

// TestIntegrationRunVerdictVerdictUnconfirmedReservationSiblingContextHashIsNoAttributableClaim: an
// unconfirmed reservation whose only committed proof shares the request id but
// carries a DIFFERENT context hash is a sibling collision, not this dispatch's
// receipt. runTrackerProofForClaim's `&& p.RunContextHash == contextHash` clause must
// reject it, so the verdict is no-attributable-claim and nothing is adopted.
// Dropping that clause reddens this test (mutation-load-bearing) — neither
// RecoversFromExactReceipt (matching hash) nor WithoutReceiptStops (no proofs)
// exercises the same-request-id sibling.
func TestIntegrationRunVerdictVerdictUnconfirmedReservationSiblingContextHashIsNoAttributableClaim(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "hb"},
	}}}

	res := RunVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "run-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q (sibling context hash must not recover)", got, want)
	}
	if res.AttributedID != 0 {
		t.Errorf("AttributedID = %d, want 0 (the sibling proof must never be attributed)", res.AttributedID)
	}
}

// TestIntegrationRunVerdictVerdictAbsentBindingAdoptsSoleProof: with no binding file at all, a single
// committed proof matching the record's context hash is adopted (reserved,
// confirmed, mirrored) and delegation proceeds; two matching proofs are unsafe
// ownership and fail closed to binding-conflict.
func TestIntegrationRunVerdictVerdictAbsentBindingAdoptsSoleProof(t *testing.T) {
	t.Run("sole proof adopted", func(t *testing.T) {
		f := newRunVerifyFixture(t, true)
		deps, wdeps, gdeps := f.deps(
			rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
			rvPR(f.head, string(prEvidenceBytes(t, f.head))),
		)
		key := runTrackerMintStarted(t, f.repo.invocation, nil, 1, "ha")
		wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
			{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "ha", Revision: "r1"},
		}}

		res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
		if got, want := res.HumanText(), "run-done "+key+" run-complete 3"; got != want {
			t.Fatalf("HumanText = %q, want %q", got, want)
		}
		b, ok, err := LoadRunTrackerClaimBinding(f.repo.invocation, key)
		if err != nil || !ok || !b.Confirmed || b.ChangeID != 3 {
			t.Fatalf("binding = %+v ok=%v err=%v, want confirmed change 3 after adoption", b, ok, err)
		}
	})

	t.Run("two proofs is binding-conflict", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		key := runTrackerMintStarted(t, repo, nil, 1, "ha")
		wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
			{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "ha", Revision: "r1"},
			{RequestID: "claim-4-v", ChangeID: 4, RunContextHash: "ha", Revision: "r2"},
		}}}

		res := RunVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
		if res.Decision != RunDecisionStop || res.Outcome != RunOutcomeUnavailable || res.Reason != ReasonRunBindingConflict {
			t.Fatalf("got %q/%q/%q, want run-stop/run-tracker-unavailable/%s", res.Decision, res.Outcome, res.Reason, ReasonRunBindingConflict)
		}
		if !res.Terminal {
			t.Errorf("binding-conflict is terminal")
		}
	})
}

// TestIntegrationRunVerdictVerdictClaimReplacedStops: a confirmed binding whose bound change carries a
// NEWER proof under a different request id means the change was reclaimed and
// re-claimed by another run — the old gate must neither retry nor take over the
// replacement. The binding is left unchanged and the retry is never spent.
func TestIntegrationRunVerdictVerdictClaimReplacedStops(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(repo, key, 3, "claim-3-v1"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmRunTrackerClaim(repo, key, 3, "claim-3-v1", "r1", ""); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v2", ChangeID: 3, RunContextHash: ""},
		{RequestID: "claim-3-v1", ChangeID: 3, RunContextHash: "ha", Revision: "r1"},
	}}}

	res := RunVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if res.Decision != RunDecisionStop || res.Outcome != RunOutcomeUnavailable || res.Reason != ReasonRunClaimReplaced {
		t.Fatalf("got %q/%q/%q, want run-stop/run-tracker-unavailable/%s", res.Decision, res.Outcome, res.Reason, ReasonRunClaimReplaced)
	}
	if runTrackerRetryMarkerExists(t, repo, key) {
		t.Errorf("a replaced claim must never spend the retry")
	}
	b, _, err := LoadRunTrackerClaimBinding(repo, key)
	if err != nil {
		t.Fatalf("load binding: %v", err)
	}
	if b.RequestID != "claim-3-v1" || !b.Confirmed || b.Revision != "r1" {
		t.Errorf("binding = %+v, want the original confirmed claim-3-v1@r1 unchanged", b)
	}
}

// TestIntegrationRunVerdictVerdictNilProofScannerFailsClosed: a claim-bound gate with no proof access
// cannot verify continuity — unlike the continuation seam, ownership can never
// proceed without proofs, so it fails closed to proof-unavailable.
func TestIntegrationRunVerdictVerdictNilProofScannerFailsClosed(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmRunTrackerClaim(repo, key, 3, "claim-3-v", "r1", ""); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	res := RunVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{ClaimProofs: nil}, GitHubDeps{}, repo, key)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunProofUnavailable {
		t.Fatalf("got %q/%q, want run-stop/%s", res.Decision, res.Reason, ReasonRunProofUnavailable)
	}
	if runTrackerRetryMarkerExists(t, repo, key) {
		t.Errorf("proof-unavailable must never spend the retry")
	}
}

// TestIntegrationRunVerdictVerdictProofScanErrorFailsClosed: a proof-scan error fails closed to
// proof-unavailable and never consumes a retry.
func TestIntegrationRunVerdictVerdictProofScanErrorFailsClosed(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(repo, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmRunTrackerClaim(repo, key, 3, "claim-3-v", "r1", ""); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{err: errors.New("boom")}}
	res := RunVerdict(context.Background(), PlanningDeps{}, wdeps, GitHubDeps{}, repo, key)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunProofUnavailable {
		t.Fatalf("got %q/%q, want run-stop/%s", res.Decision, res.Reason, ReasonRunProofUnavailable)
	}
	if runTrackerRetryMarkerExists(t, repo, key) {
		t.Errorf("a scan error must never spend the retry")
	}
}

// TestIntegrationRunVerdictVerdictCorruptBindingFailsClosed: an unparseable binding file is a typed
// binding-unreadable stop — never a silent (ok=false) fall-through to attribution.
func TestIntegrationRunVerdictVerdictCorruptBindingFailsClosed(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	common, err := runTrackerGitCommonDir(repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(common, "docket", runTrackerDirName, key, runTrackerClaimBindingName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res := RunVerdict(context.Background(), PlanningDeps{}, WorkspaceDeps{ClaimProofs: &fakeProofScanner{}}, GitHubDeps{}, repo, key)
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunBindingUnreadable {
		t.Fatalf("got %q/%q, want run-stop/%s", res.Decision, res.Reason, ReasonRunBindingUnreadable)
	}
}

// TestIntegrationRunVerdictVerdictResumeBindingSkipsContinuity: a run start --resume record
// (AttributedID set, BoundRequestID empty) is pre-bound by verified identity — the
// continuity check never runs, so a scanner that WOULD report a replacement still
// delegates to RunVerify (preserved verified-resume behavior).
func TestIntegrationRunVerdictVerdictResumeBindingSkipsContinuity(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := runTrackerMintAttributed(t, f.repo.invocation, 3)
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v2", ChangeID: 3, RunContextHash: ""},
	}}

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "run-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (resume-bound id delegates, never claim-replaced)", got, want)
	}
	if res.Reason == ReasonRunClaimReplaced {
		t.Errorf("a resume-bound verdict must never stop on continuity")
	}
}

// TestIntegrationRunVerdictVerdictOwnershipIgnoresBeforeSetAndRun pins the 0407 defect: a sibling
// in-progress claim the OLD before-set/run filters would have attributed sits in
// the corpus, but ownership reads only committed proofs. With no proof carrying the
// record's context hash, the verdict is no-attributable-claim — never the sibling.
// Mutation partner: re-introducing run/before-set inference reddens exactly this.
func TestIntegrationRunVerdictVerdictOwnershipIgnoresBeforeSetAndRun(t *testing.T) {
	repo := newRunTrackerRepo(t)
	deps := runTrackerLightDeps(t, []StatusBlob{runTrackerInProgressBlob(9, "sibling", "keep")})
	key := runTrackerMintStarted(t, repo, nil, 1, "ha")
	wdeps := WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "sib", ChangeID: 9, RunContextHash: "OTHER"},
	}}}

	res := RunVerdict(context.Background(), deps, wdeps, GitHubDeps{}, repo, key)
	if got, want := res.HumanText(), "run-done "+key+" no-attributable-claim"; got != want {
		t.Fatalf("HumanText = %q, want %q (never the sibling)", got, want)
	}
	if res.AttributedID != 0 {
		t.Errorf("AttributedID = %d, want 0 (the sibling id 9 must never be attributed)", res.AttributedID)
	}
	if len(res.AmbiguousIDs) != 0 {
		t.Errorf("no id may be named; got AmbiguousIDs=%v", res.AmbiguousIDs)
	}
}

// --- successful-run ownership closeout on a keyed run-complete (change 0441) ------
//
// A verified keyed run-complete now additionally drives the run-ownership closeout
// (runTrackerCompleteRun → completeSuccessfulRun). These tests wire Task 7's completion
// shape UNDER the key the verdict loads: the run-verify fixture drives RunVerify to
// run-complete for change 3, a confirmed binding + matching proof resolve ownership,
// and an active run bound to a worktree (with a terminal-recorded coordinator
// participant and permissive observation seams) is the run the closeout completes. The
// keyless/standalone/legacy shape (no run beside the record) keeps EXACTLY the prior
// behavior, and unattributed observe mode never touches ownership.

// TestIntegrationRunVerdictVerdictRunCompleteClosesOutRunOwnership: a keyed run-complete drives the
// closeout — run-done run-complete, the run is completed, and its drives were
// observed through the census for the run's context hash.
func TestIntegrationRunVerdictVerdictRunCompleteClosesOutRunOwnership(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	if calls := fx.launchObserver.calls; len(calls) != 1 || calls[0] != "ha" {
		t.Fatalf("census calls = %v, want [ha] (the run's context hash)", calls)
	}
}

// TestIntegrationRunVerdictVerdictRunCompleteWithoutRunUnchanged: with no run beside the record the
// verdict keeps EXACTLY the prior behavior — run-done run-complete, no run
// fabricated, no completion findings.
func TestIntegrationRunVerdictVerdictRunCompleteWithoutRunUnchanged(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	key := runTrackerMintStarted(t, f.repo.invocation, nil, 1, "ha")
	if err := ReserveRunTrackerClaim(f.repo.invocation, key, 3, "claim-3-v"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmRunTrackerClaim(f.repo.invocation, key, 3, "claim-3-v", "r1", ""); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	wdeps.ClaimProofs = &fakeProofScanner{proofs: []ClaimProof{
		{RequestID: "claim-3-v", ChangeID: 3, RunContextHash: "ha", Revision: "r1"},
	}}

	res := RunVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	if got, want := res.HumanText(), "run-done "+key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q", got, want)
	}
	if len(res.CompletionFindings) != 0 {
		t.Errorf("no-run path carried completion findings: %v", res.CompletionFindings)
	}
	if _, _, err := LoadRunRecord(f.repo.invocation, key); !isRunKind(err, ErrRunNotFound) {
		t.Fatalf("no run must be fabricated by the closeout: %v", err)
	}
}

// TestIntegrationRunVerdictVerdictRunCompleteBlockedCloseoutStopsWithoutSuccess: one unproven obligation
// (a registered execution participant whose run is observed live) blocks the
// closeout — run-stop run-tracker-unavailable completion-unaccounted with diagnostic
// findings, the run stays completing (the fence holds), and no retry is spent (AC2
// budget preservation).
func TestIntegrationRunVerdictVerdictRunCompleteBlockedCloseoutStopsWithoutSuccess(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID,
		RunParticipant{Kind: "raw-run", NativeHandle: "exec-live"}))
	fx.observer.defaultProven = false // the participant's run is not provably terminal
	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if res.Decision != RunDecisionStop || res.Outcome != RunOutcomeUnavailable {
		t.Fatalf("decision/outcome = %q/%q, want run-stop/run-tracker-unavailable", res.Decision, res.Outcome)
	}
	if res.Reason != ReasonRunCompletionUnaccounted {
		t.Fatalf("reason = %q, want %q", res.Reason, ReasonRunCompletionUnaccounted)
	}
	if len(res.CompletionFindings) == 0 {
		t.Fatal("a blocked closeout carried no diagnostic findings to settle")
	}
	if !res.Terminal {
		t.Error("a blocked closeout stop is terminal")
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleting {
		t.Fatalf("run state = %q, want completing (the success fence holds)", st)
	}
	if runTrackerRetryMarkerExists(t, fx.repo, fx.key) {
		t.Error("a blocked closeout must never spend the retry")
	}
}

// TestIntegrationRunVerdictVerdictRunCompleteCancelledRunNeverReportsSuccess: an explicit cancellation
// that already won is never relabelled successful — run-stop run-tracker-unavailable
// run-cancelled, never run-done, and the run state is untouched.
func TestIntegrationRunVerdictVerdictRunCompleteCancelledRunNeverReportsSuccess(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	forceRunState(t, fx.repo, fx.key, RunCancelled)
	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if res.Decision != RunDecisionStop || res.Outcome != RunOutcomeUnavailable {
		t.Fatalf("decision/outcome = %q/%q, want run-stop/run-tracker-unavailable", res.Decision, res.Outcome)
	}
	if res.Reason != ReasonRunCancelled {
		t.Fatalf("reason = %q, want %q (never run-done)", res.Reason, ReasonRunCancelled)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
		t.Fatalf("run state = %q, want cancelled (never relabelled)", st)
	}
}

// TestIntegrationRunVerdictVerdictRunCompleteReportPersistFailureIsReported: the closeout finishes (run
// durably completed) but the terminal gate-report save fails — run-stop
// run-tracker-unavailable report-unpersisted (the failure is reported, not hidden). A SECOND
// verdict with the fault cleared replays the completed run to run-done run-complete
// (AC6 gate-report write failure + replay).
func TestIntegrationRunVerdictVerdictRunCompleteReportPersistFailureIsReported(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	// Pre-drive the closeout so the run is durably completed: a replay does NO run
	// writes (the fence observes completed), isolating the checked report SAVE as the
	// only write the read-only key dir can fail.
	if ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key); !ok {
		t.Fatalf("pre-closeout ok=false reason=%q findings=%v", reason, findings)
	}
	keyDir, err := runKeyDir(fx.repo, fx.key, "test-persist-fault")
	if err != nil {
		t.Fatalf("runKeyDir: %v", err)
	}
	if err := os.Chmod(keyDir, 0o500); err != nil {
		t.Fatalf("chmod key dir read-only: %v", err)
	}
	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if err := os.Chmod(keyDir, 0o700); err != nil {
		t.Fatalf("restore key dir: %v", err)
	}
	if res.Decision != RunDecisionStop || res.Reason != ReasonRunReportUnpersisted {
		t.Fatalf("decision/reason = %q/%q, want run-stop/report-unpersisted", res.Decision, res.Reason)
	}

	// Fault cleared: the completed run replays to run-done run-complete.
	res2 := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res2.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("replay HumanText = %q, want %q", got, want)
	}
}

// TestIntegrationRunVerdictVerdictObserveModeNeverTouchesOwnership: the unattributed observe path over the
// same run-backed complete fixture leaves the run byte-identical (AC5) — it holds
// no key, drives no closeout (not even the verdict-mode census), and renders the
// plain observe run-complete line.
func TestIntegrationRunVerdictVerdictObserveModeNeverTouchesOwnership(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	_, genBefore, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("load run before: %v", err)
	}
	res := RunVerdictObserve(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, []string{"3"})
	if got, want := res.HumanText(), "run-observe run-complete 3"; got != want {
		t.Fatalf("observe HumanText = %q, want %q", got, want)
	}

	_, genAfter, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("load run after: %v", err)
	}
	if genAfter != genBefore {
		t.Fatalf("observe mode wrote the run: generation %q -> %q", genBefore, genAfter)
	}
	if len(fx.launchObserver.calls) != 0 {
		t.Fatalf("observe mode ran the closeout census: %v", fx.launchObserver.calls)
	}
}

// TestIntegrationRunVerdictFinishedBuildDrivesAreNotTakeoverCandidates (change 0489,
// R1) drives a REAL red gate and a REAL green re-gate — build-owned drives carrying
// the run's change id and run context, as implement-next starts them — through the
// production gate-drive service, then asks the PRODUCTION continuation seam for
// outer-takeover candidates. Finished drives must yield none, so run.verdict takes
// its retry path (TestIntegrationRunVerdictVerdictIncompleteQuiescentStillRetriesOnce
// pins zero candidates -> run-retry-once) instead of stopping takeover-ambiguous.
func TestIntegrationRunVerdictFinishedBuildDrivesAreNotTakeoverCandidates(t *testing.T) {
	requireRealGit(t)
	requireProcessSupervisor(t)
	worktree, gitDir := initGitRepo(t, "")
	const runContext = "run-ctx-0489-r1"
	sum := sha256.Sum256([]byte(runContext))
	ctxHash := hex.EncodeToString(sum[:])

	finish := func(command string) (gatedrive.Outcome, string) {
		t.Helper()
		runRoot := filepath.Join(testsupport.TempDir(t), "runs")
		t.Cleanup(func() { stopRunsUnder(runRoot) })
		svc, res, reason := NewBuildGateDriveService(gitDir, guardianExecutable(t), buildEffWithMaxAttempts(command, 4))
		if svc == nil {
			t.Fatalf("build gate-drive service was nil: %s %s", res, reason)
		}
		got := svc.Start(GateDriveStartRequest{
			RepoDir: worktree, Worktree: worktree, ChangeID: "0003", Phase: "build",
			Branch: "fix/x", Ref: "refs/heads/fix/x", Cwd: worktree, RunRoot: runRoot,
			RunContext: runContext, IdempotentSuiteGate: true,
		})
		if got.Result != ResultApplied || got.Drive == nil {
			t.Fatalf("build start refused: result=%s reason=%q message=%q", got.Result, got.Reason, got.Message)
		}
		return runDriveToTerminal(t, svc, got), got.Drive.DriveID
	}
	out, redID := finish("/usr/bin/false")
	if out != gatedrive.FAILED {
		t.Fatalf("red gate outcome = %s, want FAILED", out)
	}
	if out, _ := finish("/bin/echo green"); out != gatedrive.PASSED {
		t.Fatalf("green re-gate outcome = %s, want PASSED", out)
	}

	seam, err := NewContinuationSeam(gitDir, guardianExecutable(t))
	if err != nil {
		t.Fatalf("NewContinuationSeam: %v", err)
	}
	ids, err := seam.LocateOuterDrive(3, ctxHash)
	if err != nil {
		t.Fatalf("LocateOuterDrive: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("a red-then-green run must leave NO takeover candidate (else run.verdict stops takeover-ambiguous), got %v", ids)
	}

	// Positive control: the same red drive — same change id and run context —
	// re-marked WAITING on disk is located, and only it, so the empty scan above
	// proves the finished drives are excluded by their terminal outcome rather
	// than that the seam finds nothing at all. (A real WAITING drive would cost a
	// full 30s production observation slice plus its stop.)
	markDriveWaiting(t, gitDir, redID)
	ids, err = seam.LocateOuterDrive(3, ctxHash)
	if err != nil {
		t.Fatalf("LocateOuterDrive (positive control): %v", err)
	}
	if len(ids) != 1 || ids[0] != redID {
		t.Fatalf("a WAITING build drive must be the one takeover candidate (finished drives excluded), got %v want [%s]", ids, redID)
	}
}

// markDriveWaiting rewrites the persisted record of drive id (found under the
// gate-drive store beneath gitDir) so its last outcome reads WAITING, leaving
// every other field as the production driver wrote it.
func markDriveWaiting(t *testing.T, gitDir, id string) {
	t.Helper()
	var path string
	walkErr := filepath.WalkDir(gitDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "record.json" && filepath.Base(filepath.Dir(p)) == id {
			path = p
			return fs.SkipAll
		}
		return nil
	})
	if walkErr != nil || path == "" {
		t.Fatalf("drive %s record not found under %s: %v", id, gitDir, walkErr)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read drive record: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode drive record: %v", err)
	}
	rec, ok := env["record"].(map[string]any)
	if !ok {
		t.Fatalf("drive record %s has no record object", path)
	}
	rec["last_outcome"] = string(gatedrive.WAITING)
	if raw, err = json.Marshal(env); err != nil {
		t.Fatalf("encode drive record: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write drive record: %v", err)
	}
}

// TestIntegrationRunVerdictNeverLaunchedDriveCompletesRun (change 0491, spec T1, the
// stub's regression — 0490 review finding F3): a tracked gate.drive.start killed
// between Admit and StartAdmitted leaves a reserved drive record nothing can launch.
// The keyed run.verdict's verdict-mode census proves it never launched, settles it
// HALTED launch-abandoned, and the run completes — run-done run-complete, never
// run-stop … completion-unaccounted.
func TestIntegrationRunVerdictNeverLaunchedDriveCompletesRun(t *testing.T) {
	requireProcessSupervisorHere(t)
	fx := newVerdictCompletionFixture(t)
	common, err := runTrackerGitCommonDir(fx.repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	// The production verdict census over the real store replaces the fixture's fake.
	fx.wdeps.CancelSeams = func(string) cancelSeams {
		return cancelSeams{store: fx.store, observer: fx.observer, launchObserver: appLaunchObserver{store: fx.store}}
	}
	const orphan = "adddddddddddddddddddddddddd00492"
	// An existing, empty run root holds no reservation: ResolveReservation proves
	// never-launched. "ha" is the fixture run's context hash.
	seedNeverLaunchedDrive(t, common, orphan, fx.worktree, testsupport.TempDir(t), "ha", nil)

	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (findings=%v)", got, want, res.CompletionFindings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	if out, cause := driveOutcome(t, fx.store, orphan); out != gatedrive.HALTED || cause != "launch-abandoned" {
		t.Fatalf("drive = %s/%q, want HALTED launch-abandoned", out, cause)
	}
}
