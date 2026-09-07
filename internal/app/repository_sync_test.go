package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRepositorySyncResultShape proves the constructor stamps the operation
// envelope and that the SyncOutcome fields marshal under their protocol keys.
func TestRepositorySyncResultShape(t *testing.T) {
	r := newRepositorySyncResult(ResultApplied, RepositorySyncResult{SyncOutcome: SyncOutcome{
		Disposition: SyncDispAdvanced, IntegrationBranch: "main",
		PrimaryPath: "/repo", BeforeOID: "a", TargetOID: "b", AfterOID: "b",
	}})
	if r.Operation != OperationRepositorySyncIntegration || r.Result != ResultApplied {
		t.Fatalf("envelope = %s/%s", r.Operation, r.Result)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"disposition":"advanced"`, `"integration_branch":"main"`, `"before_oid":"a"`, `"target_oid":"b"`, `"after_oid":"b"`} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("marshal missing %s in %s", key, b)
		}
	}
}

// TestRepositorySyncDirtyMessageNamesUntrackedFiles pins the one dirty-skip
// message every path shares: it must name non-ignored untracked files as a
// blocker and tell the user to inspect and resolve the dirt.
func TestRepositorySyncDirtyMessageNamesUntrackedFiles(t *testing.T) {
	msg := syncDirtyMessage() // the one constructor every dirty-skip path uses
	for _, phrase := range []string{"untracked", "inspect"} {
		if !strings.Contains(msg, phrase) {
			t.Fatalf("dirty message %q must mention %q", msg, phrase)
		}
	}
}
