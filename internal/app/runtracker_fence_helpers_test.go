package app

import (
	"os"
	"path/filepath"
	"testing"
)

// Run-gate fence test helpers shared with default-build (untagged) test files.
// The fence tests themselves live behind the integration tag in
// runtracker_fence_integration_test.go (change 0465); these epoch seeders stay
// untagged because other untagged test files still reference them.

// seedPendingEpochMutation journals one admitted-not-completed workflow mutation on
// the epoch at key: a genuinely owned in-flight effect, which keeps the successful-run
// closeout (change 0441) fail-closed with mutation-pending. The two verdict recovery
// tests in runtracker_fence_integration_test.go use it to hold their epoch at
// completing so a later explicit cancellation is meaningful. They formerly relied on the ABSENT feature directory
// making the slot unreadable; change 0446 (spec §2) addresses a slot through its
// stored identity, so a never-reserved slot now reads as truly absent (safely
// detached) and the closeout would legitimately complete — an absent directory is not
// an obligation, an owned pending mutation is.
func seedPendingEpochMutation(t *testing.T, repo, key string) {
	t.Helper()
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations, AdmittedMutation{
			OpKey:  OperationPRPublish,
			Status: mutationStatusAdmitted,
		})
		return nil
	}); err != nil {
		t.Fatalf("journal a pending mutation: %v", err)
	}
}

// reconcilePendingEpochMutations marks every journaled mutation on the epoch at key
// completed — the in-flight effect resolved — so an explicit cancellation can account
// it and reach cancelled.
func reconcilePendingEpochMutations(t *testing.T, repo, key string) {
	t.Helper()
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		for i := range r.AdmittedMutations {
			r.AdmittedMutations[i].Status = mutationStatusCompleted
		}
		return nil
	}); err != nil {
		t.Fatalf("reconcile pending mutations: %v", err)
	}
}

// seedNamedEpoch writes an epoch record for state bound to worktree under a gate-key
// directory whose NAME the test chooses, so the directory order os.ReadDir yields is
// controlled (a first-match selector would pick the lexically first key). It writes
// the record through the store's own atomic writer and needs no gate record.
func seedNamedEpoch(t *testing.T, repo, key, worktree string, state epochState) EpochRecord {
	t.Helper()
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir gate-key dir: %v", err)
	}
	id, err := epochToken()
	if err != nil {
		t.Fatalf("epochToken: %v", err)
	}
	gen, err := epochToken()
	if err != nil {
		t.Fatalf("epochToken: %v", err)
	}
	rec := EpochRecord{
		SchemaVersion: epochSchemaVersion,
		GateKey:       key,
		ChangeID:      "7",
		State:         state,
		EpochID:       id,
		Worktree:      worktree,
	}
	if err := writeEpochAtomic(dir, storedEpoch{Generation: gen, Record: rec}); err != nil {
		t.Fatalf("writeEpochAtomic: %v", err)
	}
	return rec
}
