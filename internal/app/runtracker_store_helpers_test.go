package app

import (
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// Run-tracker store test helpers shared with default-build (untagged) test files.
// The run-tracker record store tests themselves live behind the integration tag in
// runtracker_store_integration_test.go (change 0465); these fixtures stay untagged
// because other untagged test files still reference them.

// newRunTrackerRepo initializes a temp git repo with a deterministic identity and one
// seed commit (a commit is required before `git worktree add` can attach a
// linked worktree). It returns the repo's working-tree path.
func newRunTrackerRepo(t *testing.T) string {
	t.Helper()
	requireRealGit(t)
	dir := testsupport.TempDir(t)
	runGit(t, dir, "init")
	gitIdentity(t, dir)
	writeRepoFile(t, dir, "seed.txt", "seed\n")
	runGit(t, dir, "add", "seed.txt")
	runGit(t, dir, "commit", "-m", "seed")
	return dir
}

// sampleRunTrackerRecord is a fully-populated non-authoritative record (Schema and
// Repo are stamped by the store, so they are left zero here). AttemptLimit is
// stamped to 2 — the historical single-retry default — so fixtures minted from
// this record preserve their pre-0421 one-retry semantics (change 0421).
func sampleRunTrackerRecord() RunTrackerRecord {
	return RunTrackerRecord{
		Target:       "docket-implement-next",
		CreatedAt:    1700000000,
		DispatchedAt: 1700000005,
		BeforeIDs:    []int{12, 34, 56},
		AttributedID: 0,
		Retry:        RetryUnused,
		Disposition:  "run-started",
		Terminal:     false,
		AttemptLimit: 2,
	}
}

// mintRunTrackerWithHash mints a record carrying ChildContextHash, optionally terminal.
func mintRunTrackerWithHash(t *testing.T, repoDir, hash string, terminal bool) string {
	t.Helper()
	key, err := MintRunTrackerRecord(repoDir, RunTrackerRecord{Target: "docket-implement-next", Retry: RetryUnused, ChildContextHash: hash, Terminal: terminal, AttemptLimit: 2})
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	return key
}
