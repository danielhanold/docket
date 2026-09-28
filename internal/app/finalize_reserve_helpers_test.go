package app

// Change 0465: liveStoppedCommit stays in the default build — the untagged
// finalize_rebase_test.go still uses it — while the real-git resolver-reserve tests
// moved behind the integration tag (finalize_reserve_integration_test.go).

import (
	"context"
	"testing"
)

// liveStoppedCommit is the full object id the live conflicted rebase is stopped on.
func liveStoppedCommit(t *testing.T, f *rebaseFixture) string {
	t.Helper()
	oid, err := f.deps.Client.StoppedRebaseCommit(context.Background(), f.wp)
	if err != nil {
		t.Fatalf("probe live stopped commit: %v", err)
	}
	return string(oid)
}
