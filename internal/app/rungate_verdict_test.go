package app

import (
	"context"
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
// given before-set and dispatch epoch, as gate-before would.
func gateMintArmed(t *testing.T, repoDir string, beforeIDs []int, dispatchEpoch int64) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{
		Target:        "docket-implement-next",
		CreatedAt:     1,
		DispatchEpoch: dispatchEpoch,
		BeforeIDs:     beforeIDs,
		Retry:         RetryUnused,
		Disposition:   "gate-armed",
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
	key := gateMintArmed(t, repoDir, nil, 1)
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
	key := gateMintArmed(t, repoDir, nil, 1)
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
	key := gateMintArmed(t, f.repo.invocation, nil, 1)

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
	key := gateMintArmedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1")

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
	key := gateMintArmedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1")

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
	key := gateMintArmedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1")

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
	key := gateMintArmedScoped(t, f.repo.invocation, "scope-1", "pcap-1", "ctxhash-1")

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
// attribution first resolves exactly one claim the verdict path binds that change
// id into the outer recovery scope (spec §3 defense-in-depth) so a later outer
// takeover's scopeIdentityMatch pins the change rather than skipping it on an empty
// scope field. Mutation target: dropping the BindScopeChange call at the attribution
// point reddens the bindCalls assertion below.
func TestVerdictFreshRunBindsScopeChange(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	seam := &fakeContinuationSeam{} // no candidates: attribution runs, then the
	// run-incomplete path finds zero tracked drives and takes the ordinary retry
	// route — but the bind already fired at the attribution point.
	wdeps.Continuation = seam
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
