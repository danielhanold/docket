package codexcontract

import "testing"

func TestAcceptanceReceiptKeepsTransientWriteAuditIncompleteAndRequiresFinalGate(t *testing.T) {
	r := AcceptanceReceipt{SchemaVersion: 1, SourceCommit: "abc", BinaryCommit: "abc", BinarySHA256: "hash", ResultsCommit: "results", EvidenceAuditComplete: false}
	if ValidateAcceptanceReceipt(r) == nil {
		t.Fatal("accepted results without final-head gate")
	}
	r.FinalGate = "passed"
	if err := ValidateAcceptanceReceipt(r); err != nil {
		t.Fatalf("valid deterministic receipt: %v", err)
	}
	r.EvidenceAuditComplete = true
	if ValidateAcceptanceReceipt(r) == nil {
		t.Fatal("accepted hard-isolation claim from final snapshots")
	}
}
