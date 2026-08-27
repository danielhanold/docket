package app

import (
	"testing"
)

// These are the durable gate-record store tests (change 0334, Task 1). Each
// builds a real temporary git repository via git init (reusing the package's
// runGit/gitIdentity/requireRealGit fixture helpers) so the store's git
// common-dir rooting, cross-repo refusal, and linked-worktree resolution are
// exercised against real git, not a mock. The store is the generalization of
// scripts/lib/docket-dispatch-dir.sh's durable-dir conventions.

// newGateRepo initializes a temp git repo with a deterministic identity and one
// seed commit (a commit is required before `git worktree add` can attach a
// linked worktree). It returns the repo's working-tree path.
func newGateRepo(t *testing.T) string {
	t.Helper()
	requireRealGit(t)
	dir := t.TempDir()
	runGit(t, dir, "init")
	gitIdentity(t, dir)
	writeRepoFile(t, dir, "seed.txt", "seed\n")
	runGit(t, dir, "add", "seed.txt")
	runGit(t, dir, "commit", "-m", "seed")
	return dir
}

// sampleGateRecord is a fully-populated non-authoritative record (Schema and
// Repo are stamped by the store, so they are left zero here).
func sampleGateRecord() GateRecord {
	return GateRecord{
		Target:        "docket-implement-next",
		CreatedAt:     1700000000,
		DispatchEpoch: 1700000005,
		BeforeIDs:     []int{12, 34, 56},
		AttributedID:  0,
		Retry:         RetryUnused,
		Disposition:   "gate-armed",
		Terminal:      false,
	}
}

// repeat returns s repeated n times (a tiny local helper so the malformed-key
// case can build an over-long key without importing strings just for this).
func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
