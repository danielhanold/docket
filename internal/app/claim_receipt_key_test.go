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
