package codexcontract

import (
	"strings"
	"testing"
)

func TestAcceptanceReceiptKeepsTransientWriteAuditIncompleteAndRequiresFinalGate(t *testing.T) {
	commit := strings.Repeat("a", 40)
	r := AcceptanceReceipt{SchemaVersion: 1, SourceCommit: commit, BinaryCommit: commit, BinarySHA256: strings.Repeat("b", 64), DefinitionSHA256: map[string]string{"agent": strings.Repeat("c", 64)}, ResourceSHA256: map[string]string{"skill": strings.Repeat("d", 64)}, ConfiguredRoles: []string{"docket-build-standard"}, ObservedRoles: []string{"docket-build-standard"}, RuntimeRoots: map[string]string{"primary": "/repo", "feature": "/repo/feature", "common_dir": "/repo/.git"}, NativeLineage: []string{"parent", "planner", "worker", "reviewer"}, RoleSequence: []string{"docket-implement-next", "docket-plan-writer", "docket-build-standard", "docket-review-standard"}, PlanCommit: commit, TaskCommits: []string{commit}, ResultsCommit: commit, ScopeSequence: []string{"prepare", "start", "acknowledge"}, ScopeAcknowledged: true, PublicationRevision: commit, AttachmentRevision: commit, ReviewHEAD: commit, ImplementationGate: "passed", TerminalVerdict: "gate-stop", EvidenceAuditComplete: false}
	if ValidateAcceptanceReceipt(r) == nil {
		t.Fatal("accepted results without final-head gate")
	}
	r.FinalGate = "passed"
	r.FinalGateCommit = commit
	if err := ValidateAcceptanceReceipt(r); err != nil {
		t.Fatalf("valid deterministic receipt: %v", err)
	}
	r.EvidenceAuditComplete = true
	if ValidateAcceptanceReceipt(r) == nil {
		t.Fatal("accepted hard-isolation claim from final snapshots")
	}
}

func TestAcceptanceReceiptValidatesTypedEvidenceAndFinalHead(t *testing.T) {
	commit := strings.Repeat("a", 40)
	valid := AcceptanceReceipt{SchemaVersion: 1, SourceCommit: commit, BinaryCommit: commit, BinarySHA256: strings.Repeat("b", 64), DefinitionSHA256: map[string]string{"agent": strings.Repeat("c", 64)}, ResourceSHA256: map[string]string{"skill": strings.Repeat("d", 64)}, ConfiguredRoles: []string{"docket-build-standard"}, ObservedRoles: []string{"docket-build-standard"}, RuntimeRoots: map[string]string{"primary": "/repo", "feature": "/repo/feature", "common_dir": "/repo/.git"}, NativeLineage: []string{"parent", "planner", "worker", "reviewer"}, RoleSequence: []string{"docket-implement-next", "docket-plan-writer", "docket-build-standard", "docket-review-standard"}, PlanCommit: commit, TaskCommits: []string{commit}, ResultsCommit: commit, ScopeSequence: []string{"prepare", "start", "acknowledge"}, ScopeAcknowledged: true, PublicationRevision: commit, AttachmentRevision: commit, ReviewHEAD: commit, ImplementationGate: "passed", FinalGate: "passed", FinalGateCommit: commit, TerminalVerdict: "gate-stop"}
	if err := ValidateAcceptanceReceipt(valid); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"nonhex-binary-hash", "nonhex-definition-hash", "incomplete-role-sequence", "unknown-gate", "wrong-final-head", "unpublished-results"} {
		t.Run(scenario, func(t *testing.T) {
			r := valid
			r.DefinitionSHA256 = map[string]string{"agent": strings.Repeat("c", 64)}
			switch scenario {
			case "nonhex-binary-hash":
				r.BinarySHA256 = strings.Repeat("z", 64)
			case "nonhex-definition-hash":
				r.DefinitionSHA256["agent"] = strings.Repeat("z", 64)
			case "incomplete-role-sequence":
				r.RoleSequence = r.RoleSequence[:2]
			case "unknown-gate":
				r.FinalGate = "looks-good"
			case "wrong-final-head":
				r.FinalGateCommit = strings.Repeat("e", 40)
			case "unpublished-results":
				r.PublicationRevision = strings.Repeat("e", 40)
			}
			if ValidateAcceptanceReceipt(r) == nil {
				t.Fatal("accepted incomplete or untyped acceptance evidence")
			}
		})
	}
}
