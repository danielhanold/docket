package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestClaimReceiptKeepsCommittedRunContextHashKey pins ADR-0129 Decision 3
// (change 0471): committed state keeps its spelling. Claim receipts on the docket
// branch carry gate_context_hash, and the claim idempotency digest hashes that key
// name, so the Go fields may be renamed but the JSON key may not.
func TestClaimReceiptKeepsCommittedRunContextHashKey(t *testing.T) {
	const raw = `{"gate_context_hash":"hash-1"}`
	var receipt changeClaimReceipt
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil || receipt.RunContextHash != "hash-1" {
		t.Fatalf("changeClaimReceipt decode = %q (err %v), want hash-1", receipt.RunContextHash, err)
	}
	var proof claimProofReceipt
	if err := json.Unmarshal([]byte(raw), &proof); err != nil || proof.RunContextHash != "hash-1" {
		t.Fatalf("claimProofReceipt decode = %q (err %v), want hash-1", proof.RunContextHash, err)
	}
	var digest claimDigestPayload
	if err := json.Unmarshal([]byte(raw), &digest); err != nil || digest.RunContextHash != "hash-1" {
		t.Fatalf("claimDigestPayload decode = %q (err %v), want hash-1", digest.RunContextHash, err)
	}
	for name, v := range map[string]any{
		"changeClaimReceipt": changeClaimReceipt{RunContextHash: "hash-1"},
		"claimDigestPayload": claimDigestPayload{RunContextHash: "hash-1"},
		"claimProofReceipt":  claimProofReceipt{RunContextHash: "hash-1"},
	} {
		b, err := json.Marshal(v)
		if err != nil || !strings.Contains(string(b), `"gate_context_hash":"hash-1"`) {
			t.Errorf("%s marshals %s (err %v), want the committed gate_context_hash key", name, b, err)
		}
	}
}

// TestClaimDigestStableAcrossRevisionRename pins ADR-0129 Decision 4 (change
// 0472): the claim idempotency digest hashes a `version` key, and its digests
// are committed as Docket-Request-Digest trailers on the metadata branch. The Go
// field may be renamed, but a fixed (id, record revision, run-context hash) must
// still hash to the value computed before the rename, or a lost-response claim
// retry straddling the upgrade re-allocates instead of replaying.
func TestClaimDigestStableAcrossRevisionRename(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	for _, c := range []struct{ name, hash, want string }{
		{"run-context", "sha256:run-context-hash", "sha256:da9fe54c15a964beee2ee8b4e241b5c8954bbf7b245b2c5b7b4ca3e3d110d143"},
		{"ungated", "", "sha256:c95b30b1a04b0de646ff621e7f24982df264a43c70ec7389dce8ae502ad4181a"},
	} {
		got, err := canonicalDigest(OperationChangeClaim, claimDigestPayload{ID: 472, Version: rev, RunContextHash: c.hash})
		if err != nil {
			t.Fatalf("%s: canonicalDigest: %v", c.name, err)
		}
		if string(got) != c.want {
			t.Errorf("%s: claim digest = %s, want the pre-rename %s (the payload's JSON keys must not move)", c.name, got, c.want)
		}
	}
	b, err := json.Marshal(claimDigestPayload{ID: 472, Version: rev})
	if err != nil || !strings.Contains(string(b), `"version":"`+rev+`"`) {
		t.Fatalf("claimDigestPayload marshals %s (err %v), want the committed \"version\" key", b, err)
	}
}
