//go:build integration

package gatedrive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationGatedriveRepoHandoffPerDimensionDriftRejectsClaim proves the core Task-8 property:
// a clean drive is handed off, then a single real git-level mutation in ANY
// fingerprint dimension makes the fresh claim reject, and — because a claim that
// no longer matches acquires no partial authority — the single-use receipt
// survives intact for a correct claimant. Each subtest mutates exactly one
// dimension so the resulting rejection isolates that dimension.
func TestIntegrationGatedriveRepoHandoffPerDimensionDriftRejectsClaim(t *testing.T) {
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

// TestIntegrationGatedriveDirtyHandoffIdenticalStateClaimsWithoutWIPCommit proves that dirty
// pre-commit task work is a supported handoff state: a drive started over a
// genuinely dirty worktree hands off and a fresh claimant whose worktree is
// byte-for-byte identical claims it — and NO WIP commit is created to move the
// ownership (HEAD and the dirty status are unchanged across the whole transfer).
func TestIntegrationGatedriveDirtyHandoffIdenticalStateClaimsWithoutWIPCommit(t *testing.T) {
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
