// Handoff and repository tests (spec "Verification strategy → Handoff and
// repository tests"): the REAL repository execution-identity dimensions bound to
// the ownership handoff/claim CAS, plus the driver-level consequences of a
// handoff.
//
// ownership_test.go (Task 5) proves the handoff/claim CAS logic against SYNTHETIC
// fingerprints (matchingFP/mismatchFP mutate one struct field).
// handoff_integration_test.go (behind the integration tag since change 0466, as it
// runs real git) proves the binding those synthetic tests take on faith: that a
// genuine git-level mutation in each dimension — staged bytes, unstaged bytes, an
// untracked file, a rename, a deletion, an executable-mode flip, a symlink
// retarget — produces a ComputeFingerprint value the claim path rejects, one
// dimension at a time. It also proves the two properties spec "Explicit handoff
// and nearest-owner continuation" names for dirty task work: identical dirty state
// claims WITHOUT a WIP commit, and a fingerprint that drifted between drive-start
// and claim never grants partial authority.
//
// The single-winner race and the plain-WAITING rejection are already proven by
// ownership_test.go (TestRaceOneReceiptSingleWinner,
// TestPlainWaitingDriveCannotBeClaimed) and are not duplicated here. This file
// adds the driver-level chain those primitives feed: an old owner cannot advance
// after a handoff, and a fresh owner consumes a terminal the raw run wrote while
// no agent was advancing.
package gatedrive

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestOldOwnerCannotAdvanceAfterHandoff proves the driver-level consequence of a
// handoff: once the current owner has handed off (its generation invalidated),
// an Advance presenting that old generation is an identity disagreement that
// HALTs and drives nothing — it never silently continues the suite.
func TestOldOwnerCannotAdvanceAfterHandoff(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{} // default: would observe a live run if ever consulted
	d, store := newTestDriver(t, clk, proc, stableGit())

	rec := seedRecord(t)
	id, owner := seedDrive(t, store, rec)

	// The current owner hands off, invalidating its generation.
	if _, err := store.writeHandoffReceipt(id, owner, rec.Fingerprint); err != nil {
		t.Fatalf("writeHandoffReceipt: %v", err)
	}

	got, err := d.Advance(id, owner)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got.Outcome != HALTED {
		t.Fatalf("an old owner advancing after handoff must HALT, got %s", got.Outcome)
	}
	if got.Outcome == PASSED || got.Outcome == FAILED {
		t.Fatalf("a superseded owner must never drive the suite to a verdict, got %s", got.Outcome)
	}
	// The old owner drove nothing: the process seam was never consulted.
	if proc.observeN != 0 || proc.launchN != 0 || proc.stopN != 0 {
		t.Fatalf("a rejected old-owner advance must not touch the process seam: observe=%d launch=%d stop=%d",
			proc.observeN, proc.launchN, proc.stopN)
	}
	// The drive is undisturbed: the handoff is still outstanding for a real claimant.
	after, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if after.OwnerGeneration != "" {
		t.Fatalf("a rejected old-owner advance must not reinstate an owner, got %q", after.OwnerGeneration)
	}
	if after.HandoffGeneration == "" {
		t.Fatalf("a rejected old-owner advance must leave the outstanding handoff intact")
	}
}

// TestFreshOwnerConsumesTerminalWrittenWhileNoAgentActive proves the resume half
// of the ownership chain: the raw run reaches a durable terminal while no agent
// is advancing (represented by a passed observation), the owner hands off, a
// fresh agent claims the receipt, and THAT fresh owner's first Advance consumes
// the terminal — returning PASSED and exposing the raw run dir for evidence,
// trusting the durable receipt rather than any transcript.
func TestFreshOwnerConsumesTerminalWrittenWhileNoAgentActive(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		// The suite completed green while no agent was watching; the durable
		// terminal is what a later Advance reads.
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StatePassed, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())

	rec := seedRecord(t)
	id, owner := seedDrive(t, store, rec)

	// The owner hands off (worktree unchanged), then a fresh agent claims — the
	// claimant recomputes identity over the same worktree, exactly as it would.
	receipt, err := store.writeHandoffReceipt(id, owner, rec.Fingerprint)
	if err != nil {
		t.Fatalf("writeHandoffReceipt: %v", err)
	}
	claimFP, err := ComputeFingerprint(rec.WorktreePath, stableGit())
	if err != nil {
		t.Fatalf("ComputeFingerprint: %v", err)
	}
	newOwner, err := store.consumeHandoffCAS(id, receipt.HandoffGeneration, claimFP)
	if err != nil {
		t.Fatalf("consumeHandoffCAS: %v", err)
	}

	// The fresh owner — and ONLY the fresh owner — advances and consumes the
	// terminal the raw run wrote while no agent was active.
	doc, err := d.Advance(id, newOwner)
	if err != nil {
		t.Fatalf("fresh-owner Advance: %v", err)
	}
	if doc.Outcome != PASSED {
		t.Fatalf("a fresh owner must consume the durable terminal as PASSED, got %s (%s)", doc.Outcome, doc.Cause)
	}
	if doc.RawRunDir != rec.RawRunDir {
		t.Fatalf("PASSED must expose the raw run dir for evidence, got %q want %q", doc.RawRunDir, rec.RawRunDir)
	}
	// The superseded owner still cannot consume it: an identity disagreement HALTs
	// before the terminal is ever read, never surfacing the pass to the old owner.
	stale, err := d.Advance(id, owner)
	if err != nil {
		t.Fatalf("stale Advance: %v", err)
	}
	if stale.Outcome != HALTED {
		t.Fatalf("a superseded owner must not consume the terminal, got %s", stale.Outcome)
	}
}

