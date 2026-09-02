// Process-integration coverage for the change-0359 mechanism: fast completion,
// the production slice bound, and event-authorized takeover — all driven against
// the REAL native process supervisor (internal/process.Service), never a scripted
// double. It shares TestMain, the re-exec child helper, and every fixture with
// integration_test.go (same package); this file adds the invariants a fake proc
// cannot vouch for:
//
//   - a fast child returns PASSED as soon as the pass is observed, paying NO slice
//     or budget floor (proved against a DELIBERATELY long slice);
//   - the production slice is exactly 30s, and a live child returns WAITING within
//     one injected-short slice rather than the observation budget (together these
//     pin "first slice returns by 30s" without sleeping 30s);
//   - a parent takeover of a live scope-bound drive continues the SAME supervised
//     run — same raw run dir, raw ownership, attempt, and native pid/pgid/sid — and
//     never relaunches or duplicates the child;
//   - a FRESH driver process over the same durable store takes over a
//     terminal-unconsumed drive and consumes the exact recorded verdict with no
//     relaunch.
//
// The identity oracle is ALWAYS the native receipt (manifest pid/pgid/sid), never
// process-name matching, exactly as integration_test.go documents.
//
// This file is split out of integration_test.go (per the plan's budget guidance):
// it adds four real-process tests, and grouping the takeover-identity pair here
// keeps each file's real-process wall clock modest.
package gatedrive

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationFastCompletionReturnsImmediately proves a fast child that exits 0
// returns PASSED on the very first call and pays NO floor: the driver runs with a
// deliberately LONG (30s) slice, so if a slice were a floor the call would block
// for it. A completed run instead returns as soon as the pass is observed — far
// under the slice and the 30-minute budget.
func TestIntegrationFastCompletionReturnsImmediately(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	store := OpenStore(testsupport.TempDir(t))
	reapSupervisors(t, runRoot)
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })

	// A LONG slice: a floor, were there one, would be visible as a ~30s call.
	d := NewDriver(store, systemClock{}, svc, stableGit())
	d.slice = 30 * time.Second
	d.pollInterval = intPoll
	d.sleep = time.Sleep

	start := time.Now()
	doc, err := d.Start(intStartRequest(mustExe(t), runRoot, testsupport.TempDir(t), "pass-after", "0"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	el := time.Since(start)
	if doc.Outcome != PASSED {
		t.Fatalf("a fast child must PASS on the first call, got %s (cause %q)", doc.Outcome, doc.Cause)
	}
	// Returned far under its own 30s slice (and the 30m budget): a fast test pays no
	// floor. sliceCeiling is the generous CI upper bound, itself well under 30s.
	if el > sliceCeiling {
		t.Fatalf("fast completion took %v — it paid a slice/budget floor instead of returning immediately", el)
	}
}

// TestIntegrationSliceBoundIsProductionThirtySeconds pins the production slice at
// exactly 30s AND proves a live child that outlives the (shrunk) slice returns
// WAITING within one slice plus scheduling margin — not after the 30-minute
// budget. Together these establish "the first slice returns by 30s" without ever
// sleeping 30s.
func TestIntegrationSliceBoundIsProductionThirtySeconds(t *testing.T) {
	// The constant is a spec-fixed invariant (Global Constraints): a change to it is
	// a visible failure here, not a silent drift.
	if productionSlice != 30*time.Second {
		t.Fatalf("productionSlice = %v, want exactly 30s", productionSlice)
	}

	skipUnlessSupported(t)
	svc := mustService(t)
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	store := OpenStore(testsupport.TempDir(t))
	reapSupervisors(t, runRoot)
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	d := newIntDriver(store, svc) // injected short slice (intSlice)

	start := time.Now()
	doc, err := d.Start(intStartRequest(mustExe(t), runRoot, testsupport.TempDir(t), "sleep-forever", ""))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	el := time.Since(start)
	if doc.Outcome != WAITING {
		t.Fatalf("a live child outliving the slice must WAIT, got %s (cause %q)", doc.Outcome, doc.Cause)
	}
	// Bounded by one (shrunk) slice + margin, not by the 30m budget.
	if el > sliceCeiling {
		t.Fatalf("first slice over a live child returned in %v — not slice-bounded (budget is 30m)", el)
	}
}

// TestIntegrationTakeoverKeepsRunIdentity drives a REAL scope-bound run: a parent
// prepares a recovery scope, a scope-bound Start launches a real slow child, and a
// parent Takeover then supersedes the child owner and advances the SAME run to its
// terminal pass. It proves the takeover continues one stable supervised identity —
// same raw run dir, raw ownership, attempt, and native pid/pgid/sid — with exactly
// one run slot throughout: no relaunch, no duplicate child.
func TestIntegrationTakeoverKeepsRunIdentity(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	store := OpenStore(testsupport.TempDir(t))
	reapSupervisors(t, runRoot)
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })
	d := newIntDriver(store, svc)

	// A real scope-bound start over a child that outlives several short slices and
	// is still live when the takeover happens.
	req := intStartRequest(mustExe(t), runRoot, testsupport.TempDir(t), "pass-after", "500")
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	started, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if started.Outcome != WAITING {
		t.Fatalf("scope-bound first slice over a live child must WAIT, got %s (%s)", started.Outcome, started.Cause)
	}

	// Identity BEFORE takeover: the durable raw run identity plus the native
	// manifest pid/pgid/sid (the driver-independent oracle).
	runDir := soleRunDir(t, runRoot)
	recBefore, err := store.Load(started.DriveID)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	idBefore := readManifestIdentity(t, runDir)

	// Event-authorized takeover: it supersedes the child owner without launching or
	// stopping any process.
	took, err := d.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
	if err != nil {
		t.Fatalf("Takeover: %v", err)
	}
	if took.Outcome == HALTED {
		t.Fatalf("a valid takeover of a live scope-bound drive must not HALT: %s", took.Cause)
	}
	if took.Generation == "" || took.Generation == started.Generation {
		t.Fatalf("takeover must mint a fresh owner generation distinct from the child's, got %q", took.Generation)
	}

	// The fresh owner drives the SAME live run to its terminal pass.
	term, _ := advanceUntilTerminal(t, d, started.DriveID, took.Generation)
	if term.Outcome != PASSED {
		t.Fatalf("post-takeover terminal %s (cause %q), want PASSED", term.Outcome, term.Cause)
	}

	// Same run throughout: one run slot, identical raw run dir + raw ownership +
	// attempt, identical native supervisor identity. The takeover CONTINUED the run.
	recAfter, err := store.Load(started.DriveID)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if got := soleRunDir(t, runRoot); got != runDir {
		t.Fatalf("run dir changed across takeover: %s -> %s", runDir, got)
	}
	if recAfter.RawRunDir != recBefore.RawRunDir || recAfter.RawOwnership != recBefore.RawOwnership {
		t.Fatalf("takeover changed the raw run identity: dir %q->%q ownership %q->%q",
			recBefore.RawRunDir, recAfter.RawRunDir, recBefore.RawOwnership, recAfter.RawOwnership)
	}
	if recAfter.Attempt != recBefore.Attempt {
		t.Fatalf("takeover changed the attempt %d->%d — it relaunched", recBefore.Attempt, recAfter.Attempt)
	}
	if idAfter := readManifestIdentity(t, runDir); idAfter != idBefore {
		t.Fatalf("supervised identity drifted across takeover: %+v -> %+v", idBefore, idAfter)
	}
}

