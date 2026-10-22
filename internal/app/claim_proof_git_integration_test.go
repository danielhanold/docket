//go:build integration

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
)

// This is the real-git integration test for the ClaimProofScanner seam (change
// 0407, Task 2): a metadata-branch commit carrying the engine's change.claim
// trailer block is read back as one ClaimProof. It is modeled on the fixture
// style of claim_workflow_git_test.go (real temporary git repos + the package's
// runGit/gitIdentity helpers), but it asserts only the SCANNER half against
// hand-committed trailers so it stays independent of the transaction engine and
// of the receipt's gate_context_hash field, which the full claim path grows in a
// later task.

// wireClaimReceipt is the on-the-wire change.claim receipt shape the scanner
// reads: the canonical receipt fields plus gate_context_hash. It carries
// gate_context_hash explicitly so the test can commit a gated receipt without
// depending on changeClaimReceipt having grown that field yet.
type wireClaimReceipt struct {
	Branch         string `json:"branch"`
	RunContextHash string `json:"gate_context_hash"`
	ID             int    `json:"id"`
	Op             string `json:"op"`
	Selection      string `json:"selection,omitempty"`
	Status         string `json:"status"`
}

// encodeClaimReceipt base64url-encodes a receipt exactly as the engine's
// Docket-Result trailer does (base64.RawURLEncoding of the JSON receipt).
func encodeClaimReceipt(t *testing.T, r wireClaimReceipt) string {
	t.Helper()
	buf, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// commitClaimTrailers commits an empty commit carrying an engine-shaped trailer
// block and returns the new HEAD hash.
func commitClaimTrailers(t *testing.T, dir, subject string, trailers ...string) string {
	t.Helper()
	args := []string{"commit", "--allow-empty", "-m", subject}
	for _, tr := range trailers {
		args = append(args, "--trailer", tr)
	}
	runGit(t, dir, args...)
	return runGit(t, dir, "rev-parse", "HEAD")
}

// TestIntegrationRecordOpsScanClaimProofsReadsCommittedReceipt: a metadata-branch commit carrying
// the engine's trailer block for a change.claim applied receipt is returned as
// one ClaimProof, newest-first, decoding gate_context_hash from the receipt.
func TestIntegrationRecordOpsScanClaimProofsReadsCommittedReceipt(t *testing.T) {
	repo := newRunTrackerRepo(t)

	// Older claim commit: change id 3, ungated (gate_context_hash ""), committed
	// by an older docket whose receipt still carried the retired claimed_at and
	// lease keys. The scanner must still read it as a proof.
	older := commitClaimTrailers(t, repo, "claim 3",
		"Docket-Transaction-ID: t1",
		"Docket-Operation: "+OperationChangeClaim,
		"Docket-Request-ID: claim-3-v1",
		"Docket-Request-Digest: sha256:aaaa",
		"Docket-Result: "+base64.RawURLEncoding.EncodeToString([]byte(
			`{"branch":"fix/x","claimed_at":"2026-09-07T00:00:00Z","gate_context_hash":"","id":3,"lease":"fresh","op":"change.claim","status":"in-progress"}`)))

	// A non-claim commit that carries a request id but a different operation: it
	// is returned by the trailer scan yet must be filtered out by the op check.
	commitClaimTrailers(t, repo, "groom something",
		"Docket-Transaction-ID: t2",
		"Docket-Operation: change.groom",
		"Docket-Request-ID: groom-1",
		"Docket-Request-Digest: sha256:bbbb",
		"Docket-Result: "+base64.RawURLEncoding.EncodeToString([]byte(`{"op":"change.groom"}`)))

	// Newer claim commit: change id 4, gated (gate_context_hash "abc123"). This is
	// HEAD, so its hash is the metadata tip.
	tip := commitClaimTrailers(t, repo, "claim 4",
		"Docket-Transaction-ID: t3",
		"Docket-Operation: "+OperationChangeClaim,
		"Docket-Request-ID: claim-4-v2",
		"Docket-Request-Digest: sha256:cccc",
		"Docket-Result: "+encodeClaimReceipt(t, wireClaimReceipt{
			Branch: "fix/y", RunContextHash: "abc123",
			ID: 4, Op: OperationChangeClaim, Status: "in-progress",
		}))

	deps := PlanningDeps{
		Client: newGitClient(t),
		Reader: &fakeChangeReader{pin: StatusPin{MetadataRevision: tip}},
		Clock:  testClock(),
	}
	scanner := NewClaimProofScanner(deps)

	proofs, err := scanner.ScanClaimProofs(context.Background(), repo)
	if err != nil {
		t.Fatalf("ScanClaimProofs: %v", err)
	}

	want := []ClaimProof{
		{RequestID: "claim-4-v2", ChangeID: 4, RunContextHash: "abc123", Revision: tip},
		{RequestID: "claim-3-v1", ChangeID: 3, RunContextHash: "", Revision: older},
	}
	if len(proofs) != len(want) {
		t.Fatalf("got %d proofs, want %d:\n%+v", len(proofs), len(want), proofs)
	}
	for i, w := range want {
		if proofs[i] != w {
			t.Errorf("proofs[%d] = %+v, want %+v", i, proofs[i], w)
		}
	}
	for _, p := range proofs {
		if p.RequestID == "groom-1" || p.ChangeID == 0 {
			t.Errorf("non-claim commit leaked into proofs: %+v", p)
		}
	}
}

// TestIntegrationRecordOpsScanClaimProofsReadsSelection: the selection a claim
// recorded in its committed receipt is read back onto the proof, so the verdict's
// receipt-recovery path keeps how the change was chosen; a receipt without one
// (an older claim, or an unknown selection) reads back as "".
func TestIntegrationRecordOpsScanClaimProofsReadsSelection(t *testing.T) {
	repo := newRunTrackerRepo(t)

	commitClaimTrailers(t, repo, "claim 3",
		"Docket-Transaction-ID: t1",
		"Docket-Operation: "+OperationChangeClaim,
		"Docket-Request-ID: claim-3-v1",
		"Docket-Request-Digest: sha256:aaaa",
		"Docket-Result: "+encodeClaimReceipt(t, wireClaimReceipt{
			Branch: "fix/x", RunContextHash: "h3",
			ID: 3, Op: OperationChangeClaim, Status: "in-progress",
		}))
	tip := commitClaimTrailers(t, repo, "claim 4",
		"Docket-Transaction-ID: t2",
		"Docket-Operation: "+OperationChangeClaim,
		"Docket-Request-ID: claim-4-v2",
		"Docket-Request-Digest: sha256:bbbb",
		"Docket-Result: "+encodeClaimReceipt(t, wireClaimReceipt{
			Branch: "fix/y", RunContextHash: "h4",
			ID: 4, Op: OperationChangeClaim, Selection: ClaimSelectionQueue, Status: "in-progress",
		}))

	deps := PlanningDeps{
		Client: newGitClient(t),
		Reader: &fakeChangeReader{pin: StatusPin{MetadataRevision: tip}},
		Clock:  testClock(),
	}
	proofs, err := NewClaimProofScanner(deps).ScanClaimProofs(context.Background(), repo)
	if err != nil {
		t.Fatalf("ScanClaimProofs: %v", err)
	}
	if len(proofs) != 2 {
		t.Fatalf("got %d proofs, want 2:\n%+v", len(proofs), proofs)
	}
	if proofs[0].ChangeID != 4 || proofs[0].Selection != ClaimSelectionQueue {
		t.Errorf("newest proof = %+v, want change 4 with selection queue", proofs[0])
	}
	if proofs[1].ChangeID != 3 || proofs[1].Selection != "" {
		t.Errorf("older proof = %+v, want change 3 with no selection", proofs[1])
	}
}