// TestScopedWaitingHandoffClaimClosesScope proves the WAITING → handoff → claim
// path mid-sequence (spec verification 7): after a completed predecessor, the
// current WAITING successor is handed off and cooperatively claimed; Claim closes
// the child's scope (later worker dispatches get fresh scopes), so a further
// successor start is refused ErrScopeTransferred (change 0459) and the child's original owner
// generation is dead.
func TestScopedWaitingHandoffClaimClosesScope(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if strings.HasSuffix(runDir, "run1") {
				return obs(process.StatePassed, runDir), nil
			}
			return obs(process.StateRunning, runDir), nil
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability

	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if first.Outcome != PASSED {
		t.Fatalf("first drive must PASS, got %s (%s)", first.Outcome, first.Cause)
	}

	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	second, err := d.Start(succ)
	if err != nil {
		t.Fatalf("successor Start: %v", err)
	}
	if second.Outcome != WAITING {
		t.Fatalf("successor drive must WAIT, got %s (%s)", second.Outcome, second.Cause)
	}

	// WAITING → handoff → claim.
	handoff, err := d.Handoff(second.DriveID, second.Generation)
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	claimed, err := d.Claim(second.DriveID, handoff.Generation)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if claimed.Generation == "" || claimed.Generation == second.Generation {
		t.Fatalf("Claim must mint a fresh owner generation, got %q", claimed.Generation)
	}

	// A cooperative claim closes the child's scope.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if !scope.Closed {
		t.Fatalf("a cooperative claim must close the scope")
	}

	// A further successor start under the now-closed scope is refused, launching nothing.
	further := req
	further.PredecessorDriveID = second.DriveID
	further.PredecessorOwnerGen = claimed.Generation
	launchesBefore := proc.launchN
	if _, err := d.Start(further); !isOwnershipKind(err, ErrScopeTransferred) {
		t.Fatalf("a successor start under a claimed (transferred) scope must fail ErrScopeTransferred, got %v", err)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a rejected successor start must not launch, launched %d->%d", launchesBefore, proc.launchN)
	}

	// The child's original owner generation is dead (superseded by the handoff/claim).
	stale, err := d.Advance(second.DriveID, second.Generation)
	if err != nil {
		t.Fatalf("stale Advance: %v", err)
	}
	if stale.Outcome != HALTED {
		t.Fatalf("the child's original owner must be dead after handoff/claim, got %s", stale.Outcome)
	}
}

// TestClaimedScopeAcknowledgeAndStartAreTransferred is the change-0459
// regression: a worker hands off its WAITING drive, the parent claims it and
// advances it to PASSED, and the returning worker's child-capability operations
// on the original scope — acknowledge, and a scoped start — are refused with the
// distinct ErrScopeTransferred (never the finished-scope ErrScopeClosed), with
// nothing written and nothing launched. Observed on change 0458 Task 2, where
// the old ErrScopeClosed refusal steered a finished worker into a false BLOCKED.
func TestClaimedScopeAcknowledgeAndStartAreTransferred(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	running := true
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if running {
				return obs(process.StateRunning, runDir), nil
			}
			return obs(process.StatePassed, runDir), nil
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())
	req := sampleStart()
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
		t.Fatalf("drive must WAIT, got %s (%s)", started.Outcome, started.Cause)
	}

	// Worker hands off; parent claims and advances to the terminal PASSED.
	handoff, err := d.Handoff(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	claimed, err := d.Claim(started.DriveID, handoff.Generation)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	running = false
	final, err := d.Advance(started.DriveID, claimed.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if final.Outcome != PASSED {
		t.Fatalf("parent-driven drive must PASS, got %s (%s)", final.Outcome, final.Cause)
	}

	// The returning worker's acknowledge on its original scope: transferred, no write.
	scopeBytes := readScopeBytes(t, store, grant.ScopeID)
	driveBytes := readDriveBytes(t, store, started.DriveID)
	if _, err := d.Acknowledge(grant.ScopeID, grant.ChildCapability, started.DriveID, started.Generation); !isOwnershipKind(err, ErrScopeTransferred) {
		t.Fatalf("acknowledge after a parent claim must be ErrScopeTransferred, got %v", err)
	}
	assertUnchanged(t, store, grant.ScopeID, scopeBytes, started.DriveID, driveBytes)

	// A scoped start under the same scope: transferred, nothing launched.
	further := req
	further.PredecessorDriveID = started.DriveID
	further.PredecessorOwnerGen = claimed.Generation
	launchesBefore := proc.launchN
	if _, err := d.Start(further); !isOwnershipKind(err, ErrScopeTransferred) {
		t.Fatalf("a scoped start under a claim-closed scope must be ErrScopeTransferred, got %v", err)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a transferred-scope start must not launch, launched %d->%d", launchesBefore, proc.launchN)
	}
}