// TestIntegrationTerminalConsumedFromFreshProcess drives a real scope-bound child
// to a terminal PASS in one driver process, then builds a BRAND-NEW Store+Driver
// over the same Git common dir (a fresh process) and Takeover+Advances the
// terminal-unconsumed drive. It proves the fresh process consumes the exact
// recorded verdict from the durable record alone, with no relaunch and no
// duplicate run slot.
func TestIntegrationTerminalConsumedFromFreshProcess(t *testing.T) {
	skipUnlessSupported(t)
	svc := mustService(t)
	gitCommon := testsupport.TempDir(t)
	runRoot := filepath.Join(testsupport.TempDir(t), "runs")
	reapSupervisors(t, runRoot)
	t.Cleanup(func() { stopAllRuns(t, svc, runRoot) })

	// First process: a scope-bound drive to a terminal PASS. Advance leaves the
	// owner generation set, so the terminal is UNCONSUMED (no cooperative claim).
	store1 := OpenStore(gitCommon)
	d1 := newIntDriver(store1, svc)
	req := intStartRequest(mustExe(t), runRoot, testsupport.TempDir(t), "pass-after", "300")
	grant, err := store1.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	started, err := d1.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	term, _ := advanceUntilTerminal(t, d1, started.DriveID, started.Generation)
	if term.Outcome != PASSED {
		t.Fatalf("first-process terminal %s (cause %q), want PASSED", term.Outcome, term.Cause)
	}
	runDir := soleRunDir(t, runRoot)
	recBefore, err := store1.Load(started.DriveID)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}

	// A FRESH process over the SAME Git common dir: a brand-new Store + Driver,
	// resuming purely from the durable record.
	store2 := OpenStore(gitCommon)
	d2 := newIntDriver(store2, svc)

	took, err := d2.Takeover(grant.ScopeID, grant.ParentCapability, started.DriveID)
	if err != nil {
		t.Fatalf("Takeover from fresh process: %v", err)
	}
	if took.Outcome != PASSED {
		t.Fatalf("takeover of a terminal-unconsumed drive must report the recorded PASSED, got %s (%s)", took.Outcome, took.Cause)
	}
	if took.Generation == "" || took.Generation == started.Generation {
		t.Fatalf("takeover must mint a fresh owner, got %q", took.Generation)
	}

	// The fresh owner's Advance returns the recorded verdict unchanged, with NO
	// relaunch: same attempt, same raw run dir, still exactly one run slot.
	adv, err := d2.Advance(started.DriveID, took.Generation)
	if err != nil {
		t.Fatalf("fresh-owner Advance: %v", err)
	}
	if adv.Outcome != PASSED || adv.Attempt != recBefore.Attempt {
		t.Fatalf("recorded verdict must return unchanged: got %s attempt %d, want PASSED attempt %d",
			adv.Outcome, adv.Attempt, recBefore.Attempt)
	}
	if got := soleRunDir(t, runRoot); got != runDir {
		t.Fatalf("consuming a terminal must not relaunch/duplicate: run dir %s -> %s", runDir, got)
	}
	recAfter, err := store2.Load(started.DriveID)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if recAfter.RawRunDir != recBefore.RawRunDir || recAfter.Attempt != recBefore.Attempt {
		t.Fatalf("terminal consumption changed the raw run identity/attempt: dir %q->%q attempt %d->%d",
			recBefore.RawRunDir, recAfter.RawRunDir, recBefore.Attempt, recAfter.Attempt)
	}
}
