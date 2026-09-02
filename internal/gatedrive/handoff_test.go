// Handoff and repository tests (spec "Verification strategy → Handoff and
// repository tests"): the REAL repository execution-identity dimensions bound to
// the ownership handoff/claim CAS, plus the driver-level consequences of a
// handoff.
//
// ownership_test.go (Task 5) proves the handoff/claim CAS logic against SYNTHETIC
// fingerprints (matchingFP/mismatchFP mutate one struct field). This file proves
// the binding those synthetic tests take on faith: that a genuine git-level
// mutation in each dimension — staged bytes, unstaged bytes, an untracked file, a
// rename, a deletion, an executable-mode flip, a symlink retarget — produces a
// ComputeFingerprint value the claim path rejects, one dimension at a time. It
// also proves the two properties spec "Explicit handoff and nearest-owner
// continuation" names for dirty task work: identical dirty state claims WITHOUT a
// WIP commit, and a fingerprint that drifted between drive-start and claim never
// grants partial authority.
//
// The single-winner race and the plain-WAITING rejection are already proven by
// ownership_test.go (TestRaceOneReceiptSingleWinner,
// TestPlainWaitingDriveCannotBeClaimed) and are not duplicated here. This file
// adds the driver-level chain those primitives feed: an old owner cannot advance
// after a handoff, and a fresh owner consumes a terminal the raw run wrote while
// no agent was advancing.
package gatedrive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// newRepoDrive persists a drive whose drive-start execution identity is the REAL
// fingerprint of repo (computed through realGit), so a handoff/claim over it
// exercises the actual repository dimensions rather than a synthetic struct. It
// returns the store, drive id, owner generation, and the drive-start fingerprint.
func newRepoDrive(t *testing.T, repo string) (s *Store, id, owner string, startFP Fingerprint) {
	t.Helper()
	startFP = fingerprint(t, repo)
	rec := sampleRecord()
	rec.RepoIdentity = repo
	rec.WorktreePath = repo
	rec.HeadOID = startFP.Head
	rec.Fingerprint = startFP
	owner = rec.OwnerGeneration
	s = OpenStore(testsupport.TempDir(t))
	var err error
	id, _, err = s.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	return s, id, owner, startFP
}

// TestRepoHandoffPerDimensionDriftRejectsClaim proves the core Task-8 property:
// a clean drive is handed off, then a single real git-level mutation in ANY
// fingerprint dimension makes the fresh claim reject, and — because a claim that
// no longer matches acquires no partial authority — the single-use receipt
// survives intact for a correct claimant. Each subtest mutates exactly one
// dimension so the resulting rejection isolates that dimension.
func TestRepoHandoffPerDimensionDriftRejectsClaim(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, repo string)
	}{
		{"staged bytes", func(t *testing.T, repo string) {
			writeFile(t, repo, "x.sh", "echo staged-drift\n")
			gitAdd(t, repo, "x.sh")
		}},
		{"unstaged bytes", func(t *testing.T, repo string) {
			writeFile(t, repo, "x.sh", "echo unstaged-drift\n") // not staged
		}},
		{"untracked file", func(t *testing.T, repo string) {
			writeFile(t, repo, "loose.txt", "brand new\n")
		}},
		{"rename", func(t *testing.T, repo string) {
			git(t, repo, "mv", "keep.txt", "kept.txt")
		}},
		{"deletion", func(t *testing.T, repo string) {
			if err := os.Remove(filepath.Join(repo, "keep.txt")); err != nil {
				t.Fatalf("remove: %v", err)
			}
		}},
		{"exec mode", func(t *testing.T, repo string) {
			chmod(t, repo, "x.sh", 0o755)
		}},
		{"symlink value", func(t *testing.T, repo string) {
			symlink(t, repo, "keep.txt", "link") // was -> x.sh
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newDirtyRepo(t) // clean at return
			s, id, _, startFP := newRepoDrive(t, repo)

			// A clean handoff succeeds over the undrifted worktree.
			receipt, err := s.writeHandoffReceipt(id, sampleRecord().OwnerGeneration, startFP)
			if err != nil {
				t.Fatalf("clean handoff must succeed: %v", err)
			}

			// Drift exactly one dimension, then recompute the claim-time identity.
			tc.mutate(t, repo)
			driftFP := fingerprint(t, repo)
			if startFP.Equal(driftFP) {
				t.Fatalf("mutating %q must change the fingerprint (test is vacuous otherwise)", tc.name)
			}

			// The drifted claim rejects and acquires no generation.
			got, err := s.consumeHandoffCAS(id, receipt.HandoffGeneration, driftFP)
			if oe, ok := AsOwnershipError(err); !ok || oe.Kind != ErrFingerprintMismatch {
				t.Fatalf("a %q drift must reject the claim with ErrFingerprintMismatch, got %v", tc.name, err)
			}
			if got != "" {
				t.Fatalf("a rejected claim must acquire no generation, got %q", got)
			}

			// The receipt is untouched: a correct claimant could still consume it.
			rec, err := s.Load(id)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if rec.HandoffGeneration != receipt.HandoffGeneration {
				t.Fatalf("a rejected claim must preserve the receipt, got %q", rec.HandoffGeneration)
			}
			if rec.OwnerGeneration != "" {
				t.Fatalf("a rejected claim must install no owner, got %q", rec.OwnerGeneration)
			}
		})
	}
}

