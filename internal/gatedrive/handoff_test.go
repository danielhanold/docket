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
	"testing"

	"github.com/danielhanold/docket/internal/process"
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
