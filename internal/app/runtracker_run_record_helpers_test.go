package app

import (
	"testing"
)

// Run-epoch test helpers shared with default-build (untagged) test files. The
// run-epoch registry tests themselves live behind the integration tag in
// runtracker_run_record_integration_test.go (change 0465); these fixtures stay untagged
// because other untagged test files still reference them.

// mintTestRunKey mints a minimal valid gate record and returns its key, so an
// epoch test has a real key directory (the epoch store requires one) without
// arming the whole gate. AttemptLimit is floored at 1 so the v4 write guard
// accepts it.
func mintTestRunKey(t *testing.T, repo string) string {
	t.Helper()
	key, err := MintRunTrackerRecord(repo, RunTrackerRecord{
		Target:       runStartStoredTarget,
		AttemptLimit: 1,
		Retry:        RetryUnused,
		Disposition:  "run-started",
	})
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	return key
}

// forceRunState drives the epoch record to state s through the CAS, standing in
// for the durable transitions other tasks own so a lifecycle guard can be exercised
// against an arbitrary state.
func forceRunState(t *testing.T, repo, key string, s runState) {
	t.Helper()
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		r.State = s
		return nil
	}); err != nil {
		t.Fatalf("forceRunState %q: %v", s, err)
	}
}

// must fails the test immediately when err is non-nil, so a fixture setup step
// whose failure is not the assertion under test reads as one line.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