// TestDirtyHandoffIdenticalStateClaimsWithoutWIPCommit proves that dirty
// pre-commit task work is a supported handoff state: a drive started over a
// genuinely dirty worktree hands off and a fresh claimant whose worktree is
// byte-for-byte identical claims it — and NO WIP commit is created to move the
// ownership (HEAD and the dirty status are unchanged across the whole transfer).
func TestDirtyHandoffIdenticalStateClaimsWithoutWIPCommit(t *testing.T) {
	repo := newDirtyRepo(t)
	// Make it genuinely dirty across several dimensions before the drive starts:
	// a staged edit, an unstaged edit, an untracked file, a mode flip, and a
	// symlink retarget. This is the drive-start identity a handoff must preserve.
	writeFile(t, repo, "x.sh", "echo staged\n")
	gitAdd(t, repo, "x.sh")
	writeFile(t, repo, "keep.txt", "unstaged edit\n")
	writeFile(t, repo, "untracked.txt", "loose\n")
	chmod(t, repo, "x.sh", 0o755)
	symlink(t, repo, "keep.txt", "link")

	headBefore := headOID(t, repo)
	statusBefore := porcelain(t, repo)
	if statusBefore == "" {
		t.Fatalf("test setup is not dirty: git status is clean")
	}

	s, id, owner, startFP := newRepoDrive(t, repo)
	receipt, err := s.writeHandoffReceipt(id, owner, startFP)
	if err != nil {
		t.Fatalf("a dirty handoff must succeed: %v", err)
	}

	// The claimant recomputes identity over the UNCHANGED dirty worktree; it must
	// match exactly and consume the receipt.
	claimFP := fingerprint(t, repo)
	if !startFP.Equal(claimFP) {
		t.Fatalf("identical dirty state must fingerprint Equal at claim time")
	}
	newOwner, err := s.consumeHandoffCAS(id, receipt.HandoffGeneration, claimFP)
	if err != nil {
		t.Fatalf("an identical-dirty-state claim must succeed: %v", err)
	}
	if newOwner == "" || newOwner == owner || newOwner == receipt.HandoffGeneration {
		t.Fatalf("claim must mint a fresh owner generation distinct from the chain, got %q", newOwner)
	}

	// No WIP commit was created to move ownership, and nothing was staged away or
	// cleaned: HEAD and the dirty worktree are exactly as they were.
	if got := headOID(t, repo); got != headBefore {
		t.Fatalf("handoff must not create a WIP commit: HEAD moved %q -> %q", headBefore, got)
	}
	if got := porcelain(t, repo); got != statusBefore {
		t.Fatalf("handoff must not alter the dirty worktree:\n before: %q\n after:  %q", statusBefore, got)
	}
}

// TestOldOwnerCannotAdvanceAfterHandoff proves the driver-level consequence of a
// handoff: once the current owner has handed off (its generation invalidated),
// an Advance presenting that old generation is an identity disagreement that
// HALTs and drives nothing — it never silently continues the suite.
func TestOldOwnerCannotAdvanceAfterHandoff(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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

// headOID returns the repo's current HEAD object id (empty on an unborn branch),
// so a test can prove no WIP commit was created across a handoff.
func headOID(t *testing.T, repo string) string {
	t.Helper()
	oid, err := realGit{}.HeadOID(repo)
	if err != nil {
		t.Fatalf("HeadOID: %v", err)
	}
	return oid
}

// porcelain returns `git status --porcelain=v2` for repo, the stable textual
// witness that the dirty worktree is unchanged across a handoff.
func porcelain(t *testing.T, repo string) string {
	t.Helper()
	return git(t, repo, "status", "--porcelain=v2", "--untracked-files=all")
}
