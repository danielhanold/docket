package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestClaimReceiptKeepsCommittedGateContextHashKey pins ADR-0129 Decision 3
// (change 0471): committed state keeps its spelling. Claim receipts on the docket
// branch carry gate_context_hash, and the claim idempotency digest hashes that key
// name, so the Go fields may be renamed but the JSON key may not.
func TestClaimReceiptKeepsCommittedGateContextHashKey(t *testing.T) {
	const raw = `{"gate_context_hash":"hash-1"}`
	var receipt changeClaimReceipt
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil || receipt.GateContextHash != "hash-1" {
		t.Fatalf("changeClaimReceipt decode = %q (err %v), want hash-1", receipt.GateContextHash, err)
	}
	var proof claimProofReceipt
	if err := json.Unmarshal([]byte(raw), &proof); err != nil || proof.GateContextHash != "hash-1" {
		t.Fatalf("claimProofReceipt decode = %q (err %v), want hash-1", proof.GateContextHash, err)
	}
	var digest claimDigestPayload
	if err := json.Unmarshal([]byte(raw), &digest); err != nil || digest.GateContextHash != "hash-1" {
		t.Fatalf("claimDigestPayload decode = %q (err %v), want hash-1", digest.GateContextHash, err)
	}
	for name, v := range map[string]any{
		"changeClaimReceipt": changeClaimReceipt{GateContextHash: "hash-1"},
		"claimDigestPayload": claimDigestPayload{GateContextHash: "hash-1"},
		"claimProofReceipt":  claimProofReceipt{GateContextHash: "hash-1"},
	} {
		b, err := json.Marshal(v)
		if err != nil || !strings.Contains(string(b), `"gate_context_hash":"hash-1"`) {
			t.Errorf("%s marshals %s (err %v), want the committed gate_context_hash key", name, b, err)
		}
	}
}
