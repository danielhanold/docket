package app

import (
	"os"
	"path/filepath"
	"testing"
)

// Run-tracker fence test helpers shared with default-build (untagged) test files.
// The fence tests themselves live behind the integration tag in
// runtracker_fence_integration_test.go (change 0465); these run seeders stay
// untagged because other untagged test files still reference them.

// seedPendingRunMutation journals one admitted-not-completed workflow mutation on
// the run at key: a genuinely owned in-flight effect, which keeps the successful-run
// closeout (change 0441) fail-closed with mutation-pending. The two verdict recovery
// tests in runtracker_fence_integration_test.go use it to hold their run at
// completing so a later explicit cancellation is meaningful: an absent feature
// directory is not an obligation, an owned pending mutation is.
func seedPendingRunMutation(t *testing.T, repo, key string) {
	t.Helper()
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations, AdmittedMutation{
			OpKey:  OperationPRPublish,
			Status: mutationStatusAdmitted,
		})
		return nil
	}); err != nil {
		t.Fatalf("journal a pending mutation: %v", err)
	}
}

// reconcilePendingRunMutations marks every journaled mutation on the run at key
// completed — the in-flight effect resolved — so an explicit cancellation can account
// it and reach cancelled.
func reconcilePendingRunMutations(t *testing.T, repo, key string) {
	t.Helper()
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		for i := range r.AdmittedMutations {
			r.AdmittedMutations[i].Status = mutationStatusCompleted
		}
		return nil
	}); err != nil {
		t.Fatalf("reconcile pending mutations: %v", err)
	}
}

// seedNamedRun writes a run record for state bound to worktree under a run-key
// directory whose NAME the test chooses, so the directory order os.ReadDir yields is
// controlled (a first-match selector would pick the lexically first key). It writes
// the record through the store's own atomic writer and needs no gate record.
func seedNamedRun(t *testing.T, repo, key, worktree string, state runState) RunRecord {
	t.Helper()
	common, err := runTrackerGitCommonDir(repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir run-key dir: %v", err)
	}
	id, err := runToken()
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}
	gen, err := runToken()
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}
	rec := RunRecord{
		SchemaVersion: runSchemaVersion,
		RunKey:        key,
		ChangeID:      "7",
		State:         state,
		RunID:         id,
		Worktree:      worktree,
	}
	if err := writeRunAtomic(dir, storedRun{Generation: gen, Record: rec}); err != nil {
		t.Fatalf("writeRunAtomic: %v", err)
	}
	return rec
}
