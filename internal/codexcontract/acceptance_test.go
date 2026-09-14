package codexcontract

import (
	"strings"
	"testing"
)

func TestAcceptanceReceiptKeepsTransientWriteAuditIncompleteAndRequiresFinalGate(t *testing.T) {
	commit := strings.Repeat("a", 40)
	r := AcceptanceReceipt{SchemaVersion: 1, SourceCommit: commit, BinaryCommit: commit, BinarySHA256: strings.Repeat("b", 64), DefinitionSHA256: map[string]string{"agent": strings.Repeat("c", 64)}, ResourceSHA256: map[string]string{"skill": strings.Repeat("d", 64)}, ConfiguredRoles: []string{"docket-build-standard"}, ObservedRoles: []string{"docket-build-standard"}, RuntimeRoots: map[string]string{"primary": "/repo", "feature": "/repo/feature", "common_dir": "/repo/.git"}, NativeLineage: []string{"parent", "child"}, PlanCommit: commit, TaskCommits: []string{commit}, ResultsCommit: commit, ScopeSequence: []string{"prepare", "start", "acknowledge"}, ScopeAcknowledged: true, PublicationRevision: commit, AttachmentRevision: commit, ReviewHEAD: commit, ImplementationGate: "passed", TerminalVerdict: "gate-stop", EvidenceAuditComplete: false}
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
