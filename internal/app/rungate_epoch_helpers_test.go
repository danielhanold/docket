package app

import (
	"testing"
)

// Run-epoch test helpers shared with default-build (untagged) test files. The
// run-epoch registry tests themselves live behind the integration tag in
// rungate_epoch_integration_test.go (change 0465); these fixtures stay untagged
// because other untagged test files still reference them.

// mintTestGateKey mints a minimal valid gate record and returns its key, so an
// epoch test has a real key directory (the epoch store requires one) without
// arming the whole gate. AttemptLimit is floored at 1 so the v4 write guard
// accepts it.
func mintTestGateKey(t *testing.T, repo string) string {
	t.Helper()
	key, err := MintGateRecord(repo, GateRecord{
		Target:       gateBeforeStoredTarget,
		AttemptLimit: 1,
		Retry:        RetryUnused,
		Disposition:  "run-started",
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// forceEpochState drives the epoch record to state s through the CAS, standing in
// for the durable transitions other tasks own so a lifecycle guard can be exercised
// against an arbitrary state.
func forceEpochState(t *testing.T, repo, key string, s epochState) {
	t.Helper()
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.State = s
		return nil
	}); err != nil {
		t.Fatalf("forceEpochState %q: %v", s, err)
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
